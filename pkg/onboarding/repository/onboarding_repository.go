package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/onboarding/model"
)

// OnboardingTokenRepository persists and retrieves magic-link tokens issued
// by the landing-page "Receba seu link de acesso" form.
type OnboardingTokenRepository interface {
	Create(ctx context.Context, t *model.OnboardingToken) error
	GetByToken(ctx context.Context, token string) (*model.OnboardingToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error
}

type onboardingTokenRepository struct {
	db *gorm.DB
}

func NewOnboardingTokenRepository(db *gorm.DB) OnboardingTokenRepository {
	return &onboardingTokenRepository{db: db}
}

func (r *onboardingTokenRepository) Create(ctx context.Context, t *model.OnboardingToken) error {
	if err := r.db.WithContext(ctx).Create(t).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *onboardingTokenRepository) GetByToken(ctx context.Context, token string) (*model.OnboardingToken, error) {
	var t model.OnboardingToken
	if err := r.db.WithContext(ctx).Where("token = ?", token).First(&t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &t, nil
}

func (r *onboardingTokenRepository) MarkUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&model.OnboardingToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", usedAt)
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	if res.RowsAffected == 0 {
		// Either the token doesn't exist or it's already been used.
		return errors.ErrConflictInstance.WithMessage("token already used or not found")
	}
	return nil
}
