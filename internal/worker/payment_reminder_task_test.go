package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/internal/config"
	"github.com/aseno/backend/internal/queue"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
)

// --- Mocks ---

type mockClinicSource struct {
	clinics []*clinic_model.Clinic
	err     error
}

func (m *mockClinicSource) ListActiveClinics(_ context.Context) ([]*clinic_model.Clinic, error) {
	return m.clinics, m.err
}

type mockCandidateSource struct {
	byClinic map[uuid.UUID][]*PaymentReminderCandidate
	err      error
}

func (m *mockCandidateSource) ListPaymentReminderCandidates(_ context.Context, clinicID uuid.UUID) ([]*PaymentReminderCandidate, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.byClinic[clinicID], nil
}

type mockTracker struct {
	paid       map[string]bool // key: plan|enrollment|month
	reminders  map[string]bool // key: plan|enrollment|month|channel
	recorded   []*sessionplan_model.SessionPlanPaymentReminder
	paidErr    error
	existsErr  error
	recordErr  error
	recordCall int
}

func trackerKey(plan uuid.UUID, enrollment *uuid.UUID, month string) string {
	e := "nil"
	if enrollment != nil {
		e = enrollment.String()
	}
	return plan.String() + "|" + e + "|" + month
}

func channelKey(plan uuid.UUID, enrollment *uuid.UUID, month string, ch sessionplan_model.PaymentReminderChannel) string {
	return trackerKey(plan, enrollment, month) + "|" + string(ch)
}

func (m *mockTracker) PaymentExists(_ context.Context, _, planID uuid.UUID, enrollmentID *uuid.UUID, month string) (bool, error) {
	if m.paidErr != nil {
		return false, m.paidErr
	}
	return m.paid[trackerKey(planID, enrollmentID, month)], nil
}

func (m *mockTracker) ReminderExists(_ context.Context, _, planID uuid.UUID, enrollmentID *uuid.UUID, month string, ch sessionplan_model.PaymentReminderChannel) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	return m.reminders[channelKey(planID, enrollmentID, month, ch)], nil
}

func (m *mockTracker) RecordReminder(_ context.Context, r *sessionplan_model.SessionPlanPaymentReminder) error {
	m.recordCall++
	if m.recordErr != nil {
		return m.recordErr
	}
	m.recorded = append(m.recorded, r)
	// Mirror the insert into the map so a second attempt in the same run sees it.
	if m.reminders == nil {
		m.reminders = map[string]bool{}
	}
	m.reminders[channelKey(r.SessionPlanID, r.EnrollmentID, r.ReferenceMonth, r.Channel)] = true
	return nil
}

type mockQueue struct {
	published []*queue.MessageJob
	err       error
}

func (m *mockQueue) Publish(_ context.Context, j *queue.MessageJob) error {
	if m.err != nil {
		return m.err
	}
	m.published = append(m.published, j)
	return nil
}

func (m *mockQueue) QueueLength(_ context.Context) (int, error) { return len(m.published), nil }

type mockEmailSender struct {
	sent []*PaymentReminderEmailData
	err  error
}

func (m *mockEmailSender) SendPaymentReminder(_ context.Context, d *PaymentReminderEmailData) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, d)
	return nil
}

// --- Pure logic tests: ComputeUpcomingDueDate ---

func TestComputeUpcomingDueDate_TodayIsBillingDay(t *testing.T) {
	// Today is the 15th, billing day is 15, daysBefore=3.
	// Due = today → days=0, within window → ok.
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	due, ok := ComputeUpcomingDueDate(now, 15, 3, time.Time{})
	require.True(t, ok)
	assert.Equal(t, "2026-05-15", due.Format("2006-01-02"))
}

func TestComputeUpcomingDueDate_WithinWindow(t *testing.T) {
	// Today = May 12, billing day = 15, daysBefore = 5 → due May 15, 3 days away → in window.
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	due, ok := ComputeUpcomingDueDate(now, 15, 5, time.Time{})
	require.True(t, ok)
	assert.Equal(t, "2026-05-15", due.Format("2006-01-02"))
}

func TestComputeUpcomingDueDate_OutsideWindow(t *testing.T) {
	// Today = May 1, billing day = 15, daysBefore = 3 → due May 15, 14 days away → out of window.
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	_, ok := ComputeUpcomingDueDate(now, 15, 3, time.Time{})
	assert.False(t, ok)
}

func TestComputeUpcomingDueDate_RollsToNextMonth(t *testing.T) {
	// Today = May 20, billing day = 15 (already past in May) → due June 15.
	// daysBefore of 30 pushes it inside the window.
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	due, ok := ComputeUpcomingDueDate(now, 15, 30, time.Time{})
	require.True(t, ok)
	assert.Equal(t, "2026-06-15", due.Format("2006-01-02"))
}

func TestComputeUpcomingDueDate_ClampsToShortMonth(t *testing.T) {
	// Billing day = 31, today = Feb 26, 2026 → Feb has 28 days → clamp to Feb 28.
	now := time.Date(2026, 2, 26, 10, 0, 0, 0, time.UTC)
	due, ok := ComputeUpcomingDueDate(now, 31, 5, time.Time{})
	require.True(t, ok)
	assert.Equal(t, "2026-02-28", due.Format("2006-01-02"))
}

func TestComputeUpcomingDueDate_InvalidBillingDay(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	_, ok := ComputeUpcomingDueDate(now, 0, 3, time.Time{})
	assert.False(t, ok)
	_, ok = ComputeUpcomingDueDate(now, 32, 3, time.Time{})
	assert.False(t, ok)
}

func TestComputeUpcomingDueDate_GraceWindow_SkipsSameMonth(t *testing.T) {
	// Enrolled today (May 12), billing day = 15, daysBefore = 3.
	// Without the grace window this would fire in 3 days.
	// With 30-day grace: first eligible due date = June 11 or later.
	// Next billing day after grace = June 15 → 34 days from today → outside window.
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	enrolledAt := now
	_, ok := ComputeUpcomingDueDate(now, 15, 3, enrolledAt)
	assert.False(t, ok, "grace window should push first due date out of reminder range")
}

func TestComputeUpcomingDueDate_GraceWindow_FirstDueAfter30Days(t *testing.T) {
	// Enrolled on May 12 — first due date must be ≥ June 11. Billing day = 15
	// → June 15 qualifies. Today = June 12, daysBefore = 3 → 3 days out → in window.
	enrolledAt := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	due, ok := ComputeUpcomingDueDate(now, 15, 3, enrolledAt)
	require.True(t, ok)
	assert.Equal(t, "2026-06-15", due.Format("2006-01-02"))
}

func TestComputeUpcomingDueDate_GraceWindow_RollsTwoMonths(t *testing.T) {
	// Enrolled on May 1, billing day = 10. May 10 is 9 days in (<30) → skip.
	// June 10 is 40 days in (≥30) → first eligible. Today = June 7, daysBefore = 3.
	enrolledAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	due, ok := ComputeUpcomingDueDate(now, 10, 3, enrolledAt)
	require.True(t, ok)
	assert.Equal(t, "2026-06-10", due.Format("2006-01-02"))
}

func TestComputeUpcomingDueDate_GraceWindow_OldEnrollmentUnaffected(t *testing.T) {
	// Patient enrolled a year ago — grace window never trips. Normal billing applies.
	enrolledAt := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	due, ok := ComputeUpcomingDueDate(now, 15, 5, enrolledAt)
	require.True(t, ok)
	assert.Equal(t, "2026-05-15", due.Format("2006-01-02"))
}

// --- Integration tests: Run() end-to-end with mocks ---

func activeClinic(_enabled, whatsappReminders, whatsappPR, emailPR bool, daysBefore int) *clinic_model.Clinic {
	_ = _enabled
	return &clinic_model.Clinic{
		ID:   uuid.New(),
		Name: "Clinica X",
		Settings: clinic_model.ClinicSettings{
			WhatsAppRemindersEnabled:       whatsappReminders,
			PaymentReminderWhatsAppEnabled: &whatsappPR,
			PaymentReminderEmailEnabled:    &emailPR,
			PaymentReminderDaysBefore:      &daysBefore,
		},
	}
}

func candidateNear(clinicID uuid.UUID, now time.Time, billingDay int) *PaymentReminderCandidate {
	// Default to an old enrollment so the grace window doesn't interfere
	// with the "near-due" scenarios these tests exercise. Tests that want
	// to exercise the grace window set EnrolledAt explicitly.
	return &PaymentReminderCandidate{
		ClinicID:         clinicID,
		SessionPlanID:    uuid.New(),
		EnrollmentID:     nil,
		PatientID:        uuid.New(),
		PatientName:      "Paciente",
		PatientWhatsApp:  "5511999999999",
		PatientEmail:     "paciente@example.com",
		PlanTitle:        "Fisio",
		BillingDay:       billingDay,
		Amount:           350.0,
		WhatsAppInstance: "instance-1",
		EnrolledAt:       now.AddDate(0, -6, 0),
	}
}

func baseCfg() *config.WorkerConfig {
	return &config.WorkerConfig{
		PaymentReminderWhatsAppEnabled: true,
		PaymentReminderEmailEnabled:    true,
		MessageRetryMax:                3,
	}
}

func TestRun_SendsBothChannels_WhenInWindow(t *testing.T) {
	clinic := activeClinic(true, true, true, true, 3)
	cand := candidateNear(clinic.ID, time.Now(), time.Now().Day()) // due today

	tracker := &mockTracker{}
	q := &mockQueue{}
	email := &mockEmailSender{}

	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		email,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)

	assert.Len(t, q.published, 1, "should enqueue one WhatsApp message")
	assert.Len(t, email.sent, 1, "should send one email")
	assert.Len(t, tracker.recorded, 2, "should record one reminder row per channel")
}

func TestRun_Idempotent_DoesNotDuplicateReminder(t *testing.T) {
	clinic := activeClinic(true, true, true, false, 3)
	cand := candidateNear(clinic.ID, time.Now(), time.Now().Day())

	// Simulate that whatsapp reminder already exists for this month.
	referenceMonth := time.Now().Format("2006-01")
	tracker := &mockTracker{
		reminders: map[string]bool{
			channelKey(cand.SessionPlanID, cand.EnrollmentID, referenceMonth, sessionplan_model.PaymentReminderChannelWhatsApp): true,
		},
	}
	q := &mockQueue{}
	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		nil,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, q.published, "must not enqueue when reminder already recorded")
	assert.Empty(t, tracker.recorded, "must not record a duplicate")
}

func TestRun_SkipsWhenPaymentAlreadyRegistered(t *testing.T) {
	clinic := activeClinic(true, true, true, true, 3)
	cand := candidateNear(clinic.ID, time.Now(), time.Now().Day())

	referenceMonth := time.Now().Format("2006-01")
	tracker := &mockTracker{
		paid: map[string]bool{
			trackerKey(cand.SessionPlanID, cand.EnrollmentID, referenceMonth): true,
		},
	}
	q := &mockQueue{}
	email := &mockEmailSender{}

	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		email,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, q.published)
	assert.Empty(t, email.sent)
}

func TestRun_SkipsWhenMasterReminderSwitchOff(t *testing.T) {
	clinic := activeClinic(true, false /* master off */, true, true, 3)
	cand := candidateNear(clinic.ID, time.Now(), time.Now().Day())

	tracker := &mockTracker{}
	q := &mockQueue{}
	email := &mockEmailSender{}

	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		email,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, q.published)
	assert.Empty(t, email.sent)
}

func TestRun_KillSwitchDisablesBothChannels(t *testing.T) {
	clinic := activeClinic(true, true, true, true, 3)
	cand := candidateNear(clinic.ID, time.Now(), time.Now().Day())

	cfg := baseCfg()
	cfg.PaymentReminderWhatsAppEnabled = false
	cfg.PaymentReminderEmailEnabled = false

	tracker := &mockTracker{}
	q := &mockQueue{}
	email := &mockEmailSender{}

	task := NewPaymentReminderTask(
		cfg,
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		email,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, q.published)
	assert.Empty(t, email.sent)
}

func TestRun_OnlyWhatsAppChannelOn(t *testing.T) {
	// Per-clinic email disabled, env kill switch on whatsapp only.
	clinic := activeClinic(true, true, true, false, 3)
	cand := candidateNear(clinic.ID, time.Now(), time.Now().Day())

	tracker := &mockTracker{}
	q := &mockQueue{}
	email := &mockEmailSender{}

	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		email,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Len(t, q.published, 1)
	assert.Empty(t, email.sent)
}

func TestRun_SkipsCandidateWithNoContact(t *testing.T) {
	clinic := activeClinic(true, true, true, true, 3)
	cand := candidateNear(clinic.ID, time.Now(), time.Now().Day())
	cand.PatientWhatsApp = ""
	cand.PatientEmail = ""

	tracker := &mockTracker{}
	q := &mockQueue{}
	email := &mockEmailSender{}

	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		email,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, q.published)
	assert.Empty(t, email.sent)
	assert.Empty(t, tracker.recorded)
}

func TestRun_SkipsWhenOutOfWindow(t *testing.T) {
	clinic := activeClinic(true, true, true, true, 1) // only 1 day before
	// Force due date to be 10 days away by picking billing_day = today+10 (clamped)
	day := time.Now().AddDate(0, 0, 10).Day()
	cand := candidateNear(clinic.ID, time.Now(), day)

	tracker := &mockTracker{}
	q := &mockQueue{}
	email := &mockEmailSender{}

	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		email,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, q.published)
	assert.Empty(t, email.sent)
}

func TestRun_InactiveSubscription_Skips(t *testing.T) {
	clinic := activeClinic(true, true, true, true, 3)
	cand := candidateNear(clinic.ID, time.Now(), time.Now().Day())

	tracker := &mockTracker{}
	q := &mockQueue{}
	email := &mockEmailSender{}

	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		email,
	)
	task.SetSubscriptionChecker(&fixedSubChecker{active: false})

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, q.published)
	assert.Empty(t, email.sent)
}

type fixedSubChecker struct {
	active bool
	err    error
}

func (f *fixedSubChecker) IsSubscriptionActive(_ context.Context, _ uuid.UUID) (bool, error) {
	return f.active, f.err
}

func TestRun_GraceWindow_SkipsRecentEnrollment(t *testing.T) {
	// Patient enrolled today with billing_day set to tomorrow. Without the
	// grace window this would fire immediately; with it, the first due date
	// rolls to next month and falls outside the reminder window.
	clinic := activeClinic(true, true, true, true, 3)
	cand := candidateNear(clinic.ID, time.Now(), (time.Now().Day()%28)+1)
	cand.EnrolledAt = time.Now() // just enrolled

	tracker := &mockTracker{}
	q := &mockQueue{}
	email := &mockEmailSender{}

	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{clinic}},
		&mockCandidateSource{byClinic: map[uuid.UUID][]*PaymentReminderCandidate{clinic.ID: {cand}}},
		tracker,
		q,
		email,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, q.published, "must not remind a patient enrolled today")
	assert.Empty(t, email.sent)
}

func TestRun_ListCandidatesError_DoesNotAbortOtherClinics(t *testing.T) {
	cleanClinic := activeClinic(true, true, true, false, 3)
	brokenClinic := activeClinic(true, true, true, false, 3)
	cand := candidateNear(cleanClinic.ID, time.Now(), time.Now().Day())

	tracker := &mockTracker{}
	q := &mockQueue{}

	task := NewPaymentReminderTask(
		baseCfg(),
		&mockClinicSource{clinics: []*clinic_model.Clinic{brokenClinic, cleanClinic}},
		&flakyCandidateSource{
			failFor: brokenClinic.ID,
			ok:      map[uuid.UUID][]*PaymentReminderCandidate{cleanClinic.ID: {cand}},
		},
		tracker,
		q,
		nil,
	)

	err := task.Run(context.Background())
	require.NoError(t, err)
	assert.Len(t, q.published, 1, "clean clinic should still fire despite broken one")
}

type flakyCandidateSource struct {
	failFor uuid.UUID
	ok      map[uuid.UUID][]*PaymentReminderCandidate
}

func (f *flakyCandidateSource) ListPaymentReminderCandidates(_ context.Context, clinicID uuid.UUID) ([]*PaymentReminderCandidate, error) {
	if clinicID == f.failFor {
		return nil, errors.New("boom")
	}
	return f.ok[clinicID], nil
}
