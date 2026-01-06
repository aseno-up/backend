package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/professional/model"
)

// ProfessionalRepository defines the interface for professional data operations
type ProfessionalRepository interface {
	Create(ctx context.Context, professional *model.Professional) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Professional, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID, activeOnly bool, limit, offset int) ([]*model.Professional, int64, error)
	Update(ctx context.Context, professional *model.Professional) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	GetByUserID(ctx context.Context, userID, clinicID uuid.UUID) (*model.Professional, error)
}

// professionalRepository implements ProfessionalRepository
type professionalRepository struct {
	db *gorm.DB
}

// NewProfessionalRepository creates a new professional repository
func NewProfessionalRepository(db *gorm.DB) ProfessionalRepository {
	return &professionalRepository{db: db}
}

// Create creates a new professional
func (r *professionalRepository) Create(ctx context.Context, professional *model.Professional) error {
	result := r.db.WithContext(ctx).Create(professional)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a professional by ID within a clinic
func (r *professionalRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Professional, error) {
	var professional model.Professional
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).
		First(&professional)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrProfessionalNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &professional, nil
}

// GetByClinicID retrieves professionals belonging to a clinic
func (r *professionalRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID, activeOnly bool, limit, offset int) ([]*model.Professional, int64, error) {
	var professionals []*model.Professional
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Professional{}).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID)

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results
	result := query.
		Order("name ASC").
		Limit(limit).
		Offset(offset).
		Find(&professionals)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return professionals, total, nil
}

// Update updates a professional
func (r *professionalRepository) Update(ctx context.Context, professional *model.Professional) error {
	result := r.db.WithContext(ctx).Save(professional)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft-deletes a professional
func (r *professionalRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.Professional{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrProfessionalNotFoundInstance
	}
	return nil
}

// GetByUserID retrieves a professional by their linked user ID
func (r *professionalRepository) GetByUserID(ctx context.Context, userID, clinicID uuid.UUID) (*model.Professional, error) {
	var professional model.Professional
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND clinic_id = ? AND deleted_at IS NULL", userID, clinicID).
		First(&professional)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrProfessionalNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &professional, nil
}
