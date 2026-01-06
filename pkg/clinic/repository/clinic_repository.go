package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/clinic/model"
)

// ClinicRepository defines the interface for clinic data operations
type ClinicRepository interface {
	Create(ctx context.Context, clinic *model.Clinic) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Clinic, error)
	GetBySlug(ctx context.Context, slug string) (*model.Clinic, error)
	GetByEmail(ctx context.Context, email string) (*model.Clinic, error)
	GetByWhatsAppInstance(ctx context.Context, instance string) (*model.Clinic, error)
	Update(ctx context.Context, clinic *model.Clinic) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*model.Clinic, int64, error)
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdateWhatsAppInstance(ctx context.Context, id uuid.UUID, instance string) error
	UpdateWhatsAppConnected(ctx context.Context, id uuid.UUID, connected bool) error
}

// clinicRepository implements ClinicRepository
type clinicRepository struct {
	db *gorm.DB
}

// NewClinicRepository creates a new clinic repository
func NewClinicRepository(db *gorm.DB) ClinicRepository {
	return &clinicRepository{db: db}
}

// Create creates a new clinic
func (r *clinicRepository) Create(ctx context.Context, clinic *model.Clinic) error {
	result := r.db.WithContext(ctx).Create(clinic)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a clinic by ID
func (r *clinicRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Clinic, error) {
	var clinic model.Clinic
	result := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&clinic)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrClinicNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &clinic, nil
}

// GetBySlug retrieves a clinic by slug
func (r *clinicRepository) GetBySlug(ctx context.Context, slug string) (*model.Clinic, error) {
	var clinic model.Clinic
	result := r.db.WithContext(ctx).Where("slug = ? AND deleted_at IS NULL", slug).First(&clinic)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrClinicNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &clinic, nil
}

// GetByEmail retrieves a clinic by email
func (r *clinicRepository) GetByEmail(ctx context.Context, email string) (*model.Clinic, error) {
	var clinic model.Clinic
	result := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&clinic)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrClinicNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &clinic, nil
}

// Update updates a clinic
func (r *clinicRepository) Update(ctx context.Context, clinic *model.Clinic) error {
	result := r.db.WithContext(ctx).Save(clinic)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft-deletes a clinic
func (r *clinicRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Clinic{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrClinicNotFoundInstance
	}
	return nil
}

// List retrieves a paginated list of clinics
func (r *clinicRepository) List(ctx context.Context, limit, offset int) ([]*model.Clinic, int64, error) {
	var clinics []*model.Clinic
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).Model(&model.Clinic{}).Where("deleted_at IS NULL").Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results
	result := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&clinics)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return clinics, total, nil
}

// ExistsBySlug checks if a clinic with the given slug exists
func (r *clinicRepository) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&model.Clinic{}).Where("slug = ? AND deleted_at IS NULL", slug).Count(&count)
	if result.Error != nil {
		return false, errors.ErrInternalInstance.WithError(result.Error)
	}
	return count > 0, nil
}

// ExistsByEmail checks if a clinic with the given email exists
func (r *clinicRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&model.Clinic{}).Where("email = ? AND deleted_at IS NULL", email).Count(&count)
	if result.Error != nil {
		return false, errors.ErrInternalInstance.WithError(result.Error)
	}
	return count > 0, nil
}

// GetByWhatsAppInstance retrieves a clinic by WhatsApp instance name
func (r *clinicRepository) GetByWhatsAppInstance(ctx context.Context, instance string) (*model.Clinic, error) {
	var clinic model.Clinic
	result := r.db.WithContext(ctx).Where("whats_app_instance = ? AND deleted_at IS NULL", instance).First(&clinic)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrClinicNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &clinic, nil
}

// UpdateWhatsAppInstance updates the clinic's WhatsApp instance name
func (r *clinicRepository) UpdateWhatsAppInstance(ctx context.Context, id uuid.UUID, instance string) error {
	result := r.db.WithContext(ctx).Model(&model.Clinic{}).Where("id = ?", id).Update("whats_app_instance", instance)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// UpdateWhatsAppConnected updates the clinic's WhatsApp connection status
func (r *clinicRepository) UpdateWhatsAppConnected(ctx context.Context, id uuid.UUID, connected bool) error {
	result := r.db.WithContext(ctx).Model(&model.Clinic{}).Where("id = ?", id).Update("whats_app_connected", connected)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
