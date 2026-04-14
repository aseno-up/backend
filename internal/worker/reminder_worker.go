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

// reminderAppointmentRepo captures the slice of the appointment repository used
// by the reminder worker. Declared here so tests can stub only what's needed.
type reminderAppointmentRepo interface {
	GetPendingReminders(ctx context.Context, reminderType string, windowStart, windowEnd time.Time) ([]*appointment_model.AppointmentWithClinic, error)
	MarkReminderSent(ctx context.Context, clinicID, id uuid.UUID, reminderType string) error
	SetConfirmationExpiresAt(ctx context.Context, clinicID, id uuid.UUID, expiresAt time.Time) error
}

// reminderQueue is the slice of queue.Queue used by the reminder worker.
type reminderQueue interface {
	Publish(ctx context.Context, job *queue.MessageJob) error
	QueueLength(ctx context.Context) (int, error)
}

// ClinicSubscriptionChecker tells the reminder worker whether a clinic has an
// active subscription (trial-not-expired or paying). When it returns false,
// the worker skips enqueuing reminders for that clinic so we don't keep
// messaging patients of clinics whose trial ended or whose subscription was
// canceled.
type ClinicSubscriptionChecker interface {
	IsSubscriptionActive(ctx context.Context, clinicID uuid.UUID) (bool, error)
}

// ReminderWorker scans for appointments needing reminders and enqueues messages
type ReminderWorker struct {
	config              *config.WorkerConfig
	appointmentRepo     reminderAppointmentRepo
	queue               reminderQueue
	subscriptionChecker ClinicSubscriptionChecker
	stopCh              chan struct{}
}

// NewReminderWorker creates a new reminder worker.
func NewReminderWorker(
	cfg *config.WorkerConfig,
	appointmentRepo reminderAppointmentRepo,
	q reminderQueue,
) *ReminderWorker {
	return &ReminderWorker{
		config:          cfg,
		appointmentRepo: appointmentRepo,
		queue:           q,
		stopCh:          make(chan struct{}),
	}
}

// Compile-time check: the concrete appointment repository must satisfy the
// narrow interface the worker expects.
var _ reminderAppointmentRepo = (appointment_repository.AppointmentRepository)(nil)
var _ reminderQueue = (queue.Queue)(nil)

// SetSubscriptionChecker wires a subscription checker so the worker skips
// reminders for clinics whose subscription is no longer active.
func (w *ReminderWorker) SetSubscriptionChecker(checker ClinicSubscriptionChecker) {
	w.subscriptionChecker = checker
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

	// Subscription-active lookups are cached for the duration of one run so a
	// clinic with many pending reminders doesn't trigger N identical queries.
	subCache := map[uuid.UUID]bool{}

	for _, threshold := range thresholds {
		count, err := w.processThreshold(ctx, threshold, subCache)
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

// processThreshold processes a single reminder threshold. subCache memoizes
// subscription-active lookups within a single run.
func (w *ReminderWorker) processThreshold(ctx context.Context, threshold config.ReminderThreshold, subCache map[uuid.UUID]bool) (int, error) {
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

		// Skip clinics whose subscription is not active (trial expired, canceled,
		// etc.). Without this, the worker would keep messaging patients of
		// clinics that no longer have access.
		if !w.isClinicSubscriptionActive(ctx, apt.ClinicID, subCache) {
			continue
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
		if err := w.appointmentRepo.MarkReminderSent(ctx, apt.ClinicID, apt.ID, threshold.Name); err != nil {
			logger.Printf("Failed to mark reminder sent for appointment %s: %v", apt.ID, err)
		}

		// Set confirmation timeout on the first reminder sent, regardless of
		// which threshold (48h/24h/2h) fired. Appointments booked within 48h
		// of the start time never fire reminder_48h, so gating on it left them
		// without a timeout forever — they stayed "pending" past the start and
		// the timeout worker never marked them as no_response.
		if apt.ConfirmationExpiresAt == nil {
			// Expiration is proportional to the reminder threshold: the patient
			// gets half of the remaining antecedence to reply before we give up.
			// 48h reminder → 24h window, 24h → 12h, 2h → 1h. A fixed 16h default
			// could run past the appointment's own start_time for short thresholds.
			expiresAt := time.Now().Add(threshold.Duration / 2)
			if err := w.appointmentRepo.SetConfirmationExpiresAt(ctx, apt.ClinicID, apt.ID, expiresAt); err != nil {
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

// isClinicSubscriptionActive returns true when the clinic can receive
// reminders. Results are cached per run via subCache. When no checker is
// wired, the worker falls back to the legacy behavior of always enqueuing.
// Errors fail-closed (skip) so we don't message clinics we can't verify.
func (w *ReminderWorker) isClinicSubscriptionActive(ctx context.Context, clinicID uuid.UUID, subCache map[uuid.UUID]bool) bool {
	if w.subscriptionChecker == nil {
		return true
	}
	if cached, ok := subCache[clinicID]; ok {
		return cached
	}
	active, err := w.subscriptionChecker.IsSubscriptionActive(ctx, clinicID)
	if err != nil {
		logger.Printf("[Reminder] Subscription check failed for clinic %s: %v — skipping", clinicID, err)
		subCache[clinicID] = false
		return false
	}
	if !active {
		logger.Printf("[Reminder] Clinic %s has no active subscription, skipping reminders", clinicID)
	}
	subCache[clinicID] = active
	return active
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
	MarkReminderSent(ctx context.Context, clinicID, id uuid.UUID, reminderType string) error
	SetConfirmationExpiresAt(ctx context.Context, clinicID, id uuid.UUID, expiresAt time.Time) error
} = (appointment_repository.AppointmentRepository)(nil)
