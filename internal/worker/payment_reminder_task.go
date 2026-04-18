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
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
	systemlog_model "github.com/aseno/backend/pkg/systemlog/model"
	systemlog_service "github.com/aseno/backend/pkg/systemlog/service"
)

// PaymentReminderCandidate represents one patient that may need a monthly
// payment reminder. Individual plans have EnrollmentID == nil; group plans
// set it to the enrollment row so uniqueness is scoped per-patient.
type PaymentReminderCandidate struct {
	ClinicID         uuid.UUID
	SessionPlanID    uuid.UUID
	EnrollmentID     *uuid.UUID
	PatientID        uuid.UUID
	PatientName      string
	PatientWhatsApp  string // Already normalized (whatsapp preferred, phone fallback)
	PatientEmail     string
	PlanTitle        string
	BillingDay       int     // 1..31
	Amount           float64 // Monthly amount for the message
	WhatsAppInstance string  // Clinic WhatsApp instance name (empty if not connected)
	// EnrolledAt is when the patient joined (created_at for individual plans,
	// joined_at for group enrollments). Used to enforce a grace period — the
	// first due date must be at least MinDaysBeforeFirstDueDate days away, so
	// a student added today doesn't get billed for the same month if the
	// billing day happens to be a few days ahead.
	EnrolledAt time.Time
}

// MinDaysBeforeFirstDueDate is the grace window between enrollment and the
// first billable due date. 30 days matches how most clinics think about
// "monthly" — a student added on the 12th shouldn't be charged on the 15th.
const MinDaysBeforeFirstDueDate = 30

// paymentReminderCandidateSource returns candidates for a given clinic —
// active individual monthly plans and active group monthly enrollments. It is
// the only data dependency of the task logic, which makes the task trivially
// unit-testable.
type paymentReminderCandidateSource interface {
	ListPaymentReminderCandidates(ctx context.Context, clinicID uuid.UUID) ([]*PaymentReminderCandidate, error)
}

// paymentReminderTracker tracks whether a given reference month has been paid
// or already reminded, so the task never sends two reminders to the same
// patient for the same month and never reminds someone who already paid.
type paymentReminderTracker interface {
	PaymentExists(ctx context.Context, clinicID, sessionPlanID uuid.UUID, enrollmentID *uuid.UUID, referenceMonth string) (bool, error)
	ReminderExists(ctx context.Context, clinicID, sessionPlanID uuid.UUID, enrollmentID *uuid.UUID, referenceMonth string, channel sessionplan_model.PaymentReminderChannel) (bool, error)
	RecordReminder(ctx context.Context, reminder *sessionplan_model.SessionPlanPaymentReminder) error
}

// paymentReminderClinicSource resolves which clinics to process in one tick
// and exposes their settings. The worker iterates across all active clinics;
// in practice the number is small.
type paymentReminderClinicSource interface {
	ListActiveClinics(ctx context.Context) ([]*clinic_model.Clinic, error)
}

// paymentReminderEmailSender sends a payment reminder email. Kept narrow so
// tests can stub it without pulling the full email service interface.
type paymentReminderEmailSender interface {
	SendPaymentReminder(ctx context.Context, data *PaymentReminderEmailData) error
}

// PaymentReminderEmailData is the payload passed to the email sender. It
// contains everything the template needs so the sender does not need to look
// up the clinic or patient again.
type PaymentReminderEmailData struct {
	RecipientEmail string
	RecipientName  string
	ClinicName     string
	PlanTitle      string
	DueDate        time.Time
	Amount         float64
}

// PaymentReminderTask wires the pieces the worker needs to decide whether to
// send a reminder and how. All dependencies are interfaces so the task can be
// constructed with mocks inside unit tests — the task function itself is the
// unit under test.
type PaymentReminderTask struct {
	cfg          *config.WorkerConfig
	clinics      paymentReminderClinicSource
	candidates   paymentReminderCandidateSource
	tracker      paymentReminderTracker
	queue        reminderQueue
	emailSender  paymentReminderEmailSender
	subChecker   ClinicSubscriptionChecker
	systemLog    systemlog_service.SystemLogService
	messagesMax  int
	locationName string
}

// NewPaymentReminderTask constructs the task with its dependencies. Email
// sending is optional — when emailSender is nil the task silently skips the
// email channel even if the clinic has it enabled.
func NewPaymentReminderTask(
	cfg *config.WorkerConfig,
	clinics paymentReminderClinicSource,
	candidates paymentReminderCandidateSource,
	tracker paymentReminderTracker,
	q reminderQueue,
	emailSender paymentReminderEmailSender,
) *PaymentReminderTask {
	return &PaymentReminderTask{
		cfg:          cfg,
		clinics:      clinics,
		candidates:   candidates,
		tracker:      tracker,
		queue:        q,
		emailSender:  emailSender,
		messagesMax:  cfg.MessageRetryMax,
		locationName: "America/Sao_Paulo",
	}
}

// SetSubscriptionChecker wires a subscription checker. Same pattern as the
// appointment reminder worker so trial-expired clinics stop receiving
// payment reminders without a code change.
func (t *PaymentReminderTask) SetSubscriptionChecker(checker ClinicSubscriptionChecker) {
	t.subChecker = checker
}

// SetSystemLogService wires the SystemLog service used to persist operational
// events to the database. When nil the worker falls back to stdout logging;
// callers wire this in main.go so dashboards can monitor the worker.
func (t *PaymentReminderTask) SetSystemLogService(svc systemlog_service.SystemLogService) {
	t.systemLog = svc
}

// logEvent writes a structured record to the system_logs table when a log
// service is wired; otherwise it falls back to stdout. Kept as a method so
// the non-system fields (service name, static event tag) stay consistent
// across the ~6 call sites inside the task.
func (t *PaymentReminderTask) logEvent(ctx context.Context, clinicID *uuid.UUID, event string, status systemlog_model.LogStatus, errMsg string, payload interface{}) {
	if t.systemLog == nil {
		if errMsg != "" {
			logger.Printf("[PaymentReminder] %s (clinic=%v): %s", event, clinicID, errMsg)
		}
		return
	}
	_ = t.systemLog.Log(ctx, systemlog_service.SystemLogEntry{
		ClinicID: clinicID,
		Service:  "payment_reminder_worker",
		Event:    event,
		Status:   status,
		Error:    errMsg,
		Payload:  payload,
	})
}

// Run is the function registered on the SessionPlanTasksWorker. It returns an
// error only when something unrecoverable happens at the top level (listing
// clinics); per-clinic errors are logged and the task keeps going so one bad
// clinic doesn't starve the others.
func (t *PaymentReminderTask) Run(ctx context.Context) error {
	// Respect the global kill switches. If both channels are disabled there
	// is nothing to do — bail out before touching the database.
	if !t.cfg.PaymentReminderWhatsAppEnabled && !t.cfg.PaymentReminderEmailEnabled {
		return nil
	}

	clinics, err := t.clinics.ListActiveClinics(ctx)
	if err != nil {
		return fmt.Errorf("list active clinics: %w", err)
	}

	loc, locErr := time.LoadLocation(t.locationName)
	if locErr != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	for _, c := range clinics {
		// "Lembretes" master switch gates ALL reminder emissions for this
		// clinic; payment reminders live below it by design.
		if !c.Settings.WhatsAppRemindersEnabled {
			continue
		}

		if t.subChecker != nil {
			active, subErr := t.subChecker.IsSubscriptionActive(ctx, c.ID)
			if subErr != nil {
				clinicID := c.ID
				t.logEvent(ctx, &clinicID, "subscription_check_failed", systemlog_model.LogStatusFailed, subErr.Error(), nil)
				continue
			}
			if !active {
				continue
			}
		}

		daysBefore := c.Settings.GetPaymentReminderDaysBefore()
		wantWhatsApp := c.Settings.IsPaymentReminderWhatsAppEnabled() && t.cfg.PaymentReminderWhatsAppEnabled
		wantEmail := c.Settings.IsPaymentReminderEmailEnabled() && t.cfg.PaymentReminderEmailEnabled

		if !wantWhatsApp && !wantEmail {
			continue
		}

		candidates, err := t.candidates.ListPaymentReminderCandidates(ctx, c.ID)
		if err != nil {
			clinicID := c.ID
			t.logEvent(ctx, &clinicID, "list_candidates_failed", systemlog_model.LogStatusFailed, err.Error(), nil)
			continue
		}

		for _, cand := range candidates {
			t.processCandidate(ctx, c, cand, now, daysBefore, wantWhatsApp, wantEmail)
		}
	}

	return nil
}

// processCandidate decides whether to send (and sends) for one candidate.
// Isolated so the unit tests can drive single-candidate scenarios without
// building a fake clinic list.
func (t *PaymentReminderTask) processCandidate(
	ctx context.Context,
	clinic *clinic_model.Clinic,
	cand *PaymentReminderCandidate,
	now time.Time,
	daysBefore int,
	wantWhatsApp, wantEmail bool,
) {
	dueDate, ok := ComputeUpcomingDueDate(now, cand.BillingDay, daysBefore, cand.EnrolledAt)
	if !ok {
		// Next due date is still outside the window — nothing to do.
		return
	}

	referenceMonth := dueDate.Format("2006-01")

	// If the payment for this reference month is already recorded, skip:
	// the patient already paid (or was marked as paid), and reminding them
	// would be annoying.
	paid, err := t.tracker.PaymentExists(ctx, cand.ClinicID, cand.SessionPlanID, cand.EnrollmentID, referenceMonth)
	if err != nil {
		clinicID := cand.ClinicID
		t.logEvent(ctx, &clinicID, "payment_lookup_failed", systemlog_model.LogStatusFailed, err.Error(), map[string]any{
			"session_plan_id": cand.SessionPlanID,
			"reference_month": referenceMonth,
		})
		return
	}
	if paid {
		return
	}

	if wantWhatsApp {
		t.maybeSendWhatsApp(ctx, clinic, cand, dueDate, referenceMonth)
	}
	if wantEmail {
		t.maybeSendEmail(ctx, clinic, cand, dueDate, referenceMonth)
	}
}

func (t *PaymentReminderTask) maybeSendWhatsApp(
	ctx context.Context,
	clinic *clinic_model.Clinic,
	cand *PaymentReminderCandidate,
	dueDate time.Time,
	referenceMonth string,
) {
	channel := sessionplan_model.PaymentReminderChannelWhatsApp
	clinicID := cand.ClinicID
	payload := map[string]any{
		"session_plan_id": cand.SessionPlanID,
		"enrollment_id":   cand.EnrollmentID,
		"patient_id":      cand.PatientID,
		"reference_month": referenceMonth,
		"channel":         string(channel),
	}

	if cand.PatientWhatsApp == "" || cand.WhatsAppInstance == "" {
		return
	}

	already, err := t.tracker.ReminderExists(ctx, cand.ClinicID, cand.SessionPlanID, cand.EnrollmentID, referenceMonth, channel)
	if err != nil {
		t.logEvent(ctx, &clinicID, "reminder_lookup_failed", systemlog_model.LogStatusFailed, err.Error(), payload)
		return
	}
	if already {
		return
	}

	message := buildWhatsAppMessage(cand.PatientName, cand.PlanTitle, dueDate, cand.Amount)

	job := &queue.MessageJob{
		ID:          uuid.New().String(),
		ClinicID:    cand.ClinicID.String(),
		Instance:    cand.WhatsAppInstance,
		Phone:       cand.PatientWhatsApp,
		PatientName: cand.PatientName,
		Message:     message,
		MessageType: "payment_reminder",
		Attempts:    0,
		MaxAttempts: t.messagesMax,
		CreatedAt:   time.Now(),
		ScheduledAt: time.Now(),
	}

	if err := t.queue.Publish(ctx, job); err != nil {
		t.logEvent(ctx, &clinicID, "enqueue_whatsapp_failed", systemlog_model.LogStatusFailed, err.Error(), payload)
		return
	}

	if err := t.tracker.RecordReminder(ctx, &sessionplan_model.SessionPlanPaymentReminder{
		ClinicID:       cand.ClinicID,
		SessionPlanID:  cand.SessionPlanID,
		EnrollmentID:   cand.EnrollmentID,
		PatientID:      cand.PatientID,
		ReferenceMonth: referenceMonth,
		Channel:        channel,
		SentAt:         time.Now(),
	}); err != nil {
		t.logEvent(ctx, &clinicID, "record_whatsapp_reminder_failed", systemlog_model.LogStatusFailed, err.Error(), payload)
		return
	}

	t.logEvent(ctx, &clinicID, "reminder_sent", systemlog_model.LogStatusSuccess, "", payload)
}

func (t *PaymentReminderTask) maybeSendEmail(
	ctx context.Context,
	clinic *clinic_model.Clinic,
	cand *PaymentReminderCandidate,
	dueDate time.Time,
	referenceMonth string,
) {
	channel := sessionplan_model.PaymentReminderChannelEmail
	clinicID := cand.ClinicID
	payload := map[string]any{
		"session_plan_id": cand.SessionPlanID,
		"enrollment_id":   cand.EnrollmentID,
		"patient_id":      cand.PatientID,
		"reference_month": referenceMonth,
		"channel":         string(channel),
	}

	if cand.PatientEmail == "" || t.emailSender == nil {
		return
	}

	already, err := t.tracker.ReminderExists(ctx, cand.ClinicID, cand.SessionPlanID, cand.EnrollmentID, referenceMonth, channel)
	if err != nil {
		t.logEvent(ctx, &clinicID, "reminder_lookup_failed", systemlog_model.LogStatusFailed, err.Error(), payload)
		return
	}
	if already {
		return
	}

	if err := t.emailSender.SendPaymentReminder(ctx, &PaymentReminderEmailData{
		RecipientEmail: cand.PatientEmail,
		RecipientName:  cand.PatientName,
		ClinicName:     clinic.Name,
		PlanTitle:      cand.PlanTitle,
		DueDate:        dueDate,
		Amount:         cand.Amount,
	}); err != nil {
		t.logEvent(ctx, &clinicID, "send_email_failed", systemlog_model.LogStatusFailed, err.Error(), payload)
		return
	}

	if err := t.tracker.RecordReminder(ctx, &sessionplan_model.SessionPlanPaymentReminder{
		ClinicID:       cand.ClinicID,
		SessionPlanID:  cand.SessionPlanID,
		EnrollmentID:   cand.EnrollmentID,
		PatientID:      cand.PatientID,
		ReferenceMonth: referenceMonth,
		Channel:        channel,
		SentAt:         time.Now(),
	}); err != nil {
		t.logEvent(ctx, &clinicID, "record_email_reminder_failed", systemlog_model.LogStatusFailed, err.Error(), payload)
		return
	}

	t.logEvent(ctx, &clinicID, "reminder_sent", systemlog_model.LogStatusSuccess, "", payload)
}

// ComputeUpcomingDueDate returns the next upcoming due date if it falls
// within `daysBefore` days from `now` (both dates inclusive). The returned
// date has zeroed time-of-day so the reference month is unambiguous. The
// ok=false return signals "outside window" so the caller can skip cheaply.
//
// billingDay > days-in-month is clamped to the last day of the month, which
// matches how calendars handle "bill on the 31st" for February.
//
// enrolledAt enforces the grace window: if the computed due date is less
// than MinDaysBeforeFirstDueDate days after the patient joined, we roll
// forward a month at a time until it isn't. Passing the zero time disables
// the grace window (used by legacy callers and unit tests focused on the
// billing_day math alone).
func ComputeUpcomingDueDate(now time.Time, billingDay, daysBefore int, enrolledAt time.Time) (time.Time, bool) {
	if billingDay < 1 || billingDay > 31 {
		return time.Time{}, false
	}
	if daysBefore < 0 {
		return time.Time{}, false
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Candidate 1: current month's billing day (possibly already past).
	curDay := clampDay(today.Year(), today.Month(), billingDay)
	due := time.Date(today.Year(), today.Month(), curDay, 0, 0, 0, 0, today.Location())

	// Roll forward until the due date is today or in the future. A single
	// AddDate(0,1,0) is enough for the normal "this month already past" case,
	// but we keep it as a loop so the grace-window check below can reuse the
	// same mechanism.
	for due.Before(today) {
		due = rollOneMonth(due, billingDay)
	}

	// Grace window: the first due date must be at least MinDaysBeforeFirstDueDate
	// days after enrollment. Without this, a patient added on day 12 with
	// billing_day=15 would get charged 3 days later.
	if !enrolledAt.IsZero() {
		minDue := time.Date(enrolledAt.Year(), enrolledAt.Month(), enrolledAt.Day(), 0, 0, 0, 0, today.Location()).
			AddDate(0, 0, MinDaysBeforeFirstDueDate)
		for due.Before(minDue) {
			due = rollOneMonth(due, billingDay)
		}
	}

	// Distance in whole days from today to the due date. Negative means past,
	// which the logic above already prevents.
	days := int(due.Sub(today).Hours() / 24)
	if days < 0 || days > daysBefore {
		return time.Time{}, false
	}
	return due, true
}

// rollOneMonth advances `from` to the next month's billing day, re-clamping
// for short months (Jan 31 + 1 month → Feb 28).
func rollOneMonth(from time.Time, billingDay int) time.Time {
	next := from.AddDate(0, 1, 0)
	day := clampDay(next.Year(), next.Month(), billingDay)
	return time.Date(next.Year(), next.Month(), day, 0, 0, 0, 0, from.Location())
}

func clampDay(year int, month time.Month, day int) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		return last
	}
	return day
}

func buildWhatsAppMessage(patientName, planTitle string, dueDate time.Time, amount float64) string {
	dateStr := dueDate.Format("02/01/2006")
	if amount > 0 {
		return fmt.Sprintf(
			"Olá, %s! 💳\n\n"+
				"Lembrete da mensalidade do plano *%s*.\n\n"+
				"📆 Vencimento: %s\n"+
				"💰 Valor: R$ %.2f\n\n"+
				"Qualquer dúvida é só responder esta mensagem.",
			patientName, planTitle, dateStr, amount,
		)
	}
	return fmt.Sprintf(
		"Olá, %s! 💳\n\n"+
			"Lembrete da mensalidade do plano *%s*.\n\n"+
			"📆 Vencimento: %s\n\n"+
			"Qualquer dúvida é só responder esta mensagem.",
		patientName, planTitle, dateStr,
	)
}
