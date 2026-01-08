package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQConfig holds RabbitMQ configuration
type RabbitMQConfig struct {
	URL           string
	QueueName     string
	DLQName       string
	PrefetchCount int
}

// rabbitMQ implements Queue interface using RabbitMQ
type rabbitMQ struct {
	config     RabbitMQConfig
	conn       *amqp.Connection
	channel    *amqp.Channel
	mu         sync.RWMutex
	connected  bool
	closeCh    chan struct{}
	reconnectCh chan struct{}
}

// NewRabbitMQ creates a new RabbitMQ queue instance
func NewRabbitMQ(config RabbitMQConfig) Queue {
	if config.PrefetchCount == 0 {
		config.PrefetchCount = 1
	}
	return &rabbitMQ{
		config:      config,
		closeCh:     make(chan struct{}),
		reconnectCh: make(chan struct{}),
	}
}

// Connect establishes connection to RabbitMQ
func (r *rabbitMQ) Connect(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, err := amqp.Dial(r.config.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	r.conn = conn

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}
	r.channel = channel

	// Set prefetch count for fair dispatch
	if err := channel.Qos(r.config.PrefetchCount, 0, false); err != nil {
		channel.Close()
		conn.Close()
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Declare main queue
	if err := r.declareQueue(r.config.QueueName); err != nil {
		channel.Close()
		conn.Close()
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Declare dead letter queue
	if err := r.declareQueue(r.config.DLQName); err != nil {
		channel.Close()
		conn.Close()
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	r.connected = true

	// Start connection monitoring
	go r.monitorConnection()

	log.Printf("Connected to RabbitMQ, queue: %s", r.config.QueueName)
	return nil
}

// declareQueue declares a queue with standard settings
func (r *rabbitMQ) declareQueue(name string) error {
	_, err := r.channel.QueueDeclare(
		name,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		amqp.Table{
			"x-message-ttl": int32(86400000), // 24h TTL for messages
		},
	)
	return err
}

// monitorConnection monitors the connection and attempts reconnection
func (r *rabbitMQ) monitorConnection() {
	connClose := r.conn.NotifyClose(make(chan *amqp.Error))

	select {
	case err := <-connClose:
		if err != nil {
			log.Printf("RabbitMQ connection closed: %v", err)
			r.mu.Lock()
			r.connected = false
			r.mu.Unlock()

			// Attempt reconnection
			r.reconnect()
		}
	case <-r.closeCh:
		return
	}
}

// reconnect attempts to reconnect to RabbitMQ
func (r *rabbitMQ) reconnect() {
	for {
		select {
		case <-r.closeCh:
			return
		default:
		}

		log.Println("Attempting to reconnect to RabbitMQ...")
		time.Sleep(5 * time.Second)

		if err := r.Connect(context.Background()); err != nil {
			log.Printf("Reconnection failed: %v", err)
			continue
		}

		log.Println("Reconnected to RabbitMQ")
		close(r.reconnectCh)
		r.reconnectCh = make(chan struct{})
		return
	}
}

// Close closes the RabbitMQ connection
func (r *rabbitMQ) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	close(r.closeCh)
	r.connected = false

	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// Publish publishes a message to the queue
func (r *rabbitMQ) Publish(ctx context.Context, job *MessageJob) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.connected {
		return fmt.Errorf("not connected to RabbitMQ")
	}

	body, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	return r.channel.PublishWithContext(
		ctx,
		"",                  // exchange
		r.config.QueueName,  // routing key
		false,               // mandatory
		false,               // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
			MessageId:    job.ID,
		},
	)
}

// Consume starts consuming messages from the queue
func (r *rabbitMQ) Consume(ctx context.Context, handler func(job *MessageJob) error) error {
	r.mu.RLock()
	if !r.connected {
		r.mu.RUnlock()
		return fmt.Errorf("not connected to RabbitMQ")
	}
	r.mu.RUnlock()

	msgs, err := r.channel.Consume(
		r.config.QueueName,
		"",    // consumer tag
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.closeCh:
			return nil
		case msg, ok := <-msgs:
			if !ok {
				// Channel closed, wait for reconnection
				<-r.reconnectCh
				return r.Consume(ctx, handler)
			}

			job, err := FromJSON(msg.Body)
			if err != nil {
				log.Printf("Failed to parse message: %v", err)
				msg.Nack(false, false) // Don't requeue malformed messages
				continue
			}

			if err := handler(job); err != nil {
				log.Printf("Failed to process message %s: %v", job.ID, err)

				// Increment attempts and check if should retry
				job.Attempts++
				if job.Attempts < job.MaxAttempts {
					// Requeue for retry
					msg.Nack(false, true)
				} else {
					// Move to dead letter queue
					if dlqErr := r.PublishToDeadLetter(ctx, job, err.Error()); dlqErr != nil {
						log.Printf("Failed to publish to DLQ: %v", dlqErr)
					}
					msg.Ack(false)
				}
				continue
			}

			msg.Ack(false)
		}
	}
}

// PublishToDeadLetter moves a failed message to dead letter queue
func (r *rabbitMQ) PublishToDeadLetter(ctx context.Context, job *MessageJob, reason string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.connected {
		return fmt.Errorf("not connected to RabbitMQ")
	}

	dlJob := DeadLetterJob{
		MessageJob: *job,
		FailedAt:   time.Now(),
		Reason:     reason,
	}

	body, err := json.Marshal(dlJob)
	if err != nil {
		return fmt.Errorf("failed to serialize DLQ job: %w", err)
	}

	return r.channel.PublishWithContext(
		ctx,
		"",               // exchange
		r.config.DLQName, // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
			MessageId:    job.ID,
		},
	)
}

// IsConnected returns true if connected to RabbitMQ
func (r *rabbitMQ) IsConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.connected
}

// QueueLength returns the number of messages in the queue
func (r *rabbitMQ) QueueLength(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.connected {
		return 0, fmt.Errorf("not connected to RabbitMQ")
	}

	queue, err := r.channel.QueueInspect(r.config.QueueName)
	if err != nil {
		return 0, fmt.Errorf("failed to inspect queue: %w", err)
	}

	return queue.Messages, nil
}
