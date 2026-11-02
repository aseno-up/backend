package service

import (
	"context"

	"github.com/google/uuid"

	"time"

	"github.com/aseno/backend/internal/errors"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
	"github.com/aseno/backend/pkg/attendance/model"
	"github.com/aseno/backend/pkg/attendance/repository"
	enrollment_repository "github.com/aseno/backend/pkg/enrollment/repository"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
	sessionplan_repository "github.com/aseno/backend/pkg/sessionplan/repository"
)

// AttendanceService defines the interface for attendance operations
type AttendanceService interface {
	RecordAttendance(ctx context.Context, clinicID, appointmentID uuid.UUID, req *model.RecordAttendanceRequest) (*model.AppointmentAttendance, error)
	GetByAppointment(ctx context.Context, clinicID, appointmentID uuid.UUID) ([]*model.AppointmentAttendance, error)
	HasAttendance(ctx context.Context, clinicID, appointmentID uuid.UUID) (bool, error)
	AutoRecordPresent(ctx context.Context, clinicID, appointmentID uuid.UUID) error
	SetPaymentRepo(repo appointment_repository.AppointmentPaymentRepository)
}

type attendanceService struct {
	repo            repository.AttendanceRepository
	appointmentRepo appointment_repository.AppointmentRepository
	planRepo        sessionplan_repository.SessionPlanRepository
	enrollmentRepo  enrollment_repository.EnrollmentRepository
	paymentRepo     appointment_repository.AppointmentPaymentRepository
}

// NewAttendanceService creates a new attendance service
func NewAttendanceService(
	repo repository.AttendanceRepository,
	appointmentRepo appointment_repository.AppointmentRepository,
	planRepo sessionplan_repository.SessionPlanRepository,
	enrollmentRepo enrollment_repository.EnrollmentRepository,
) AttendanceService {
	return &attendanceService{
		repo:            repo,
		appointmentRepo: appointmentRepo,
		planRepo:        planRepo,
		enrollmentRepo:  enrollmentRepo,
	}
}

// SetPaymentRepo sets the payment repository for guest avulso payments
func (s *attendanceService) SetPaymentRepo(repo appointment_repository.AppointmentPaymentRepository) {
	s.paymentRepo = repo
}

// RecordAttendance records attendance for a patient at an appointment.
// When status is "present" and is_guest is false, increments sessions_done on the plan.
func (s *attendanceService) RecordAttendance(ctx context.Context, clinicID, appointmentID uuid.UUID, req *model.RecordAttendanceRequest) (*model.AppointmentAttendance, error) {
	// Validate appointment exists
	apt, err := s.appointmentRepo.GetByID(ctx, appointmentID, clinicID)
	if err != nil {
		return nil, err
	}

	// Validate status
	if req.Status != model.AttendanceStatusPresent &&
		req.Status != model.AttendanceStatusAbsent &&
		req.Status != model.AttendanceStatusSubstituted {
		return nil, errors.ErrValidationInstance.WithMessage("Invalid attendance status")
	}

	// Check if attendance already recorded for this patient
	existing, err := s.repo.GetByAppointmentAndPatient(ctx, clinicID, appointmentID, req.PatientID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		// Update existing record
		oldStatus := existing.Status
		existing.Status = req.Status
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}

		// Handle sessions_done delta
		if apt.SessionPlanID != nil && !req.IsGuest {
			if oldStatus != model.AttendanceStatusPresent && req.Status == model.AttendanceStatusPresent {
				s.incrementSessionsDone(ctx, clinicID, *apt.SessionPlanID, req.PatientID)
			}
		}

		return existing, nil
	}

	// Create new attendance record
	attendance := &model.AppointmentAttendance{
		ClinicID:      clinicID,
		AppointmentID: appointmentID,
		PatientID:     req.PatientID,
		Status:        req.Status,
		IsGuest:       req.IsGuest,
	}

	// If guest with payment info, create avulso payment and link it
	if req.IsGuest && req.PaymentMethodID != nil && req.PaymentAmount != nil && *req.PaymentAmount > 0 && s.paymentRepo != nil {
		payment := &appointment_model.AppointmentPayment{
			ClinicID:        clinicID,
			AppointmentID:   appointmentID,
			PaymentMethodID: *req.PaymentMethodID,
			Amount:          *req.PaymentAmount,
			PaidAt:          time.Now(),
			Notes:           "Pagamento avulso - convidado",
		}
		if err := s.paymentRepo.Create(payment); err != nil {
			return nil, err
		}
		attendance.PaymentID = &payment.ID
	}

	if err := s.repo.Create(ctx, attendance); err != nil {
		return nil, err
	}

	// If present and not guest, increment sessions_done
	if apt.SessionPlanID != nil && req.Status == model.AttendanceStatusPresent && !req.IsGuest {
		s.incrementSessionsDone(ctx, clinicID, *apt.SessionPlanID, req.PatientID)
	}

	return attendance, nil
}

// GetByAppointment retrieves all attendance records for an appointment
func (s *attendanceService) GetByAppointment(ctx context.Context, clinicID, appointmentID uuid.UUID) ([]*model.AppointmentAttendance, error) {
	return s.repo.GetByAppointmentID(ctx, clinicID, appointmentID)
}

// HasAttendance checks if any attendance record exists for an appointment
func (s *attendanceService) HasAttendance(ctx context.Context, clinicID, appointmentID uuid.UUID) (bool, error) {
	records, err := s.repo.GetByAppointmentID(ctx, clinicID, appointmentID)
	if err != nil {
		return false, err
	}
	return len(records) > 0, nil
}

// AutoRecordPresent automatically records a "present" attendance for the appointment's patient.
// Used when an appointment is completed without explicit attendance recording.
func (s *attendanceService) AutoRecordPresent(ctx context.Context, clinicID, appointmentID uuid.UUID) error {
	apt, err := s.appointmentRepo.GetByID(ctx, appointmentID, clinicID)
	if err != nil {
		return err
	}

	// Only auto-record for session plan appointments
	if apt.SessionPlanID == nil {
		return nil
	}

	// Check if attendance already exists
	has, err := s.HasAttendance(ctx, clinicID, appointmentID)
	if err != nil {
		return err
	}
	if has {
		return nil // Already recorded
	}

	// Auto-create present attendance
	_, err = s.RecordAttendance(ctx, clinicID, appointmentID, &model.RecordAttendanceRequest{
		PatientID: apt.PatientID,
		Status:    model.AttendanceStatusPresent,
		IsGuest:   false,
	})
	return err
}

// incrementSessionsDone increments sessions_done on the correct entity and
// auto-completes the plan/enrollment when total sessions are reached.
// - Individual plan: increments plan.sessions_done, completes plan if done
// - Group plan: increments enrollment.sessions_done, completes enrollment if done
func (s *attendanceService) incrementSessionsDone(ctx context.Context, clinicID, sessionPlanID, patientID uuid.UUID) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return
	}

	if plan.Type == sessionplan_model.PlanTypeGroup {
		// Group: increment on the patient's enrollment
		enrollment, err := s.enrollmentRepo.GetByPatientAndPlan(ctx, clinicID, patientID, sessionPlanID)
		if err != nil || enrollment == nil {
			return
		}
		s.enrollmentRepo.IncrementSessionsDone(ctx, clinicID, enrollment.ID)

		// Auto-complete enrollment if total sessions reached
		if enrollment.TotalSessions != nil && *enrollment.TotalSessions > 0 && enrollment.SessionsDone+1 >= *enrollment.TotalSessions {
			s.enrollmentRepo.UpdateStatus(ctx, clinicID, enrollment.ID, "cancelled")
		}
	} else {
		// Individual: increment on the plan
		s.planRepo.IncrementSessionsDone(ctx, clinicID, sessionPlanID)

		// Auto-complete plan if total sessions reached
		if plan.TotalSessions > 0 && plan.SessionsDone+1 >= plan.TotalSessions {
			s.planRepo.UpdateStatus(ctx, clinicID, sessionPlanID, sessionplan_model.PlanStatusCompleted)
		}
	}
}
