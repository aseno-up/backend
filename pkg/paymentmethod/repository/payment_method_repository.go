package repository

import (
	"github.com/aseno/backend/pkg/paymentmethod/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PaymentMethodRepository interface {
	Create(method *model.PaymentMethod) error
	GetByID(id, clinicID uuid.UUID) (*model.PaymentMethod, error)
	Update(method *model.PaymentMethod) error
	Delete(id, clinicID uuid.UUID) error
	ListByClinic(clinicID uuid.UUID) ([]*model.PaymentMethod, error)
	ListActiveByClinic(clinicID uuid.UUID) ([]*model.PaymentMethod, error)
	CountByClinic(clinicID uuid.UUID) (int64, error)
	CreateDefaults(clinicID uuid.UUID) error
}

type paymentMethodRepository struct {
	db *gorm.DB
}

func NewPaymentMethodRepository(db *gorm.DB) PaymentMethodRepository {
	return &paymentMethodRepository{db: db}
}

func (r *paymentMethodRepository) Create(method *model.PaymentMethod) error {
	return r.db.Create(method).Error
}

func (r *paymentMethodRepository) GetByID(id, clinicID uuid.UUID) (*model.PaymentMethod, error) {
	var method model.PaymentMethod
	err := r.db.Where("id = ? AND clinic_id = ?", id, clinicID).First(&method).Error
	return &method, err
}

func (r *paymentMethodRepository) Update(method *model.PaymentMethod) error {
	return r.db.Where("id = ? AND clinic_id = ?", method.ID, method.ClinicID).Updates(method).Error
}

func (r *paymentMethodRepository) Delete(id, clinicID uuid.UUID) error {
	return r.db.Where("id = ? AND clinic_id = ?", id, clinicID).Delete(&model.PaymentMethod{}).Error
}

func (r *paymentMethodRepository) ListByClinic(clinicID uuid.UUID) ([]*model.PaymentMethod, error) {
	var methods []*model.PaymentMethod
	err := r.db.Where("clinic_id = ?", clinicID).Order("name ASC").Find(&methods).Error
	return methods, err
}

func (r *paymentMethodRepository) ListActiveByClinic(clinicID uuid.UUID) ([]*model.PaymentMethod, error) {
	var methods []*model.PaymentMethod
	err := r.db.Where("clinic_id = ? AND active = true", clinicID).Order("name ASC").Find(&methods).Error
	return methods, err
}

func (r *paymentMethodRepository) CountByClinic(clinicID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.PaymentMethod{}).Where("clinic_id = ?", clinicID).Count(&count).Error
	return count, err
}

// CreateDefaults inserts the 4 default payment methods for a clinic if it has none
func (r *paymentMethodRepository) CreateDefaults(clinicID uuid.UUID) error {
	count, err := r.CountByClinic(clinicID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	defaults := []model.PaymentMethod{
		{ClinicID: clinicID, Name: "PIX", Type: model.PaymentMethodTypePix, Active: true},
		{ClinicID: clinicID, Name: "Dinheiro", Type: model.PaymentMethodTypeCash, Active: true},
		{ClinicID: clinicID, Name: "Cartão de Crédito", Type: model.PaymentMethodTypeCredit, Active: true},
		{ClinicID: clinicID, Name: "Cartão de Débito", Type: model.PaymentMethodTypeDebit, Active: true},
	}

	return r.db.Create(&defaults).Error
}
