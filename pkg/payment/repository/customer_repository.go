package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/payment/model"
	"github.com/aseno/backend/pkg/payment/service"
)

// customerRepository implements service.CustomerRepository
type customerRepository struct {
	db *gorm.DB
}

// NewCustomerRepository creates a new customer repository
func NewCustomerRepository(db *gorm.DB) service.CustomerRepository {
	return &customerRepository{db: db}
}

// Create creates a new customer
func (r *customerRepository) Create(ctx context.Context, customer *model.Customer) error {
	result := r.db.WithContext(ctx).Create(customer)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a customer by ID
func (r *customerRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Customer, error) {
	var customer model.Customer
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&customer)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrCustomerNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &customer, nil
}

// GetByClinicID retrieves a customer by clinic ID
func (r *customerRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.Customer, error) {
	var customer model.Customer
	result := r.db.WithContext(ctx).Where("clinic_id = ?", clinicID).First(&customer)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrCustomerNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &customer, nil
}

// GetByAsaasID retrieves a customer by Asaas ID
func (r *customerRepository) GetByAsaasID(ctx context.Context, asaasID string) (*model.Customer, error) {
	var customer model.Customer
	result := r.db.WithContext(ctx).Where("asaas_customer_id = ?", asaasID).First(&customer)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrCustomerNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &customer, nil
}

// Update updates a customer
func (r *customerRepository) Update(ctx context.Context, customer *model.Customer) error {
	result := r.db.WithContext(ctx).Save(customer)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft deletes a customer
func (r *customerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&model.Customer{}, "id = ?", id)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
