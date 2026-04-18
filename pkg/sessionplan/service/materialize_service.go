package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/repository"
)

// MaterializeService handles converting virtual RRULE occurrences into real appointments.
type MaterializeService interface {
	Materialize(ctx context.Context, clinicID uuid.UUID, sessionPlanID uuid.UUID, date string) (*appointment_model.Appointment, error)
	CancelOccurrence(ctx context.Context, clinicID uuid.UUID, sessionPlanID uuid.UUID, date string) (*appointment_model.Appointment, error)
	RescheduleOccurrence(ctx context.Context, clinicID uuid.UUID, sessionPlanID uuid.UUID, originalDate string, newTime time.Time) (*appointment_model.Appointment, error)
}

// ProfessionalResolver is a minimal dependency used to pick a fallback
// professional when an individual plan was created without a fixed one.
type ProfessionalResolver interface {
	FirstActiveIDByClinic(ctx context.Context, clinicID uuid.UUID) (*uuid.UUID, error)
}

type materializeService struct {
	planRepo         repository.SessionPlanRepository
	appointmentRepo  appointment_repository.AppointmentRepository
	professionalRepo ProfessionalResolver
}

// NewMaterializeService creates a new MaterializeService. professionalRepo
// may be nil in tests that don't exercise the "any professional" fallback.
func NewMaterializeService(
	planRepo repository.SessionPlanRepository,
	appointmentRepo appointment_repository.AppointmentRepository,
	professionalRepo ProfessionalResolver,
) MaterializeService {
	return &materializeService{
		planRepo:         planRepo,
		appointmentRepo:  appointmentRepo,
		professionalRepo: professionalRepo,
	}
}

// Materialize creates a real appointment from a virtual occurrence of a session plan.
// Group plans cannot be materialized — they are managed via the attendance flow.
func (s *materializeService) Materialize(ctx context.Context, clinicID uuid.UUID, sessionPlanID uuid.UUID, dateStr string) (*appointment_model.Appointment, error) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	if !plan.IsActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Session plan is not active")
	}

	// Group plans should not be materialized into individual appointments.
	// Group sessions are managed via the attendance flow directly.
	if plan.Type == model.PlanTypeGroup {
		return nil, errors.ErrValidationInstance.WithMessage("Cannot materialize a group plan. Use the group attendance flow instead.")
	}

	// Prevent materialization when session limit has been reached
	if plan.TotalSessions > 0 && plan.SessionsDone >= plan.TotalSessions {
		return nil, errors.ErrValidationInstance.WithMessage("Plan has reached maximum sessions")
	}

	// Parse the target date
	targetDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, errors.ErrValidationInstance.WithMessage("Invalid date format. Use YYYY-MM-DD")
	}

	// Verify this date is a valid occurrence of the plan's RRULE
	rruleStr := plan.RRule
	if rruleStr == "" {
		rruleStr = FrequencyToRRule(plan.Frequency, plan.WeekDays)
	}

	dtstart := plan.StartDate
	if plan.DtStart != nil {
		dtstart = *plan.DtStart
	}

	// Calculate occurrences for just this day
	dayStart := targetDate
	dayEnd := targetDate.AddDate(0, 0, 1)
	var occurrences []time.Time
	if len(plan.DayTimes) > 0 {
		occurrences, err = CalculateOccurrencesWithDayTimes(rruleStr, dtstart, plan.DayTimes, dayStart, dayEnd, plan.TotalSessions, plan.SessionsDone)
	} else {
		occurrences, err = CalculateOccurrences(rruleStr, dtstart, dayStart, dayEnd, plan.TotalSessions, plan.SessionsDone)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to calculate occurrences: %w", err)
	}

	if len(occurrences) == 0 {
		return nil, errors.ErrValidationInstance.WithMessage("No session scheduled for this date")
	}

	sessionStart := occurrences[0]
	sessionEnd := sessionStart.Add(time.Duration(plan.DurationMins) * time.Minute)

	// Check if already materialized for this date
	existingAppts, err := s.appointmentRepo.GetByDateRange(ctx, clinicID, dayStart, dayEnd, plan.ProfessionalID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing appointments: %w", err)
	}

	for _, apt := range existingAppts {
		if apt.SessionPlanID != nil && *apt.SessionPlanID == sessionPlanID {
			// Already materialized — return the existing one
			return apt, nil
		}
	}

	// Determine professional. Flexible individual plans ("qualquer profissional")
	// arrive here with a nil ProfessionalID — fall back to the first active
	// professional of the clinic so the user isn't blocked. They can change
	// the professional on the appointment after it opens.
	var professionalID uuid.UUID
	if plan.ProfessionalID != nil {
		professionalID = *plan.ProfessionalID
	} else if s.professionalRepo != nil {
		fallback, err := s.professionalRepo.FirstActiveIDByClinic(ctx, clinicID)
		if err != nil {
			return nil, err
		}
		if fallback == nil {
			return nil, errors.ErrValidationInstance.WithMessage("Nenhum profissional ativo encontrado na clínica para atribuir a este plano.")
		}
		professionalID = *fallback
	} else {
		return nil, errors.ErrValidationInstance.WithMessage("Nenhum profissional ativo encontrado na clínica para atribuir a este plano.")
	}

	// Determine price
	price := plan.GetEffectivePrice()

	// Create the appointment
	appointment := &appointment_model.Appointment{
		ClinicID:       clinicID,
		PatientID:      plan.PatientID,
		ProfessionalID: professionalID,
		SessionPlanID:  &plan.ID,
		StartTime:      sessionStart,
		EndTime:        sessionEnd,
		Status:         appointment_model.AppointmentStatusPending,
		Type:           appointment_model.AppointmentTypeRecurring,
		Title:          plan.Title,
		Price:          price,
	}

	// Transfer procedure from session plan to appointment
	if plan.ProcedureID != nil {
		appointment.Procedures = []appointment_model.AppointmentProcedure{
			{
				ProcedureID: *plan.ProcedureID,
				Price:       price,
			},
		}
	}

	if err := s.appointmentRepo.Create(ctx, appointment); err != nil {
		return nil, fmt.Errorf("failed to create appointment: %w", err)
	}

	// Reload with relations
	return s.appointmentRepo.GetByID(ctx, appointment.ID, clinicID)
}

// CancelOccurrence materializes a virtual occurrence and immediately marks it as cancelled exception.
// This prevents the RRULE from generating a virtual event for this date.
func (s *materializeService) CancelOccurrence(ctx context.Context, clinicID uuid.UUID, sessionPlanID uuid.UUID, dateStr string) (*appointment_model.Appointment, error) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	if !plan.IsActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Session plan is not active")
	}

	targetDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, errors.ErrValidationInstance.WithMessage("Invalid date format. Use YYYY-MM-DD")
	}

	// Verify this date is a valid occurrence
	rruleStr := plan.RRule
	if rruleStr == "" {
		rruleStr = FrequencyToRRule(plan.Frequency, plan.WeekDays)
	}

	dtstart := plan.StartDate
	if plan.DtStart != nil {
		dtstart = *plan.DtStart
	}

	dayStart := targetDate
	dayEnd := targetDate.AddDate(0, 0, 1)
	var occurrences []time.Time
	if len(plan.DayTimes) > 0 {
		occurrences, err = CalculateOccurrencesWithDayTimes(rruleStr, dtstart, plan.DayTimes, dayStart, dayEnd, plan.TotalSessions, plan.SessionsDone)
	} else {
		occurrences, err = CalculateOccurrences(rruleStr, dtstart, dayStart, dayEnd, plan.TotalSessions, plan.SessionsDone)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to calculate occurrences: %w", err)
	}

	if len(occurrences) == 0 {
		return nil, errors.ErrValidationInstance.WithMessage("No session scheduled for this date")
	}

	sessionStart := occurrences[0]
	sessionEnd := sessionStart.Add(time.Duration(plan.DurationMins) * time.Minute)

	// Check if already materialized
	existingAppts, err := s.appointmentRepo.GetByDateRange(ctx, clinicID, dayStart, dayEnd, plan.ProfessionalID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing appointments: %w", err)
	}

	for _, apt := range existingAppts {
		if apt.SessionPlanID != nil && *apt.SessionPlanID == sessionPlanID {
			// Already materialized — just cancel it and mark as exception
			apt.Status = appointment_model.AppointmentStatusCanceled
			apt.IsException = true
			apt.OriginalDate = &sessionStart
			now := time.Now()
			apt.CanceledAt = &now
			if err := s.appointmentRepo.Update(ctx, apt); err != nil {
				return nil, fmt.Errorf("failed to cancel existing appointment: %w", err)
			}
			return apt, nil
		}
	}

	var professionalID uuid.UUID
	if plan.ProfessionalID != nil {
		professionalID = *plan.ProfessionalID
	} else {
		return nil, errors.ErrValidationInstance.WithMessage("Cannot cancel occurrence without a professional assigned")
	}

	// Create a cancelled exception appointment
	appointment := &appointment_model.Appointment{
		ClinicID:       clinicID,
		PatientID:      plan.PatientID,
		ProfessionalID: professionalID,
		SessionPlanID:  &plan.ID,
		StartTime:      sessionStart,
		EndTime:        sessionEnd,
		Status:         appointment_model.AppointmentStatusCanceled,
		Type:           appointment_model.AppointmentTypeRecurring,
		Title:          plan.Title,
		IsException:    true,
		OriginalDate:   &sessionStart,
	}

	now := time.Now()
	appointment.CanceledAt = &now

	if err := s.appointmentRepo.Create(ctx, appointment); err != nil {
		return nil, fmt.Errorf("failed to create cancelled exception: %w", err)
	}

	return s.appointmentRepo.GetByID(ctx, appointment.ID, clinicID)
}

// RescheduleOccurrence materializes a virtual occurrence at a new time, marking it as exception.
// The original date is preserved so the RRULE knows this occurrence was moved.
func (s *materializeService) RescheduleOccurrence(ctx context.Context, clinicID uuid.UUID, sessionPlanID uuid.UUID, originalDateStr string, newTime time.Time) (*appointment_model.Appointment, error) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	if !plan.IsActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Session plan is not active")
	}

	if plan.TotalSessions > 0 && plan.SessionsDone >= plan.TotalSessions {
		return nil, errors.ErrValidationInstance.WithMessage("Plan has reached maximum sessions")
	}

	originalDate, err := time.Parse("2006-01-02", originalDateStr)
	if err != nil {
		return nil, errors.ErrValidationInstance.WithMessage("Invalid date format. Use YYYY-MM-DD")
	}

	// Verify the original date is a valid occurrence
	rruleStr := plan.RRule
	if rruleStr == "" {
		rruleStr = FrequencyToRRule(plan.Frequency, plan.WeekDays)
	}

	dtstart := plan.StartDate
	if plan.DtStart != nil {
		dtstart = *plan.DtStart
	}

	dayStart := originalDate
	dayEnd := originalDate.AddDate(0, 0, 1)
	var occurrences []time.Time
	if len(plan.DayTimes) > 0 {
		occurrences, err = CalculateOccurrencesWithDayTimes(rruleStr, dtstart, plan.DayTimes, dayStart, dayEnd, plan.TotalSessions, plan.SessionsDone)
	} else {
		occurrences, err = CalculateOccurrences(rruleStr, dtstart, dayStart, dayEnd, plan.TotalSessions, plan.SessionsDone)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to calculate occurrences: %w", err)
	}

	if len(occurrences) == 0 {
		return nil, errors.ErrValidationInstance.WithMessage("No session scheduled for the original date")
	}

	originalStart := occurrences[0]
	newEnd := newTime.Add(time.Duration(plan.DurationMins) * time.Minute)

	// Check if already materialized for the original date
	existingAppts, err := s.appointmentRepo.GetByDateRange(ctx, clinicID, dayStart, dayEnd, plan.ProfessionalID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing appointments: %w", err)
	}

	for _, apt := range existingAppts {
		if apt.SessionPlanID != nil && *apt.SessionPlanID == sessionPlanID {
			// Already materialized — update it
			apt.IsException = true
			apt.OriginalDate = &originalStart
			apt.StartTime = newTime
			apt.EndTime = newEnd
			if err := s.appointmentRepo.Update(ctx, apt); err != nil {
				return nil, fmt.Errorf("failed to update existing appointment: %w", err)
			}
			return apt, nil
		}
	}

	var professionalID uuid.UUID
	if plan.ProfessionalID != nil {
		professionalID = *plan.ProfessionalID
	} else {
		return nil, errors.ErrValidationInstance.WithMessage("Cannot reschedule without a professional assigned")
	}

	price := plan.GetEffectivePrice()

	// Create a rescheduled exception appointment
	appointment := &appointment_model.Appointment{
		ClinicID:       clinicID,
		PatientID:      plan.PatientID,
		ProfessionalID: professionalID,
		SessionPlanID:  &plan.ID,
		StartTime:      newTime,
		EndTime:        newEnd,
		Status:         appointment_model.AppointmentStatusPending,
		Type:           appointment_model.AppointmentTypeRecurring,
		Title:          plan.Title,
		Price:          price,
		IsException:    true,
		OriginalDate:   &originalStart,
	}

	// Transfer procedure from session plan to appointment
	if plan.ProcedureID != nil {
		appointment.Procedures = []appointment_model.AppointmentProcedure{
			{
				ProcedureID: *plan.ProcedureID,
				Price:       price,
			},
		}
	}

	if err := s.appointmentRepo.Create(ctx, appointment); err != nil {
		return nil, fmt.Errorf("failed to create rescheduled exception: %w", err)
	}

	return s.appointmentRepo.GetByID(ctx, appointment.ID, clinicID)
}
