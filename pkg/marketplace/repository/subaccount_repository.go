package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/marketplace/model"
	"github.com/aseno/backend/pkg/marketplace/service"
)

type subaccountRepository struct {
	db *gorm.DB
}

func NewSubaccountRepository(db *gorm.DB) service.SubaccountRepository {
	return &subaccountRepository{db: db}
}

func (r *subaccountRepository) Create(ctx context.Context, sub *model.Subaccount) error {
	result := r.db.WithContext(ctx).Create(sub)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *subaccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Subaccount, error) {
	var sub model.Subaccount
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&sub)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Subaccount not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &sub, nil
}

func (r *subaccountRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.Subaccount, error) {
	var sub model.Subaccount
	result := r.db.WithContext(ctx).Where("clinic_id = ?", clinicID).First(&sub)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Subaccount not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &sub, nil
}

func (r *subaccountRepository) GetByAsaasID(ctx context.Context, asaasID string) (*model.Subaccount, error) {
	var sub model.Subaccount
	result := r.db.WithContext(ctx).Where("asaas_account_id = ?", asaasID).First(&sub)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Subaccount not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &sub, nil
}

func (r *subaccountRepository) Update(ctx context.Context, sub *model.Subaccount) error {
	result := r.db.WithContext(ctx).Save(sub)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *subaccountRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.SubaccountStatus) error {
	result := r.db.WithContext(ctx).Model(&model.Subaccount{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
