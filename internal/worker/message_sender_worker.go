package worker

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/config"
	"github.com/aseno/backend/internal/queue"
	"github.com/aseno/backend/internal/redis"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	whatsapp_service "github.com/aseno/backend/pkg/whatsapp/service"
)

// MessageSenderWorker consumes messages from the queue and sends them via WhatsApp
type MessageSenderWorker struct {
	config          *config.WorkerConfig
	queue           queue.Queue
	whatsappService whatsapp_service.WhatsAppService
	rateLimiter     *redis.RateLimiter
	clinicRepo      clinic_repository.ClinicRepository
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
	rateLimiter *redis.RateLimiter,
	clinicRepo clinic_repository.ClinicRepository,
) *MessageSenderWorker {
	return &MessageSenderWorker{
		config:          cfg,
		queue:           q,
		whatsappService: whatsappService,
		rateLimiter:     rateLimiter,
		clinicRepo:      clinicRepo,
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get clinic-specific delay settings (clinic overrides env default)
	minDelaySeconds := w.config.MinDelayBetweenPatientsSeconds
	if w.clinicRepo != nil {
		clinicID, err := uuid.Parse(job.ClinicID)
		if err != nil {
			log.Printf("[MessageSender] Warning: invalid clinic ID format: %v", err)
		} else if clinic, err := w.clinicRepo.GetByID(ctx, clinicID); err == nil {
			minDelaySeconds = clinic.GetMinDelayBetweenPatients(w.config.MinDelayBetweenPatientsSeconds)
		} else {
			log.Printf("[MessageSender] Warning: failed to fetch clinic settings, using default delay: %v", err)
		}
	}

	// Step 1: Check rate limit between different patients (if Redis available)
	if w.rateLimiter != nil && w.rateLimiter.IsAvailable() {
		minDelay := time.Duration(minDelaySeconds) * time.Second
		delayNeeded := w.rateLimiter.CheckRateLimit(ctx, job.ClinicID, job.Phone, minDelay)

		if delayNeeded > 0 {
			log.Printf("[MessageSender] Rate limit: waiting %v before sending to different patient %s", delayNeeded, job.Phone)

			// Republish with delay
			if err := w.queue.PublishWithDelay(ctx, job, delayNeeded); err != nil {
				log.Printf("[MessageSender] Failed to republish with delay: %v", err)
				// Continue anyway - don't fail the job
			} else {
				// Successfully republished with delay, don't process now
				log.Printf("[MessageSender] Message for %s rescheduled with %v delay", job.Phone, delayNeeded)
				return nil
			}
		} else {
			log.Printf("[MessageSender] Rate limit OK for %s (no delay needed)", job.Phone)
		}
	} else {
		if w.rateLimiter == nil {
			log.Printf("[MessageSender] Rate limiter is nil - Redis not initialized")
		} else if !w.rateLimiter.IsAvailable() {
			log.Printf("[MessageSender] Rate limiter not available - Redis not connected")
		}
	}

	// Step 2: Apply global rate limiting (1s between any messages)
	time.Sleep(time.Duration(w.config.MessageRateLimitSeconds) * time.Second)

	// Log retry info if this is a retry attempt
	if job.Attempts > 0 {
		atomic.AddInt64(&w.messagesRetried, 1)
		log.Printf("Retrying %s message to %s (attempt %d/%d, last error: %s)",
			job.MessageType, job.Phone, job.Attempts+1, job.MaxAttempts, job.LastError)
	} else {
		log.Printf("Sending %s message to %s (appointment: %s)", job.MessageType, job.Phone, job.AppointmentID)
	}

	// Step 3: Send the message via WhatsApp
	_, err := w.whatsappService.SendTextMessage(ctx, job.Instance, job.Phone, job.Message)
	if err != nil {
		atomic.AddInt64(&w.messagesFailed, 1)
		// Return error to trigger retry with exponential backoff
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Step 4: Record message sent for rate limiting (TTL = 2x min delay)
	if w.rateLimiter != nil && w.rateLimiter.IsAvailable() {
		ttl := time.Duration(minDelaySeconds*2) * time.Second
		if err := w.rateLimiter.RecordMessageSent(ctx, job.ClinicID, job.Phone, ttl); err != nil {
			log.Printf("[MessageSender] Warning: Failed to record rate limit: %v", err)
			// Don't fail the job due to Redis error
		}
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
