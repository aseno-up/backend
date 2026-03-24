package repository

import (
	"context"

	"github.com/aseno/backend/pkg/plan/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OfferRepository interface {
	Create(ctx context.Context, offer *model.Offer) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Offer, error)
	GetByToken(ctx context.Context, token string) (*model.Offer, error)
	List(ctx context.Context) ([]model.Offer, error)
	Update(ctx context.Context, offer *model.Offer) error
	IncrementUsage(ctx context.Context, id uuid.UUID) error
}

type offerRepository struct {
	db *gorm.DB
}

func NewOfferRepository(db *gorm.DB) OfferRepository {
	return &offerRepository{db: db}
}

func (r *offerRepository) Create(ctx context.Context, offer *model.Offer) error {
	return r.db.WithContext(ctx).Omit("Plan").Create(offer).Error
}

func (r *offerRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Offer, error) {
	var offer model.Offer
	if err := r.db.WithContext(ctx).Preload("Cycles").Preload("Plan").First(&offer, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &offer, nil
}

func (r *offerRepository) GetByToken(ctx context.Context, token string) (*model.Offer, error) {
	var offer model.Offer
	if err := r.db.WithContext(ctx).Preload("Cycles").Preload("Plan").First(&offer, "token = ?", token).Error; err != nil {
		return nil, err
	}
	return &offer, nil
}

func (r *offerRepository) List(ctx context.Context) ([]model.Offer, error) {
	var offers []model.Offer
	if err := r.db.WithContext(ctx).Preload("Cycles").Preload("Plan").Find(&offers).Error; err != nil {
		return nil, err
	}
	return offers, nil
}

func (r *offerRepository) Update(ctx context.Context, offer *model.Offer) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Hard-delete old cycles
		if err := tx.Unscoped().Where("offer_id = ?", offer.ID).Delete(&model.OfferCycle{}).Error; err != nil {
			return err
		}

		// Save offer fields only
		if err := tx.Omit("Cycles").Save(offer).Error; err != nil {
			return err
		}

		// Create new cycles with correct offer_id
		for i := range offer.Cycles {
			offer.Cycles[i].ID = uuid.UUID{}
			offer.Cycles[i].OfferID = offer.ID
		}
		if len(offer.Cycles) > 0 {
			return tx.Create(&offer.Cycles).Error
		}
		return nil
	})
}

func (r *offerRepository) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.Offer{}).Where("id = ?", id).UpdateColumn("current_uses", gorm.Expr("current_uses + ?", 1)).Error
}
