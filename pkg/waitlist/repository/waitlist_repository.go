package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/waitlist/model"
)

// WaitlistRepository defines the interface for waitlist data operations
type WaitlistRepository interface {
	Create(ctx context.Context, entry *model.WaitlistEntry) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.WaitlistEntry, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID, status string, limit, offset int) ([]*model.WaitlistEntry, int64, error)
	Update(ctx context.Context, entry *model.WaitlistEntry) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error

	// FindNextMatchingPatient finds the oldest patient in the waitlist that matches the slot
	// Orders by created_at ASC (oldest first - FIFO)
	FindNextMatchingPatient(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, dayOfWeek string, slotTime time.Time) (*model.WaitlistEntryWithPatient, error)

	// GetPendingByPatientPhone finds a waitlist entry with status "notified" for a patient phone
	GetPendingByPatientPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*model.WaitlistEntry, error)

	// GetByPatientID finds waitlist entries for a patient
	GetByPatientID(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.WaitlistEntry, error)

	// UpdateStatus updates only the status and related fields
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.WaitlistStatus, notifiedAt *time.Time, pendingSlotTime *time.Time, pendingSlotProfessional *uuid.UUID) error

	// GetWithPatientInfo retrieves a waitlist entry with patient information
	GetWithPatientInfo(ctx context.Context, id, clinicID uuid.UUID) (*model.WaitlistEntryWithPatient, error)
}

// waitlistRepository implements WaitlistRepository
type waitlistRepository struct {
	db *gorm.DB
}

// NewWaitlistRepository creates a new waitlist repository
func NewWaitlistRepository(db *gorm.DB) WaitlistRepository {
	return &waitlistRepository{db: db}
}

// Create creates a new waitlist entry
func (r *waitlistRepository) Create(ctx context.Context, entry *model.WaitlistEntry) error {
	result := r.db.WithContext(ctx).Create(entry)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a waitlist entry by ID within a clinic
func (r *waitlistRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.WaitlistEntry, error) {
	var entry model.WaitlistEntry
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		First(&entry)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Waitlist entry not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &entry, nil
}

// GetByClinicID retrieves waitlist entries for a clinic, optionally filtered by status
func (r *waitlistRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID, status string, limit, offset int) ([]*model.WaitlistEntry, int64, error) {
	var entries []*model.WaitlistEntry
	var total int64

	query := r.db.WithContext(ctx).Model(&model.WaitlistEntry{}).
		Where("clinic_id = ?", clinicID)

	// Filter by status if provided
	if status != "" {
		query = query.Where("status = ?", status)
	} else {
		// By default, show only active entries (waiting or notified)
		query = query.Where("status IN ?", []string{string(model.WaitlistStatusWaiting), string(model.WaitlistStatusNotified)})
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results ordered by creation date (oldest first)
	result := query.
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&entries)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return entries, total, nil
}

// Update updates a waitlist entry
func (r *waitlistRepository) Update(ctx context.Context, entry *model.WaitlistEntry) error {
	result := r.db.WithContext(ctx).Save(entry)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete removes a waitlist entry (marks as cancelled)
func (r *waitlistRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.WaitlistEntry{}).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Update("status", model.WaitlistStatusCancelled)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFoundInstance.WithMessage("Waitlist entry not found")
	}
	return nil
}

// FindNextMatchingPatient finds the oldest patient that matches the slot criteria
func (r *waitlistRepository) FindNextMatchingPatient(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, dayOfWeek string, slotTime time.Time) (*model.WaitlistEntryWithPatient, error) {
	var result model.WaitlistEntryWithPatient

	// Build the query with proper JSON array checking
	// The slot time format is "HH:MM"
	slotTimeStr := slotTime.Format("15:04")

	query := r.db.WithContext(ctx).
		Table("waitlist_entries w").
		Select(`w.*, p.name as patient_name, p.phone as patient_phone, COALESCE(p.whatsapp, p.phone) as patient_whatsapp`).
		Joins("JOIN patients p ON p.id = w.patient_id AND p.deleted_at IS NULL").
		Where("w.clinic_id = ?", clinicID).
		Where("w.status = ?", model.WaitlistStatusWaiting).
		Where("(w.expires_at IS NULL OR w.expires_at > ?)", time.Now()).
		// Check if dayOfWeek is in preferred_days JSON array
		Where("w.preferred_days @> ?", `["`+dayOfWeek+`"]`).
		// Check if slot time is within preferred time range
		Where("? >= w.preferred_time_start AND ? <= w.preferred_time_end", slotTimeStr, slotTimeStr)

	// If professionalID is provided, filter by it or allow any (null professional_id)
	if professionalID != nil {
		query = query.Where("(w.professional_id IS NULL OR w.professional_id = ?)", *professionalID)
	}

	// Order by creation date (oldest first - FIFO)
	query = query.Order("w.created_at ASC").Limit(1)

	err := query.Scan(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No matching patient found
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Check if we got a result
	if result.ID == uuid.Nil {
		return nil, nil
	}

	return &result, nil
}

// GetPendingByPatientPhone finds a waitlist entry with status "notified" for a patient phone
func (r *waitlistRepository) GetPendingByPatientPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*model.WaitlistEntry, error) {
	var entry model.WaitlistEntry

	// Clean phone number - remove non-digits
	cleanPhone := phone
	for _, c := range "+-() " {
		cleanPhone = removeChar(cleanPhone, c)
	}

	err := r.db.WithContext(ctx).
		Table("waitlist_entries w").
		Select("w.*").
		Joins("JOIN patients p ON p.id = w.patient_id AND p.deleted_at IS NULL").
		Where("w.clinic_id = ?", clinicID).
		Where("w.status = ?", model.WaitlistStatusNotified).
		Where("(p.phone LIKE ? OR p.whatsapp LIKE ? OR p.phone LIKE ? OR p.whatsapp LIKE ?)",
			"%"+cleanPhone+"%", "%"+cleanPhone+"%", "%"+phone+"%", "%"+phone+"%").
		Order("w.notified_at DESC").
		Limit(1).
		Scan(&entry).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	if entry.ID == uuid.Nil {
		return nil, nil
	}

	return &entry, nil
}

// GetByPatientID finds waitlist entries for a patient
func (r *waitlistRepository) GetByPatientID(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.WaitlistEntry, error) {
	var entries []*model.WaitlistEntry

	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND patient_id = ? AND status IN ?", clinicID, patientID,
			[]string{string(model.WaitlistStatusWaiting), string(model.WaitlistStatusNotified)}).
		Order("created_at ASC").
		Find(&entries)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}

	return entries, nil
}

// UpdateStatus updates only status and related notification fields
func (r *waitlistRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.WaitlistStatus, notifiedAt *time.Time, pendingSlotTime *time.Time, pendingSlotProfessional *uuid.UUID) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if notifiedAt != nil {
		updates["notified_at"] = notifiedAt
	}

	updates["pending_slot_time"] = pendingSlotTime
	updates["pending_slot_professional"] = pendingSlotProfessional

	result := r.db.WithContext(ctx).
		Model(&model.WaitlistEntry{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}

	return nil
}

// GetWithPatientInfo retrieves a waitlist entry with patient information
func (r *waitlistRepository) GetWithPatientInfo(ctx context.Context, id, clinicID uuid.UUID) (*model.WaitlistEntryWithPatient, error) {
	var result model.WaitlistEntryWithPatient

	err := r.db.WithContext(ctx).
		Table("waitlist_entries w").
		Select(`w.*, p.name as patient_name, p.phone as patient_phone, COALESCE(p.whatsapp, p.phone) as patient_whatsapp`).
		Joins("JOIN patients p ON p.id = w.patient_id AND p.deleted_at IS NULL").
		Where("w.id = ? AND w.clinic_id = ?", id, clinicID).
		Scan(&result).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Waitlist entry not found")
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	if result.ID == uuid.Nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Waitlist entry not found")
	}

	return &result, nil
}

// removeChar removes all occurrences of a character from a string
func removeChar(s string, c rune) string {
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r != c {
			result = append(result, r)
		}
	}
	return string(result)
}
