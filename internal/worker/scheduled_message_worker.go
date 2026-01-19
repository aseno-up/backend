package worker

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aseno/backend/internal/config"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	whatsapp_model "github.com/aseno/backend/pkg/whatsapp/model"
	whatsapp_repository "github.com/aseno/backend/pkg/whatsapp/repository"
	whatsapp_service "github.com/aseno/backend/pkg/whatsapp/service"
)

// ScheduledMessageWorker processes scheduled messages
type ScheduledMessageWorker struct {
	config           *config.WorkerConfig
	scheduledMsgRepo whatsapp_repository.ScheduledMessageRepository
	clinicRepo       clinic_repository.ClinicRepository
	whatsappService  whatsapp_service.WhatsAppService
	stopCh           chan struct{}

	// Stats
	messagesProcessed int64
	messagesFailed    int64
}

// NewScheduledMessageWorker creates a new scheduled message worker
func NewScheduledMessageWorker(
	cfg *config.WorkerConfig,
	scheduledMsgRepo whatsapp_repository.ScheduledMessageRepository,
	clinicRepo clinic_repository.ClinicRepository,
	whatsappService whatsapp_service.WhatsAppService,
) *ScheduledMessageWorker {
	return &ScheduledMessageWorker{
		config:           cfg,
		scheduledMsgRepo: scheduledMsgRepo,
		clinicRepo:       clinicRepo,
		whatsappService:  whatsappService,
		stopCh:           make(chan struct{}),
	}
}

// Start starts the scheduled message worker
func (w *ScheduledMessageWorker) Start() {
	// Worker runs at the same rate as delay between patients
	// Each execution processes 1 patient per clinic (up to 100 clinics)
	pollInterval := time.Duration(w.config.MinDelayBetweenPatientsSeconds) * time.Second

	log.Printf("Starting scheduled message worker (poll interval: %d seconds)",
		w.config.MinDelayBetweenPatientsSeconds)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Process immediately on start
	w.processScheduledMessages()

	for {
		select {
		case <-ticker.C:
			w.processScheduledMessages()
		case <-w.stopCh:
			log.Println("Scheduled message worker stopped")
			return
		}
	}
}

// Stop stops the scheduled message worker
func (w *ScheduledMessageWorker) Stop() {
	close(w.stopCh)
}

// processScheduledMessages processes pending messages (1 patient per clinic)
func (w *ScheduledMessageWorker) processScheduledMessages() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get messages for first patient of each clinic (up to 100 clinics)
	// Returns ALL messages for those patients
	messages, err := w.scheduledMsgRepo.GetPendingMessages(ctx, 100)
	if err != nil {
		log.Printf("[ScheduledMessageWorker] Error fetching pending messages: %v", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	// Group messages by patient (clinic_id + phone)
	patientMessages := make(map[string][]*whatsapp_model.ScheduledMessage)
	for _, msg := range messages {
		key := msg.ClinicID.String() + ":" + msg.Phone
		patientMessages[key] = append(patientMessages[key], msg)
	}

	log.Printf("[ScheduledMessageWorker] Processing %d patients (%d total messages)",
		len(patientMessages), len(messages))

	// Process each patient's messages IN PARALLEL (one goroutine per patient)
	// This ensures delays are PER CLINIC, not between different clinics
	var wg sync.WaitGroup
	for _, msgs := range patientMessages {
		wg.Add(1)
		go func(messages []*whatsapp_model.ScheduledMessage) {
			defer wg.Done()
			w.processPatientMessages(ctx, messages)
		}(msgs)
	}

	// Wait for all patients to complete
	wg.Wait()

	log.Printf("[ScheduledMessageWorker] Batch complete: %d processed, %d failed",
		atomic.LoadInt64(&w.messagesProcessed), atomic.LoadInt64(&w.messagesFailed))
}

// processPatientMessages sends all messages for a single patient with proper delays
func (w *ScheduledMessageWorker) processPatientMessages(ctx context.Context, messages []*whatsapp_model.ScheduledMessage) {
	if len(messages) == 0 {
		return
	}

	// Get clinic settings for delay configuration
	var minDelayBetweenMessages int = 3 // default
	if w.clinicRepo != nil {
		if clinic, err := w.clinicRepo.GetByID(ctx, messages[0].ClinicID); err == nil {
			minDelayBetweenMessages = clinic.GetMinDelayBetweenMessages(3)
		}
	}

	log.Printf("[ScheduledMessageWorker] Sending %d messages to %s (clinic %s)",
		len(messages), messages[0].Phone, messages[0].ClinicID)

	// Send all messages for this patient in sequence with proper delays
	for i, msg := range messages {
		// Apply delay between segments (except first one)
		if i > 0 {
			delay := time.Duration(minDelayBetweenMessages) * time.Second
			time.Sleep(delay)
		}

		w.processSingleMessage(ctx, msg)
	}
}

// processSingleMessage processes a single scheduled message
func (w *ScheduledMessageWorker) processSingleMessage(ctx context.Context, msg *whatsapp_model.ScheduledMessage) {
	log.Printf("[ScheduledMessageWorker] Sending scheduled message %s to %s (scheduled at: %s)",
		msg.ID, msg.Phone, msg.ScheduledAt.Format(time.RFC3339))

	// Send the message via WhatsApp
	_, err := w.whatsappService.SendTextMessage(ctx, msg.Instance, msg.Phone, msg.Message)
	if err != nil {
		atomic.AddInt64(&w.messagesFailed, 1)
		log.Printf("[ScheduledMessageWorker] Failed to send message %s: %v", msg.ID, err)

		// Mark as failed (but don't mark as sent, so it can be retried)
		if markErr := w.scheduledMsgRepo.MarkAsFailed(ctx, msg.ID, err.Error()); markErr != nil {
			log.Printf("[ScheduledMessageWorker] Failed to mark message as failed: %v", markErr)
		}
		return
	}

	// Mark as sent
	if err := w.scheduledMsgRepo.MarkAsSent(ctx, msg.ID); err != nil {
		log.Printf("[ScheduledMessageWorker] Failed to mark message as sent: %v", err)
		return
	}

	atomic.AddInt64(&w.messagesProcessed, 1)
	log.Printf("[ScheduledMessageWorker] Successfully sent scheduled message %s to %s", msg.ID, msg.Phone)
}

// GetStats returns current statistics
func (w *ScheduledMessageWorker) GetStats() ScheduledMessageStats {
	return ScheduledMessageStats{
		MessagesProcessed: atomic.LoadInt64(&w.messagesProcessed),
		MessagesFailed:    atomic.LoadInt64(&w.messagesFailed),
	}
}

// ScheduledMessageStats holds statistics about scheduled message processing
type ScheduledMessageStats struct {
	MessagesProcessed int64 `json:"messages_processed"`
	MessagesFailed    int64 `json:"messages_failed"`
}
