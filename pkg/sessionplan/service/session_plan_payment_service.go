package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
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

	SetFinancePlanUpserter(u FinancePlanUpserter)
}

// FinancePlanUpserter is a narrow port into the finance module. Upfront/monthly
// plan payments live in session_plan_payments (separate table from appointment
// payments), so they need their own hook to appear in Caixa/Lançamentos as
// source='session_plan'.
type FinancePlanUpserter interface {
	UpsertFromSessionPlanPayment(ctx context.Context, payment *model.SessionPlanPayment, clinicID uuid.UUID, planTitle, patientName string) error
	DeleteFromSessionPlanPayment(ctx context.Context, sessionPlanPaymentID, clinicID uuid.UUID) error
}

type planPaymentService struct {
	paymentRepo     repository.PlanPaymentRepository
	planRepo        repository.SessionPlanRepository
	aptPaymentRepo  appointment_repository.AppointmentPaymentRepository
	financeUpserter FinancePlanUpserter
}

// NewPlanPaymentService creates a new plan payment service
func NewPlanPaymentService(
	paymentRepo repository.PlanPaymentRepository,
	planRepo repository.SessionPlanRepository,
	aptPaymentRepo appointment_repository.AppointmentPaymentRepository,
) PlanPaymentService {
	return &planPaymentService{
		paymentRepo:    paymentRepo,
		planRepo:       planRepo,
		aptPaymentRepo: aptPaymentRepo,
	}
}

func (s *planPaymentService) SetFinancePlanUpserter(u FinancePlanUpserter) {
	s.financeUpserter = u
}

func (s *planPaymentService) syncFinanceUpsert(ctx context.Context, payment *model.SessionPlanPayment, clinicID uuid.UUID) {
	if s.financeUpserter == nil {
		return
	}
	// Best-effort: pull plan title + patient name so the Caixa/Lançamentos
	// row identifies WHO paid, not just which plan. Fall back silently when
	// the lookup fails so the payment flow never breaks.
	// For group plans (PatientID empty), patientName stays empty and the
	// finance description falls back to just the plan title.
	var planTitle, patientName string
	if plan, err := s.planRepo.GetByID(ctx, clinicID, payment.SessionPlanID); err == nil && plan != nil {
		planTitle = plan.Title
		patientName = plan.PatientName // populated by repo.loadRelations
	}
	_ = s.financeUpserter.UpsertFromSessionPlanPayment(ctx, payment, clinicID, planTitle, patientName)
}

func (s *planPaymentService) syncFinanceDelete(ctx context.Context, paymentID, clinicID uuid.UUID) {
	if s.financeUpserter == nil {
		return
	}
	_ = s.financeUpserter.DeleteFromSessionPlanPayment(ctx, paymentID, clinicID)
}

// CreatePlanPayment creates a payment for an individual plan.
// Supports upfront (a única parcela total) and monthly (uma parcela por mês
// de referência — `reference_month` no formato YYYY-MM). Planos per_session
// são pagos via appointment_payment, então não passam por aqui.
func (s *planPaymentService) CreatePlanPayment(ctx context.Context, clinicID, sessionPlanID uuid.UUID, req *model.CreatePlanPaymentRequest) (*model.SessionPlanPayment, error) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	switch plan.PaymentMode {
	case model.PaymentModeUpfront:
		// ok
	case model.PaymentModeMonthly:
		if req.ReferenceMonth == "" {
			return nil, errors.ErrValidationInstance.WithMessage("Informe o mês de referência (reference_month) para planos mensais.")
		}
		if _, err := time.Parse("2006-01", req.ReferenceMonth); err != nil {
			return nil, errors.ErrValidationInstance.WithMessage("Formato de reference_month inválido. Use YYYY-MM.")
		}
	default:
		return nil, errors.ErrValidationInstance.WithMessage("Este plano é pago por sessão; registre o pagamento no agendamento.")
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
		ReferenceMonth:  req.ReferenceMonth,
		Notes:           req.Notes,
		PaidAt:          resolvePaidAt(req.PaidAt),
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	s.syncFinanceUpsert(ctx, payment, clinicID)

	return payment, nil
}

// resolvePaidAt returns the caller-supplied payment timestamp or the current server time.
func resolvePaidAt(paidAt *time.Time) time.Time {
	if paidAt != nil && !paidAt.IsZero() {
		return *paidAt
	}
	return time.Now()
}

// GetPlanPayments retrieves all payments for a plan.
// For per_session plans, it aggregates appointment payments from linked appointments
// plus any direct SessionPlanPayments (e.g. guest payments for group plans).
func (s *planPaymentService) GetPlanPayments(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanPayment, error) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	if plan.PaymentMode == model.PaymentModePerSession {
		aptPayments, err := s.getAppointmentPaymentsAsPlanPayments(ctx, clinicID, sessionPlanID)
		if err != nil {
			return nil, err
		}

		// Also include direct plan payments (guest avulso payments for group plans)
		directPayments, err := s.paymentRepo.GetByPlan(ctx, clinicID, sessionPlanID)
		if err != nil {
			return nil, err
		}

		if len(directPayments) > 0 {
			aptPayments = append(aptPayments, directPayments...)
		}

		return aptPayments, nil
	}

	return s.paymentRepo.GetByPlan(ctx, clinicID, sessionPlanID)
}

// getAppointmentPaymentsAsPlanPayments converts appointment payments into
// SessionPlanPayment format so the frontend can display them uniformly.
func (s *planPaymentService) getAppointmentPaymentsAsPlanPayments(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanPayment, error) {
	aptPayments, err := s.aptPaymentRepo.ListBySessionPlan(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.SessionPlanPayment, len(aptPayments))
	for i, ap := range aptPayments {
		result[i] = appointmentPaymentToPlanPayment(ap, sessionPlanID, clinicID)
	}
	return result, nil
}

func appointmentPaymentToPlanPayment(ap *appointment_model.AppointmentPayment, sessionPlanID, clinicID uuid.UUID) *model.SessionPlanPayment {
	methodName := ""
	if ap.PaymentMethod != nil {
		methodName = ap.PaymentMethod.Name
	}
	return &model.SessionPlanPayment{
		ID:                   ap.ID,
		ClinicID:             clinicID,
		SessionPlanID:        sessionPlanID,
		PaymentMethodID:      ap.PaymentMethodID,
		Amount:               ap.Amount,
		Discount:             ap.Discount,
		Installments:         ap.Installments,
		Notes:                ap.Notes,
		PaidAt:               ap.PaidAt,
		CreatedAt:            ap.CreatedAt,
		UpdatedAt:            ap.UpdatedAt,
		PaymentMethodName:    methodName,
		AppointmentID:        &ap.AppointmentID,
		AppointmentStartTime: ap.AppointmentStartTime,
		AppointmentPrice:     ap.AppointmentPrice,
	}
}

// CreateEnrollmentPayment creates a payment for a group enrollment (upfront/monthly)
func (s *planPaymentService) CreateEnrollmentPayment(ctx context.Context, clinicID, sessionPlanID, enrollmentID uuid.UUID, req *model.CreatePlanPaymentRequest) (*model.SessionPlanPayment, error) {
	installments := req.Installments
	if installments < 1 {
		installments = 1
	}

	// Validate per_session: payment cannot exceed session price for that date
	if req.SessionDate != "" {
		plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
		if err == nil && plan != nil {
			sessionPrice := plan.GetEffectivePrice()
			if sessionPrice > 0 {
				alreadyPaid, err := s.paymentRepo.GetTotalPaidByEnrollmentAndDate(ctx, clinicID, enrollmentID, req.SessionDate)
				if err == nil {
					remaining := sessionPrice - alreadyPaid
					if req.Amount > remaining+0.01 {
						return nil, errors.ErrValidationInstance.WithMessage("Valor excede o preço da sessão")
					}
				}
			}
		}
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
		SessionDate:     req.SessionDate,
		Notes:           req.Notes,
		PaidAt:          resolvePaidAt(req.PaidAt),
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	s.syncFinanceUpsert(ctx, payment, clinicID)

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
	if err := s.paymentRepo.Delete(ctx, clinicID, paymentID); err != nil {
		return err
	}
	s.syncFinanceDelete(ctx, paymentID, clinicID)
	return nil
}
