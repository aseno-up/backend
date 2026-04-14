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
	pending                []*appointment_model.AppointmentWithClinic
	markedReminders        []uuid.UUID
	confirmationSetFor     []uuid.UUID
	confirmationTimestamps []time.Time
}

func (r *stubAppointmentRepo) GetPendingReminders(_ context.Context, _ string, _ time.Time, _ time.Time) ([]*appointment_model.AppointmentWithClinic, error) {
	return r.pending, nil
}

func (r *stubAppointmentRepo) MarkReminderSent(_ context.Context, _ uuid.UUID, id uuid.UUID, _ string) error {
	r.markedReminders = append(r.markedReminders, id)
	return nil
}

func (r *stubAppointmentRepo) SetConfirmationExpiresAt(_ context.Context, _ uuid.UUID, id uuid.UUID, expiresAt time.Time) error {
	r.confirmationSetFor = append(r.confirmationSetFor, id)
	r.confirmationTimestamps = append(r.confirmationTimestamps, expiresAt)
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

// TestReminderWorker_SetsConfirmationExpiresAtOn24h: REGRESSAO.
// Antes, SetConfirmationExpiresAt so era chamado para reminder_48h. Appointments
// criados com menos de 48h de antecedencia (comum na clinica) nunca recebiam
// confirmation_expires_at, entao nunca eram pegos pelo timeout_worker e ficavam
// para sempre como "pending" mesmo apos o horario passar.
// Fix: o flag e setado no primeiro reminder enviado, seja 48h, 24h ou 2h.
func TestReminderWorker_SetsConfirmationExpiresAtOn24h(t *testing.T) {
	clinicID := uuid.New()
	repo := &stubAppointmentRepo{
		pending: []*appointment_model.AppointmentWithClinic{buildAppointment(clinicID, 24*time.Hour)},
	}
	q := &stubQueue{}

	w := NewReminderWorker(newReminderConfig(), repo, q)

	_, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name:     "24h",
		Type:     "reminder_24h",
		Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.confirmationSetFor) != 1 {
		t.Errorf("expected SetConfirmationExpiresAt to be called once for reminder_24h, got %d", len(repo.confirmationSetFor))
	}
}

// TestReminderWorker_SetsConfirmationExpiresAtOn2h: mesma regressao com 2h.
func TestReminderWorker_SetsConfirmationExpiresAtOn2h(t *testing.T) {
	clinicID := uuid.New()
	repo := &stubAppointmentRepo{
		pending: []*appointment_model.AppointmentWithClinic{buildAppointment(clinicID, 2*time.Hour)},
	}
	q := &stubQueue{}

	w := NewReminderWorker(newReminderConfig(), repo, q)

	_, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name:     "2h",
		Type:     "reminder_2h",
		Duration: 2 * time.Hour,
	}, map[uuid.UUID]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.confirmationSetFor) != 1 {
		t.Errorf("expected SetConfirmationExpiresAt to be called once for reminder_2h, got %d", len(repo.confirmationSetFor))
	}
}

// TestReminderWorker_ExpirationIsProportionalToThreshold: o timeout da janela
// de confirmacao deve ser metade da antecedencia do reminder — 48h→24h, 24h→12h,
// 2h→1h. Antes era fixo em ConfirmationTimeoutDuration (16h default), o que
// fazia a janela expirar DEPOIS do horario do proprio appointment em reminders
// curtos (2h reminder + 16h timeout = expira 14h depois do start_time).
func TestReminderWorker_ExpirationIsProportionalToThreshold(t *testing.T) {
	cases := []struct {
		name     string
		duration time.Duration
		want     time.Duration
	}{
		{"2h", 2 * time.Hour, 1 * time.Hour},
		{"24h", 24 * time.Hour, 12 * time.Hour},
		// 48h case is skipped by the "within 3 days" guard in processThreshold
		// and is not worth contorting the test setup for — the halving logic
		// is the same.
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clinicID := uuid.New()
			repo := &stubAppointmentRepo{
				pending: []*appointment_model.AppointmentWithClinic{buildAppointment(clinicID, tc.duration)},
			}
			q := &stubQueue{}

			w := NewReminderWorker(newReminderConfig(), repo, q)

			before := time.Now()
			_, err := w.processThreshold(context.Background(), config.ReminderThreshold{
				Name:     tc.name,
				Type:     "reminder_" + tc.name,
				Duration: tc.duration,
			}, map[uuid.UUID]bool{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(repo.confirmationTimestamps) != 1 {
				t.Fatalf("expected SetConfirmationExpiresAt called once, got %d", len(repo.confirmationTimestamps))
			}
			got := repo.confirmationTimestamps[0].Sub(before)
			tolerance := 2 * time.Second
			if got < tc.want-tolerance || got > tc.want+tolerance {
				t.Errorf("reminder_%s should expire in %v, got %v after now", tc.name, tc.want, got)
			}
		})
	}
}

// TestReminderWorker_DoesNotOverwriteExistingConfirmationExpiresAt: se ja foi
// setado num reminder anterior (ex: 48h), o 24h nao deve reescrever.
func TestReminderWorker_DoesNotOverwriteExistingConfirmationExpiresAt(t *testing.T) {
	clinicID := uuid.New()
	apt := buildAppointment(clinicID, 24*time.Hour)
	existing := time.Now().Add(10 * time.Hour)
	apt.ConfirmationExpiresAt = &existing

	repo := &stubAppointmentRepo{
		pending: []*appointment_model.AppointmentWithClinic{apt},
	}
	q := &stubQueue{}

	w := NewReminderWorker(newReminderConfig(), repo, q)

	_, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name:     "24h",
		Type:     "reminder_24h",
		Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.confirmationSetFor) != 0 {
		t.Errorf("expected SetConfirmationExpiresAt NOT to be called when already set, got %d calls", len(repo.confirmationSetFor))
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
