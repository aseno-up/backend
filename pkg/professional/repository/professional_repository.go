package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/professional/model"
)

// ProfessionalRepository defines the interface for professional data operations
type ProfessionalRepository interface {
	Create(ctx context.Context, professional *model.Professional) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Professional, error)
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Professional, int64, error)
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

// List retrieves professionals belonging to a clinic using standard pagination filters
func (r *professionalRepository) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Professional, int64, error) {
	var professionals []*model.Professional
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Professional{}).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID)

	// Status filtering
	switch filter.Status {
	case "active":
		query = query.Where("is_active = ?", true)
	case "inactive":
		query = query.Where("is_active = ?", false)
	}

	// Search string
	if filter.Search != "" {
		searchTerm := "%" + filter.Search + "%"
		query = query.Where(
			"name ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR specialty ILIKE ?",
			searchTerm, searchTerm, searchTerm, searchTerm,
		)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Sorting
	switch filter.SortBy {
	case "name_desc":
		query = query.Order("name DESC")
	case "created_asc":
		query = query.Order("created_at ASC")
	case "created_desc":
		query = query.Order("created_at DESC")
	default: // "name_asc"
		query = query.Order("name ASC")
	}

	// Get paginated results
	result := query.
		Scopes(pagination.Paginate(&filter)).
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
