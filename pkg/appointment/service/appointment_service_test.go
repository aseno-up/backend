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
	appointment               *model.Appointment
	hasConflict               bool
	conflictErr               error
	updateCalled              bool
	hardDeleteCalled          bool
	tombstoneCalled           bool
	tombstoneDeletePaymentArg bool
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
func (m *mockAppointmentRepo) DeleteCascade(_ context.Context, _, _ uuid.UUID, _ bool) error {
	m.hardDeleteCalled = true
	return nil
}
func (m *mockAppointmentRepo) SoftDeleteAsCanceledException(_ context.Context, _, _ uuid.UUID, deletePayments bool) error {
	m.tombstoneCalled = true
	m.tombstoneDeletePaymentArg = deletePayments
	return nil
}
func (m *mockAppointmentRepo) GetDeletePreview(_ context.Context, _, _ uuid.UUID) (*model.AppointmentDeletePreview, error) {
	return &model.AppointmentDeletePreview{}, nil
}
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
func (m *mockAppointmentRepo) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, _ model.PaymentStatus) error {
	return nil
}
func (m *mockAppointmentRepo) GetByDateRangeWithExceptions(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) FindPlanAppointmentOnDate(_ context.Context, _, _ uuid.UUID, _ time.Time) (*model.Appointment, error) {
	return nil, nil
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

// --- Mock SessionPlanCompletionHandler ---

type mockSessionPlanHandler struct {
	called bool
}

func (m *mockSessionPlanHandler) OnAppointmentCompleted(_ context.Context, _, _ uuid.UUID) error {
	m.called = true
	return nil
}

// Regression: deleting an appointment that belongs to a session plan must leave
// a suppression tombstone instead of hard-deleting. Hard delete removes the
// record entirely and the CalendarFacade re-materializes the virtual occurrence
// at the same slot on the next fetch — making the appointment appear to "come
// back" after the user asked to delete it.
func TestDelete_SessionPlanAppointmentSoftDeletesAsTombstone(t *testing.T) {
	planID := uuid.New()
	apt := newAppointment(model.AppointmentStatusPending)
	apt.SessionPlanID = &planID

	repo := &mockAppointmentRepo{appointment: apt}
	svc := buildService(repo)

	err := svc.Delete(context.Background(), apt.ID, apt.ClinicID, true)
	require.NoError(t, err)

	assert.True(t, repo.tombstoneCalled, "Delete on a session-plan appointment must call SoftDeleteAsCanceledException so the virtual occurrence stays suppressed")
	assert.False(t, repo.hardDeleteCalled, "Delete on a session-plan appointment must NOT hard-delete; that is what makes the virtual re-appear")
	assert.True(t, repo.tombstoneDeletePaymentArg, "deletePayments flag must propagate so the caller can still clear linked payments")
}

// A plain (non-plan) appointment has nothing to suppress — the delete path
// must keep hard-deleting so we don't leave ghost cancelled rows behind.
func TestDelete_RegularAppointmentStillHardDeletes(t *testing.T) {
	apt := newAppointment(model.AppointmentStatusConfirmed)
	apt.SessionPlanID = nil

	repo := &mockAppointmentRepo{appointment: apt}
	svc := buildService(repo)

	err := svc.Delete(context.Background(), apt.ID, apt.ClinicID, false)
	require.NoError(t, err)

	assert.True(t, repo.hardDeleteCalled, "Regular appointments must still be hard-deleted")
	assert.False(t, repo.tombstoneCalled, "Regular appointments have no RRULE to suppress, so no tombstone is needed")
}

// --- Mock PaymentLister + FinanceUpserter --------------------------------

type mockPaymentLister struct {
	payments []*model.AppointmentPayment
}

func (m *mockPaymentLister) ListByAppointment(appointmentID, _ uuid.UUID) ([]*model.AppointmentPayment, error) {
	out := make([]*model.AppointmentPayment, 0, len(m.payments))
	for _, p := range m.payments {
		if p.AppointmentID == appointmentID {
			out = append(out, p)
		}
	}
	return out, nil
}

type mockDeleteFinanceUpserter struct {
	deletedPaymentIDs []uuid.UUID
}

func (m *mockDeleteFinanceUpserter) UpsertFromAppointmentPayment(_ context.Context, _ *model.Appointment, _ *model.AppointmentPayment, _ uuid.UUID) error {
	return nil
}

func (m *mockDeleteFinanceUpserter) DeleteFromAppointmentPayment(_ context.Context, appointmentPaymentID, _ uuid.UUID) error {
	m.deletedPaymentIDs = append(m.deletedPaymentIDs, appointmentPaymentID)
	return nil
}

// Regression: deleting an appointment that has payments, with deletePayments=true,
// must also remove the corresponding rows from financial_transactions. Otherwise
// the Lançamentos tab keeps showing income entries for appointments that no
// longer exist.
func TestDelete_RegularAppointment_CleansFinanceWhenPaymentsDeleted(t *testing.T) {
	apt := newAppointment(model.AppointmentStatusConfirmed)
	apt.SessionPlanID = nil

	pay1 := &model.AppointmentPayment{ID: uuid.New(), ClinicID: apt.ClinicID, AppointmentID: apt.ID, Amount: 100}
	pay2 := &model.AppointmentPayment{ID: uuid.New(), ClinicID: apt.ClinicID, AppointmentID: apt.ID, Amount: 50}

	repo := &mockAppointmentRepo{appointment: apt}
	paymentLister := &mockPaymentLister{payments: []*model.AppointmentPayment{pay1, pay2}}
	finance := &mockDeleteFinanceUpserter{}

	svc := buildService(repo)
	svc.SetPaymentLister(paymentLister)
	svc.SetFinanceUpserter(finance)

	err := svc.Delete(context.Background(), apt.ID, apt.ClinicID, true)
	require.NoError(t, err)

	assert.True(t, repo.hardDeleteCalled)
	assert.ElementsMatch(t,
		[]uuid.UUID{pay1.ID, pay2.ID},
		finance.deletedPaymentIDs,
		"finance rows for every payment under the appointment must be cleaned up",
	)
}

// deletePayments=false preserves the payments as financial history per the
// existing repository contract — the finance rows must stay too.
func TestDelete_RegularAppointment_KeepsFinanceWhenPaymentsKept(t *testing.T) {
	apt := newAppointment(model.AppointmentStatusConfirmed)
	apt.SessionPlanID = nil

	pay := &model.AppointmentPayment{ID: uuid.New(), ClinicID: apt.ClinicID, AppointmentID: apt.ID, Amount: 100}

	repo := &mockAppointmentRepo{appointment: apt}
	paymentLister := &mockPaymentLister{payments: []*model.AppointmentPayment{pay}}
	finance := &mockDeleteFinanceUpserter{}

	svc := buildService(repo)
	svc.SetPaymentLister(paymentLister)
	svc.SetFinanceUpserter(finance)

	err := svc.Delete(context.Background(), apt.ID, apt.ClinicID, false)
	require.NoError(t, err)

	assert.Empty(t, finance.deletedPaymentIDs,
		"when the caller opts to keep payments, finance rows must be preserved too")
}

// Session-plan path uses the tombstone delete — same cleanup rule must apply.
func TestDelete_SessionPlanAppointment_CleansFinanceWhenPaymentsDeleted(t *testing.T) {
	planID := uuid.New()
	apt := newAppointment(model.AppointmentStatusPending)
	apt.SessionPlanID = &planID

	pay := &model.AppointmentPayment{ID: uuid.New(), ClinicID: apt.ClinicID, AppointmentID: apt.ID, Amount: 200}

	repo := &mockAppointmentRepo{appointment: apt}
	paymentLister := &mockPaymentLister{payments: []*model.AppointmentPayment{pay}}
	finance := &mockDeleteFinanceUpserter{}

	svc := buildService(repo)
	svc.SetPaymentLister(paymentLister)
	svc.SetFinanceUpserter(finance)

	err := svc.Delete(context.Background(), apt.ID, apt.ClinicID, true)
	require.NoError(t, err)

	assert.True(t, repo.tombstoneCalled)
	assert.Equal(t, []uuid.UUID{pay.ID}, finance.deletedPaymentIDs)
}

// Cenário 5: Complete NÃO deve incrementar sessions_done diretamente —
// o handler do endpoint já chama AutoRecordPresent que faz o incremento via attendance.
// Se Complete também incrementar, sessions_done é contado duas vezes (bug de contagem dupla).
func TestComplete_ShouldNotIncrementSessionsDone(t *testing.T) {
	planID := uuid.New()
	apt := newAppointment(model.AppointmentStatusConfirmed)
	apt.SessionPlanID = &planID

	repo := &mockAppointmentRepo{appointment: apt}
	handler := &mockSessionPlanHandler{}

	svc := buildService(repo)
	svc.SetSessionPlanHandler(handler)

	_, err := svc.Complete(context.Background(), apt.ID, apt.ClinicID)

	require.NoError(t, err)
	assert.False(t, handler.called,
		"Complete() must NOT call OnAppointmentCompleted — attendance flow handles sessions_done increment. "+
			"If this fails, sessions_done will be double-counted for plan appointments.")
}
