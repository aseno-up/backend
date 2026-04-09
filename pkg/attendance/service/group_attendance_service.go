package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/attendance/model"
	enrollment_model "github.com/aseno/backend/pkg/enrollment/model"
	enrollment_repository "github.com/aseno/backend/pkg/enrollment/repository"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
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
}

type groupAttendanceService struct {
	repo       groupAttendanceRepo
	planRepo   groupPlanReader
	enrollRepo enrollment_repository.EnrollmentRepository
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

			// Increment if present and not guest
			if entry.Status == model.AttendanceStatusPresent && !entry.IsGuest {
				s.incrementEnrollmentSessions(ctx, clinicID, sessionPlanID, entry.PatientID)
			}

			results = append(results, record)
		}
	}

	return results, nil
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
