package worker

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/aseno/backend/internal/config"
	"github.com/aseno/backend/internal/queue"
	whatsapp_service "github.com/aseno/backend/pkg/whatsapp/service"
)

// MessageSenderWorker consumes messages from the queue and sends them via WhatsApp
type MessageSenderWorker struct {
	config          *config.WorkerConfig
	queue           queue.Queue
	whatsappService whatsapp_service.WhatsAppService
	stopCh          chan struct{}

	// Stats
	messagesSent    int64
	messagesFailed  int64
	messagesRetried int64
}

// NewMessageSenderWorker creates a new message sender worker
func NewMessageSenderWorker(
	cfg *config.WorkerConfig,
	q queue.Queue,
	whatsappService whatsapp_service.WhatsAppService,
) *MessageSenderWorker {
	return &MessageSenderWorker{
		config:          cfg,
		queue:           q,
		whatsappService: whatsappService,
		stopCh:          make(chan struct{}),
	}
}

// Start starts the message sender worker
func (w *MessageSenderWorker) Start() {
	log.Printf("Starting message sender worker (rate limit: 1 msg per %d seconds)", w.config.MessageRateLimitSeconds)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle stop signal
	go func() {
		<-w.stopCh
		cancel()
	}()

	// Start consuming messages
	err := w.queue.Consume(ctx, w.processMessage)
	if err != nil && ctx.Err() == nil {
		log.Printf("Message sender worker error: %v", err)
	}

	log.Println("Message sender worker stopped")
}

// Stop stops the message sender worker
func (w *MessageSenderWorker) Stop() {
	close(w.stopCh)
}

// processMessage processes a single message from the queue
func (w *MessageSenderWorker) processMessage(job *queue.MessageJob) error {
	// Apply rate limiting
	time.Sleep(time.Duration(w.config.MessageRateLimitSeconds) * time.Second)

	// Log retry info if this is a retry attempt
	if job.Attempts > 0 {
		atomic.AddInt64(&w.messagesRetried, 1)
		log.Printf("Retrying %s message to %s (attempt %d/%d, last error: %s)",
			job.MessageType, job.Phone, job.Attempts+1, job.MaxAttempts, job.LastError)
	} else {
		log.Printf("Sending %s message to %s (appointment: %s)", job.MessageType, job.Phone, job.AppointmentID)
	}

	// Send the message via WhatsApp
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := w.whatsappService.SendTextMessage(ctx, job.Instance, job.Phone, job.Message)
	if err != nil {
		atomic.AddInt64(&w.messagesFailed, 1)
		// Return error to trigger retry with exponential backoff
		return fmt.Errorf("failed to send message: %w", err)
	}

	atomic.AddInt64(&w.messagesSent, 1)
	if job.Attempts > 0 {
		log.Printf("Successfully sent %s message to %s after %d retries", job.MessageType, job.Phone, job.Attempts)
	} else {
		log.Printf("Successfully sent %s message to %s", job.MessageType, job.Phone)
	}

	return nil
}

// GetStats returns current statistics
func (w *MessageSenderWorker) GetStats() MessageSenderStats {
	return MessageSenderStats{
		MessagesSent:    atomic.LoadInt64(&w.messagesSent),
		MessagesFailed:  atomic.LoadInt64(&w.messagesFailed),
		MessagesRetried: atomic.LoadInt64(&w.messagesRetried),
	}
}

// MessageSenderStats holds statistics about message sending
type MessageSenderStats struct {
	MessagesSent    int64 `json:"messages_sent"`
	MessagesFailed  int64 `json:"messages_failed"`
	MessagesRetried int64 `json:"messages_retried"`
}
