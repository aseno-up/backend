package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
	enrollment_repository "github.com/aseno/backend/pkg/enrollment/repository"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/repository"
)

// CalendarFacade provides calendar events by merging virtual RRULE-calculated
// occurrences with materialized (real) appointments.
type CalendarFacade interface {
	GetCalendarEvents(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, rangeStart, rangeEnd time.Time) ([]*model.CalendarEvent, error)
}

type calendarFacade struct {
	planRepo        repository.SessionPlanRepository
	appointmentRepo appointment_repository.AppointmentRepository
	enrollmentRepo  enrollment_repository.EnrollmentRepository
}

// NewCalendarFacade creates a new CalendarFacade
func NewCalendarFacade(
	planRepo repository.SessionPlanRepository,
	appointmentRepo appointment_repository.AppointmentRepository,
	enrollmentRepo enrollment_repository.EnrollmentRepository,
) CalendarFacade {
	return &calendarFacade{
		planRepo:        planRepo,
		appointmentRepo: appointmentRepo,
		enrollmentRepo:  enrollmentRepo,
	}
}

// GetCalendarEvents returns a merged list of virtual and real appointments for the calendar.
func (f *calendarFacade) GetCalendarEvents(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, rangeStart, rangeEnd time.Time) ([]*model.CalendarEvent, error) {
	// 1. Fetch all real appointments in the range, including cancelled exceptions
	// so we can suppress virtual events for dates with cancelled exceptions.
	realAppointments, err := f.appointmentRepo.GetByDateRangeWithExceptions(ctx, clinicID, rangeStart, rangeEnd, professionalID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch appointments: %w", err)
	}

	// 2. Convert real appointments to CalendarEvents
	events := make([]*model.CalendarEvent, 0, len(realAppointments))
	materializedByPlan := make(map[string]map[string]bool) // planID -> date -> true

	for _, apt := range realAppointments {
		event := realAppointmentToEvent(apt)
		events = append(events, event)

		// Track materialized dates per session plan
		if apt.SessionPlanID != nil {
			planID := apt.SessionPlanID.String()
			if materializedByPlan[planID] == nil {
				materializedByPlan[planID] = make(map[string]bool)
			}
			dateKey := apt.StartTime.Format("2006-01-02")
			materializedByPlan[planID][dateKey] = true

			// For exception appointments, also track by OriginalDate so the
			// virtual occurrence for the original date is suppressed.
			if apt.IsException && apt.OriginalDate != nil {
				origKey := apt.OriginalDate.Format("2006-01-02")
				materializedByPlan[planID][origKey] = true
			}
		}
	}

	// 3. Fetch active session plans
	var plans []*model.SessionPlan
	if professionalID != nil {
		plans, err = f.planRepo.GetByProfessional(ctx, clinicID, *professionalID)
	} else {
		plans, err = f.planRepo.GetAllActive(ctx, clinicID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch session plans: %w", err)
	}

	// 4. For each active plan, calculate virtual occurrences
	for _, plan := range plans {
		if !plan.IsActive() {
			continue
		}

		rruleStr := plan.RRule
		if rruleStr == "" {
			rruleStr = FrequencyToRRule(plan.Frequency, plan.WeekDays)
		}

		dtstart := plan.StartDate
		if plan.DtStart != nil {
			dtstart = *plan.DtStart
		}

		var occurrences []time.Time
		if len(plan.DayTimes) > 0 {
			occurrences, err = CalculateOccurrencesWithDayTimes(rruleStr, dtstart, plan.DayTimes, rangeStart, rangeEnd, plan.TotalSessions, plan.SessionsDone)
		} else {
			occurrences, err = CalculateOccurrences(rruleStr, dtstart, rangeStart, rangeEnd, plan.TotalSessions, plan.SessionsDone)
		}
		if err != nil {
			continue // Skip plans with invalid RRULEs
		}

		// For group plans, get enrolled count once
		var enrolledCount int
		if plan.Type == model.PlanTypeGroup {
			count, _ := f.enrollmentRepo.CountActiveByPlan(ctx, clinicID, plan.ID)
			enrolledCount = int(count)
		}

		// For each occurrence, check if already materialized
		planDates := materializedByPlan[plan.ID.String()]
		for _, occ := range occurrences {
			dateKey := occ.Format("2006-01-02")
			if planDates != nil && planDates[dateKey] {
				continue // Already materialized, skip virtual
			}

			event := virtualOccurrenceToEvent(plan, occ)
			event.EnrolledCount = enrolledCount
			events = append(events, event)
		}
	}

	return events, nil
}

// realAppointmentToEvent converts a real Appointment to a CalendarEvent
func realAppointmentToEvent(apt *appointment_model.Appointment) *model.CalendarEvent {
	event := &model.CalendarEvent{
		IsVirtual:      false,
		AppointmentID:  &apt.ID,
		SessionPlanID:  apt.SessionPlanID,
		StartTime:      apt.StartTime,
		EndTime:        apt.EndTime,
		Title:          apt.Title,
		PatientID:      &apt.PatientID,
		ProfessionalID: &apt.ProfessionalID,
		Status:         string(apt.Status),
		Type:           string(apt.Type),
		Notes:          apt.Notes,
		PatientNotes:   apt.PatientNotes,
		Price:          apt.Price,
		PaymentStatus:  string(apt.PaymentStatus),
		IsException:    apt.IsException,
	}

	if apt.Patient != nil {
		event.PatientName = apt.Patient.Name
	}
	if apt.Professional != nil {
		event.ProfessionalName = apt.Professional.Name
		event.ProfessionalColor = apt.Professional.Color
	}
	if len(apt.Procedures) > 0 && apt.Procedures[0].Procedure != nil {
		event.ProcedureName = apt.Procedures[0].Procedure.Name
	}

	return event
}

// virtualOccurrenceToEvent creates a virtual CalendarEvent from a plan occurrence
func virtualOccurrenceToEvent(plan *model.SessionPlan, occurrence time.Time) *model.CalendarEvent {
	endTime := occurrence.Add(time.Duration(plan.DurationMins) * time.Minute)

	event := &model.CalendarEvent{
		IsVirtual:        true,
		SessionPlanID:    &plan.ID,
		StartTime:        occurrence,
		EndTime:          endTime,
		Title:            plan.Title,
		ProfessionalID:   plan.ProfessionalID,
		PlanType:         plan.Type,
		DurationMins:     plan.DurationMins,
		VirtualDate:      occurrence.Format("2006-01-02"),
		ProfessionalName: plan.ProfessionalName,
		ProcedureName:    plan.ProcedureName,
	}

	if plan.Type == model.PlanTypeIndividual {
		event.PatientID = &plan.PatientID
		event.PatientName = plan.PatientName
	}

	if plan.Capacity != nil {
		event.Capacity = plan.Capacity
	}

	return event
}
