package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/patientrecord/model"
)

// PatientRecordRepository defines the interface for patient record data operations
type PatientRecordRepository interface {
	// PatientRecord operations
	CreateRecord(ctx context.Context, record *model.PatientRecord) error
	GetRecordByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRecord, error)
	GetLatestRecordByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientRecord, error)
	GetRecordHistoryByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) ([]*model.PatientRecord, error)
	DeleteRecord(ctx context.Context, id, clinicID uuid.UUID) error

	// AnamneseTemplate operations
	CreateTemplate(ctx context.Context, template *model.AnamneseTemplate) error
	GetTemplateByID(ctx context.Context, id, clinicID uuid.UUID) (*model.AnamneseTemplate, error)
	ListTemplates(ctx context.Context, clinicID uuid.UUID) ([]*model.AnamneseTemplate, error)
	UpdateTemplate(ctx context.Context, template *model.AnamneseTemplate) error
	DeleteTemplate(ctx context.Context, id, clinicID uuid.UUID) error

	// Template Field operations
	CreateTemplateField(ctx context.Context, field *model.AnamneseTemplateField) error
	UpdateTemplateField(ctx context.Context, field *model.AnamneseTemplateField) error
	DeleteTemplateField(ctx context.Context, id, templateID uuid.UUID) error
	ReorderTemplateFields(ctx context.Context, templateID uuid.UUID, fieldIDs []uuid.UUID) error

	// Specialty Mapping
	SetTemplateSpecialties(ctx context.Context, templateID uuid.UUID, specialtyIDs []uuid.UUID) error
	GetTemplateBySpecialty(ctx context.Context, clinicID, specialtyID uuid.UUID) (*model.AnamneseTemplate, error)
	GetDefaultTemplate(ctx context.Context, clinicID uuid.UUID) (*model.AnamneseTemplate, error)

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

// CreateRecord creates a new patient record version
func (r *patientRecordRepository) CreateRecord(ctx context.Context, record *model.PatientRecord) error {
	result := r.db.WithContext(ctx).Create(record)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetRecordByID retrieves a patient record version by ID
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

// GetLatestRecordByPatientID retrieves the most recent patient record version
func (r *patientRecordRepository) GetLatestRecordByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientRecord, error) {
	var record model.PatientRecord
	result := r.db.WithContext(ctx).
		Where("patient_id = ? AND clinic_id = ? AND deleted_at IS NULL", patientID, clinicID).
		Order("recorded_at DESC").
		First(&record)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPatientRecordNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &record, nil
}

// GetRecordHistoryByPatientID retrieves all versions of a patient record
func (r *patientRecordRepository) GetRecordHistoryByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) ([]*model.PatientRecord, error) {
	var records []*model.PatientRecord
	result := r.db.WithContext(ctx).
		Where("patient_id = ? AND clinic_id = ? AND deleted_at IS NULL", patientID, clinicID).
		Order("recorded_at DESC").
		Find(&records)
	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return records, nil
}

// DeleteRecord soft-deletes a patient record version
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

// CreateTemplate creates a new anamnese template
func (r *patientRecordRepository) CreateTemplate(ctx context.Context, template *model.AnamneseTemplate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(template).Error; err != nil {
			return errors.ErrInternalInstance.WithError(err)
		}
		return nil
	})
}

// GetTemplateByID retrieves a template by ID with its fields
func (r *patientRecordRepository) GetTemplateByID(ctx context.Context, id, clinicID uuid.UUID) (*model.AnamneseTemplate, error) {
	var template model.AnamneseTemplate
	result := r.db.WithContext(ctx).
		Preload("Fields", func(db *gorm.DB) *gorm.DB {
			return db.Order("display_order ASC").Where("deleted_at IS NULL")
		}).
		Where("id = ? AND (clinic_id = ? OR is_default = true) AND deleted_at IS NULL", id, clinicID).
		First(&template)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrInternalInstance.WithMessage("Template não encontrado")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &template, nil
}

// ListTemplates lists templates for a clinic
func (r *patientRecordRepository) ListTemplates(ctx context.Context, clinicID uuid.UUID) ([]*model.AnamneseTemplate, error) {
	var templates []*model.AnamneseTemplate
	result := r.db.WithContext(ctx).
		Where("(clinic_id = ? OR is_default = true) AND deleted_at IS NULL", clinicID).
		Order("is_default DESC, name ASC").
		Find(&templates)
	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return templates, nil
}

// UpdateTemplate updates template metadata
func (r *patientRecordRepository) UpdateTemplate(ctx context.Context, template *model.AnamneseTemplate) error {
	result := r.db.WithContext(ctx).Model(template).
		Where("id = ? AND clinic_id = ?", template.ID, template.ClinicID).
		Updates(map[string]interface{}{
			"name":       template.Name,
			"is_default": template.IsDefault,
		})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// DeleteTemplate soft-deletes a template
func (r *patientRecordRepository) DeleteTemplate(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.AnamneseTemplate{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// CreateTemplateField adds a field to a template
func (r *patientRecordRepository) CreateTemplateField(ctx context.Context, field *model.AnamneseTemplateField) error {
	return r.db.WithContext(ctx).Create(field).Error
}

// UpdateTemplateField updates a template field
func (r *patientRecordRepository) UpdateTemplateField(ctx context.Context, field *model.AnamneseTemplateField) error {
	return r.db.WithContext(ctx).Model(field).
		Where("id = ? AND template_id = ?", field.ID, field.TemplateID).
		Updates(field).Error
}

// DeleteTemplateField soft-deletes a template field
func (r *patientRecordRepository) DeleteTemplateField(ctx context.Context, id, templateID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.AnamneseTemplateField{}).
		Where("id = ? AND template_id = ?", id, templateID).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}

// ReorderTemplateFields updates display order for all fields in a template
func (r *patientRecordRepository) ReorderTemplateFields(ctx context.Context, templateID uuid.UUID, fieldIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range fieldIDs {
			if err := tx.Model(&model.AnamneseTemplateField{}).
				Where("id = ? AND template_id = ?", id, templateID).
				Update("display_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SetTemplateSpecialties manages specialty mapping for a template
func (r *patientRecordRepository) SetTemplateSpecialties(ctx context.Context, templateID uuid.UUID, specialtyIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove existing mapping
		if err := tx.Where("template_id = ?", templateID).Delete(&model.AnamneseTemplateSpecialty{}).Error; err != nil {
			return err
		}
		// Add new mapping
		for _, specID := range specialtyIDs {
			mapping := model.AnamneseTemplateSpecialty{
				TemplateID:  templateID,
				SpecialtyID: specID,
			}
			if err := tx.Create(&mapping).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetTemplateBySpecialty finds the template associated with a specialty
func (r *patientRecordRepository) GetTemplateBySpecialty(ctx context.Context, clinicID, specialtyID uuid.UUID) (*model.AnamneseTemplate, error) {
	var template model.AnamneseTemplate
	result := r.db.WithContext(ctx).
		Joins("JOIN anamnese_template_specialties ats ON ats.template_id = anamnese_templates.id").
		Where("anamnese_templates.clinic_id = ? AND ats.specialty_id = ? AND anamnese_templates.deleted_at IS NULL", clinicID, specialtyID).
		Preload("Fields", func(db *gorm.DB) *gorm.DB {
			return db.Order("display_order ASC").Where("deleted_at IS NULL")
		}).
		First(&template)

	if result.Error != nil {
		return nil, result.Error
	}
	return &template, nil
}

// GetDefaultTemplate finds the default template for a clinic
func (r *patientRecordRepository) GetDefaultTemplate(ctx context.Context, clinicID uuid.UUID) (*model.AnamneseTemplate, error) {
	var template model.AnamneseTemplate
	// 1. Try clinic default
	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND is_default = true AND deleted_at IS NULL", clinicID).
		Preload("Fields", func(db *gorm.DB) *gorm.DB {
			return db.Order("display_order ASC").Where("deleted_at IS NULL")
		}).
		First(&template)

	if result.Error == nil {
		return &template, nil
	}

	// 2. Try global default
	result = r.db.WithContext(ctx).
		Where("is_default = true AND clinic_id = '00000000-0000-0000-0000-000000000000' AND deleted_at IS NULL").
		Preload("Fields", func(db *gorm.DB) *gorm.DB {
			return db.Order("display_order ASC").Where("deleted_at IS NULL")
		}).
		First(&template)

	return &template, result.Error
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
