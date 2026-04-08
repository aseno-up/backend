package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/repository"
)

// PlanPaymentService defines the interface for plan payment operations
type PlanPaymentService interface {
	// Individual plan payments (upfront)
	CreatePlanPayment(ctx context.Context, clinicID, sessionPlanID uuid.UUID, req *model.CreatePlanPaymentRequest) (*model.SessionPlanPayment, error)
	GetPlanPayments(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanPayment, error)

	// Enrollment payments (group upfront/monthly)
	CreateEnrollmentPayment(ctx context.Context, clinicID, sessionPlanID, enrollmentID uuid.UUID, req *model.CreatePlanPaymentRequest) (*model.SessionPlanPayment, error)
	GetEnrollmentPayments(ctx context.Context, clinicID, enrollmentID uuid.UUID) ([]*model.SessionPlanPayment, error)

	// Check if plan is fully paid (upfront)
	IsPlanFullyPaid(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (bool, error)

	DeletePayment(ctx context.Context, clinicID, paymentID uuid.UUID) error
}

type planPaymentService struct {
	paymentRepo repository.PlanPaymentRepository
	planRepo    repository.SessionPlanRepository
}

// NewPlanPaymentService creates a new plan payment service
func NewPlanPaymentService(
	paymentRepo repository.PlanPaymentRepository,
	planRepo repository.SessionPlanRepository,
) PlanPaymentService {
	return &planPaymentService{
		paymentRepo: paymentRepo,
		planRepo:    planRepo,
	}
}

// CreatePlanPayment creates a payment for an individual plan (upfront)
func (s *planPaymentService) CreatePlanPayment(ctx context.Context, clinicID, sessionPlanID uuid.UUID, req *model.CreatePlanPaymentRequest) (*model.SessionPlanPayment, error) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	if plan.PaymentMode != model.PaymentModeUpfront {
		return nil, errors.ErrValidationInstance.WithMessage("Plan payment is only available for upfront payment mode")
	}

	installments := req.Installments
	if installments < 1 {
		installments = 1
	}

	payment := &model.SessionPlanPayment{
		ClinicID:        clinicID,
		SessionPlanID:   sessionPlanID,
		PaymentMethodID: req.PaymentMethodID,
		Amount:          req.Amount,
		Discount:        req.Discount,
		Installments:    installments,
		Notes:           req.Notes,
		PaidAt:          time.Now(),
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

// GetPlanPayments retrieves all payments for a plan
func (s *planPaymentService) GetPlanPayments(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanPayment, error) {
	return s.paymentRepo.GetByPlan(ctx, clinicID, sessionPlanID)
}

// CreateEnrollmentPayment creates a payment for a group enrollment (upfront/monthly)
func (s *planPaymentService) CreateEnrollmentPayment(ctx context.Context, clinicID, sessionPlanID, enrollmentID uuid.UUID, req *model.CreatePlanPaymentRequest) (*model.SessionPlanPayment, error) {
	installments := req.Installments
	if installments < 1 {
		installments = 1
	}

	payment := &model.SessionPlanPayment{
		ClinicID:        clinicID,
		SessionPlanID:   sessionPlanID,
		EnrollmentID:    &enrollmentID,
		PaymentMethodID: req.PaymentMethodID,
		Amount:          req.Amount,
		Discount:        req.Discount,
		Installments:    installments,
		ReferenceMonth:  req.ReferenceMonth,
		Notes:           req.Notes,
		PaidAt:          time.Now(),
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

// GetEnrollmentPayments retrieves all payments for an enrollment
func (s *planPaymentService) GetEnrollmentPayments(ctx context.Context, clinicID, enrollmentID uuid.UUID) ([]*model.SessionPlanPayment, error) {
	return s.paymentRepo.GetByEnrollment(ctx, clinicID, enrollmentID)
}

// IsPlanFullyPaid checks if a plan has been fully paid (upfront)
func (s *planPaymentService) IsPlanFullyPaid(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (bool, error) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return false, err
	}

	if plan.PaymentMode != model.PaymentModeUpfront {
		return false, nil
	}

	// Calculate total plan value
	price := plan.GetEffectivePrice()
	totalSessions := plan.TotalSessions
	if totalSessions <= 0 {
		return false, nil // Can't determine if fully paid for indeterminate plans
	}

	totalValue := price * float64(totalSessions)

	// Get total paid
	totalPaid, err := s.paymentRepo.GetTotalPaidByPlan(ctx, clinicID, sessionPlanID)
	if err != nil {
		return false, err
	}

	return totalPaid >= totalValue, nil
}

// DeletePayment deletes a plan payment
func (s *planPaymentService) DeletePayment(ctx context.Context, clinicID, paymentID uuid.UUID) error {
	return s.paymentRepo.Delete(ctx, clinicID, paymentID)
}
