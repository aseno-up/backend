package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalerrors "github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/appointment/service"
)

// --- Minimal mock repository ---

type mockAppointmentRepo struct {
	appointment  *model.Appointment
	hasConflict  bool
	conflictErr  error
	updateCalled bool
}

func (m *mockAppointmentRepo) GetByID(_ context.Context, id, _ uuid.UUID) (*model.Appointment, error) {
	if m.appointment == nil || m.appointment.ID != id {
		return nil, internalerrors.ErrAppointmentNotFoundInstance
	}
	return m.appointment, nil
}

func (m *mockAppointmentRepo) CheckConflict(_ context.Context, _, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) (bool, error) {
	return m.hasConflict, m.conflictErr
}

func (m *mockAppointmentRepo) Update(_ context.Context, _ *model.Appointment) error {
	m.updateCalled = true
	return nil
}

// Remaining interface methods — not needed for these tests
func (m *mockAppointmentRepo) Create(_ context.Context, _ *model.Appointment) error {
	return nil
}
func (m *mockAppointmentRepo) List(_ context.Context, _ uuid.UUID, _ *model.AppointmentFilter) ([]*model.Appointment, int64, error) {
	return nil, 0, nil
}
func (m *mockAppointmentRepo) GetByDateRange(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) GetByDay(_ context.Context, _ uuid.UUID, _ time.Time, _ *uuid.UUID) ([]*model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockAppointmentRepo) GetByPatient(_ context.Context, _, _ uuid.UUID, _ int) ([]*model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ model.AppointmentStatus) error {
	return nil
}
func (m *mockAppointmentRepo) GetPendingReminders(_ context.Context, _ string, _, _ time.Time) ([]*model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) MarkReminderSent(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockAppointmentRepo) GetPendingByPhone(_ context.Context, _ uuid.UUID, _ string) (*model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) GetExpiredConfirmations(_ context.Context, _ int) ([]*model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) MarkConfirmationExpired(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *mockAppointmentRepo) SetConfirmationExpiresAt(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockAppointmentRepo) GetSlotPerformanceStats(_ context.Context, _, _ uuid.UUID) ([]*model.SlotPerformanceStats, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) CountByProfessionalID(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}

// --- Helpers ---

func newAppointment(status model.AppointmentStatus) *model.Appointment {
	now := time.Now().Add(24 * time.Hour)
	return &model.Appointment{
		ID:             uuid.New(),
		ClinicID:       uuid.New(),
		ProfessionalID: uuid.New(),
		PatientID:      uuid.New(),
		StartTime:      now,
		EndTime:        now.Add(60 * time.Minute),
		Status:         status,
		Type:           model.AppointmentTypeRegular,
	}
}

func buildService(repo *mockAppointmentRepo) service.AppointmentService {
	return service.NewAppointmentService(repo, nil)
}

// --- Tests ---

// Cenário 1: Confirmar agendamento PENDING não verifica conflito e deve ter sucesso.
func TestConfirm_PendingAppointment_NoConflictCheckNeeded(t *testing.T) {
	apt := newAppointment(model.AppointmentStatusPending)
	repo := &mockAppointmentRepo{appointment: apt, hasConflict: true} // conflict exists but shouldn't be checked
	svc := buildService(repo)

	_, err := svc.Confirm(context.Background(), apt.ID, apt.ClinicID, nil)

	require.NoError(t, err, "PENDING appointment should be confirmed without checking conflicts")
	assert.True(t, repo.updateCalled)
}

// Cenário 2: Reativar agendamento CANCELADO quando o horário está LIVRE — deve permitir.
func TestConfirm_CancelledAppointment_SlotFree_ShouldConfirm(t *testing.T) {
	apt := newAppointment(model.AppointmentStatusCanceled)
	repo := &mockAppointmentRepo{appointment: apt, hasConflict: false}
	svc := buildService(repo)

	_, err := svc.Confirm(context.Background(), apt.ID, apt.ClinicID, nil)

	require.NoError(t, err, "Reactivating a CANCELLED appointment with a free slot should succeed")
	assert.True(t, repo.updateCalled)
}

// Cenário 3: Reativar agendamento CANCELADO quando o horário já está OCUPADO — deve bloquear.
// Este é o teste de regressão para o bug de double-booking via reativação.
func TestConfirm_CancelledAppointment_SlotTaken_ShouldReturnConflictError(t *testing.T) {
	apt := newAppointment(model.AppointmentStatusCanceled)
	repo := &mockAppointmentRepo{appointment: apt, hasConflict: true}
	svc := buildService(repo)

	_, err := svc.Confirm(context.Background(), apt.ID, apt.ClinicID, nil)

	require.Error(t, err, "Reactivating a CANCELLED appointment with a taken slot should return error")
	assert.False(t, repo.updateCalled, "Update should NOT be called when slot is taken")
	assert.True(t, internalerrors.IsSlotUnavailable(err),
		"Error must be ErrSlotUnavailable, got: %v", err)
}

// Cenário 4: Confirmar agendamento já CONFIRMADO — deve ser idempotente e ter sucesso.
func TestConfirm_AlreadyConfirmed_ShouldBeIdempotent(t *testing.T) {
	apt := newAppointment(model.AppointmentStatusConfirmed)
	repo := &mockAppointmentRepo{appointment: apt, hasConflict: true} // conflict wouldn't matter
	svc := buildService(repo)

	_, err := svc.Confirm(context.Background(), apt.ID, apt.ClinicID, nil)

	require.NoError(t, err, "Confirming an already-confirmed appointment should be idempotent")
}
