package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/salespackage/model"
)

// SalesPackageReceiptTokenRepository persists the opaque tokens used to share
// a sale's public receipt link.
type SalesPackageReceiptTokenRepository interface {
	GetOrCreateBySalesPackage(ctx context.Context, clinicID, salesPackageID uuid.UUID) (*model.SalesPackageReceiptToken, error)
	GetByToken(ctx context.Context, token uuid.UUID) (*model.SalesPackageReceiptToken, error)
}

type salesPackageReceiptTokenRepository struct {
	db *gorm.DB
}

func NewSalesPackageReceiptTokenRepository(db *gorm.DB) SalesPackageReceiptTokenRepository {
	return &salesPackageReceiptTokenRepository{db: db}
}

// GetOrCreateBySalesPackage is idempotent: subsequent calls return the same
// token, so multiple "share" clicks always hand out the same public URL.
func (r *salesPackageReceiptTokenRepository) GetOrCreateBySalesPackage(ctx context.Context, clinicID, salesPackageID uuid.UUID) (*model.SalesPackageReceiptToken, error) {
	var existing model.SalesPackageReceiptToken
	err := r.db.WithContext(ctx).
		Where("sales_package_id = ? AND clinic_id = ?", salesPackageID, clinicID).
		First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	token := &model.SalesPackageReceiptToken{
		ClinicID:       clinicID,
		SalesPackageID: salesPackageID,
	}
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		// Race: a parallel call inserted between SELECT and INSERT — re-read.
		var again model.SalesPackageReceiptToken
		if err2 := r.db.WithContext(ctx).
			Where("sales_package_id = ? AND clinic_id = ?", salesPackageID, clinicID).
			First(&again).Error; err2 == nil {
			return &again, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return token, nil
}

func (r *salesPackageReceiptTokenRepository) GetByToken(ctx context.Context, token uuid.UUID) (*model.SalesPackageReceiptToken, error) {
	var row model.SalesPackageReceiptToken
	err := r.db.WithContext(ctx).
		Where("token = ?", token).
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &row, nil
}
