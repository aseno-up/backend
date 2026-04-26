package repository

import (
	"context"

	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SessionPlanSlotRepository interface {
	Create(ctx context.Context, slot *model.SessionPlanSlot) error
	BulkCreate(ctx context.Context, slots []*model.SessionPlanSlot) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.SessionPlanSlot, error)
	GetByIDScoped(ctx context.Context, id, clinicID uuid.UUID) (*model.SessionPlanSlot, error)
	ListByPlan(ctx context.Context, sessionPlanID uuid.UUID) ([]*model.SessionPlanSlot, error)
	ListActiveByPlan(ctx context.Context, sessionPlanID uuid.UUID) ([]*model.SessionPlanSlot, error)
	Update(ctx context.Context, slot *model.SessionPlanSlot) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	DeleteByPlan(ctx context.Context, sessionPlanID uuid.UUID) error
}

type sessionPlanSlotRepository struct {
	db *gorm.DB
}

func NewSessionPlanSlotRepository(db *gorm.DB) SessionPlanSlotRepository {
	return &sessionPlanSlotRepository{db: db}
}

func (r *sessionPlanSlotRepository) Create(ctx context.Context, slot *model.SessionPlanSlot) error {
	return r.db.WithContext(ctx).Create(slot).Error
}

func (r *sessionPlanSlotRepository) BulkCreate(ctx context.Context, slots []*model.SessionPlanSlot) error {
	if len(slots) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&slots).Error
}

func (r *sessionPlanSlotRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.SessionPlanSlot, error) {
	var slot model.SessionPlanSlot
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&slot).Error
	return &slot, err
}

// GetByIDScoped joins session_plans to enforce clinic_id scoping.
func (r *sessionPlanSlotRepository) GetByIDScoped(ctx context.Context, id, clinicID uuid.UUID) (*model.SessionPlanSlot, error) {
	var slot model.SessionPlanSlot
	err := r.db.WithContext(ctx).
		Joins("JOIN session_plans sp ON sp.id = session_plan_slots.session_plan_id").
		Where("session_plan_slots.id = ? AND sp.clinic_id = ?", id, clinicID).
		Where("sp.deleted_at IS NULL").
		First(&slot).Error
	return &slot, err
}

func (r *sessionPlanSlotRepository) ListByPlan(ctx context.Context, sessionPlanID uuid.UUID) ([]*model.SessionPlanSlot, error) {
	var slots []*model.SessionPlanSlot
	err := r.db.WithContext(ctx).
		Where("session_plan_id = ?", sessionPlanID).
		Order("weekday ASC, start_time ASC").
		Find(&slots).Error
	return slots, err
}

func (r *sessionPlanSlotRepository) ListActiveByPlan(ctx context.Context, sessionPlanID uuid.UUID) ([]*model.SessionPlanSlot, error) {
	var slots []*model.SessionPlanSlot
	err := r.db.WithContext(ctx).
		Where("session_plan_id = ? AND is_active = true", sessionPlanID).
		Order("weekday ASC, start_time ASC").
		Find(&slots).Error
	return slots, err
}

func (r *sessionPlanSlotRepository) Update(ctx context.Context, slot *model.SessionPlanSlot) error {
	// Use Save (not Updates) so boolean fields like is_active=false persist —
	// GORM's Updates(struct) skips zero values.
	return r.db.WithContext(ctx).Save(slot).Error
}

func (r *sessionPlanSlotRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	subq := r.db.
		Table("session_plan_slots AS sps").
		Select("sps.id").
		Joins("JOIN session_plans sp ON sp.id = sps.session_plan_id").
		Where("sps.id = ? AND sp.clinic_id = ?", id, clinicID)

	return r.db.WithContext(ctx).
		Where("id IN (?)", subq).
		Delete(&model.SessionPlanSlot{}).Error
}

func (r *sessionPlanSlotRepository) DeleteByPlan(ctx context.Context, sessionPlanID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("session_plan_id = ?", sessionPlanID).
		Delete(&model.SessionPlanSlot{}).Error
}
