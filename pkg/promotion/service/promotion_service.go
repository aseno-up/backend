package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/promotion/model"
	"github.com/aseno/backend/pkg/promotion/repository"
)

// Clock isolates time.Now so tests can advance it deterministically.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type PromotionService interface {
	Create(ctx context.Context, req *model.CreatePromotionRequest) (*model.Promotion, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdatePromotionRequest) (*model.Promotion, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) (*model.PromotionListResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Promotion, error)
	GetActive(ctx context.Context) (*model.Promotion, error)
}

type promotionService struct {
	repo  repository.PromotionRepository
	clock Clock
}

func NewPromotionService(repo repository.PromotionRepository) PromotionService {
	return &promotionService{repo: repo, clock: systemClock{}}
}

// NewPromotionServiceWithClock is intended for tests that need to control time.
func NewPromotionServiceWithClock(repo repository.PromotionRepository, clock Clock) PromotionService {
	return &promotionService{repo: repo, clock: clock}
}

func validateDates(starts, expires time.Time) error {
	if !starts.Before(expires) {
		return errors.ErrValidationInstance.WithMessage("starts_at deve ser anterior a expires_at")
	}
	return nil
}

func validateDiscount(percent int) error {
	if percent < 1 || percent > 90 {
		return errors.ErrValidationInstance.WithMessage("discount_percent deve estar entre 1 e 90")
	}
	return nil
}

func validatePlans(plans []string) error {
	if len(plans) == 0 {
		return errors.ErrValidationInstance.WithMessage("plans_affected não pode estar vazio")
	}
	return nil
}

func (s *promotionService) Create(ctx context.Context, req *model.CreatePromotionRequest) (*model.Promotion, error) {
	if err := validateDates(req.StartsAt, req.ExpiresAt); err != nil {
		return nil, err
	}
	if err := validateDiscount(req.DiscountPercent); err != nil {
		return nil, err
	}
	if err := validatePlans(req.PlansAffected); err != nil {
		return nil, err
	}

	p := &model.Promotion{
		Name:            req.Name,
		Description:     req.Description,
		DiscountPercent: req.DiscountPercent,
		StartsAt:        req.StartsAt,
		ExpiresAt:       req.ExpiresAt,
		IsActive:        req.IsActive,
		PlansAffected:   pq.StringArray(req.PlansAffected),
		BannerLabel:     req.BannerLabel,
		BannerShowTimer: req.BannerShowTimer,
	}

	// If creating as active, transactionally deactivate any other active row first.
	if p.IsActive {
		tx := s.repo.DB().Begin()
		if tx.Error != nil {
			return nil, errors.ErrInternalInstance.WithError(tx.Error)
		}
		txRepo := s.repo.WithTx(tx)
		// New row doesn't have an ID yet; use a zero UUID — DeactivateAllExcept
		// will clear every is_active row, which is what we want.
		if err := txRepo.DeactivateAllExcept(ctx, tx, uuid.Nil); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := txRepo.Create(ctx, p); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Commit().Error; err != nil {
			return nil, errors.ErrInternalInstance.WithError(err)
		}
		return p, nil
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *promotionService) Update(ctx context.Context, id uuid.UUID, req *model.UpdatePromotionRequest) (*model.Promotion, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.DiscountPercent != nil {
		if err := validateDiscount(*req.DiscountPercent); err != nil {
			return nil, err
		}
		p.DiscountPercent = *req.DiscountPercent
	}
	if req.StartsAt != nil {
		p.StartsAt = *req.StartsAt
	}
	if req.ExpiresAt != nil {
		p.ExpiresAt = *req.ExpiresAt
	}
	if err := validateDates(p.StartsAt, p.ExpiresAt); err != nil {
		return nil, err
	}
	if req.PlansAffected != nil {
		if err := validatePlans(*req.PlansAffected); err != nil {
			return nil, err
		}
		p.PlansAffected = pq.StringArray(*req.PlansAffected)
	}
	if req.BannerLabel != nil {
		p.BannerLabel = *req.BannerLabel
	}
	if req.BannerShowTimer != nil {
		p.BannerShowTimer = *req.BannerShowTimer
	}

	// Toggling is_active to true requires deactivating others first.
	if req.IsActive != nil && *req.IsActive && !p.IsActive {
		tx := s.repo.DB().Begin()
		if tx.Error != nil {
			return nil, errors.ErrInternalInstance.WithError(tx.Error)
		}
		txRepo := s.repo.WithTx(tx)
		if err := txRepo.DeactivateAllExcept(ctx, tx, p.ID); err != nil {
			tx.Rollback()
			return nil, err
		}
		p.IsActive = true
		if err := txRepo.Update(ctx, p); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Commit().Error; err != nil {
			return nil, errors.ErrInternalInstance.WithError(err)
		}
		return p, nil
	}
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}

	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *promotionService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *promotionService) List(ctx context.Context) (*model.PromotionListResponse, error) {
	promos, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	return &model.PromotionListResponse{Data: promos}, nil
}

func (s *promotionService) GetByID(ctx context.Context, id uuid.UUID) (*model.Promotion, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *promotionService) GetActive(ctx context.Context) (*model.Promotion, error) {
	return s.repo.GetActive(ctx, s.clock.Now())
}
