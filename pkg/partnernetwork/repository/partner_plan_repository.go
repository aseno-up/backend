package repository

import (
	"github.com/aseno/backend/pkg/partnernetwork/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PartnerPlanRepository interface {
	Create(plan *model.PartnerPlan) error
	GetByID(id uuid.UUID) (*model.PartnerPlan, error)
	// GetByIDScoped joins partner_networks to enforce clinic_id scoping.
	GetByIDScoped(id, clinicID uuid.UUID) (*model.PartnerPlan, error)
	GetByNetworkAndName(networkID uuid.UUID, name string) (*model.PartnerPlan, error)
	Update(plan *model.PartnerPlan) error
	Delete(id, clinicID uuid.UUID) error
	ListByNetwork(networkID uuid.UUID) ([]*model.PartnerPlan, error)
}

type partnerPlanRepository struct {
	db *gorm.DB
}

func NewPartnerPlanRepository(db *gorm.DB) PartnerPlanRepository {
	return &partnerPlanRepository{db: db}
}

func (r *partnerPlanRepository) Create(plan *model.PartnerPlan) error {
	return r.db.Create(plan).Error
}

func (r *partnerPlanRepository) GetByID(id uuid.UUID) (*model.PartnerPlan, error) {
	var p model.PartnerPlan
	err := r.db.Where("id = ?", id).First(&p).Error
	return &p, err
}

func (r *partnerPlanRepository) GetByIDScoped(id, clinicID uuid.UUID) (*model.PartnerPlan, error) {
	var p model.PartnerPlan
	err := r.db.
		Joins("JOIN partner_networks ON partner_networks.id = partner_plans.partner_network_id").
		Where("partner_plans.id = ? AND partner_networks.clinic_id = ?", id, clinicID).
		Where("partner_networks.deleted_at IS NULL").
		First(&p).Error
	return &p, err
}

func (r *partnerPlanRepository) GetByNetworkAndName(networkID uuid.UUID, name string) (*model.PartnerPlan, error) {
	var p model.PartnerPlan
	err := r.db.Where("partner_network_id = ? AND name = ?", networkID, name).First(&p).Error
	return &p, err
}

func (r *partnerPlanRepository) Update(plan *model.PartnerPlan) error {
	// Save (not Updates) so is_active=false actually persists — Updates skips zero values.
	return r.db.Save(plan).Error
}

// Delete scopes by clinic via the parent partner_network.
func (r *partnerPlanRepository) Delete(id, clinicID uuid.UUID) error {
	subq := r.db.
		Table("partner_plans AS pp").
		Select("pp.id").
		Joins("JOIN partner_networks pn ON pn.id = pp.partner_network_id").
		Where("pp.id = ? AND pn.clinic_id = ?", id, clinicID)

	return r.db.
		Where("id IN (?)", subq).
		Delete(&model.PartnerPlan{}).Error
}

func (r *partnerPlanRepository) ListByNetwork(networkID uuid.UUID) ([]*model.PartnerPlan, error) {
	var ps []*model.PartnerPlan
	err := r.db.Where("partner_network_id = ?", networkID).Order("name ASC").Find(&ps).Error
	return ps, err
}
