package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
	enrollmentModel "github.com/aseno/backend/pkg/enrollment/model"
	enrollment_repository "github.com/aseno/backend/pkg/enrollment/repository"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/repository"
)

// CalendarFacade provides calendar events by merging virtual RRULE-calculated
// occurrences with materialized (real) appointments.
type CalendarFacade interface {
	GetCalendarEvents(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, rangeStart, rangeEnd time.Time) ([]*model.CalendarEvent, error)
	// SetSlotRepo wires the slot repository to enable slot-based rendering of
	// group plans. Optional — when not set, group plans use the legacy
	// DayTimes/WeekDays path.
	SetSlotRepo(slotRepo repository.SessionPlanSlotRepository)
}

type calendarFacade struct {
	planRepo        repository.SessionPlanRepository
	appointmentRepo appointment_repository.AppointmentRepository
	enrollmentRepo  enrollment_repository.EnrollmentRepository
	slotRepo        repository.SessionPlanSlotRepository // optional; used for slot-based group plans
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

// SetSlotRepo wires the slot repository so the facade can render slot-based
// group plans. Optional — when nil, group plans fall back to DayTimes/WeekDays.
func (f *calendarFacade) SetSlotRepo(slotRepo repository.SessionPlanSlotRepository) {
	f.slotRepo = slotRepo
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
		// Soft-deleted cancelled exceptions are suppression-only tombstones
		// (the user deleted the appointment and expects it gone). They must
		// still populate the materialization map so the RRULE does not
		// re-materialize the slot, but they must NOT render as calendar
		// events.
		isTombstone := apt.DeletedAt != nil && apt.IsException && apt.Status == appointment_model.AppointmentStatusCanceled
		if !isTombstone {
			event := realAppointmentToEvent(apt)
			events = append(events, event)
		}

		// Track materialized dates per session plan (tombstones included —
		// that is the whole point of keeping the row around).
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

		dtstart := plan.StartDate
		if plan.DtStart != nil {
			dtstart = *plan.DtStart
		}

		// For group plans with slots, render one event per slot occurrence.
		// This is the new path — multiple times per weekday, capacity per slot.
		if plan.Type == model.PlanTypeGroup && f.slotRepo != nil {
			slots, slotErr := f.slotRepo.ListActiveByPlan(ctx, plan.ID)
			if slotErr == nil && len(slots) > 0 {
				slotEvents, err := f.renderSlotBasedEvents(ctx, clinicID, plan, slots, dtstart, rangeStart, rangeEnd, materializedByPlan[plan.ID.String()])
				if err == nil {
					events = append(events, slotEvents...)
					continue
				}
				// On error, fall through to legacy path so the plan still renders.
			}
		}

		// Legacy path: DayTimes/WeekDays-based rendering.
		rruleStr := plan.RRule
		if rruleStr == "" {
			rruleStr = FrequencyToRRule(plan.Frequency, plan.WeekDays)
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

		// For group plans, load active enrollments once so we can compute per-day counts.
		var groupEnrollments []*enrollmentModel.SessionPlanEnrollment
		if plan.Type == model.PlanTypeGroup {
			groupEnrollments, _ = f.enrollmentRepo.GetActiveByPlan(ctx, clinicID, plan.ID)
		}

		// For each occurrence, check if already materialized
		planDates := materializedByPlan[plan.ID.String()]
		for _, occ := range occurrences {
			dateKey := occ.Format("2006-01-02")
			if planDates != nil && planDates[dateKey] {
				continue // Already materialized, skip virtual
			}

			event := virtualOccurrenceToEvent(plan, occ)
			if plan.Type == model.PlanTypeGroup {
				dow := int(occ.Weekday())
				count := 0
				for _, e := range groupEnrollments {
					if e.HasWeekDay(dow) {
						count++
					}
				}
				event.EnrolledCount = count
			}
			events = append(events, event)
		}
	}

	return events, nil
}

// renderSlotBasedEvents returns one CalendarEvent per slot occurrence in range.
// Capacity is per-slot. EnrolledCount counts enrollments that include the slot.
// ProfessionalID falls back to the plan's professional when the slot does not
// override it.
//
// IMPORTANT: virtual slot events are only emitted for dates >= today (in
// clinic timezone). Past dates depend on real attendance records — re-rendering
// past virtuals from current slot state would retroactively rewrite history
// when, e.g., the operator changes a slot's professional.
func (f *calendarFacade) renderSlotBasedEvents(
	ctx context.Context,
	clinicID uuid.UUID,
	plan *model.SessionPlan,
	slots []*model.SessionPlanSlot,
	dtstart, rangeStart, rangeEnd time.Time,
	materializedDates map[string]bool,
) ([]*model.CalendarEvent, error) {
	// Convert []*Slot to []Slot value for the rrule helper signature.
	values := make([]model.SessionPlanSlot, 0, len(slots))
	for _, s := range slots {
		values = append(values, *s)
	}

	occs, err := CalculateOccurrencesFromSlots(values, dtstart, rangeStart, rangeEnd, plan.TotalSessions, plan.SessionsDone)
	if err != nil {
		return nil, err
	}

	// Cut off virtual events at the start of today in clinic timezone, so
	// changes to slot.professional_id (or capacity) never rewrite past events.
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	nowLocal := time.Now().In(loc)
	startOfToday := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	filtered := occs[:0]
	for _, o := range occs {
		if o.StartTime.Before(startOfToday) {
			continue
		}
		filtered = append(filtered, o)
	}
	occs = filtered

	// Index slots by ID for quick lookup.
	slotByID := make(map[uuid.UUID]*model.SessionPlanSlot, len(slots))
	for _, s := range slots {
		slotByID[s.ID] = s
	}

	// Pre-compute per-slot enrolled counts so we don't hit the DB per occurrence.
	enrolledBySlot := make(map[uuid.UUID]int, len(slots))
	for _, s := range slots {
		c, err := f.enrollmentRepo.CountActiveBySlot(ctx, clinicID, plan.ID, s.ID)
		if err != nil {
			continue
		}
		enrolledBySlot[s.ID] = int(c)
	}

	events := make([]*model.CalendarEvent, 0, len(occs))
	for _, occ := range occs {
		dateKey := occ.StartTime.Format("2006-01-02")
		if materializedDates != nil && materializedDates[dateKey] {
			continue
		}
		slot := slotByID[occ.SlotID]
		if slot == nil {
			continue
		}
		event := virtualOccurrenceToEvent(plan, occ.StartTime)
		event.SlotID = &slot.ID
		event.EnrolledCount = enrolledBySlot[slot.ID]
		// Slot-level capacity overrides plan-level capacity.
		cap := slot.Capacity
		event.Capacity = &cap
		// Professional override takes precedence.
		if slot.ProfessionalID != nil {
			event.ProfessionalID = slot.ProfessionalID
			event.ProfessionalName = "" // caller can populate later if needed
		}
		events = append(events, event)
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
