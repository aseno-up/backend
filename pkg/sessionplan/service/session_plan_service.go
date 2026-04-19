package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/aseno/backend/internal/errors"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/repository"
)

// SessionPlanService defines the interface for session plan operations
type SessionPlanService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateSessionPlanRequest) (*model.SessionPlan, error)
	GetByID(ctx context.Context, clinicID, id uuid.UUID) (*model.SessionPlan, error)
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.SessionPlanResponse], error)
	GetByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.SessionPlan, error)
	Update(ctx context.Context, clinicID, id uuid.UUID, req *model.UpdateSessionPlanRequest) (*model.SessionPlan, error)
	Pause(ctx context.Context, clinicID, id uuid.UUID) error
	Resume(ctx context.Context, clinicID, id uuid.UUID) error
	Cancel(ctx context.Context, clinicID, id uuid.UUID) error
	Delete(ctx context.Context, clinicID, id uuid.UUID) error
	BulkCancel(ctx context.Context, clinicID uuid.UUID, ids []uuid.UUID) error
	OnAppointmentCompleted(ctx context.Context, clinicID uuid.UUID, appointmentID uuid.UUID) error
}

// sessionPlanService implements SessionPlanService
type sessionPlanService struct {
	repo            repository.SessionPlanRepository
	appointmentRepo appointment_repository.AppointmentRepository
}

// NewSessionPlanService creates a new session plan service
func NewSessionPlanService(
	repo repository.SessionPlanRepository,
	appointmentRepo appointment_repository.AppointmentRepository,
) SessionPlanService {
	return &sessionPlanService{
		repo:            repo,
		appointmentRepo: appointmentRepo,
	}
}

// Create creates a new session plan and generates initial appointments
func (s *sessionPlanService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateSessionPlanRequest) (*model.SessionPlan, error) {
	// Parse start date in clinic timezone so weekday calculation is correct
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	if loc == nil {
		loc = time.UTC
	}
	startDate, err := time.ParseInLocation("2006-01-02", req.StartDate, loc)
	if err != nil {
		return nil, errors.ErrValidationInstance.WithMessage("Invalid start date format. Use YYYY-MM-DD")
	}

	// Procedure is required for commission calculation
	if req.ProcedureID == nil {
		return nil, errors.ErrValidationInstance.WithMessage("Procedimento é obrigatório para criar um plano de sessão.")
	}

	// Validate frequency and week days
	if req.Frequency == model.FrequencyCustom && len(req.WeekDays) == 0 {
		return nil, errors.ErrValidationInstance.WithMessage("Week days are required for custom frequency")
	}

	// Set default week days based on frequency and start date
	var weekDays []int
	if len(req.WeekDays) > 0 {
		weekDays = req.WeekDays
	} else {
		weekDays = s.getDefaultWeekDays(req.Frequency, startDate)
	}

	// Convert to pq.Int64Array
	var pqWeekDays pq.Int64Array
	for _, wd := range weekDays {
		pqWeekDays = append(pqWeekDays, int64(wd))
	}

	// Build RRULE: use provided rrule or compute from legacy frequency
	rruleStr := req.RRule
	if rruleStr == "" {
		rruleStr = FrequencyToRRule(req.Frequency, pqWeekDays)
	} else {
		if err := ValidateRRule(rruleStr); err != nil {
			return nil, errors.ErrValidationInstance.WithMessage("Invalid RRULE: " + err.Error())
		}
	}

	// Build DtStart from start_date + preferred_time
	dtstart, err := BuildDtStart(req.StartDate, req.PreferredTime)
	if err != nil {
		return nil, errors.ErrValidationInstance.WithMessage("Invalid start date/time combination")
	}

	// Default plan type
	planType := req.Type
	if planType == "" {
		planType = model.PlanTypeIndividual
	}

	// Default payment mode
	paymentMode := req.PaymentMode
	if paymentMode == "" {
		paymentMode = model.PaymentModePerSession
	}

	// BillingDay is mandatory for individual monthly plans so the payment
	// reminder worker can compute the due date. For group plans, billing
	// day lives on the enrollment instead, so it must not be set here.
	if planType == model.PlanTypeIndividual && paymentMode == model.PaymentModeMonthly {
		if req.BillingDay == nil {
			return nil, errors.ErrValidationInstance.WithMessage("billing_day is required for individual monthly plans")
		}
		if *req.BillingDay < 1 || *req.BillingDay > 31 {
			return nil, errors.ErrValidationInstance.WithMessage("billing_day must be between 1 and 31")
		}
	}

	// Calculate generate_until (2 weeks ahead by default)
	generateUntil := time.Now().AddDate(0, 0, 14)

	plan := &model.SessionPlan{
		ClinicID:       clinicID,
		PatientID:      req.PatientID,
		ProfessionalID: req.ProfessionalID,
		Title:          req.Title,
		Description:    req.Description,
		Status:         model.PlanStatusActive,
		Type:           planType,
		RRule:          rruleStr,
		DtStart:        &dtstart,
		DayTimes:       req.DayTimes,
		ProcedureID:    req.ProcedureID,
		PriceOverride:  req.PriceOverride,
		Capacity:       req.Capacity,
		PaymentMode:    paymentMode,
		Frequency:      req.Frequency,
		WeekDays:       pqWeekDays,
		PreferredTime:  req.PreferredTime,
		DurationMins:   req.DurationMins,
		TotalSessions:  req.TotalSessions,
		SessionsDone:   0,
		StartDate:      startDate,
		GenerateUntil:  generateUntil,
		SessionPrice:   req.SessionPrice,
		BillingDay:     req.BillingDay,
		Notes:          req.Notes,
	}

	if err := s.repo.Create(ctx, plan); err != nil {
		return nil, err
	}

	// No appointments are pre-generated. Occurrences are calculated on demand
	// via RRULE when the calendar is loaded (CalendarFacade).
	// Appointments are only materialized when the user interacts with them.

	return s.repo.GetByID(ctx, clinicID, plan.ID)
}

// GetByID retrieves a session plan by ID
func (s *sessionPlanService) GetByID(ctx context.Context, clinicID, id uuid.UUID) (*model.SessionPlan, error) {
	return s.repo.GetByID(ctx, clinicID, id)
}

// List retrieves all session plans for a clinic
func (s *sessionPlanService) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.SessionPlanResponse], error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 10
	}

	plans, total, err := s.repo.GetByClinic(ctx, clinicID, filter)
	if err != nil {
		return pagination.Response[*model.SessionPlanResponse]{}, err
	}

	data := make([]*model.SessionPlanResponse, len(plans))
	for i, plan := range plans {
		data[i] = plan.ToResponse()
	}

	totalPages := int((total + int64(filter.Limit) - 1) / int64(filter.Limit))

	return pagination.Response[*model.SessionPlanResponse]{
		Data: data,
		Meta: pagination.Meta{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// GetByPatient retrieves all session plans for a patient
func (s *sessionPlanService) GetByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.SessionPlan, error) {
	return s.repo.GetByPatient(ctx, clinicID, patientID)
}

// Update updates a session plan
func (s *sessionPlanService) Update(ctx context.Context, clinicID, id uuid.UUID, req *model.UpdateSessionPlanRequest) (*model.SessionPlan, error) {
	plan, err := s.repo.GetByID(ctx, clinicID, id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		plan.Title = *req.Title
	}
	if req.Description != nil {
		plan.Description = *req.Description
	}
	if req.PreferredTime != nil {
		plan.PreferredTime = *req.PreferredTime
	}
	if req.DurationMins != nil {
		plan.DurationMins = *req.DurationMins
	}
	if req.SessionPrice != nil {
		plan.SessionPrice = *req.SessionPrice
	}
	if req.Notes != nil {
		plan.Notes = *req.Notes
	}
	if req.ProfessionalID != nil {
		plan.ProfessionalID = req.ProfessionalID
	}
	if req.Capacity != nil {
		plan.Capacity = req.Capacity
	}
	if req.DayTimes != nil {
		plan.DayTimes = *req.DayTimes
	}
	if req.BillingDay != nil {
		if *req.BillingDay < 1 || *req.BillingDay > 31 {
			return nil, errors.ErrValidationInstance.WithMessage("billing_day must be between 1 and 31")
		}
		plan.BillingDay = req.BillingDay
	}

	// Recalculate RRULE if frequency or week_days changed
	recalcRRule := false
	if req.Frequency != nil {
		plan.Frequency = *req.Frequency
		recalcRRule = true
	}
	if req.WeekDays != nil {
		weekDays := make(pq.Int64Array, len(*req.WeekDays))
		for i, d := range *req.WeekDays {
			weekDays[i] = int64(d)
		}
		plan.WeekDays = weekDays
		recalcRRule = true
	}
	if recalcRRule {
		plan.RRule = FrequencyToRRule(plan.Frequency, plan.WeekDays)
	}

	if err := s.repo.Update(ctx, plan); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, clinicID, id)
}

// Pause pauses a session plan
func (s *sessionPlanService) Pause(ctx context.Context, clinicID, id uuid.UUID) error {
	plan, err := s.repo.GetByID(ctx, clinicID, id)
	if err != nil {
		return err
	}

	if plan.Status != model.PlanStatusActive {
		return errors.ErrValidationInstance.WithMessage("Only active plans can be paused")
	}

	return s.repo.UpdateStatus(ctx, clinicID, id, model.PlanStatusPaused)
}

// Resume resumes a paused session plan
func (s *sessionPlanService) Resume(ctx context.Context, clinicID, id uuid.UUID) error {
	plan, err := s.repo.GetByID(ctx, clinicID, id)
	if err != nil {
		return err
	}

	if plan.Status != model.PlanStatusPaused {
		return errors.ErrValidationInstance.WithMessage("Only paused plans can be resumed")
	}

	return s.repo.UpdateStatus(ctx, clinicID, id, model.PlanStatusActive)
}

// Cancel cancels a session plan
func (s *sessionPlanService) Cancel(ctx context.Context, clinicID, id uuid.UUID) error {
	plan, err := s.repo.GetByID(ctx, clinicID, id)
	if err != nil {
		return err
	}

	if plan.Status == model.PlanStatusCancelled || plan.Status == model.PlanStatusCompleted {
		return errors.ErrValidationInstance.WithMessage("Plan is already cancelled or completed")
	}

	return s.repo.UpdateStatus(ctx, clinicID, id, model.PlanStatusCancelled)
}

// Delete deletes a session plan
func (s *sessionPlanService) Delete(ctx context.Context, clinicID, id uuid.UUID) error {
	return s.repo.Delete(ctx, clinicID, id)
}

// BulkCancel cancels multiple session plans
func (s *sessionPlanService) BulkCancel(ctx context.Context, clinicID uuid.UUID, ids []uuid.UUID) error {
	return s.repo.BulkCancel(ctx, clinicID, ids)
}

// OnAppointmentCompleted handles when an appointment from a session plan is completed
func (s *sessionPlanService) OnAppointmentCompleted(ctx context.Context, clinicID uuid.UUID, appointmentID uuid.UUID) error {
	// Get the appointment to find its session plan
	appointment, err := s.appointmentRepo.GetByID(ctx, appointmentID, clinicID)
	if err != nil {
		return err
	}

	if appointment.SessionPlanID == nil {
		return nil // Not part of a session plan
	}

	// Increment sessions done
	return s.repo.IncrementSessionsDone(ctx, clinicID, *appointment.SessionPlanID)
}

// getDefaultWeekDays returns default week days based on frequency and start date.
// For weekly, uses the start date's weekday so sessions start on the chosen day.
func (s *sessionPlanService) getDefaultWeekDays(frequency model.Frequency, startDate time.Time) []int {
	startDay := int(startDate.Weekday()) // 0=Sun, 1=Mon, ..., 6=Sat
	switch frequency {
	case model.FrequencyDaily:
		return []int{1, 2, 3, 4, 5}
	case model.FrequencyWeekly:
		return []int{startDay}
	case model.FrequencyBiweekly:
		// Start day + 3 days later (wrapping within weekdays)
		secondDay := (startDay + 3) % 7
		return []int{startDay, secondDay}
	case model.FrequencyTriweekly:
		d2 := (startDay + 2) % 7
		d3 := (startDay + 4) % 7
		return []int{startDay, d2, d3}
	default:
		return []int{startDay}
	}
}

// parseTime parses a time string (HH:MM) and returns hour and minute
func (s *sessionPlanService) parseTime(timeStr string) (int, int) {
	var hour, min int
	fmt.Sscanf(timeStr, "%d:%d", &hour, &min)
	return hour, min
}
