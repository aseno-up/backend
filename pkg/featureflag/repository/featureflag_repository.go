package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/featureflag/model"
)

// FeatureFlagRepository defines the interface for feature flag persistence
type FeatureFlagRepository interface {
	Create(ctx context.Context, flag *model.FeatureFlag) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.FeatureFlag, error)
	Update(ctx context.Context, flag *model.FeatureFlag) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]model.FeatureFlag, error)
	IsEnabled(ctx context.Context, key string, clinicID uuid.UUID) (bool, error)
}

type featureFlagRepository struct {
	db *gorm.DB
}

// NewFeatureFlagRepository creates a new feature flag repository
func NewFeatureFlagRepository(db *gorm.DB) FeatureFlagRepository {
	return &featureFlagRepository{db: db}
}

func (r *featureFlagRepository) Create(ctx context.Context, flag *model.FeatureFlag) error {
	if err := r.db.WithContext(ctx).Create(flag).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *featureFlagRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.FeatureFlag, error) {
	var flag model.FeatureFlag
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&flag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &flag, nil
}

func (r *featureFlagRepository) Update(ctx context.Context, flag *model.FeatureFlag) error {
	if err := r.db.WithContext(ctx).Save(flag).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *featureFlagRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&model.FeatureFlag{}, "id = ?", id).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *featureFlagRepository) List(ctx context.Context) ([]model.FeatureFlag, error) {
	var flags []model.FeatureFlag
	if err := r.db.WithContext(ctx).Order("key ASC, clinic_id ASC NULLS FIRST").Find(&flags).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return flags, nil
}

// IsEnabled checks if a feature is enabled for a specific clinic.
// It first checks for a clinic-specific flag, then falls back to the global flag.
func (r *featureFlagRepository) IsEnabled(ctx context.Context, key string, clinicID uuid.UUID) (bool, error) {
	var flag model.FeatureFlag

	// First try clinic-specific flag
	err := r.db.WithContext(ctx).
		Where("key = ? AND clinic_id = ?", key, clinicID).
		First(&flag).Error
	if err == nil {
		return flag.Enabled, nil
	}

	// Fall back to global flag (clinic_id IS NULL)
	err = r.db.WithContext(ctx).
		Where("key = ? AND clinic_id IS NULL", key).
		First(&flag).Error
	if err == nil {
		return flag.Enabled, nil
	}

	// Flag not found = disabled
	return false, nil
}
