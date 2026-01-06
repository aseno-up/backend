package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/payment/model"
	"github.com/saas/backend/pkg/payment/service"
)

// subscriptionRepository implements service.SubscriptionRepository
type subscriptionRepository struct {
	db *gorm.DB
}

// NewSubscriptionRepository creates a new subscription repository
func NewSubscriptionRepository(db *gorm.DB) service.SubscriptionRepository {
	return &subscriptionRepository{db: db}
}

// Create creates a new subscription
func (r *subscriptionRepository) Create(ctx context.Context, subscription *model.Subscription) error {
	result := r.db.WithContext(ctx).Create(subscription)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a subscription by ID
func (r *subscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Subscription, error) {
	var subscription model.Subscription
	result := r.db.WithContext(ctx).
		Preload("Customer").
		Where("id = ?", id).
		First(&subscription)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrSubscriptionNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &subscription, nil
}

// GetByAsaasID retrieves a subscription by Asaas ID
func (r *subscriptionRepository) GetByAsaasID(ctx context.Context, asaasID string) (*model.Subscription, error) {
	var subscription model.Subscription
	result := r.db.WithContext(ctx).
		Preload("Customer").
		Where("asaas_subscription_id = ?", asaasID).
		First(&subscription)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrSubscriptionNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &subscription, nil
}

// GetByClinicID retrieves the active subscription for a clinic
func (r *subscriptionRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.Subscription, error) {
	var subscription model.Subscription
	result := r.db.WithContext(ctx).
		Preload("Customer").
		Where("clinic_id = ? AND status = ?", clinicID, model.SubscriptionStatusActive).
		First(&subscription)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrSubscriptionNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &subscription, nil
}

// GetByCustomerID retrieves subscriptions for a customer
func (r *subscriptionRepository) GetByCustomerID(ctx context.Context, customerID uuid.UUID) ([]*model.Subscription, error) {
	var subscriptions []*model.Subscription
	result := r.db.WithContext(ctx).
		Preload("Customer").
		Where("customer_id = ?", customerID).
		Order("created_at DESC").
		Find(&subscriptions)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}

	return subscriptions, nil
}

// Update updates a subscription
func (r *subscriptionRepository) Update(ctx context.Context, subscription *model.Subscription) error {
	result := r.db.WithContext(ctx).Save(subscription)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// UpdateStatus updates the status of a subscription
func (r *subscriptionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.SubscriptionStatus) error {
	result := r.db.WithContext(ctx).
		Model(&model.Subscription{}).
		Where("id = ?", id).
		Update("status", status)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
