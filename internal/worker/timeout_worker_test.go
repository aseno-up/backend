package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/config"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
)

type stubTimeoutRepo struct {
	appointment_repository.AppointmentRepository
	expired []*appointment_model.AppointmentWithClinic
	marked  []uuid.UUID
}

func (r *stubTimeoutRepo) GetExpiredConfirmations(_ context.Context, _ int) ([]*appointment_model.AppointmentWithClinic, error) {
	return r.expired, nil
}

func (r *stubTimeoutRepo) MarkConfirmationExpired(_ context.Context, _ uuid.UUID, id uuid.UUID) error {
	r.marked = append(r.marked, id)
	return nil
}

func buildExpiredAppointment(clinicID uuid.UUID) *appointment_model.AppointmentWithClinic {
	apt := &appointment_model.AppointmentWithClinic{}
	apt.ID = uuid.New()
	apt.ClinicID = clinicID
	apt.StartTime = time.Now().Add(24 * time.Hour)
	apt.Patient = &appointment_model.PatientInfo{Name: "Maria"}
	apt.ClinicName = "Clinica Teste"
	return apt
}

func TestTimeoutWorker_InactiveSubscription_SkipsAppointment(t *testing.T) {
	clinicID := uuid.New()
	repo := &stubTimeoutRepo{
		expired: []*appointment_model.AppointmentWithClinic{buildExpiredAppointment(clinicID)},
	}
	checker := &stubSubscriptionChecker{activeByClinic: map[uuid.UUID]bool{clinicID: false}}

	w := NewTimeoutWorker(&config.WorkerConfig{TimeoutWorkerEnabled: true}, repo)
	w.SetSubscriptionChecker(checker)

	w.processTimeouts()

	if len(repo.marked) != 0 {
		t.Errorf("should not mark appointments for inactive clinic, got %d marked", len(repo.marked))
	}
}

func TestTimeoutWorker_ActiveSubscription_MarksAppointment(t *testing.T) {
	clinicID := uuid.New()
	repo := &stubTimeoutRepo{
		expired: []*appointment_model.AppointmentWithClinic{buildExpiredAppointment(clinicID)},
	}
	checker := &stubSubscriptionChecker{activeByClinic: map[uuid.UUID]bool{clinicID: true}}

	w := NewTimeoutWorker(&config.WorkerConfig{TimeoutWorkerEnabled: true}, repo)
	w.SetSubscriptionChecker(checker)

	w.processTimeouts()

	if len(repo.marked) != 1 {
		t.Errorf("expected 1 appointment marked, got %d", len(repo.marked))
	}
}

func TestTimeoutWorker_CachesSubscriptionLookup(t *testing.T) {
	clinicID := uuid.New()
	repo := &stubTimeoutRepo{
		expired: []*appointment_model.AppointmentWithClinic{
			buildExpiredAppointment(clinicID),
			buildExpiredAppointment(clinicID),
			buildExpiredAppointment(clinicID),
		},
	}
	checker := &stubSubscriptionChecker{activeByClinic: map[uuid.UUID]bool{clinicID: true}}

	w := NewTimeoutWorker(&config.WorkerConfig{TimeoutWorkerEnabled: true}, repo)
	w.SetSubscriptionChecker(checker)

	w.processTimeouts()

	if checker.calls != 1 {
		t.Errorf("expected subscription check to be called once (cached), got %d", checker.calls)
	}
	if len(repo.marked) != 3 {
		t.Errorf("expected 3 appointments marked, got %d", len(repo.marked))
	}
}
