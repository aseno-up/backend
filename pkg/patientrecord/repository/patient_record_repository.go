package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/patientrecord/model"
)

// PatientRecordRepository defines the interface for patient record data operations
type PatientRecordRepository interface {
	// PatientRecord operations
	CreateRecord(ctx context.Context, record *model.PatientRecord) error
	GetRecordByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRecord, error)
	GetRecordByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientRecord, error)
	UpdateRecord(ctx context.Context, record *model.PatientRecord) error
	DeleteRecord(ctx context.Context, id, clinicID uuid.UUID) error

	// SessionNote operations
	CreateNote(ctx context.Context, note *model.SessionNote) error
	GetNoteByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SessionNote, error)
	GetNotesByPatientID(ctx context.Context, patientID, clinicID uuid.UUID, limit, offset int) ([]*model.SessionNote, int64, error)
	GetNotesByProfessionalID(ctx context.Context, professionalID, clinicID uuid.UUID, limit, offset int) ([]*model.SessionNote, int64, error)
	UpdateNote(ctx context.Context, note *model.SessionNote) error
	DeleteNote(ctx context.Context, id, clinicID uuid.UUID) error
}

// patientRecordRepository implements PatientRecordRepository
type patientRecordRepository struct {
	db *gorm.DB
}

// NewPatientRecordRepository creates a new patient record repository
func NewPatientRecordRepository(db *gorm.DB) PatientRecordRepository {
	return &patientRecordRepository{db: db}
}

// CreateRecord creates a new patient record
func (r *patientRecordRepository) CreateRecord(ctx context.Context, record *model.PatientRecord) error {
	result := r.db.WithContext(ctx).Create(record)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetRecordByID retrieves a patient record by ID
func (r *patientRecordRepository) GetRecordByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRecord, error) {
	var record model.PatientRecord
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).
		First(&record)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPatientRecordNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &record, nil
}

// GetRecordByPatientID retrieves a patient record by patient ID
func (r *patientRecordRepository) GetRecordByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientRecord, error) {
	var record model.PatientRecord
	result := r.db.WithContext(ctx).
		Where("patient_id = ? AND clinic_id = ? AND deleted_at IS NULL", patientID, clinicID).
		First(&record)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPatientRecordNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &record, nil
}

// UpdateRecord updates a patient record
func (r *patientRecordRepository) UpdateRecord(ctx context.Context, record *model.PatientRecord) error {
	result := r.db.WithContext(ctx).Save(record)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// DeleteRecord soft-deletes a patient record
func (r *patientRecordRepository) DeleteRecord(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.PatientRecord{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrPatientRecordNotFoundInstance
	}
	return nil
}

// CreateNote creates a new session note
func (r *patientRecordRepository) CreateNote(ctx context.Context, note *model.SessionNote) error {
	result := r.db.WithContext(ctx).Create(note)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetNoteByID retrieves a session note by ID
func (r *patientRecordRepository) GetNoteByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SessionNote, error) {
	var note model.SessionNote
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).
		First(&note)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrSessionNoteNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &note, nil
}

// GetNotesByPatientID retrieves session notes by patient ID with pagination
func (r *patientRecordRepository) GetNotesByPatientID(ctx context.Context, patientID, clinicID uuid.UUID, limit, offset int) ([]*model.SessionNote, int64, error) {
	var notes []*model.SessionNote
	var total int64

	query := r.db.WithContext(ctx).Model(&model.SessionNote{}).
		Where("patient_id = ? AND clinic_id = ? AND deleted_at IS NULL", patientID, clinicID)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results ordered by session date descending
	result := query.
		Order("session_date DESC").
		Limit(limit).
		Offset(offset).
		Find(&notes)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return notes, total, nil
}

// GetNotesByProfessionalID retrieves session notes by professional ID with pagination
func (r *patientRecordRepository) GetNotesByProfessionalID(ctx context.Context, professionalID, clinicID uuid.UUID, limit, offset int) ([]*model.SessionNote, int64, error) {
	var notes []*model.SessionNote
	var total int64

	query := r.db.WithContext(ctx).Model(&model.SessionNote{}).
		Where("professional_id = ? AND clinic_id = ? AND deleted_at IS NULL", professionalID, clinicID)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results ordered by session date descending
	result := query.
		Order("session_date DESC").
		Limit(limit).
		Offset(offset).
		Find(&notes)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return notes, total, nil
}

// UpdateNote updates a session note
func (r *patientRecordRepository) UpdateNote(ctx context.Context, note *model.SessionNote) error {
	result := r.db.WithContext(ctx).Save(note)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// DeleteNote soft-deletes a session note
func (r *patientRecordRepository) DeleteNote(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.SessionNote{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrSessionNoteNotFoundInstance
	}
	return nil
}
