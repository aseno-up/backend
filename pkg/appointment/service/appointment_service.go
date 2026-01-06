package service

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/appointment/model"
	"github.com/saas/backend/pkg/appointment/repository"
)

// AppointmentService defines the interface for appointment business logic
type AppointmentService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateAppointmentRequest) (*model.AppointmentResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, filter *model.AppointmentFilter) (*model.AppointmentListResponse, error)
	GetByDay(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error)
	GetByWeek(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateAppointmentRequest) (*model.AppointmentResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	Confirm(ctx context.Context, id, clinicID uuid.UUID, req *model.ConfirmAppointmentRequest) (*model.AppointmentResponse, error)
	Cancel(ctx context.Context, id, clinicID uuid.UUID, req *model.CancelAppointmentRequest) (*model.AppointmentResponse, error)
	Complete(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error)
	MarkNoShow(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error)
}

// appointmentService implements AppointmentService
type appointmentService struct {
	repo repository.AppointmentRepository
}

// NewAppointmentService creates a new appointment service
func NewAppointmentService(repo repository.AppointmentRepository) AppointmentService {
	return &appointmentService{repo: repo}
}

// Create creates a new appointment
func (s *appointmentService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateAppointmentRequest) (*model.AppointmentResponse, error) {
	endTime := req.StartTime.Add(time.Duration(req.DurationMinutes) * time.Minute)

	// Check for conflicts
	hasConflict, err := s.repo.CheckConflict(ctx, clinicID, req.ProfessionalID, req.StartTime, endTime, nil)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, errors.ErrSlotUnavailableInstance
	}

	appointmentType := req.Type
	if appointmentType == "" {
		appointmentType = model.AppointmentTypeRegular
	}

	appointment := &model.Appointment{
		ClinicID:       clinicID,
		PatientID:      req.PatientID,
		ProfessionalID: req.ProfessionalID,
		StartTime:      req.StartTime,
		EndTime:        endTime,
		Status:         model.AppointmentStatusPending,
		Type:           appointmentType,
		Title:          req.Title,
		Notes:          req.Notes,
		Price:          req.Price,
	}

	if err := s.repo.Create(ctx, appointment); err != nil {
		return nil, err
	}

	// Reload with relations
	return s.GetByID(ctx, appointment.ID, clinicID)
}

// GetByID retrieves an appointment by ID
func (s *appointmentService) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error) {
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}
	return appointment.ToResponse(), nil
}

// List retrieves a paginated list of appointments
func (s *appointmentService) List(ctx context.Context, clinicID uuid.UUID, filter *model.AppointmentFilter) (*model.AppointmentListResponse, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 || filter.PerPage > 100 {
		filter.PerPage = 20
	}

	appointments, total, err := s.repo.List(ctx, clinicID, filter)
	if err != nil {
		return nil, err
	}

	data := make([]*model.AppointmentResponse, len(appointments))
	for i, a := range appointments {
		data[i] = a.ToResponse()
	}

	totalPages := int(math.Ceil(float64(total) / float64(filter.PerPage)))

	return &model.AppointmentListResponse{
		Data:       data,
		Total:      total,
		Page:       filter.Page,
		PerPage:    filter.PerPage,
		TotalPages: totalPages,
	}, nil
}

// GetByDay retrieves appointments for a specific day
func (s *appointmentService) GetByDay(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error) {
	appointments, err := s.repo.GetByDay(ctx, clinicID, date, professionalID)
	if err != nil {
		return nil, err
	}

	data := make([]*model.AppointmentResponse, len(appointments))
	for i, a := range appointments {
		data[i] = a.ToResponse()
	}

	return data, nil
}

// GetByWeek retrieves appointments for a week starting from the given date
func (s *appointmentService) GetByWeek(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error) {
	// Get start of week (Monday)
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday
	}
	start := date.AddDate(0, 0, -(weekday - 1))
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, date.Location())

	// End of week (Sunday 23:59:59)
	end := start.AddDate(0, 0, 7)

	appointments, err := s.repo.GetByDateRange(ctx, clinicID, start, end, professionalID)
	if err != nil {
		return nil, err
	}

	data := make([]*model.AppointmentResponse, len(appointments))
	for i, a := range appointments {
		data[i] = a.ToResponse()
	}

	return data, nil
}

// Update updates an appointment
func (s *appointmentService) Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateAppointmentRequest) (*model.AppointmentResponse, error) {
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	// Calculate new times if provided
	newStart := appointment.StartTime
	newEnd := appointment.EndTime

	if req.StartTime != nil {
		newStart = *req.StartTime
		if req.DurationMinutes != nil {
			newEnd = newStart.Add(time.Duration(*req.DurationMinutes) * time.Minute)
		} else {
			// Keep same duration
			duration := appointment.EndTime.Sub(appointment.StartTime)
			newEnd = newStart.Add(duration)
		}
	} else if req.DurationMinutes != nil {
		newEnd = newStart.Add(time.Duration(*req.DurationMinutes) * time.Minute)
	}

	professionalID := appointment.ProfessionalID
	if req.ProfessionalID != nil {
		professionalID = *req.ProfessionalID
	}

	// Check for conflicts if time or professional changed
	if newStart != appointment.StartTime || newEnd != appointment.EndTime || professionalID != appointment.ProfessionalID {
		hasConflict, err := s.repo.CheckConflict(ctx, clinicID, professionalID, newStart, newEnd, &id)
		if err != nil {
			return nil, err
		}
		if hasConflict {
			return nil, errors.ErrSlotUnavailableInstance
		}
	}

	// Apply updates
	appointment.StartTime = newStart
	appointment.EndTime = newEnd
	appointment.ProfessionalID = professionalID

	if req.PatientID != nil {
		appointment.PatientID = *req.PatientID
	}
	if req.Status != "" {
		appointment.Status = req.Status
	}
	if req.Title != "" {
		appointment.Title = req.Title
	}
	if req.Notes != "" {
		appointment.Notes = req.Notes
	}
	if req.PatientNotes != "" {
		appointment.PatientNotes = req.PatientNotes
	}
	if req.Price != nil {
		appointment.Price = *req.Price
	}
	if req.CancelReason != "" {
		appointment.CancelReason = req.CancelReason
	}

	if err := s.repo.Update(ctx, appointment); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id, clinicID)
}

// Delete deletes an appointment
func (s *appointmentService) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.Delete(ctx, id, clinicID)
}

// Confirm confirms an appointment
func (s *appointmentService) Confirm(ctx context.Context, id, clinicID uuid.UUID, req *model.ConfirmAppointmentRequest) (*model.AppointmentResponse, error) {
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	appointment.Status = model.AppointmentStatusConfirmed
	appointment.ConfirmedAt = &now
	if req != nil && req.PatientNotes != "" {
		appointment.PatientNotes = req.PatientNotes
	}

	if err := s.repo.Update(ctx, appointment); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id, clinicID)
}

// Cancel cancels an appointment
func (s *appointmentService) Cancel(ctx context.Context, id, clinicID uuid.UUID, req *model.CancelAppointmentRequest) (*model.AppointmentResponse, error) {
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	appointment.Status = model.AppointmentStatusCanceled
	appointment.CanceledAt = &now
	if req != nil && req.Reason != "" {
		appointment.CancelReason = req.Reason
	}

	if err := s.repo.Update(ctx, appointment); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id, clinicID)
}

// Complete marks an appointment as completed
func (s *appointmentService) Complete(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error) {
	if err := s.repo.UpdateStatus(ctx, id, clinicID, model.AppointmentStatusCompleted); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id, clinicID)
}

// MarkNoShow marks an appointment as no-show
func (s *appointmentService) MarkNoShow(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error) {
	if err := s.repo.UpdateStatus(ctx, id, clinicID, model.AppointmentStatusNoShow); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id, clinicID)
}
