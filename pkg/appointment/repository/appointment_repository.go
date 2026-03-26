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
	MarkReminderSent(ctx context.Context, clinicID, id uuid.UUID, reminderType string) error

	// WhatsApp response handling
	GetPendingByPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*model.Appointment, error)

	// Confirmation timeout methods
	GetExpiredConfirmations(ctx context.Context, limit int) ([]*model.AppointmentWithClinic, error)
	MarkConfirmationExpired(ctx context.Context, clinicID, id uuid.UUID) error
	SetConfirmationExpiresAt(ctx context.Context, clinicID, id uuid.UUID, expiresAt time.Time) error

	// Smart Scheduling
	GetSlotPerformanceStats(ctx context.Context, clinicID, professionalID uuid.UUID) ([]*model.SlotPerformanceStats, error)

	// CountByProfessionalID returns the count of non-deleted appointments for a professional
	CountByProfessionalID(ctx context.Context, professionalID, clinicID uuid.UUID) (int64, error)
}

// appointmentRepository implements AppointmentRepository
type appointmentRepository struct {
	db *gorm.DB
}

// NewAppointmentRepository creates a new appointment repository
func NewAppointmentRepository(db *gorm.DB) AppointmentRepository {
	return &appointmentRepository{db: db}
}

// GetSlotPerformanceStats retrieves historical performance of time slots
func (r *appointmentRepository) GetSlotPerformanceStats(ctx context.Context, clinicID, professionalID uuid.UUID) ([]*model.SlotPerformanceStats, error) {
	var stats []*model.SlotPerformanceStats

	// Query last 90 days of appointments to calculate completion rates by day/hour
	query := `
		SELECT
			EXTRACT(DOW FROM start_time) as day_of_week,
			EXTRACT(HOUR FROM start_time) as hour_of_day,
			COUNT(*) as total,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed
		FROM appointments
		WHERE clinic_id = ? 
		  AND professional_id = ?
		  AND deleted_at IS NULL
		  AND start_time > NOW() - INTERVAL '90 days'
		GROUP BY 1, 2
	`

	if err := r.db.WithContext(ctx).Raw(query, clinicID, professionalID).Scan(&stats).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Calculate rates
	for _, s := range stats {
		if s.Total > 0 {
			s.Rate = float64(s.Completed) / float64(s.Total)
		}
	}

	return stats, nil
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
		Preload("Procedures.Procedure").
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
		Preload("Procedures.Procedure").
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
		Preload("Procedures.Procedure").
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Save the core appointment but omit relations so we handle Procedures explicitly
		if err := tx.Omit("Patient", "Professional", "Procedures").Save(appointment).Error; err != nil {
			return errors.ErrInternalInstance.WithError(err)
		}

		// Explicitly replace the Procedures association
		// Rather than rely on GORM's Replace (which attempts to NULL the foreign key causing constraint failures),
		// we explicitly delete old records for this appointment and insert the new ones.
		if appointment.Procedures != nil {
			if err := tx.Where("appointment_id = ?", appointment.ID).Delete(&model.AppointmentProcedure{}).Error; err != nil {
				return errors.ErrInternalInstance.WithError(err)
			}

			if len(appointment.Procedures) > 0 {
				// Ensure the appointment_id is mapped correctly and insert the new procedures
				for i := range appointment.Procedures {
					appointment.Procedures[i].AppointmentID = appointment.ID
				}
				if err := tx.Create(&appointment.Procedures).Error; err != nil {
					return errors.ErrInternalInstance.WithError(err)
				}
			}
		}

		return nil
	})
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
		Preload("Procedures.Procedure").
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
		reminderField = "reminder48h_sent_at"
	case "24h":
		reminderField = "reminder24h_sent_at"
	case "2h":
		reminderField = "reminder2h_sent_at"
	default:
		return nil, errors.ErrValidationInstance.WithMessage("invalid reminder type")
	}

	// Include no_response status so reminders continue after timeout
	query := `
		SELECT
			a.*,
			c.name as clinic_name,
			wc.instance as whatsapp_instance,
			wc.connected as whatsapp_connected
		FROM appointments a
		INNER JOIN clinics c ON a.clinic_id = c.id
		LEFT JOIN whatsapp_connections wc ON wc.clinic_id = c.id AND wc.is_active = true AND wc.deleted_at IS NULL
		WHERE a.deleted_at IS NULL
			AND a.status IN ('pending', 'no_response')
			AND a.start_time >= ?
			AND a.start_time < ?
			AND a.` + reminderField + ` IS NULL
			AND wc.connected = true
			AND wc.instance IS NOT NULL
			AND wc.instance != ''
			AND COALESCE((c.settings->>'whatsapp_reminders_enabled')::boolean, false) = true
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
func (r *appointmentRepository) MarkReminderSent(ctx context.Context, clinicID, id uuid.UUID, reminderType string) error {
	var fieldName string
	switch reminderType {
	case "48h":
		fieldName = "reminder48h_sent_at"
	case "24h":
		fieldName = "reminder24h_sent_at"
	case "2h":
		fieldName = "reminder2h_sent_at"
	default:
		return errors.ErrValidationInstance.WithMessage("invalid reminder type")
	}

	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.Appointment{}).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Update(fieldName, now)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetPendingByPhone finds the most recent pending/confirmed appointment for a patient by phone
// This is used to match incoming WhatsApp responses to appointments
func (r *appointmentRepository) GetPendingByPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*model.Appointment, error) {
	var appointment model.Appointment

	// Clean phone - remove non-digits for comparison
	// The query will handle flexible matching

	// Include no_response so patient can still confirm after timeout
	query := `
		SELECT a.*
		FROM appointments a
		INNER JOIN patients p ON a.patient_id = p.id AND p.deleted_at IS NULL
		WHERE a.clinic_id = ?
			AND a.deleted_at IS NULL
			AND a.status IN ('pending', 'confirmed', 'no_response')
			AND a.start_time > NOW()
			AND (
				REGEXP_REPLACE(p.phone, '[^0-9]', '', 'g') LIKE '%' || REGEXP_REPLACE(?, '[^0-9]', '', 'g') || '%'
				OR REGEXP_REPLACE(p.whatsapp, '[^0-9]', '', 'g') LIKE '%' || REGEXP_REPLACE(?, '[^0-9]', '', 'g') || '%'
				OR REGEXP_REPLACE(?, '[^0-9]', '', 'g') LIKE '%' || REGEXP_REPLACE(p.phone, '[^0-9]', '', 'g') || '%'
				OR REGEXP_REPLACE(?, '[^0-9]', '', 'g') LIKE '%' || REGEXP_REPLACE(p.whatsapp, '[^0-9]', '', 'g') || '%'
			)
		ORDER BY a.start_time ASC
		LIMIT 1
	`

	err := r.db.WithContext(ctx).Raw(query, clinicID, phone, phone, phone, phone).Scan(&appointment).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Check if we got a result
	if appointment.ID == uuid.Nil {
		return nil, nil
	}

	// Load relations
	r.db.WithContext(ctx).
		Model(&model.PatientInfo{}).
		Where("id = ?", appointment.PatientID).
		First(&appointment.Patient)

	r.db.WithContext(ctx).
		Model(&model.ProfessionalInfo{}).
		Where("id = ?", appointment.ProfessionalID).
		First(&appointment.Professional)

	return &appointment, nil
}

// GetExpiredConfirmations retrieves appointments where confirmation has expired
// These are appointments with status=pending and confirmation_expires_at < now
func (r *appointmentRepository) GetExpiredConfirmations(ctx context.Context, limit int) ([]*model.AppointmentWithClinic, error) {
	var results []*model.AppointmentWithClinic

	query := `
		SELECT
			a.*,
			c.name as clinic_name,
			wc.instance as whatsapp_instance,
			wc.connected as whatsapp_connected
		FROM appointments a
		INNER JOIN clinics c ON a.clinic_id = c.id
		LEFT JOIN whatsapp_connections wc ON wc.clinic_id = c.id AND wc.is_active = true AND wc.deleted_at IS NULL
		WHERE a.deleted_at IS NULL
			AND a.status = 'pending'
			AND a.confirmation_expires_at IS NOT NULL
			AND a.confirmation_expires_at < NOW()
			AND a.start_time > NOW()
		ORDER BY a.confirmation_expires_at ASC
		LIMIT ?
	`

	if err := r.db.WithContext(ctx).Raw(query, limit).Scan(&results).Error; err != nil {
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

// MarkConfirmationExpired marks an appointment as no_response due to confirmation timeout
func (r *appointmentRepository) MarkConfirmationExpired(ctx context.Context, clinicID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.Appointment{}).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Update("status", model.AppointmentStatusNoResponse)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// SetConfirmationExpiresAt sets the confirmation timeout for an appointment
func (r *appointmentRepository) SetConfirmationExpiresAt(ctx context.Context, clinicID, id uuid.UUID, expiresAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.Appointment{}).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Update("confirmation_expires_at", expiresAt)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// CountByProfessionalID returns the count of non-deleted appointments for a professional
func (r *appointmentRepository) CountByProfessionalID(ctx context.Context, professionalID, clinicID uuid.UUID) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).
		Model(&model.Appointment{}).
		Where("professional_id = ? AND clinic_id = ? AND deleted_at IS NULL", professionalID, clinicID).
		Count(&count)
	if result.Error != nil {
		return 0, errors.ErrInternalInstance.WithError(result.Error)
	}
	return count, nil
}
