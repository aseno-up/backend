package repository

import (
	"github.com/aseno/backend/pkg/partnernetwork/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PartnerNetworkRepository interface {
	Create(network *model.PartnerNetwork) error
	GetByID(id, clinicID uuid.UUID) (*model.PartnerNetwork, error)
	GetByIDWithPlans(id, clinicID uuid.UUID) (*model.PartnerNetwork, error)
	GetByName(name string, clinicID uuid.UUID) (*model.PartnerNetwork, error)
	Update(network *model.PartnerNetwork) error
	Delete(id, clinicID uuid.UUID) error
	ListByClinic(clinicID uuid.UUID) ([]*model.PartnerNetwork, error)
	ListActiveByClinic(clinicID uuid.UUID) ([]*model.PartnerNetwork, error)
}

type partnerNetworkRepository struct {
	db *gorm.DB
}

func NewPartnerNetworkRepository(db *gorm.DB) PartnerNetworkRepository {
	return &partnerNetworkRepository{db: db}
}

func (r *partnerNetworkRepository) Create(network *model.PartnerNetwork) error {
	return r.db.Create(network).Error
}

func (r *partnerNetworkRepository) GetByID(id, clinicID uuid.UUID) (*model.PartnerNetwork, error) {
	var n model.PartnerNetwork
	err := r.db.Where("id = ? AND clinic_id = ?", id, clinicID).First(&n).Error
	return &n, err
}

func (r *partnerNetworkRepository) GetByIDWithPlans(id, clinicID uuid.UUID) (*model.PartnerNetwork, error) {
	var n model.PartnerNetwork
	err := r.db.
		Preload("Plans", "deleted_at IS NULL").
		Where("id = ? AND clinic_id = ?", id, clinicID).
		First(&n).Error
	return &n, err
}

func (r *partnerNetworkRepository) GetByName(name string, clinicID uuid.UUID) (*model.PartnerNetwork, error) {
	var n model.PartnerNetwork
	err := r.db.Where("name = ? AND clinic_id = ?", name, clinicID).First(&n).Error
	return &n, err
}

func (r *partnerNetworkRepository) Update(network *model.PartnerNetwork) error {
	// Save (not Updates) so is_active=false actually persists — Updates skips zero values.
	return r.db.Save(network).Error
}

func (r *partnerNetworkRepository) Delete(id, clinicID uuid.UUID) error {
	return r.db.Where("id = ? AND clinic_id = ?", id, clinicID).Delete(&model.PartnerNetwork{}).Error
}

func (r *partnerNetworkRepository) ListByClinic(clinicID uuid.UUID) ([]*model.PartnerNetwork, error) {
	var ns []*model.PartnerNetwork
	err := r.db.
		Preload("Plans", "deleted_at IS NULL").
		Where("clinic_id = ?", clinicID).
		Order("name ASC").
		Find(&ns).Error
	return ns, err
}

func (r *partnerNetworkRepository) ListActiveByClinic(clinicID uuid.UUID) ([]*model.PartnerNetwork, error) {
	var ns []*model.PartnerNetwork
	err := r.db.
		Preload("Plans", "is_active = true AND deleted_at IS NULL").
		Where("clinic_id = ? AND is_active = true", clinicID).
		Order("name ASC").
		Find(&ns).Error
	return ns, err
}
