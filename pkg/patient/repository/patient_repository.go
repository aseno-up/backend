package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/patient/model"
)

// PatientRepository defines the interface for patient data operations
type PatientRepository interface {
	Create(ctx context.Context, patient *model.Patient) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Patient, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*model.Patient, int64, error)
	Search(ctx context.Context, clinicID uuid.UUID, query string, limit, offset int) ([]*model.Patient, int64, error)
	Update(ctx context.Context, patient *model.Patient) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	GetByPhone(ctx context.Context, phone string, clinicID uuid.UUID) (*model.Patient, error)
	GetByDocument(ctx context.Context, document string, clinicID uuid.UUID) (*model.Patient, error)
}

// patientRepository implements PatientRepository
type patientRepository struct {
	db *gorm.DB
}

// NewPatientRepository creates a new patient repository
func NewPatientRepository(db *gorm.DB) PatientRepository {
	return &patientRepository{db: db}
}

// Create creates a new patient
func (r *patientRepository) Create(ctx context.Context, patient *model.Patient) error {
	result := r.db.WithContext(ctx).Create(patient)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a patient by ID within a clinic
func (r *patientRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Patient, error) {
	var patient model.Patient
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).
		First(&patient)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPatientNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &patient, nil
}

// GetByClinicID retrieves patients belonging to a clinic
func (r *patientRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*model.Patient, int64, error) {
	var patients []*model.Patient
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Patient{}).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results
	result := query.
		Order("name ASC").
		Limit(limit).
		Offset(offset).
		Find(&patients)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return patients, total, nil
}

// Search searches patients by name, phone, or email
func (r *patientRepository) Search(ctx context.Context, clinicID uuid.UUID, query string, limit, offset int) ([]*model.Patient, int64, error) {
	var patients []*model.Patient
	var total int64

	searchPattern := "%" + query + "%"

	baseQuery := r.db.WithContext(ctx).Model(&model.Patient{}).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID).
		Where("name ILIKE ? OR phone ILIKE ? OR email ILIKE ? OR whatsapp ILIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern)

	// Count total
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results
	result := baseQuery.
		Order("name ASC").
		Limit(limit).
		Offset(offset).
		Find(&patients)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return patients, total, nil
}

// Update updates a patient
func (r *patientRepository) Update(ctx context.Context, patient *model.Patient) error {
	result := r.db.WithContext(ctx).Save(patient)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft-deletes a patient
func (r *patientRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.Patient{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrPatientNotFoundInstance
	}
	return nil
}

// GetByPhone retrieves a patient by phone number
func (r *patientRepository) GetByPhone(ctx context.Context, phone string, clinicID uuid.UUID) (*model.Patient, error) {
	var patient model.Patient
	result := r.db.WithContext(ctx).
		Where("(phone = ? OR whatsapp = ?) AND clinic_id = ? AND deleted_at IS NULL", phone, phone, clinicID).
		First(&patient)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPatientNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &patient, nil
}

// GetByDocument retrieves a patient by document (CPF)
func (r *patientRepository) GetByDocument(ctx context.Context, document string, clinicID uuid.UUID) (*model.Patient, error) {
	var patient model.Patient
	result := r.db.WithContext(ctx).
		Where("document = ? AND clinic_id = ? AND deleted_at IS NULL", document, clinicID).
		First(&patient)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPatientNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &patient, nil
}
