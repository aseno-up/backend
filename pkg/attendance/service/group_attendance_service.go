package service

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/attendance/model"
	"github.com/aseno/backend/pkg/attendance/repository"
	commission_service "github.com/aseno/backend/pkg/commission/service"
	enrollment_model "github.com/aseno/backend/pkg/enrollment/model"
	enrollment_repository "github.com/aseno/backend/pkg/enrollment/repository"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
	sessionplan_repository "github.com/aseno/backend/pkg/sessionplan/repository"
)

// groupPlanReader is the subset of SessionPlanRepository needed here
type groupPlanReader interface {
	GetByID(ctx context.Context, clinicID, id uuid.UUID) (*sessionplan_model.SessionPlan, error)
}

// groupAttendanceRepo is the subset of GroupAttendanceRepository needed here
type groupAttendanceRepo interface {
	Create(ctx context.Context, attendance *model.GroupSessionAttendance) error
	GetByPlanAndDate(ctx context.Context, clinicID, sessionPlanID uuid.UUID, date string) ([]*model.GroupSessionAttendance, error)
	GetByPlanDateAndPatient(ctx context.Context, clinicID, sessionPlanID, patientID uuid.UUID, date string) (*model.GroupSessionAttendance, error)
	Update(ctx context.Context, attendance *model.GroupSessionAttendance) error
}

// GroupAttendanceService handles attendance for group session plans
type GroupAttendanceService interface {
	RecordGroupAttendance(ctx context.Context, clinicID, sessionPlanID uuid.UUID, req *model.RecordGroupAttendanceRequest) ([]*model.GroupSessionAttendance, error)
	GetGroupAttendance(ctx context.Context, clinicID, sessionPlanID uuid.UUID, date string) ([]*model.GroupSessionAttendance, error)
	SetCommissionCalculator(calculator commission_service.CommissionService)
	SetSessionProfessionalRepo(repo repository.GroupSessionProfessionalRepository)
	SetPlanPaymentRepo(repo sessionplan_repository.PlanPaymentRepository)
}

type groupAttendanceService struct {
	repo                    groupAttendanceRepo
	planRepo                groupPlanReader
	enrollRepo              enrollment_repository.EnrollmentRepository
	commissionCalculator    commission_service.CommissionService
	sessionProfessionalRepo repository.GroupSessionProfessionalRepository
	planPaymentRepo         sessionplan_repository.PlanPaymentRepository
}

// NewGroupAttendanceService creates a new group attendance service
func NewGroupAttendanceService(
	repo groupAttendanceRepo,
	planRepo groupPlanReader,
	enrollRepo enrollment_repository.EnrollmentRepository,
) GroupAttendanceService {
	return &groupAttendanceService{
		repo:       repo,
		planRepo:   planRepo,
		enrollRepo: enrollRepo,
	}
}

func (s *groupAttendanceService) SetCommissionCalculator(calculator commission_service.CommissionService) {
	s.commissionCalculator = calculator
}

func (s *groupAttendanceService) SetSessionProfessionalRepo(repo repository.GroupSessionProfessionalRepository) {
	s.sessionProfessionalRepo = repo
}

func (s *groupAttendanceService) SetPlanPaymentRepo(repo sessionplan_repository.PlanPaymentRepository) {
	s.planPaymentRepo = repo
}

// RecordGroupAttendance records attendance for all entries in a group session.
// For each present non-guest entry, increments enrollment sessions_done.
func (s *groupAttendanceService) RecordGroupAttendance(
	ctx context.Context,
	clinicID, sessionPlanID uuid.UUID,
	req *model.RecordGroupAttendanceRequest,
) ([]*model.GroupSessionAttendance, error) {
	// Validate plan exists and is a group plan
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}
	if plan.Type != sessionplan_model.PlanTypeGroup {
		return nil, errors.ErrValidationInstance.WithMessage("Only group plans support group attendance")
	}

	results := make([]*model.GroupSessionAttendance, 0, len(req.Entries))

	for _, entry := range req.Entries {
		// Check if record already exists for this patient+plan+date
		existing, err := s.repo.GetByPlanDateAndPatient(ctx, clinicID, sessionPlanID, entry.PatientID, req.Date)
		if err != nil {
			return nil, err
		}

		// Find enrollment for non-guests
		var enrollmentID *uuid.UUID
		if !entry.IsGuest {
			enrollment, _ := s.enrollRepo.GetByPatientAndPlan(ctx, clinicID, entry.PatientID, sessionPlanID)
			if enrollment != nil {
				enrollmentID = &enrollment.ID
			}
		}

		if existing != nil {
			// Update existing record
			oldStatus := existing.Status
			existing.Status = entry.Status
			if err := s.repo.Update(ctx, existing); err != nil {
				return nil, err
			}

			// Increment if status changed TO present (and not guest)
			if oldStatus != model.AttendanceStatusPresent && entry.Status == model.AttendanceStatusPresent && !entry.IsGuest {
				s.incrementEnrollmentSessions(ctx, clinicID, sessionPlanID, entry.PatientID)
			}

			results = append(results, existing)
		} else {
			// Create new record
			record := &model.GroupSessionAttendance{
				ClinicID:      clinicID,
				SessionPlanID: sessionPlanID,
				SessionDate:   req.Date,
				PatientID:     entry.PatientID,
				EnrollmentID:  enrollmentID,
				Status:        entry.Status,
				IsGuest:       entry.IsGuest,
			}

			if err := s.repo.Create(ctx, record); err != nil {
				return nil, err
			}

			// Create guest payment if present with payment info
			if entry.IsGuest && entry.Status == model.AttendanceStatusPresent &&
				entry.PaymentMethodID != nil && entry.PaymentAmount != nil && *entry.PaymentAmount > 0 &&
				s.planPaymentRepo != nil {
				payment := &sessionplan_model.SessionPlanPayment{
					ClinicID:        clinicID,
					SessionPlanID:   sessionPlanID,
					PaymentMethodID: *entry.PaymentMethodID,
					Amount:          *entry.PaymentAmount,
					SessionDate:     req.Date,
					Notes:           "Pagamento avulso - convidado: " + record.PatientName,
					PaidAt:          time.Now(),
				}
				if err := s.planPaymentRepo.Create(ctx, payment); err != nil {
					log.Printf("[GroupAttendance] Error creating guest payment for patient %s: %v", entry.PatientID, err)
				}
			}

			// Increment if present and not guest
			if entry.Status == model.AttendanceStatusPresent && !entry.IsGuest {
				s.incrementEnrollmentSessions(ctx, clinicID, sessionPlanID, entry.PatientID)
			}

			results = append(results, record)
		}
	}

	// Calculate commission for this group session (once per session date, not per student)
	s.calculateGroupSessionCommission(ctx, clinicID, plan, req.Date)

	return results, nil
}

// calculateGroupSessionCommission calculates commission for the professional assigned to this group session
func (s *groupAttendanceService) calculateGroupSessionCommission(ctx context.Context, clinicID uuid.UUID, plan *sessionplan_model.SessionPlan, date string) {
	if s.commissionCalculator == nil || s.sessionProfessionalRepo == nil {
		log.Printf("[GroupAttendance] SKIP commission: commissionCalculator=%v sessionProfessionalRepo=%v", s.commissionCalculator != nil, s.sessionProfessionalRepo != nil)
		return
	}

	if plan.ProcedureID == nil {
		log.Printf("[GroupAttendance] SKIP commission for plan %s: ProcedureID is nil", plan.ID)
		return // No procedure linked to plan — cannot calculate commission
	}

	// Determine the professional for this session
	var professionalID uuid.UUID

	record, err := s.sessionProfessionalRepo.GetByPlanAndDate(ctx, clinicID, plan.ID, date)
	if err == nil && record != nil {
		professionalID = record.ProfessionalID
		log.Printf("[GroupAttendance] Using session-specific professional %s for plan %s date %s", professionalID, plan.ID, date)
	} else if plan.ProfessionalID != nil {
		professionalID = *plan.ProfessionalID
		log.Printf("[GroupAttendance] Using plan default professional %s for plan %s date %s", professionalID, plan.ID, date)
	} else {
		log.Printf("[GroupAttendance] SKIP commission for plan %s: no professional assigned (session lookup err: %v)", plan.ID, err)
		return // No professional assigned — cannot calculate commission
	}

	log.Printf("[GroupAttendance] Calculating commission: plan=%s date=%s prof=%s proc=%s price=%.2f procName=%s",
		plan.ID, date, professionalID, *plan.ProcedureID, plan.GetEffectivePrice(), plan.ProcedureName)

	input := &commission_service.GroupSessionCommissionInput{
		SessionPlanID:  plan.ID,
		SessionDate:    date,
		ProfessionalID: professionalID,
		ProcedureID:    *plan.ProcedureID,
		ProcedureName:  plan.ProcedureName,
		SessionPrice:   plan.GetEffectivePrice(),
	}

	if err := s.commissionCalculator.CalculateAndStoreForGroupSession(ctx, input, clinicID); err != nil {
		log.Printf("[GroupAttendance] Error calculating commission for plan %s date %s: %v", plan.ID, date, err)
	} else {
		log.Printf("[GroupAttendance] Commission calculated successfully for plan %s date %s", plan.ID, date)
	}
}

// GetGroupAttendance retrieves all attendance records for a group session on a specific date
func (s *groupAttendanceService) GetGroupAttendance(
	ctx context.Context,
	clinicID, sessionPlanID uuid.UUID,
	date string,
) ([]*model.GroupSessionAttendance, error) {
	return s.repo.GetByPlanAndDate(ctx, clinicID, sessionPlanID, date)
}

// incrementEnrollmentSessions increments sessions_done on the patient's enrollment
// and auto-completes if the total is reached.
func (s *groupAttendanceService) incrementEnrollmentSessions(ctx context.Context, clinicID, sessionPlanID, patientID uuid.UUID) {
	enrollment, err := s.enrollRepo.GetByPatientAndPlan(ctx, clinicID, patientID, sessionPlanID)
	if err != nil || enrollment == nil {
		return
	}

	s.enrollRepo.IncrementSessionsDone(ctx, clinicID, enrollment.ID)

	// Auto-complete enrollment if total sessions reached
	if enrollment.TotalSessions != nil && *enrollment.TotalSessions > 0 && enrollment.SessionsDone+1 >= *enrollment.TotalSessions {
		s.enrollRepo.UpdateStatus(ctx, clinicID, enrollment.ID, enrollment_model.EnrollmentStatusCancelled)
	}
}
