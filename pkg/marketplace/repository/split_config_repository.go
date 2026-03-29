package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/marketplace/model"
	"github.com/aseno/backend/pkg/marketplace/service"
)

type splitConfigRepository struct {
	db *gorm.DB
}

func NewSplitConfigRepository(db *gorm.DB) service.SplitConfigRepository {
	return &splitConfigRepository{db: db}
}

func (r *splitConfigRepository) Create(ctx context.Context, config *model.SplitConfig) error {
	result := r.db.WithContext(ctx).Create(config)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *splitConfigRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.SplitConfig, error) {
	var config model.SplitConfig
	result := r.db.WithContext(ctx).Where("clinic_id = ?", clinicID).First(&config)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Split config not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &config, nil
}

func (r *splitConfigRepository) Update(ctx context.Context, config *model.SplitConfig) error {
	result := r.db.WithContext(ctx).Save(config)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
