package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/promotion/model"
)

type PromotionRepository interface {
	Create(ctx context.Context, p *model.Promotion) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Promotion, error)
	Update(ctx context.Context, p *model.Promotion) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]model.Promotion, error)
	// GetActive returns the single active promotion whose window contains `now`,
	// or (nil, nil) if none. It does NOT return ErrNotFound for the empty case
	// because callers (landing page) expect "no promotion" as a normal state.
	GetActive(ctx context.Context, now time.Time) (*model.Promotion, error)
	// DeactivateAllExcept clears the is_active flag on every row except the one
	// with the given id. Used inside a transaction when activating a new promo,
	// to enforce the single-active invariant before the unique index would block us.
	DeactivateAllExcept(ctx context.Context, tx *gorm.DB, exceptID uuid.UUID) error
	// WithTx returns a repository bound to the given transaction.
	WithTx(tx *gorm.DB) PromotionRepository
	// DB exposes the underlying *gorm.DB so the service can open transactions.
	DB() *gorm.DB
}

type promotionRepository struct {
	db *gorm.DB
}

func NewPromotionRepository(db *gorm.DB) PromotionRepository {
	return &promotionRepository{db: db}
}

func (r *promotionRepository) DB() *gorm.DB { return r.db }

func (r *promotionRepository) WithTx(tx *gorm.DB) PromotionRepository {
	return &promotionRepository{db: tx}
}

func (r *promotionRepository) Create(ctx context.Context, p *model.Promotion) error {
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *promotionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Promotion, error) {
	var p model.Promotion
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &p, nil
}

func (r *promotionRepository) Update(ctx context.Context, p *model.Promotion) error {
	if err := r.db.WithContext(ctx).Save(p).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *promotionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&model.Promotion{}, "id = ?", id).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *promotionRepository) List(ctx context.Context) ([]model.Promotion, error) {
	var promos []model.Promotion
	if err := r.db.WithContext(ctx).
		Order("is_active DESC, starts_at DESC").
		Find(&promos).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return promos, nil
}

func (r *promotionRepository) GetActive(ctx context.Context, now time.Time) (*model.Promotion, error) {
	var p model.Promotion
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND starts_at <= ? AND expires_at > ?", true, now, now).
		First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &p, nil
}

func (r *promotionRepository) DeactivateAllExcept(ctx context.Context, tx *gorm.DB, exceptID uuid.UUID) error {
	db := tx
	if db == nil {
		db = r.db
	}
	if err := db.WithContext(ctx).
		Model(&model.Promotion{}).
		Where("is_active = ? AND id <> ?", true, exceptID).
		Update("is_active", false).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}
