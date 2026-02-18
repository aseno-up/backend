package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/aseno/backend/internal/logger"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/config"
	"github.com/aseno/backend/internal/queue"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
)

// ReminderWorker scans for appointments needing reminders and enqueues messages
type ReminderWorker struct {
	config          *config.WorkerConfig
	appointmentRepo appointment_repository.AppointmentRepository
	queue           queue.Queue
	stopCh          chan struct{}
}

// NewReminderWorker creates a new reminder worker
func NewReminderWorker(
	cfg *config.WorkerConfig,
	appointmentRepo appointment_repository.AppointmentRepository,
	q queue.Queue,
) *ReminderWorker {
	return &ReminderWorker{
		config:          cfg,
		appointmentRepo: appointmentRepo,
		queue:           q,
		stopCh:          make(chan struct{}),
	}
}

// Start starts the reminder worker
func (w *ReminderWorker) Start() {
	if !w.config.ReminderWorkerEnabled {
		logger.Println("Reminder worker is disabled")
		return
	}

	logger.Printf("Starting reminder worker (interval: %v)", w.config.ReminderWorkerInterval)

	// Run immediately on start
	w.processReminders()

	ticker := time.NewTicker(w.config.ReminderWorkerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processReminders()
		case <-w.stopCh:
			logger.Println("Reminder worker stopped")
			return
		}
	}
}

// Stop stops the reminder worker
func (w *ReminderWorker) Stop() {
	close(w.stopCh)
}

// processReminders processes all reminder thresholds
func (w *ReminderWorker) processReminders() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Println("Processing reminders...")

	thresholds := w.config.GetReminderThresholds()
	totalEnqueued := 0

	for _, threshold := range thresholds {
		count, err := w.processThreshold(ctx, threshold)
		if err != nil {
			logger.Printf("Error processing %s reminders: %v", threshold.Name, err)
			continue
		}
		totalEnqueued += count
	}

	if totalEnqueued > 0 {
		logger.Printf("Reminder processing completed: %d messages enqueued", totalEnqueued)
	} else {
		logger.Println("Reminder processing completed: no messages to send")
	}
}

// processThreshold processes a single reminder threshold
func (w *ReminderWorker) processThreshold(ctx context.Context, threshold config.ReminderThreshold) (int, error) {
	now := time.Now()

	// Calculate the window for this threshold
	// For 48h reminder: appointments between 47h and 49h from now (1 hour window around 48h)
	windowBuffer := 30 * time.Minute
	windowStart := now.Add(threshold.Duration - windowBuffer)
	windowEnd := now.Add(threshold.Duration + windowBuffer)

	appointments, err := w.appointmentRepo.GetPendingReminders(ctx, threshold.Name, windowStart, windowEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending reminders: %w", err)
	}

	enqueued := 0
	for _, apt := range appointments {
		// Skip 48h reminder if appointment is within the next 2 days (tomorrow or day after tomorrow).
		// In these cases, the 24h reminder is sufficient. This prevents awkward early reminders,
		// e.g., sending a reminder on Monday for a Wednesday appointment requested on Sunday.
		if threshold.Type == "reminder_48h" {
			loc, err := time.LoadLocation("America/Sao_Paulo")
			if err != nil {
				logger.Printf("Failed to load location: %v", err)
				continue
			}

			nowLocal := time.Now().In(loc)
			aptLocal := apt.StartTime.In(loc)
			todayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
			dayAfterTomorrowEnd := todayStart.AddDate(0, 0, 3) // start of 3 days from now
			if aptLocal.Before(dayAfterTomorrowEnd) {
				continue
			}
		}

		// Skip if no patient info
		if apt.Patient == nil {
			logger.Printf("Skipping appointment %s: no patient info", apt.ID)
			continue
		}

		// Skip if no patient phone
		phone := apt.Patient.WhatsApp
		if phone == "" {
			phone = apt.Patient.Phone
		}
		if phone == "" {
			logger.Printf("Skipping appointment %s: no patient phone", apt.ID)
			continue
		}

		// Get professional name
		professionalName := ""
		if apt.Professional != nil {
			professionalName = apt.Professional.Name
		}

		// Check if this is a follow-up message (previous reminder was sent but patient didn't respond)
		isFollowUp := w.isFollowUpReminder(apt, threshold)

		// Build the reminder message
		message := w.buildReminderMessage(apt.Patient.Name, professionalName, apt.StartTime, threshold, isFollowUp)

		// Create the job
		job := &queue.MessageJob{
			ID:            uuid.New().String(),
			ClinicID:      apt.ClinicID.String(),
			AppointmentID: apt.ID.String(),
			Instance:      apt.WhatsAppInstance,
			Phone:         phone,
			PatientName:   apt.Patient.Name,
			Message:       message,
			MessageType:   threshold.Type,
			Attempts:      0,
			MaxAttempts:   w.config.MessageRetryMax,
			CreatedAt:     time.Now(),
			ScheduledAt:   time.Now(),
		}

		// Enqueue the message
		if err := w.queue.Publish(ctx, job); err != nil {
			logger.Printf("Failed to enqueue reminder for appointment %s: %v", apt.ID, err)
			continue
		}

		// Mark reminder as sent
		if err := w.appointmentRepo.MarkReminderSent(ctx, apt.ID, threshold.Name); err != nil {
			logger.Printf("Failed to mark reminder sent for appointment %s: %v", apt.ID, err)
		}

		// Set confirmation timeout on first reminder (48h)
		// This starts the countdown for no_response status
		if threshold.Type == "reminder_48h" && apt.ConfirmationExpiresAt == nil {
			expiresAt := time.Now().Add(w.config.ConfirmationTimeoutDuration)
			if err := w.appointmentRepo.SetConfirmationExpiresAt(ctx, apt.ID, expiresAt); err != nil {
				logger.Printf("Failed to set confirmation timeout for appointment %s: %v", apt.ID, err)
			}
		}

		enqueued++
	}

	if enqueued > 0 {
		logger.Printf("Enqueued %d %s reminders", enqueued, threshold.Name)
	}

	return enqueued, nil
}

// isFollowUpReminder checks if this reminder is a follow-up (previous reminder was sent but patient didn't respond)
func (w *ReminderWorker) isFollowUpReminder(apt *appointment_model.AppointmentWithClinic, threshold config.ReminderThreshold) bool {
	switch threshold.Type {
	case "reminder_24h":
		// If 48h reminder was already sent, this is a follow-up
		return apt.Reminder48hSentAt != nil
	case "reminder_2h":
		// If 24h reminder was already sent, this is a follow-up
		return apt.Reminder24hSentAt != nil
	default:
		return false
	}
}

// buildReminderMessage builds the reminder message based on threshold
func (w *ReminderWorker) buildReminderMessage(patientName, professionalName string, startTime time.Time, threshold config.ReminderThreshold, isFollowUp bool) string {
	// Format the date and time in Brazilian format
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	startTimeLocal := startTime.In(loc)
	dateStr := startTimeLocal.Format("02/01/2006")
	timeStr := startTimeLocal.Format("15:04")

	var greeting string
	hour := time.Now().In(loc).Hour()
	switch {
	case hour < 12:
		greeting = "Bom dia"
	case hour < 18:
		greeting = "Boa tarde"
	default:
		greeting = "Boa noite"
	}

	switch threshold.Type {
	case "reminder_48h":
		return fmt.Sprintf(
			"%s, %s! 📅\n\n"+
				"Lembramos que você tem uma consulta agendada:\n\n"+
				"📆 Data: %s\n"+
				"🕐 Horário: %s\n"+
				"👨‍⚕️ Profissional: %s\n\n"+
				"Por favor, confirme sua presença respondendo:\n"+
				"✅ *SIM* - para confirmar\n"+
				"❌ *NÃO* - para cancelar\n\n"+
				"Caso precise reagendar, entre em contato conosco.",
			greeting, patientName, dateStr, timeStr, professionalName,
		)

	case "reminder_24h":
		if isFollowUp {
			// Follow-up message - patient didn't respond to 48h reminder
			return fmt.Sprintf(
				"%s, %s! ⚠️\n\n"+
					"*Ainda não recebemos sua confirmação.*\n\n"+
					"Sua consulta está marcada para *amanhã*:\n\n"+
					"📆 Data: %s\n"+
					"🕐 Horário: %s\n"+
					"👨‍⚕️ Profissional: %s\n\n"+
					"⚡ Por favor, responda *SIM* para confirmar ou *NÃO* para cancelar.\n\n"+
					"Precisamos da sua confirmação para manter o horário reservado.",
				greeting, patientName, dateStr, timeStr, professionalName,
			)
		}
		return fmt.Sprintf(
			"%s, %s! ⏰\n\n"+
				"Sua consulta está marcada para *amanhã*:\n\n"+
				"📆 Data: %s\n"+
				"🕐 Horário: %s\n"+
				"👨‍⚕️ Profissional: %s\n\n"+
				"Confirme sua presença respondendo *SIM* ou *NÃO*.",
			greeting, patientName, dateStr, timeStr, professionalName,
		)

	case "reminder_2h":
		if isFollowUp {
			// Follow-up message - patient didn't respond to previous reminders
			return fmt.Sprintf(
				"%s, %s! 🚨\n\n"+
					"*Última chamada!*\n\n"+
					"Sua consulta é *HOJE* às *%s*.\n\n"+
					"👨‍⚕️ Profissional: %s\n\n"+
					"Você irá comparecer? Responda *SIM* ou *NÃO*.",
				greeting, patientName, timeStr, professionalName,
			)
		}
		return fmt.Sprintf(
			"%s, %s! 🔔\n\n"+
				"Sua consulta é *hoje* às *%s*.\n\n"+
				"👨‍⚕️ Profissional: %s\n\n"+
				"Estamos te esperando!",
			greeting, patientName, timeStr, professionalName,
		)

	default:
		return fmt.Sprintf(
			"Lembrete de consulta: %s às %s com %s",
			dateStr, timeStr, professionalName,
		)
	}
}

// ReminderStats holds statistics about reminder processing
type ReminderStats struct {
	ThresholdName string
	Processed     int
	Enqueued      int
	Failed        int
}

// GetStats returns current statistics (for monitoring)
func (w *ReminderWorker) GetStats(ctx context.Context) ([]ReminderStats, error) {
	// This could be expanded to track actual stats
	// For now, just return queue length
	length, err := w.queue.QueueLength(ctx)
	if err != nil {
		return nil, err
	}

	return []ReminderStats{
		{
			ThresholdName: "queue",
			Processed:     length,
		},
	}, nil
}

// Compile-time check that the repository has the required methods
var _ interface {
	GetPendingReminders(ctx context.Context, reminderType string, windowStart, windowEnd time.Time) ([]*appointment_model.AppointmentWithClinic, error)
	MarkReminderSent(ctx context.Context, id uuid.UUID, reminderType string) error
	SetConfirmationExpiresAt(ctx context.Context, id uuid.UUID, expiresAt time.Time) error
} = (appointment_repository.AppointmentRepository)(nil)
