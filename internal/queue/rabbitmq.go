package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aseno/backend/internal/logger"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQConfig holds RabbitMQ configuration
type RabbitMQConfig struct {
	URL            string
	QueueName      string
	DLQName        string
	DelayQueueName string // Queue for delayed retries
	PrefetchCount  int
}

// rabbitMQ implements Queue interface using RabbitMQ
type rabbitMQ struct {
	config      RabbitMQConfig
	conn        *amqp.Connection
	channel     *amqp.Channel
	mu          sync.RWMutex
	connected   bool
	closeCh     chan struct{}
	reconnectCh chan struct{}
}

// NewRabbitMQ creates a new RabbitMQ queue instance
func NewRabbitMQ(config RabbitMQConfig) Queue {
	if config.PrefetchCount == 0 {
		config.PrefetchCount = 1
	}
	if config.DelayQueueName == "" {
		config.DelayQueueName = config.QueueName + "_delay"
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

	// Declare delay queue with DLX back to main queue
	if err := r.declareDelayQueue(); err != nil {
		channel.Close()
		conn.Close()
		return fmt.Errorf("failed to declare delay queue: %w", err)
	}

	r.connected = true

	// Start connection monitoring
	go r.monitorConnection()

	logger.Printf("Connected to RabbitMQ, queue: %s", r.config.QueueName)
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

// declareDelayQueue declares a delay queue that routes expired messages to main queue
func (r *rabbitMQ) declareDelayQueue() error {
	_, err := r.channel.QueueDeclare(
		r.config.DelayQueueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		amqp.Table{
			"x-dead-letter-exchange":    "",                 // Default exchange
			"x-dead-letter-routing-key": r.config.QueueName, // Route to main queue
			"x-message-ttl":             int32(600000),      // Max 10 min default TTL (per-message TTL takes precedence)
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
			logger.Printf("RabbitMQ connection closed: %v", err)
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

		logger.Println("Attempting to reconnect to RabbitMQ...")
		time.Sleep(5 * time.Second)

		if err := r.Connect(context.Background()); err != nil {
			logger.Printf("Reconnection failed: %v", err)
			continue
		}

		logger.Println("Reconnected to RabbitMQ")
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
		"",                 // exchange
		r.config.QueueName, // routing key
		false,              // mandatory
		false,              // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
			MessageId:    job.ID,
		},
	)
}

// PublishWithDelay publishes a message with a delay before processing
// The message is sent to a delay queue with per-message TTL, then routed back to main queue
func (r *rabbitMQ) PublishWithDelay(ctx context.Context, job *MessageJob, delay time.Duration) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.connected {
		return fmt.Errorf("not connected to RabbitMQ")
	}

	// Cap delay at 10 minutes (queue max TTL)
	if delay > 10*time.Minute {
		delay = 10 * time.Minute
	}

	body, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	// Per-message TTL in milliseconds
	ttlMs := fmt.Sprintf("%d", delay.Milliseconds())

	logger.Printf("[RabbitMQ] Publishing with delay: job_id=%s, delay=%v, ttl_ms=%s, queue=%s",
		job.ID, delay, ttlMs, r.config.DelayQueueName)

	return r.channel.PublishWithContext(
		ctx,
		"",                      // exchange
		r.config.DelayQueueName, // routing key (delay queue)
		false,                   // mandatory
		false,                   // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
			MessageId:    job.ID,
			Expiration:   ttlMs, // Per-message TTL
		},
	)
}

// Consume starts consuming messages from the queue
func (r *rabbitMQ) Consume(ctx context.Context, handler func(job *MessageJob) error) error {
	return r.ConsumeWithRetry(ctx, handler, nil)
}

// ConsumeWithRetry starts consuming messages with configurable retry strategy
// If retryStrategy is nil, uses default exponential backoff
func (r *rabbitMQ) ConsumeWithRetry(ctx context.Context, handler func(job *MessageJob) error, retryStrategy RetryStrategy) error {
	r.mu.RLock()
	if !r.connected {
		r.mu.RUnlock()
		return fmt.Errorf("not connected to RabbitMQ")
	}
	r.mu.RUnlock()

	// Use default strategy if none provided
	if retryStrategy == nil {
		retryStrategy = DefaultRetryStrategy()
	}

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
				return r.ConsumeWithRetry(ctx, handler, retryStrategy)
			}

			job, err := FromJSON(msg.Body)
			if err != nil {
				logger.Printf("Failed to parse message: %v", err)
				msg.Nack(false, false) // Don't requeue malformed messages
				continue
			}

			if err := handler(job); err != nil {
				r.handleFailedMessage(ctx, job, err, retryStrategy, &msg)
				continue
			}

			msg.Ack(false)
		}
	}
}

// handleFailedMessage handles a failed message with exponential backoff retry
func (r *rabbitMQ) handleFailedMessage(ctx context.Context, job *MessageJob, err error, strategy RetryStrategy, msg *amqp.Delivery) {
	// Calculate delay for next retry based on current attempt
	delay := strategy.NextDelay(job.Attempts)
	nextRetryAt := time.Now().Add(delay)

	// Update job with failure info
	job.MarkFailed(err.Error(), nextRetryAt)

	logger.Printf("Message %s failed (attempt %d/%d): %v. Next retry in %v",
		job.ID, job.Attempts, job.MaxAttempts, err, delay)

	// Check if we should retry
	if job.ShouldRetry() {
		// Publish to delay queue with exponential backoff
		if pubErr := r.PublishWithDelay(ctx, job, delay); pubErr != nil {
			logger.Printf("Failed to publish retry for %s: %v", job.ID, pubErr)
			// If we can't publish retry, move to DLQ
			if dlqErr := r.PublishToDeadLetter(ctx, job, fmt.Sprintf("retry publish failed: %v (original: %v)", pubErr, err)); dlqErr != nil {
				logger.Printf("Failed to publish to DLQ: %v", dlqErr)
			}
		}
	} else {
		// Max attempts reached, move to dead letter queue
		logger.Printf("Message %s exceeded max attempts (%d), moving to DLQ", job.ID, job.MaxAttempts)
		if dlqErr := r.PublishToDeadLetter(ctx, job, err.Error()); dlqErr != nil {
			logger.Printf("Failed to publish to DLQ: %v", dlqErr)
		}
	}

	// Acknowledge the original message (we've handled it by republishing or DLQ)
	msg.Ack(false)
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
