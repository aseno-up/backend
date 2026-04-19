package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/enrollment/model"
	"github.com/aseno/backend/pkg/enrollment/repository"
	sessionplan_repository "github.com/aseno/backend/pkg/sessionplan/repository"
)

// EnrollmentService defines the interface for enrollment operations
type EnrollmentService interface {
	Enroll(ctx context.Context, clinicID, sessionPlanID uuid.UUID, req *model.EnrollRequest) (*model.SessionPlanEnrollment, error)
	Suspend(ctx context.Context, clinicID, enrollmentID uuid.UUID) error
	Reactivate(ctx context.Context, clinicID, enrollmentID uuid.UUID) error
	Cancel(ctx context.Context, clinicID, enrollmentID uuid.UUID) error
	ListByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanEnrollment, error)
	GetByID(ctx context.Context, clinicID, enrollmentID uuid.UUID) (*model.SessionPlanEnrollment, error)
	CountActiveByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (int64, error)
}

type enrollmentService struct {
	repo     repository.EnrollmentRepository
	planRepo sessionplan_repository.SessionPlanRepository
}

// NewEnrollmentService creates a new enrollment service
func NewEnrollmentService(
	repo repository.EnrollmentRepository,
	planRepo sessionplan_repository.SessionPlanRepository,
) EnrollmentService {
	return &enrollmentService{
		repo:     repo,
		planRepo: planRepo,
	}
}

// Enroll adds a patient to a group session plan
func (s *enrollmentService) Enroll(ctx context.Context, clinicID, sessionPlanID uuid.UUID, req *model.EnrollRequest) (*model.SessionPlanEnrollment, error) {
	// Validate the plan exists and is a group plan
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	if plan.Type != "group" {
		return nil, errors.ErrValidationInstance.WithMessage("Can only enroll patients in group plans")
	}

	if !plan.IsActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Plan is not active")
	}

	// Validate week_days: required and each day must be 0..6
	if len(req.WeekDays) == 0 {
		return nil, errors.ErrValidationInstance.WithMessage("week_days is required for group enrollments")
	}
	weekDays, err := normalizeWeekDays(req.WeekDays)
	if err != nil {
		return nil, err
	}

	// Check per-day capacity: for each requested day, count active enrollments on that day.
	if plan.Capacity != nil {
		for _, day := range weekDays {
			count, err := s.repo.CountActiveByPlanAndWeekDay(ctx, clinicID, sessionPlanID, int(day))
			if err != nil {
				return nil, err
			}
			if int(count) >= *plan.Capacity {
				return nil, errors.ErrValidationInstance.WithMessage("Plan has reached maximum capacity for the selected day")
			}
		}
	}

	// Check if patient is already enrolled
	existing, err := s.repo.GetByPatientAndPlan(ctx, clinicID, req.PatientID, sessionPlanID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.IsActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Patient is already enrolled in this plan")
	}

	// If patient was previously cancelled, reactivate
	if existing != nil && existing.Status == model.EnrollmentStatusCancelled {
		// Same billing_day guard as below so reactivation into monthly also
		// requires the field.
		effectiveMode := req.PaymentMode
		if effectiveMode == "" {
			effectiveMode = existing.PaymentMode
		}
		if effectiveMode == model.PaymentModeMonthly {
			if req.BillingDay == nil {
				return nil, errors.ErrValidationInstance.WithMessage("billing_day is required for monthly enrollments")
			}
			if *req.BillingDay < 1 || *req.BillingDay > 31 {
				return nil, errors.ErrValidationInstance.WithMessage("billing_day must be between 1 and 31")
			}
		}
		existing.Status = model.EnrollmentStatusActive
		existing.LeftAt = nil
		existing.WeekDays = weekDays
		if req.TotalSessions != nil {
			existing.TotalSessions = req.TotalSessions
		}
		if req.PaymentMode != "" {
			existing.PaymentMode = req.PaymentMode
		}
		existing.MonthlyAmount = req.MonthlyAmount
		existing.BillingDay = req.BillingDay
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// Default payment mode
	paymentMode := req.PaymentMode
	if paymentMode == "" {
		paymentMode = model.PaymentModePerSession
	}

	// Monthly enrollments require a billing day so the payment reminder
	// worker can compute the next due date. We validate it here (instead of
	// relying solely on the frontend) because enrollments can be created
	// from other callers too (e.g. import scripts).
	if paymentMode == model.PaymentModeMonthly {
		if req.BillingDay == nil {
			return nil, errors.ErrValidationInstance.WithMessage("billing_day is required for monthly enrollments")
		}
		if *req.BillingDay < 1 || *req.BillingDay > 31 {
			return nil, errors.ErrValidationInstance.WithMessage("billing_day must be between 1 and 31")
		}
	}

	enrollment := &model.SessionPlanEnrollment{
		ClinicID:      clinicID,
		SessionPlanID: sessionPlanID,
		PatientID:     req.PatientID,
		Status:        model.EnrollmentStatusActive,
		WeekDays:      weekDays,
		TotalSessions: req.TotalSessions,
		SessionsDone:  0,
		PaymentMode:   paymentMode,
		MonthlyAmount: req.MonthlyAmount,
		BillingDay:    req.BillingDay,
		JoinedAt:      time.Now(),
	}

	if err := s.repo.Create(ctx, enrollment); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, clinicID, enrollment.ID)
}

// normalizeWeekDays validates each weekday is in [0,6] and returns a deduplicated
// pq.Int64Array ready for persistence.
func normalizeWeekDays(days []int) (pq.Int64Array, error) {
	if len(days) == 0 {
		return nil, nil
	}
	seen := make(map[int]bool, len(days))
	out := make(pq.Int64Array, 0, len(days))
	for _, d := range days {
		if d < 0 || d > 6 {
			return nil, errors.ErrValidationInstance.WithMessage("week_days must contain values between 0 and 6")
		}
		if seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, int64(d))
	}
	return out, nil
}

// Suspend suspends an enrollment
func (s *enrollmentService) Suspend(ctx context.Context, clinicID, enrollmentID uuid.UUID) error {
	enrollment, err := s.repo.GetByID(ctx, clinicID, enrollmentID)
	if err != nil {
		return err
	}

	if enrollment.Status != model.EnrollmentStatusActive {
		return errors.ErrValidationInstance.WithMessage("Only active enrollments can be suspended")
	}

	return s.repo.UpdateStatus(ctx, clinicID, enrollmentID, model.EnrollmentStatusSuspended)
}

// Reactivate reactivates a suspended enrollment
func (s *enrollmentService) Reactivate(ctx context.Context, clinicID, enrollmentID uuid.UUID) error {
	enrollment, err := s.repo.GetByID(ctx, clinicID, enrollmentID)
	if err != nil {
		return err
	}

	if enrollment.Status != model.EnrollmentStatusSuspended {
		return errors.ErrValidationInstance.WithMessage("Only suspended enrollments can be reactivated")
	}

	return s.repo.UpdateStatus(ctx, clinicID, enrollmentID, model.EnrollmentStatusActive)
}

// Cancel cancels an enrollment
func (s *enrollmentService) Cancel(ctx context.Context, clinicID, enrollmentID uuid.UUID) error {
	enrollment, err := s.repo.GetByID(ctx, clinicID, enrollmentID)
	if err != nil {
		return err
	}

	if enrollment.Status == model.EnrollmentStatusCancelled {
		return errors.ErrValidationInstance.WithMessage("Enrollment is already cancelled")
	}

	return s.repo.UpdateStatus(ctx, clinicID, enrollmentID, model.EnrollmentStatusCancelled)
}

// ListByPlan retrieves all enrollments for a plan
func (s *enrollmentService) ListByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanEnrollment, error) {
	return s.repo.GetByPlan(ctx, clinicID, sessionPlanID)
}

// GetByID retrieves an enrollment by ID
func (s *enrollmentService) GetByID(ctx context.Context, clinicID, enrollmentID uuid.UUID) (*model.SessionPlanEnrollment, error) {
	return s.repo.GetByID(ctx, clinicID, enrollmentID)
}

// CountActiveByPlan counts active enrollments for a plan
func (s *enrollmentService) CountActiveByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (int64, error) {
	return s.repo.CountActiveByPlan(ctx, clinicID, sessionPlanID)
}
