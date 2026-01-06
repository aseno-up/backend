package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/payment/model"
	"github.com/saas/backend/pkg/payment/service"
)

// paymentRepository implements service.PaymentRepository
type paymentRepository struct {
	db *gorm.DB
}

// NewPaymentRepository creates a new payment repository
func NewPaymentRepository(db *gorm.DB) service.PaymentRepository {
	return &paymentRepository{db: db}
}

// Create creates a new payment
func (r *paymentRepository) Create(ctx context.Context, payment *model.Payment) error {
	result := r.db.WithContext(ctx).Create(payment)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a payment by ID
func (r *paymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Payment, error) {
	var payment model.Payment
	result := r.db.WithContext(ctx).
		Preload("Customer").
		Where("id = ?", id).
		First(&payment)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPaymentNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &payment, nil
}

// GetByAsaasID retrieves a payment by Asaas ID
func (r *paymentRepository) GetByAsaasID(ctx context.Context, asaasID string) (*model.Payment, error) {
	var payment model.Payment
	result := r.db.WithContext(ctx).
		Preload("Customer").
		Where("asaas_payment_id = ?", asaasID).
		First(&payment)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPaymentNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &payment, nil
}

// GetByClinicID retrieves payments by clinic ID with pagination
func (r *paymentRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*model.Payment, int64, error) {
	var payments []*model.Payment
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("clinic_id = ?", clinicID).
		Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results
	result := r.db.WithContext(ctx).
		Preload("Customer").
		Where("clinic_id = ?", clinicID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&payments)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return payments, total, nil
}

// GetByCustomerID retrieves payments by customer ID with pagination
func (r *paymentRepository) GetByCustomerID(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*model.Payment, int64, error) {
	var payments []*model.Payment
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("customer_id = ?", customerID).
		Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results
	result := r.db.WithContext(ctx).
		Preload("Customer").
		Where("customer_id = ?", customerID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&payments)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return payments, total, nil
}

// Update updates a payment
func (r *paymentRepository) Update(ctx context.Context, payment *model.Payment) error {
	result := r.db.WithContext(ctx).Save(payment)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// UpdateStatus updates the status of a payment
func (r *paymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.PaymentStatus) error {
	result := r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("id = ?", id).
		Update("status", status)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
