package repository

import (
	"context"

	"github.com/aseno/backend/pkg/plan/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlanRepository interface {
	Create(ctx context.Context, plan *model.Plan) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Plan, error)
	GetBySlug(ctx context.Context, slug string) (*model.Plan, error)
	List(ctx context.Context) ([]model.Plan, error)
	Update(ctx context.Context, plan *model.Plan) error
}

type planRepository struct {
	db *gorm.DB
}

func NewPlanRepository(db *gorm.DB) PlanRepository {
	return &planRepository{db: db}
}

func (r *planRepository) Create(ctx context.Context, plan *model.Plan) error {
	return r.db.WithContext(ctx).Create(plan).Error
}

func (r *planRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Plan, error) {
	var plan model.Plan
	if err := r.db.WithContext(ctx).Preload("Cycles").First(&plan, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *planRepository) GetBySlug(ctx context.Context, slug string) (*model.Plan, error) {
	var plan model.Plan
	if err := r.db.WithContext(ctx).Preload("Cycles").First(&plan, "slug = ?", slug).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *planRepository) List(ctx context.Context) ([]model.Plan, error) {
	var plans []model.Plan
	if err := r.db.WithContext(ctx).Preload("Cycles").Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

func (r *planRepository) Update(ctx context.Context, plan *model.Plan) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Hard-delete old cycles (Unscoped bypasses soft delete)
		if err := tx.Unscoped().Where("plan_id = ?", plan.ID).Delete(&model.PlanCycle{}).Error; err != nil {
			return err
		}

		// Save plan fields only (without associations)
		if err := tx.Omit("Cycles").Save(plan).Error; err != nil {
			return err
		}

		// Create new cycles with correct plan_id
		for i := range plan.Cycles {
			plan.Cycles[i].ID = uuid.UUID{} // Reset ID so GORM creates new rows
			plan.Cycles[i].PlanID = plan.ID
		}
		if len(plan.Cycles) > 0 {
			return tx.Create(&plan.Cycles).Error
		}
		return nil
	})
}
