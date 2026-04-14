package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/config"
	"github.com/aseno/backend/internal/queue"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
)

type stubAppointmentRepo struct {
	pending             []*appointment_model.AppointmentWithClinic
	markedReminders     []uuid.UUID
	confirmationSetFor  []uuid.UUID
}

func (r *stubAppointmentRepo) GetPendingReminders(_ context.Context, _ string, _ time.Time, _ time.Time) ([]*appointment_model.AppointmentWithClinic, error) {
	return r.pending, nil
}

func (r *stubAppointmentRepo) MarkReminderSent(_ context.Context, _ uuid.UUID, id uuid.UUID, _ string) error {
	r.markedReminders = append(r.markedReminders, id)
	return nil
}

func (r *stubAppointmentRepo) SetConfirmationExpiresAt(_ context.Context, _ uuid.UUID, id uuid.UUID, _ time.Time) error {
	r.confirmationSetFor = append(r.confirmationSetFor, id)
	return nil
}

type stubSubscriptionChecker struct {
	activeByClinic map[uuid.UUID]bool
	calls          int
}

func (s *stubSubscriptionChecker) IsSubscriptionActive(_ context.Context, clinicID uuid.UUID) (bool, error) {
	s.calls++
	active, ok := s.activeByClinic[clinicID]
	if !ok {
		return true, nil
	}
	return active, nil
}

type stubQueue struct {
	published []*queue.MessageJob
}

func (q *stubQueue) Publish(_ context.Context, job *queue.MessageJob) error {
	q.published = append(q.published, job)
	return nil
}

func (q *stubQueue) Consume(_ context.Context, _ func(*queue.MessageJob) error) error {
	return nil
}

func (q *stubQueue) Connect(_ context.Context) error { return nil }

func (q *stubQueue) Close() error { return nil }

func (q *stubQueue) QueueLength(_ context.Context) (int, error) { return 0, nil }

func buildAppointment(clinicID uuid.UUID, startsIn time.Duration) *appointment_model.AppointmentWithClinic {
	apt := &appointment_model.AppointmentWithClinic{}
	apt.ID = uuid.New()
	apt.ClinicID = clinicID
	apt.StartTime = time.Now().Add(startsIn)
	apt.Patient = &appointment_model.PatientInfo{Name: "Maria", Phone: "+5511988887777"}
	apt.WhatsAppInstance = "instance-1"
	return apt
}

func newReminderConfig() *config.WorkerConfig {
	return &config.WorkerConfig{
		MessageRetryMax:             3,
		ConfirmationTimeoutDuration: 24 * time.Hour,
	}
}

func TestReminderWorker_InactiveSubscription_DoesNotEnqueue(t *testing.T) {
	clinicID := uuid.New()
	repo := &stubAppointmentRepo{
		pending: []*appointment_model.AppointmentWithClinic{buildAppointment(clinicID, 24*time.Hour)},
	}
	q := &stubQueue{}
	checker := &stubSubscriptionChecker{activeByClinic: map[uuid.UUID]bool{clinicID: false}}

	w := NewReminderWorker(newReminderConfig(), repo, q)
	w.SetSubscriptionChecker(checker)

	count, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name:     "reminder_24h",
		Type:     "reminder_24h",
		Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 enqueued, got %d", count)
	}
	if len(q.published) != 0 {
		t.Errorf("expected no published messages, got %d", len(q.published))
	}
	if len(repo.markedReminders) != 0 {
		t.Errorf("reminder should not be marked sent for inactive subscription")
	}
}

func TestReminderWorker_ActiveSubscription_Enqueues(t *testing.T) {
	clinicID := uuid.New()
	repo := &stubAppointmentRepo{
		pending: []*appointment_model.AppointmentWithClinic{buildAppointment(clinicID, 24*time.Hour)},
	}
	q := &stubQueue{}
	checker := &stubSubscriptionChecker{activeByClinic: map[uuid.UUID]bool{clinicID: true}}

	w := NewReminderWorker(newReminderConfig(), repo, q)
	w.SetSubscriptionChecker(checker)

	count, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name:     "reminder_24h",
		Type:     "reminder_24h",
		Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 enqueued, got %d", count)
	}
	if len(q.published) != 1 {
		t.Errorf("expected 1 published message, got %d", len(q.published))
	}
}

func TestReminderWorker_NoChecker_Enqueues(t *testing.T) {
	clinicID := uuid.New()
	repo := &stubAppointmentRepo{
		pending: []*appointment_model.AppointmentWithClinic{buildAppointment(clinicID, 24*time.Hour)},
	}
	q := &stubQueue{}

	w := NewReminderWorker(newReminderConfig(), repo, q)
	// No SetSubscriptionChecker — backward compat path.

	count, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name:     "reminder_24h",
		Type:     "reminder_24h",
		Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 enqueued when no checker wired, got %d", count)
	}
}

func TestReminderWorker_SubscriptionCached_WithinRun(t *testing.T) {
	clinicID := uuid.New()
	apt1 := buildAppointment(clinicID, 24*time.Hour)
	apt2 := buildAppointment(clinicID, 24*time.Hour)
	repo := &stubAppointmentRepo{
		pending: []*appointment_model.AppointmentWithClinic{apt1, apt2},
	}
	q := &stubQueue{}
	checker := &stubSubscriptionChecker{activeByClinic: map[uuid.UUID]bool{clinicID: true}}

	w := NewReminderWorker(newReminderConfig(), repo, q)
	w.SetSubscriptionChecker(checker)

	cache := map[uuid.UUID]bool{}
	_, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name:     "reminder_24h",
		Type:     "reminder_24h",
		Duration: 24 * time.Hour,
	}, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checker.calls != 1 {
		t.Errorf("expected subscription checker to be called once (cached), got %d", checker.calls)
	}
}
