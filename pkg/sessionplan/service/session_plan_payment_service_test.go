package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/sessionplan/model"
)

// --- Mock: PlanPaymentRepository ---

type mockPlanPaymentRepo struct {
	payments []*model.SessionPlanPayment
}

func (m *mockPlanPaymentRepo) Create(_ context.Context, p *model.SessionPlanPayment) error {
	m.payments = append(m.payments, p)
	return nil
}

func (m *mockPlanPaymentRepo) GetByPlan(_ context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanPayment, error) {
	var result []*model.SessionPlanPayment
	for _, p := range m.payments {
		if p.ClinicID == clinicID && p.SessionPlanID == sessionPlanID && p.EnrollmentID == nil {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPlanPaymentRepo) GetByEnrollment(_ context.Context, clinicID, enrollmentID uuid.UUID) ([]*model.SessionPlanPayment, error) {
	return nil, nil
}

func (m *mockPlanPaymentRepo) GetTotalPaidByPlan(_ context.Context, _, _ uuid.UUID) (float64, error) {
	return 0, nil
}

func (m *mockPlanPaymentRepo) GetTotalPaidByEnrollment(_ context.Context, _, _ uuid.UUID) (float64, error) {
	return 0, nil
}

func (m *mockPlanPaymentRepo) Delete(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

// --- Mock: AppointmentPaymentRepository (for per_session aggregation) ---

type mockAppointmentPaymentRepo struct {
	payments []*appointment_model.AppointmentPayment
}

func (m *mockAppointmentPaymentRepo) Create(_ *appointment_model.AppointmentPayment) error {
	return nil
}

func (m *mockAppointmentPaymentRepo) GetByID(_ uuid.UUID, _ uuid.UUID) (*appointment_model.AppointmentPayment, error) {
	return nil, nil
}

func (m *mockAppointmentPaymentRepo) Update(_ *appointment_model.AppointmentPayment) error {
	return nil
}

func (m *mockAppointmentPaymentRepo) Delete(_, _ uuid.UUID) error {
	return nil
}

func (m *mockAppointmentPaymentRepo) ListByAppointment(appointmentID, clinicID uuid.UUID) ([]*appointment_model.AppointmentPayment, error) {
	return nil, nil
}

func (m *mockAppointmentPaymentRepo) SumByAppointment(_ uuid.UUID) (float64, error) {
	return 0, nil
}

func (m *mockAppointmentPaymentRepo) ListBySessionPlan(_ context.Context, clinicID, sessionPlanID uuid.UUID) ([]*appointment_model.AppointmentPayment, error) {
	var result []*appointment_model.AppointmentPayment
	for _, p := range m.payments {
		if p.ClinicID == clinicID {
			result = append(result, p)
		}
	}
	return result, nil
}

// --- Tests ---

func TestGetPlanPayments_PerSession_ReturnsAppointmentPayments(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	appointmentID := uuid.New()
	methodID := uuid.New()

	plan := &model.SessionPlan{
		ID:           planID,
		ClinicID:     clinicID,
		PaymentMode:  model.PaymentModePerSession,
		SessionPrice: 100.0,
	}

	planRepo := &mockPlanRepo{plan: plan}
	planPaymentRepo := &mockPlanPaymentRepo{}

	// Appointment payment made through the calendar
	aptPayments := []*appointment_model.AppointmentPayment{
		{
			ID:              uuid.New(),
			ClinicID:        clinicID,
			AppointmentID:   appointmentID,
			PaymentMethodID: methodID,
			Amount:          100.0,
			Discount:        0,
			Installments:    1,
			PaidAt:          time.Now(),
			PaymentMethod: &appointment_model.PaymentMethodInfo{
				ID:   methodID,
				Name: "Pix",
				Type: "pix",
			},
		},
	}
	aptPaymentRepo := &mockAppointmentPaymentRepo{payments: aptPayments}

	svc := NewPlanPaymentService(planPaymentRepo, planRepo, aptPaymentRepo)

	payments, err := svc.GetPlanPayments(context.Background(), clinicID, planID)
	require.NoError(t, err)
	require.Len(t, payments, 1, "per_session plan should return appointment payments")
	assert.Equal(t, 100.0, payments[0].Amount)
	assert.Equal(t, "Pix", payments[0].PaymentMethodName)
}

func TestGetPlanPayments_Upfront_ReturnsOnlyPlanPayments(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	methodID := uuid.New()

	plan := &model.SessionPlan{
		ID:           planID,
		ClinicID:     clinicID,
		PaymentMode:  model.PaymentModeUpfront,
		SessionPrice: 100.0,
	}

	planRepo := &mockPlanRepo{plan: plan}

	planPayment := &model.SessionPlanPayment{
		ID:              uuid.New(),
		ClinicID:        clinicID,
		SessionPlanID:   planID,
		PaymentMethodID: methodID,
		Amount:          500.0,
		PaidAt:          time.Now(),
		PaymentMethodName: "Pix",
	}
	planPaymentRepo := &mockPlanPaymentRepo{payments: []*model.SessionPlanPayment{planPayment}}

	// Should NOT call appointment payment repo for upfront plans
	aptPaymentRepo := &mockAppointmentPaymentRepo{}

	svc := NewPlanPaymentService(planPaymentRepo, planRepo, aptPaymentRepo)

	payments, err := svc.GetPlanPayments(context.Background(), clinicID, planID)
	require.NoError(t, err)
	require.Len(t, payments, 1)
	assert.Equal(t, 500.0, payments[0].Amount)
}

func TestCreatePlanPayment_PerSession_ReturnsError(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()

	plan := &model.SessionPlan{
		ID:          planID,
		ClinicID:    clinicID,
		PaymentMode: model.PaymentModePerSession,
	}

	planRepo := &mockPlanRepo{plan: plan}
	planPaymentRepo := &mockPlanPaymentRepo{}
	aptPaymentRepo := &mockAppointmentPaymentRepo{}

	svc := NewPlanPaymentService(planPaymentRepo, planRepo, aptPaymentRepo)

	_, err := svc.CreatePlanPayment(context.Background(), clinicID, planID, &model.CreatePlanPaymentRequest{
		PaymentMethodID: uuid.New(),
		Amount:          100.0,
	})

	require.Error(t, err, "should reject payment creation for per_session plans")
}

func TestGetPlanPayments_PerSession_NoAppointmentPayments_ReturnsEmpty(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()

	plan := &model.SessionPlan{
		ID:           planID,
		ClinicID:     clinicID,
		PaymentMode:  model.PaymentModePerSession,
		SessionPrice: 100.0,
	}

	planRepo := &mockPlanRepo{plan: plan}
	planPaymentRepo := &mockPlanPaymentRepo{}
	aptPaymentRepo := &mockAppointmentPaymentRepo{}

	svc := NewPlanPaymentService(planPaymentRepo, planRepo, aptPaymentRepo)

	payments, err := svc.GetPlanPayments(context.Background(), clinicID, planID)
	require.NoError(t, err)
	assert.Empty(t, payments, "should return empty when no appointment payments exist")
}
