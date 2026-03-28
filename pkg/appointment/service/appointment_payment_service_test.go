package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/appointment/service"
	pm_model "github.com/aseno/backend/pkg/paymentmethod/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock payment repository ---

type mockPaymentRepo struct {
	payments []*model.AppointmentPayment
	sum      float64
}

func (m *mockPaymentRepo) Create(payment *model.AppointmentPayment) error {
	payment.ID = uuid.New()
	m.payments = append(m.payments, payment)
	m.sum += payment.Amount + payment.Discount
	return nil
}

func (m *mockPaymentRepo) GetByID(id, clinicID uuid.UUID) (*model.AppointmentPayment, error) {
	for _, p := range m.payments {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, assert.AnError
}

func (m *mockPaymentRepo) Update(payment *model.AppointmentPayment) error {
	for i, p := range m.payments {
		if p.ID == payment.ID {
			m.sum -= (p.Amount + p.Discount)
			m.payments[i] = payment
			m.sum += (payment.Amount + payment.Discount)
			return nil
		}
	}
	return assert.AnError
}

func (m *mockPaymentRepo) Delete(id, clinicID uuid.UUID) error {
	for i, p := range m.payments {
		if p.ID == id {
			m.sum -= (p.Amount + p.Discount)
			m.payments = append(m.payments[:i], m.payments[i+1:]...)
			return nil
		}
	}
	return assert.AnError
}

func (m *mockPaymentRepo) ListByAppointment(appointmentID, clinicID uuid.UUID) ([]*model.AppointmentPayment, error) {
	var result []*model.AppointmentPayment
	for _, p := range m.payments {
		if p.AppointmentID == appointmentID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPaymentRepo) SumByAppointment(appointmentID uuid.UUID) (float64, error) {
	return m.sum, nil
}

// --- Mock appointment repo (reusing from existing test but simplified) ---

type mockAptRepoForPayment struct {
	appointment   *model.Appointment
	paymentStatus model.PaymentStatus
}

func (m *mockAptRepoForPayment) GetByID(_ context.Context, id, _ uuid.UUID) (*model.Appointment, error) {
	if m.appointment != nil && m.appointment.ID == id {
		return m.appointment, nil
	}
	return nil, assert.AnError
}

func (m *mockAptRepoForPayment) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, status model.PaymentStatus) error {
	m.paymentStatus = status
	return nil
}

// Unused interface methods
func (m *mockAptRepoForPayment) Create(_ context.Context, _ *model.Appointment) error { return nil }
func (m *mockAptRepoForPayment) List(_ context.Context, _ uuid.UUID, _ *model.AppointmentFilter) ([]*model.Appointment, int64, error) {
	return nil, 0, nil
}
func (m *mockAptRepoForPayment) GetByDateRange(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepoForPayment) GetByDay(_ context.Context, _ uuid.UUID, _ time.Time, _ *uuid.UUID) ([]*model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepoForPayment) Update(_ context.Context, _ *model.Appointment) error { return nil }
func (m *mockAptRepoForPayment) Delete(_ context.Context, _, _ uuid.UUID) error       { return nil }
func (m *mockAptRepoForPayment) CheckConflict(_ context.Context, _, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockAptRepoForPayment) GetByPatient(_ context.Context, _, _ uuid.UUID, _ int) ([]*model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepoForPayment) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ model.AppointmentStatus) error {
	return nil
}
func (m *mockAptRepoForPayment) GetPendingReminders(_ context.Context, _ string, _, _ time.Time) ([]*model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockAptRepoForPayment) MarkReminderSent(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockAptRepoForPayment) GetPendingByPhone(_ context.Context, _ uuid.UUID, _ string) (*model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepoForPayment) GetExpiredConfirmations(_ context.Context, _ int) ([]*model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockAptRepoForPayment) MarkConfirmationExpired(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *mockAptRepoForPayment) SetConfirmationExpiresAt(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockAptRepoForPayment) GetSlotPerformanceStats(_ context.Context, _, _ uuid.UUID) ([]*model.SlotPerformanceStats, error) {
	return nil, nil
}
func (m *mockAptRepoForPayment) CountByProfessionalID(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}

// --- Mock payment method repo ---

type mockPaymentMethodRepo struct {
	method *pm_model.PaymentMethod
}

func (m *mockPaymentMethodRepo) Create(_ *pm_model.PaymentMethod) error          { return nil }
func (m *mockPaymentMethodRepo) GetByID(_, _ uuid.UUID) (*pm_model.PaymentMethod, error) {
	if m.method != nil {
		return m.method, nil
	}
	return nil, assert.AnError
}
func (m *mockPaymentMethodRepo) Update(_ *pm_model.PaymentMethod) error          { return nil }
func (m *mockPaymentMethodRepo) Delete(_, _ uuid.UUID) error                     { return nil }
func (m *mockPaymentMethodRepo) ListByClinic(_ uuid.UUID) ([]*pm_model.PaymentMethod, error) {
	return nil, nil
}
func (m *mockPaymentMethodRepo) ListActiveByClinic(_ uuid.UUID) ([]*pm_model.PaymentMethod, error) {
	return nil, nil
}
func (m *mockPaymentMethodRepo) CountByClinic(_ uuid.UUID) (int64, error) { return 0, nil }
func (m *mockPaymentMethodRepo) CreateDefaults(_ uuid.UUID) error         { return nil }

var defaultCreditMethod = &pm_model.PaymentMethod{
	ID:     uuid.New(),
	Type:   pm_model.PaymentMethodTypeCredit,
	Name:   "Cartao de Credito",
	Active: true,
}

var defaultPixMethod = &pm_model.PaymentMethod{
	ID:     uuid.New(),
	Type:   pm_model.PaymentMethodTypePix,
	Name:   "PIX",
	Active: true,
}

func newPaymentService(paymentRepo *mockPaymentRepo, aptRepo *mockAptRepoForPayment, method *pm_model.PaymentMethod) service.AppointmentPaymentService {
	return service.NewAppointmentPaymentService(paymentRepo, aptRepo, &mockPaymentMethodRepo{method: method})
}

// --- Tests ---

func TestCreatePayment_UpdatesStatusToPaid(t *testing.T) {
	clinicID := uuid.New()
	aptID := uuid.New()
	apt := &model.Appointment{
		ID:       aptID,
		ClinicID: clinicID,
		Price:    150.00,
	}

	paymentRepo := &mockPaymentRepo{}
	aptRepo := &mockAptRepoForPayment{appointment: apt}
	svc := newPaymentService(paymentRepo, aptRepo, defaultPixMethod)

	resp, err := svc.Create(context.Background(), clinicID, aptID, &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultPixMethod.ID,
		Amount:          150.00,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 150.00, resp.Amount)
	assert.Equal(t, model.PaymentStatusPaid, aptRepo.paymentStatus)
}

func TestCreatePayment_PartialAmount_StatusPartial(t *testing.T) {
	clinicID := uuid.New()
	aptID := uuid.New()
	apt := &model.Appointment{
		ID:       aptID,
		ClinicID: clinicID,
		Price:    200.00,
	}

	paymentRepo := &mockPaymentRepo{}
	aptRepo := &mockAptRepoForPayment{appointment: apt}
	svc := newPaymentService(paymentRepo, aptRepo, defaultPixMethod)

	_, err := svc.Create(context.Background(), clinicID, aptID, &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultPixMethod.ID,
		Amount:          100.00,
	})

	require.NoError(t, err)
	assert.Equal(t, model.PaymentStatusPartial, aptRepo.paymentStatus)
}

func TestCreatePayment_TwoPayments_StatusPaid(t *testing.T) {
	clinicID := uuid.New()
	aptID := uuid.New()
	apt := &model.Appointment{
		ID:       aptID,
		ClinicID: clinicID,
		Price:    200.00,
	}

	paymentRepo := &mockPaymentRepo{}
	aptRepo := &mockAptRepoForPayment{appointment: apt}
	svc := newPaymentService(paymentRepo, aptRepo, defaultPixMethod)

	// First payment: R$100 (partial)
	_, err := svc.Create(context.Background(), clinicID, aptID, &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultPixMethod.ID,
		Amount:          100.00,
	})
	require.NoError(t, err)
	assert.Equal(t, model.PaymentStatusPartial, aptRepo.paymentStatus)

	// Second payment: R$100 (now paid)
	_, err = svc.Create(context.Background(), clinicID, aptID, &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultPixMethod.ID,
		Amount:          100.00,
	})
	require.NoError(t, err)
	assert.Equal(t, model.PaymentStatusPaid, aptRepo.paymentStatus)
}

func TestCreatePayment_WithDiscount_FullyCovered(t *testing.T) {
	clinicID := uuid.New()
	aptID := uuid.New()
	apt := &model.Appointment{
		ID:       aptID,
		ClinicID: clinicID,
		Price:    60.00,
	}

	paymentRepo := &mockPaymentRepo{}
	aptRepo := &mockAptRepoForPayment{appointment: apt}
	svc := newPaymentService(paymentRepo, aptRepo, defaultPixMethod)

	// Sessao custa R$60, desconto de R$10, paciente paga R$50
	// amount(50) + discount(10) = 60 = price → paid
	resp, err := svc.Create(context.Background(), clinicID, aptID, &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultPixMethod.ID,
		Amount:          50.00,
		Discount:        10.00,
	})

	require.NoError(t, err)
	assert.Equal(t, 50.00, resp.Amount)
	assert.Equal(t, 10.00, resp.Discount)
	assert.Equal(t, model.PaymentStatusPaid, aptRepo.paymentStatus)
}

func TestCreatePayment_WithDiscount_ExceedsPrice_ShouldFail(t *testing.T) {
	clinicID := uuid.New()
	aptID := uuid.New()
	apt := &model.Appointment{
		ID:       aptID,
		ClinicID: clinicID,
		Price:    60.00,
	}

	paymentRepo := &mockPaymentRepo{}
	aptRepo := &mockAptRepoForPayment{appointment: apt}
	svc := newPaymentService(paymentRepo, aptRepo, defaultPixMethod)

	// amount(60) + discount(10) = 70 > price(60) → erro
	_, err := svc.Create(context.Background(), clinicID, aptID, &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultPixMethod.ID,
		Amount:          60.00,
		Discount:        10.00,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "excede")
}

func TestCreatePayment_ExceedsPrice_ShouldFail(t *testing.T) {
	clinicID := uuid.New()
	aptID := uuid.New()
	apt := &model.Appointment{
		ID:       aptID,
		ClinicID: clinicID,
		Price:    100.00,
	}

	paymentRepo := &mockPaymentRepo{}
	aptRepo := &mockAptRepoForPayment{appointment: apt}
	svc := newPaymentService(paymentRepo, aptRepo, defaultPixMethod)

	_, err := svc.Create(context.Background(), clinicID, aptID, &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultPixMethod.ID,
		Amount:          150.00,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "excede")
}

func TestCreatePayment_InstallmentsOnPix_ShouldFail(t *testing.T) {
	clinicID := uuid.New()
	aptID := uuid.New()
	apt := &model.Appointment{
		ID:       aptID,
		ClinicID: clinicID,
		Price:    100.00,
	}

	paymentRepo := &mockPaymentRepo{}
	aptRepo := &mockAptRepoForPayment{appointment: apt}
	svc := newPaymentService(paymentRepo, aptRepo, defaultPixMethod)

	_, err := svc.Create(context.Background(), clinicID, aptID, &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultPixMethod.ID,
		Amount:          100.00,
		Installments:    3,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parcelas")
}

func TestCreatePayment_InstallmentsOnCredit_ShouldWork(t *testing.T) {
	clinicID := uuid.New()
	aptID := uuid.New()
	apt := &model.Appointment{
		ID:       aptID,
		ClinicID: clinicID,
		Price:    300.00,
	}

	paymentRepo := &mockPaymentRepo{}
	aptRepo := &mockAptRepoForPayment{appointment: apt}
	svc := newPaymentService(paymentRepo, aptRepo, defaultCreditMethod)

	resp, err := svc.Create(context.Background(), clinicID, aptID, &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultCreditMethod.ID,
		Amount:          300.00,
		Installments:    3,
	})

	require.NoError(t, err)
	assert.Equal(t, 3, resp.Installments)
}

func TestCreatePayment_InvalidAppointment(t *testing.T) {
	paymentRepo := &mockPaymentRepo{}
	aptRepo := &mockAptRepoForPayment{appointment: nil}
	svc := newPaymentService(paymentRepo, aptRepo, defaultPixMethod)

	_, err := svc.Create(context.Background(), uuid.New(), uuid.New(), &model.CreateAppointmentPaymentRequest{
		PaymentMethodID: defaultPixMethod.ID,
		Amount:          50.00,
	})

	assert.Error(t, err)
}
