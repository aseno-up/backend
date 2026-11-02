package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/config"
	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/internal/queue"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	systemlog_model "github.com/aseno/backend/pkg/systemlog/model"
	systemlog_service "github.com/aseno/backend/pkg/systemlog/service"
)

// ClassReminderCandidate is one upcoming class occurrence × enrolled patient.
// The source resolves all the joins (active group plans → active slots →
// active enrollments × matching weekdays → patient phone/name/instance) so
// the worker is a thin loop over candidates, not a multi-table query host.
type ClassReminderCandidate struct {
	ClinicID         uuid.UUID
	ClinicSettings   clinic_model.ClinicSettings
	PlanID           uuid.UUID
	PlanTitle        string
	PatientID        uuid.UUID
	PatientName      string
	PatientPhone     string
	WhatsAppInstance string
	SessionStart     time.Time // class start datetime in clinic-local time
}

// ClassReminderSource is the upcoming-class lookup. Implementations walk
// session_plans (group + active) × session_plan_slots (active) × active
// enrollments to produce ClassReminderCandidate rows whose SessionStart
// falls within [windowStart, windowEnd]. The worker passes the threshold-
// derived window so the source knows what to compute.
type ClassReminderSource interface {
	UpcomingClasses(ctx context.Context, windowStart, windowEnd time.Time) ([]ClassReminderCandidate, error)
}

// ClassReminderLog dedups reminders so the same (clinic, plan, patient,
// session_date, threshold) tuple is sent at most once. Backed by a unique
// index on the dedicated log table; in tests, an in-memory map.
type ClassReminderLog interface {
	HasBeenSent(ctx context.Context, clinicID, planID, patientID uuid.UUID, sessionDate, reminderType string) (bool, error)
	MarkSent(ctx context.Context, clinicID, planID, patientID uuid.UUID, sessionDate, reminderType string) error
}

// ClassAttendanceChecker reports whether the patient already engaged with
// the class via group_session_attendance (any status: confirmed, present,
// absent, substituted). Either way the patient should not be re-prompted —
// the reminder would be redundant or annoying.
type ClassAttendanceChecker interface {
	HasAttendanceRow(ctx context.Context, clinicID, planID, patientID uuid.UUID, sessionDate string) (bool, error)
}

// GroupClassReminderWorker scans for upcoming group classes and enqueues
// WhatsApp reminders. Distinct from ReminderWorker (appointments) because
// group plans never materialize into appointments — the worker walks
// session_plan_slots virtually.
type GroupClassReminderWorker struct {
	config              *config.WorkerConfig
	source              ClassReminderSource
	log                 ClassReminderLog
	attendance          ClassAttendanceChecker
	queue               reminderQueue
	subscriptionChecker ClinicSubscriptionChecker
	systemLog           systemlog_service.SystemLogService
	stopCh              chan struct{}
}

// NewGroupClassReminderWorker creates a new group class reminder worker.
func NewGroupClassReminderWorker(
	cfg *config.WorkerConfig,
	source ClassReminderSource,
	log ClassReminderLog,
	attendance ClassAttendanceChecker,
	q reminderQueue,
) *GroupClassReminderWorker {
	return &GroupClassReminderWorker{
		config:     cfg,
		source:     source,
		log:        log,
		attendance: attendance,
		queue:      q,
		stopCh:     make(chan struct{}),
	}
}

// SetSubscriptionChecker wires the same gate the appointment worker uses so
// clinics whose subscription is canceled/expired stop sending reminders.
func (w *GroupClassReminderWorker) SetSubscriptionChecker(checker ClinicSubscriptionChecker) {
	w.subscriptionChecker = checker
}

// SetSystemLogService wires structured logging.
func (w *GroupClassReminderWorker) SetSystemLogService(svc systemlog_service.SystemLogService) {
	w.systemLog = svc
}

func (w *GroupClassReminderWorker) logEvent(ctx context.Context, clinicID *uuid.UUID, event string, status systemlog_model.LogStatus, errMsg string, payload interface{}) {
	if w.systemLog == nil {
		if errMsg != "" {
			logger.Printf("[ClassReminder] %s (clinic=%v): %s", event, clinicID, errMsg)
		}
		return
	}
	_ = w.systemLog.Log(ctx, systemlog_service.SystemLogEntry{
		ClinicID: clinicID,
		Service:  "group_class_reminder_worker",
		Event:    event,
		Status:   status,
		Error:    errMsg,
		Payload:  payload,
	})
}

// Start runs the worker on the configured ticker. Reuses ReminderWorker's
// thresholds list but filters out 48h (group classes are weekly recurring,
// 48h heads-up is mostly noise).
func (w *GroupClassReminderWorker) Start() {
	if !w.config.ReminderWorkerEnabled {
		logger.Println("Group class reminder worker is disabled (shares ReminderWorkerEnabled with appointment reminders)")
		return
	}
	logger.Printf("Starting group class reminder worker (interval: %v)", w.config.ReminderWorkerInterval)
	w.processReminders()
	ticker := time.NewTicker(w.config.ReminderWorkerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.processReminders()
		case <-w.stopCh:
			logger.Println("Group class reminder worker stopped")
			return
		}
	}
}

// Stop signals the worker to exit its loop.
func (w *GroupClassReminderWorker) Stop() {
	close(w.stopCh)
}

func (w *GroupClassReminderWorker) processReminders() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Println("Processing group class reminders...")
	totalEnqueued := 0
	subCache := map[uuid.UUID]bool{}

	for _, threshold := range w.config.GetReminderThresholds() {
		// Skip 48h: classes are weekly-recurring and a 48h heads-up on top
		// of a 24h reminder is almost always noise. Operators who want it
		// can re-enable by editing this filter.
		if threshold.Type == "reminder_48h" {
			continue
		}
		count, err := w.processThreshold(ctx, threshold, subCache)
		if err != nil {
			logger.Printf("Error processing class %s reminders: %v", threshold.Name, err)
			continue
		}
		totalEnqueued += count
	}

	if totalEnqueued > 0 {
		logger.Printf("Group class reminder processing completed: %d enqueued", totalEnqueued)
	}
}

// processThreshold enqueues class reminders for a single threshold window.
// Public-on-the-package for tests (mirrors ReminderWorker.processThreshold).
func (w *GroupClassReminderWorker) processThreshold(ctx context.Context, threshold config.ReminderThreshold, subCache map[uuid.UUID]bool) (int, error) {
	now := time.Now()
	windowBuffer := 30 * time.Minute
	windowStart := now.Add(threshold.Duration - windowBuffer)
	windowEnd := now.Add(threshold.Duration + windowBuffer)

	candidates, err := w.source.UpcomingClasses(ctx, windowStart, windowEnd)
	if err != nil {
		w.logEvent(ctx, nil, "list_candidates_failed", systemlog_model.LogStatusFailed, err.Error(), map[string]any{
			"threshold": threshold.Name,
		})
		return 0, fmt.Errorf("failed to list upcoming classes: %w", err)
	}

	enqueued := 0
	for _, c := range candidates {
		if !c.ClinicSettings.WhatsAppRemindersEnabled {
			continue
		}
		if !c.ClinicSettings.IsGroupClassReminderEnabled(threshold.Type) {
			continue
		}
		if !w.isClinicSubscriptionActive(ctx, c.ClinicID, subCache) {
			continue
		}
		if c.PatientPhone == "" {
			continue
		}

		sessionDateStr := c.SessionStart.In(saoPauloLoc()).Format("2006-01-02")

		// Skip if the patient already engaged with this class — confirmed
		// via AI, marked present/absent by operator, etc. The reminder
		// would be redundant in any of these cases.
		if w.attendance != nil {
			already, err := w.attendance.HasAttendanceRow(ctx, c.ClinicID, c.PlanID, c.PatientID, sessionDateStr)
			if err != nil {
				logger.Printf("[ClassReminder] attendance lookup failed for plan=%s patient=%s date=%s: %v — skipping to avoid re-prompting", c.PlanID, c.PatientID, sessionDateStr, err)
				continue
			}
			if already {
				continue
			}
		}

		// Dedup against the reminder log: if we've already sent this same
		// (clinic, plan, patient, date, threshold) tuple, skip. The unique
		// index on the underlying table is the safety net — this read is
		// the optimization to avoid the queue write entirely when we know.
		alreadySent, err := w.log.HasBeenSent(ctx, c.ClinicID, c.PlanID, c.PatientID, sessionDateStr, threshold.Type)
		if err != nil {
			logger.Printf("[ClassReminder] log lookup failed: %v — skipping to fail closed", err)
			continue
		}
		if alreadySent {
			continue
		}

		message := w.buildMessage(c, threshold)

		job := &queue.MessageJob{
			ID:          uuid.New().String(),
			ClinicID:    c.ClinicID.String(),
			Instance:    c.WhatsAppInstance,
			Phone:       c.PatientPhone,
			PatientName: c.PatientName,
			Message:     message,
			MessageType: threshold.Type,
			Attempts:    0,
			MaxAttempts: w.config.MessageRetryMax,
			CreatedAt:   time.Now(),
			ScheduledAt: time.Now(),
			// AppointmentID intentionally empty — group plan classes never
			// materialize into appointments, so there is no UUID to attach.
			// The downstream message sender / WhatsApp service must not
			// require it for class reminder messages.
		}

		clinicID := c.ClinicID
		payload := map[string]any{
			"plan_id":      c.PlanID.String(),
			"patient_id":   c.PatientID.String(),
			"session_date": sessionDateStr,
			"threshold":    threshold.Name,
		}

		if err := w.queue.Publish(ctx, job); err != nil {
			logger.Printf("Failed to enqueue class reminder for plan=%s patient=%s: %v", c.PlanID, c.PatientID, err)
			w.logEvent(ctx, &clinicID, "enqueue_whatsapp_failed", systemlog_model.LogStatusFailed, err.Error(), payload)
			continue
		}

		if err := w.log.MarkSent(ctx, c.ClinicID, c.PlanID, c.PatientID, sessionDateStr, threshold.Type); err != nil {
			// Warning, not failed: the message is in the queue. The next
			// tick may try to re-enqueue the same reminder — surface this
			// on the dashboard so the operator can investigate.
			logger.Printf("[ClassReminder] failed to mark sent: %v", err)
			w.logEvent(ctx, &clinicID, "mark_reminder_sent_failed", systemlog_model.LogStatusWarning, err.Error(), payload)
		}

		w.logEvent(ctx, &clinicID, "reminder_sent", systemlog_model.LogStatusSuccess, "", payload)
		enqueued++
	}

	return enqueued, nil
}

func (w *GroupClassReminderWorker) isClinicSubscriptionActive(ctx context.Context, clinicID uuid.UUID, subCache map[uuid.UUID]bool) bool {
	if w.subscriptionChecker == nil {
		return true
	}
	if cached, ok := subCache[clinicID]; ok {
		return cached
	}
	active, err := w.subscriptionChecker.IsSubscriptionActive(ctx, clinicID)
	if err != nil {
		logger.Printf("[ClassReminder] subscription check failed for clinic %s: %v — skipping", clinicID, err)
		subCache[clinicID] = false
		return false
	}
	subCache[clinicID] = active
	return active
}

func (w *GroupClassReminderWorker) buildMessage(c ClassReminderCandidate, threshold config.ReminderThreshold) string {
	loc := saoPauloLoc()
	startLocal := c.SessionStart.In(loc)
	dateStr := startLocal.Format("02/01/2006")
	timeStr := startLocal.Format("15:04")
	weekdayPT := weekdayLabelPT(startLocal.Weekday())

	greeting := greetingForHour(time.Now().In(loc).Hour())

	switch threshold.Type {
	case "reminder_24h":
		return fmt.Sprintf(
			"%s, %s! 📅\n\n"+
				"Lembrando da sua aula de *%s*:\n\n"+
				"📆 %s, %s\n"+
				"🕐 %s\n\n"+
				"Você confirma sua presença? Responda *SIM* ou *NÃO*.",
			greeting, c.PatientName, c.PlanTitle, weekdayPT, dateStr, timeStr,
		)
	case "reminder_2h":
		return fmt.Sprintf(
			"%s, %s! 🔔\n\n"+
				"Sua aula de *%s* é hoje às *%s*.\n\n"+
				"Te esperamos!",
			greeting, c.PatientName, c.PlanTitle, timeStr,
		)
	default:
		return fmt.Sprintf("Lembrete: aula de %s — %s, %s às %s", c.PlanTitle, weekdayPT, dateStr, timeStr)
	}
}

func saoPauloLoc() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.UTC
	}
	return loc
}

func greetingForHour(h int) string {
	switch {
	case h < 12:
		return "Bom dia"
	case h < 18:
		return "Boa tarde"
	default:
		return "Boa noite"
	}
}

func weekdayLabelPT(d time.Weekday) string {
	switch d {
	case time.Sunday:
		return "Domingo"
	case time.Monday:
		return "Segunda"
	case time.Tuesday:
		return "Terça"
	case time.Wednesday:
		return "Quarta"
	case time.Thursday:
		return "Quinta"
	case time.Friday:
		return "Sexta"
	case time.Saturday:
		return "Sábado"
	}
	return ""
}
