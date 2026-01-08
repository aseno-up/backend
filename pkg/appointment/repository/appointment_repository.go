package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/appointment/model"
)

// AppointmentRepository defines the interface for appointment data operations
type AppointmentRepository interface {
	Create(ctx context.Context, appointment *model.Appointment) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Appointment, error)
	List(ctx context.Context, clinicID uuid.UUID, filter *model.AppointmentFilter) ([]*model.Appointment, int64, error)
	GetByDateRange(ctx context.Context, clinicID uuid.UUID, start, end time.Time, professionalID *uuid.UUID) ([]*model.Appointment, error)
	GetByDay(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.Appointment, error)
	Update(ctx context.Context, appointment *model.Appointment) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	CheckConflict(ctx context.Context, clinicID, professionalID uuid.UUID, start, end time.Time, excludeID *uuid.UUID) (bool, error)
	GetByPatient(ctx context.Context, patientID, clinicID uuid.UUID, limit int) ([]*model.Appointment, error)
	UpdateStatus(ctx context.Context, id, clinicID uuid.UUID, status model.AppointmentStatus) error

	// Reminder-related methods
	GetPendingReminders(ctx context.Context, reminderType string, windowStart, windowEnd time.Time) ([]*model.AppointmentWithClinic, error)
	MarkReminderSent(ctx context.Context, id uuid.UUID, reminderType string) error
}

// appointmentRepository implements AppointmentRepository
type appointmentRepository struct {
	db *gorm.DB
}

// NewAppointmentRepository creates a new appointment repository
func NewAppointmentRepository(db *gorm.DB) AppointmentRepository {
	return &appointmentRepository{db: db}
}

// Create creates a new appointment
func (r *appointmentRepository) Create(ctx context.Context, appointment *model.Appointment) error {
	result := r.db.WithContext(ctx).Create(appointment)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves an appointment by ID with relations
func (r *appointmentRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Appointment, error) {
	var appointment model.Appointment
	result := r.db.WithContext(ctx).
		Preload("Patient").
		Preload("Professional").
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).
		First(&appointment)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrAppointmentNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &appointment, nil
}

// List retrieves appointments with filters
func (r *appointmentRepository) List(ctx context.Context, clinicID uuid.UUID, filter *model.AppointmentFilter) ([]*model.Appointment, int64, error) {
	var appointments []*model.Appointment
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Appointment{}).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID)

	// Apply filters
	if filter.ProfessionalID != nil {
		query = query.Where("professional_id = ?", *filter.ProfessionalID)
	}
	if filter.PatientID != nil {
		query = query.Where("patient_id = ?", *filter.PatientID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.StartDate != nil {
		query = query.Where("start_time >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("start_time <= ?", *filter.EndDate)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Pagination
	page := filter.Page
	perPage := filter.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	// Get results
	result := query.
		Preload("Patient").
		Preload("Professional").
		Order("start_time ASC").
		Limit(perPage).
		Offset(offset).
		Find(&appointments)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return appointments, total, nil
}

// GetByDateRange retrieves appointments within a date range
func (r *appointmentRepository) GetByDateRange(ctx context.Context, clinicID uuid.UUID, start, end time.Time, professionalID *uuid.UUID) ([]*model.Appointment, error) {
	var appointments []*model.Appointment

	query := r.db.WithContext(ctx).
		Preload("Patient").
		Preload("Professional").
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID).
		Where("start_time >= ? AND start_time < ?", start, end).
		Where("status NOT IN (?)", []model.AppointmentStatus{model.AppointmentStatusCanceled})

	if professionalID != nil {
		query = query.Where("professional_id = ?", *professionalID)
	}

	result := query.Order("start_time ASC").Find(&appointments)
	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}

	return appointments, nil
}

// GetByDay retrieves appointments for a specific day
func (r *appointmentRepository) GetByDay(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.Appointment, error) {
	// Start of day
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	// End of day
	end := start.AddDate(0, 0, 1)

	return r.GetByDateRange(ctx, clinicID, start, end, professionalID)
}

// Update updates an appointment
func (r *appointmentRepository) Update(ctx context.Context, appointment *model.Appointment) error {
	result := r.db.WithContext(ctx).Omit("Patient", "Professional").Save(appointment)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft-deletes an appointment
func (r *appointmentRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.Appointment{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrAppointmentNotFoundInstance
	}
	return nil
}

// CheckConflict checks if there's a conflicting appointment for a professional
func (r *appointmentRepository) CheckConflict(ctx context.Context, clinicID, professionalID uuid.UUID, start, end time.Time, excludeID *uuid.UUID) (bool, error) {
	var count int64

	query := r.db.WithContext(ctx).Model(&model.Appointment{}).
		Where("clinic_id = ? AND professional_id = ? AND deleted_at IS NULL", clinicID, professionalID).
		Where("status NOT IN (?)", []model.AppointmentStatus{model.AppointmentStatusCanceled}).
		Where("(start_time < ? AND end_time > ?)", end, start) // Overlapping condition

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	if err := query.Count(&count).Error; err != nil {
		return false, errors.ErrInternalInstance.WithError(err)
	}

	return count > 0, nil
}

// GetByPatient retrieves recent appointments for a patient
func (r *appointmentRepository) GetByPatient(ctx context.Context, patientID, clinicID uuid.UUID, limit int) ([]*model.Appointment, error) {
	var appointments []*model.Appointment

	result := r.db.WithContext(ctx).
		Preload("Professional").
		Where("patient_id = ? AND clinic_id = ? AND deleted_at IS NULL", patientID, clinicID).
		Order("start_time DESC").
		Limit(limit).
		Find(&appointments)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}

	return appointments, nil
}

// UpdateStatus updates only the status of an appointment
func (r *appointmentRepository) UpdateStatus(ctx context.Context, id, clinicID uuid.UUID, status model.AppointmentStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if status == model.AppointmentStatusConfirmed {
		now := time.Now()
		updates["confirmed_at"] = &now
	} else if status == model.AppointmentStatusCanceled {
		now := time.Now()
		updates["canceled_at"] = &now
	}

	result := r.db.WithContext(ctx).
		Model(&model.Appointment{}).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Updates(updates)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrAppointmentNotFoundInstance
	}
	return nil
}

// GetPendingReminders retrieves appointments that need reminders
// reminderType: "48h", "24h", or "2h"
// windowStart/windowEnd: the time window for appointments
func (r *appointmentRepository) GetPendingReminders(ctx context.Context, reminderType string, windowStart, windowEnd time.Time) ([]*model.AppointmentWithClinic, error) {
	var results []*model.AppointmentWithClinic

	// Determine which reminder field to check
	var reminderField string
	switch reminderType {
	case "48h":
		reminderField = "reminder_48h_sent_at"
	case "24h":
		reminderField = "reminder_24h_sent_at"
	case "2h":
		reminderField = "reminder_2h_sent_at"
	default:
		return nil, errors.ErrValidationInstance.WithMessage("invalid reminder type")
	}

	query := `
		SELECT
			a.*,
			c.name as clinic_name,
			c.whatsapp_instance,
			c.whatsapp_connected
		FROM appointments a
		INNER JOIN clinics c ON a.clinic_id = c.id
		WHERE a.deleted_at IS NULL
			AND a.status IN ('pending', 'confirmed')
			AND a.start_time >= ?
			AND a.start_time < ?
			AND a.` + reminderField + ` IS NULL
			AND c.whatsapp_connected = true
			AND c.whatsapp_instance IS NOT NULL
			AND c.whatsapp_instance != ''
		ORDER BY a.start_time ASC
		LIMIT 100
	`

	if err := r.db.WithContext(ctx).Raw(query, windowStart, windowEnd).Scan(&results).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Load patient and professional info for each appointment
	for i := range results {
		r.db.WithContext(ctx).
			Model(&model.PatientInfo{}).
			Where("id = ?", results[i].PatientID).
			First(&results[i].Patient)

		r.db.WithContext(ctx).
			Model(&model.ProfessionalInfo{}).
			Where("id = ?", results[i].ProfessionalID).
			First(&results[i].Professional)
	}

	return results, nil
}

// MarkReminderSent marks a specific reminder type as sent for an appointment
func (r *appointmentRepository) MarkReminderSent(ctx context.Context, id uuid.UUID, reminderType string) error {
	var fieldName string
	switch reminderType {
	case "48h":
		fieldName = "reminder_48h_sent_at"
	case "24h":
		fieldName = "reminder_24h_sent_at"
	case "2h":
		fieldName = "reminder_2h_sent_at"
	default:
		return errors.ErrValidationInstance.WithMessage("invalid reminder type")
	}

	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.Appointment{}).
		Where("id = ?", id).
		Update(fieldName, now)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
