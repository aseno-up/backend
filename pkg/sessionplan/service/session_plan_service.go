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

// SessionPlanService defines the interface for session plan operations
type SessionPlanService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateSessionPlanRequest) (*model.SessionPlan, error)
	GetByID(ctx context.Context, clinicID, id uuid.UUID) (*model.SessionPlan, error)
	List(ctx context.Context, clinicID uuid.UUID, page, perPage int, status *model.PlanStatus) ([]*model.SessionPlan, int64, error)
	GetByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.SessionPlan, error)
	Update(ctx context.Context, clinicID, id uuid.UUID, req *model.UpdateSessionPlanRequest) (*model.SessionPlan, error)
	Pause(ctx context.Context, clinicID, id uuid.UUID) error
	Resume(ctx context.Context, clinicID, id uuid.UUID) error
	Cancel(ctx context.Context, clinicID, id uuid.UUID) error
	Delete(ctx context.Context, clinicID, id uuid.UUID) error
	GenerateAppointments(ctx context.Context, plan *model.SessionPlan, weeksAhead int) (int, error)
	GenerateAllPending(ctx context.Context, weeksAhead int) error
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
	// Parse start date
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, errors.ErrValidationInstance.WithMessage("Invalid start date format. Use YYYY-MM-DD")
	}

	// Validate frequency and week days
	if req.Frequency == model.FrequencyCustom && len(req.WeekDays) == 0 {
		return nil, errors.ErrValidationInstance.WithMessage("Week days are required for custom frequency")
	}

	// Set default week days based on frequency
	weekDays := req.WeekDays
	if len(weekDays) == 0 {
		weekDays = s.getDefaultWeekDays(req.Frequency)
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
		Frequency:      req.Frequency,
		WeekDays:       weekDays,
		PreferredTime:  req.PreferredTime,
		DurationMins:   req.DurationMins,
		TotalSessions:  req.TotalSessions,
		SessionsDone:   0,
		StartDate:      startDate,
		GenerateUntil:  generateUntil,
		SessionPrice:   req.SessionPrice,
		Notes:          req.Notes,
	}

	if err := s.repo.Create(ctx, plan); err != nil {
		return nil, err
	}

	// Generate initial appointments (2 weeks ahead)
	_, err = s.GenerateAppointments(ctx, plan, 2)
	if err != nil {
		// Log error but don't fail the creation
		fmt.Printf("Warning: Failed to generate initial appointments: %v\n", err)
	}

	return s.repo.GetByID(ctx, clinicID, plan.ID)
}

// GetByID retrieves a session plan by ID
func (s *sessionPlanService) GetByID(ctx context.Context, clinicID, id uuid.UUID) (*model.SessionPlan, error) {
	return s.repo.GetByID(ctx, clinicID, id)
}

// List retrieves all session plans for a clinic
func (s *sessionPlanService) List(ctx context.Context, clinicID uuid.UUID, page, perPage int, status *model.PlanStatus) ([]*model.SessionPlan, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return s.repo.GetByClinic(ctx, clinicID, page, perPage, status)
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

// GenerateAppointments generates appointments for a session plan
func (s *sessionPlanService) GenerateAppointments(ctx context.Context, plan *model.SessionPlan, weeksAhead int) (int, error) {
	if !plan.IsActive() {
		return 0, nil
	}

	// Calculate the date range for generation
	startDate := time.Now()
	if plan.LastGeneratedAt != nil && plan.LastGeneratedAt.After(startDate) {
		startDate = *plan.LastGeneratedAt
	}
	if plan.StartDate.After(startDate) {
		startDate = plan.StartDate
	}

	endDate := time.Now().AddDate(0, 0, weeksAhead*7)

	// Parse preferred time
	preferredHour, preferredMin := s.parseTime(plan.PreferredTime)

	// Get dates based on frequency
	dates := s.getSessionDates(plan, startDate, endDate)

	// Limit dates to remaining sessions
	remainingSessions := plan.RemainingSessions()
	if len(dates) > remainingSessions {
		dates = dates[:remainingSessions]
	}

	created := 0
	for _, date := range dates {
		// Set the time
		sessionStart := time.Date(
			date.Year(), date.Month(), date.Day(),
			preferredHour, preferredMin, 0, 0,
			time.Local,
		)
		sessionEnd := sessionStart.Add(time.Duration(plan.DurationMins) * time.Minute)

		// Check for conflicts
		hasConflict, err := s.appointmentRepo.CheckConflict(
			ctx,
			plan.ClinicID,
			plan.ProfessionalID,
			sessionStart,
			sessionEnd,
			nil,
		)
		if err != nil {
			continue
		}
		if hasConflict {
			// Try to find an alternative time slot (skip for now)
			continue
		}

		// Create appointment
		appointment := &appointment_model.Appointment{
			ClinicID:       plan.ClinicID,
			PatientID:      plan.PatientID,
			ProfessionalID: plan.ProfessionalID,
			SessionPlanID:  &plan.ID,
			StartTime:      sessionStart,
			EndTime:        sessionEnd,
			Status:         appointment_model.AppointmentStatusPending,
			Type:           appointment_model.AppointmentTypeRecurring,
			Title:          plan.Title,
			Price:          plan.SessionPrice,
		}

		if err := s.appointmentRepo.Create(ctx, appointment); err != nil {
			continue
		}

		created++
	}

	// Update last generated timestamp
	now := time.Now()
	plan.LastGeneratedAt = &now
	plan.GenerateUntil = endDate
	s.repo.UpdateLastGenerated(ctx, plan.ID, now)

	return created, nil
}

// GenerateAllPending generates appointments for all active plans that need it
func (s *sessionPlanService) GenerateAllPending(ctx context.Context, weeksAhead int) error {
	generateBefore := time.Now().AddDate(0, 0, 7) // Plans that need generation within a week
	plans, err := s.repo.GetActivePlansForGeneration(ctx, generateBefore)
	if err != nil {
		return err
	}

	for _, plan := range plans {
		_, err := s.GenerateAppointments(ctx, plan, weeksAhead)
		if err != nil {
			// Log error but continue with other plans
			fmt.Printf("Error generating appointments for plan %s: %v\n", plan.ID, err)
		}
	}

	return nil
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

// getDefaultWeekDays returns default week days based on frequency
func (s *sessionPlanService) getDefaultWeekDays(frequency model.Frequency) []int {
	switch frequency {
	case model.FrequencyDaily:
		return []int{1, 2, 3, 4, 5} // Monday to Friday
	case model.FrequencyWeekly:
		return []int{1} // Monday
	case model.FrequencyBiweekly:
		return []int{1, 4} // Monday and Thursday
	case model.FrequencyTriweekly:
		return []int{1, 3, 5} // Monday, Wednesday, Friday
	default:
		return []int{1} // Monday
	}
}

// parseTime parses a time string (HH:MM) and returns hour and minute
func (s *sessionPlanService) parseTime(timeStr string) (int, int) {
	var hour, min int
	fmt.Sscanf(timeStr, "%d:%d", &hour, &min)
	return hour, min
}

// getSessionDates calculates the dates for sessions based on frequency
func (s *sessionPlanService) getSessionDates(plan *model.SessionPlan, startDate, endDate time.Time) []time.Time {
	var dates []time.Time

	switch plan.Frequency {
	case model.FrequencyMonthly:
		// Once per month on the same day
		current := startDate
		for current.Before(endDate) {
			dates = append(dates, current)
			current = current.AddDate(0, 1, 0)
		}

	case model.FrequencyBimonthly:
		// Every two weeks
		current := startDate
		for current.Before(endDate) {
			dates = append(dates, current)
			current = current.AddDate(0, 0, 14)
		}

	default:
		// Weekly-based frequencies (use week days)
		current := startDate
		for current.Before(endDate) {
			weekday := int(current.Weekday())
			for _, wd := range plan.WeekDays {
				if weekday == wd {
					dates = append(dates, current)
					break
				}
			}
			current = current.AddDate(0, 0, 1)
		}
	}

	return dates
}
