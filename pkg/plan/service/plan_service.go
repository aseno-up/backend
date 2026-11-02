package service

import (
	"context"
	"errors"

	"github.com/aseno/backend/pkg/plan/model"
	"github.com/aseno/backend/pkg/plan/repository"
	"github.com/google/uuid"
)

var (
	ErrOfferInvalid      = errors.New("offer is invalid or expired")
	ErrOfferNotFound     = errors.New("offer not found")
	ErrOfferPlanMismatch = errors.New("offer is not valid for this plan")
	ErrPlanNotFound      = errors.New("plan not found")
	ErrSoloNotAvailable  = errors.New("plan does not offer a solo variant for this cycle")
	ErrCycleNotSupported = errors.New("plan does not support the requested billing cycle")
)

type PlanService interface {
	ListPlans(ctx context.Context) ([]model.Plan, error)
	GetPlanByID(ctx context.Context, id uuid.UUID) (*model.Plan, error)
	GetBySlug(ctx context.Context, slug string) (*model.Plan, error)
	ValidateOffer(ctx context.Context, token string) (*model.Offer, error)
	CalculateBestPrice(ctx context.Context, plan *model.Plan, offer *model.Offer, cycleType model.CycleType) (float64, error)
	ResolvePrice(ctx context.Context, plan *model.Plan, offer *model.Offer, cycleType model.CycleType, variant model.PlanVariant) (float64, error)
	UseOffer(ctx context.Context, offerID uuid.UUID) error
	CreatePlan(ctx context.Context, plan *model.Plan) error
	UpdatePlan(ctx context.Context, plan *model.Plan) error
	CreateOffer(ctx context.Context, offer *model.Offer) error
	ListOffers(ctx context.Context) ([]model.Offer, error)
	UpdateOffer(ctx context.Context, offer *model.Offer) error
}

type planService struct {
	planRepo  repository.PlanRepository
	offerRepo repository.OfferRepository
}

func NewPlanService(planRepo repository.PlanRepository, offerRepo repository.OfferRepository) PlanService {
	return &planService{
		planRepo:  planRepo,
		offerRepo: offerRepo,
	}
}

func (s *planService) ListPlans(ctx context.Context) ([]model.Plan, error) {
	return s.planRepo.List(ctx)
}

func (s *planService) GetPlanByID(ctx context.Context, id uuid.UUID) (*model.Plan, error) {
	return s.planRepo.GetByID(ctx, id)
}

func (s *planService) GetBySlug(ctx context.Context, slug string) (*model.Plan, error) {
	return s.planRepo.GetBySlug(ctx, slug)
}

func (s *planService) ValidateOffer(ctx context.Context, token string) (*model.Offer, error) {
	offer, err := s.offerRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, ErrOfferNotFound
	}
	if !offer.IsValid() {
		return nil, ErrOfferInvalid
	}
	return offer, nil
}

func (s *planService) CalculateBestPrice(ctx context.Context, plan *model.Plan, offer *model.Offer, cycleType model.CycleType) (float64, error) {
	return s.ResolvePrice(ctx, plan, offer, cycleType, model.PlanVariantClinic)
}

// ResolvePrice returns the price for a plan/cycle/variant combination.
// For solo variant the cycle must have SoloPriceBRL set and offers are ignored.
// For clinic variant the existing offer-vs-standard price rule is preserved.
func (s *planService) ResolvePrice(ctx context.Context, plan *model.Plan, offer *model.Offer, cycleType model.CycleType, variant model.PlanVariant) (float64, error) {
	var planCycle *model.PlanCycle
	for i := range plan.Cycles {
		if plan.Cycles[i].CycleType == cycleType {
			planCycle = &plan.Cycles[i]
			break
		}
	}
	if planCycle == nil {
		return 0, ErrCycleNotSupported
	}

	if variant.IsSolo() {
		if planCycle.SoloPriceBRL == nil {
			return 0, ErrSoloNotAvailable
		}
		return *planCycle.SoloPriceBRL, nil
	}

	if offer != nil && offer.IsValid() {
		if offer.PlanID != plan.ID {
			return 0, ErrOfferPlanMismatch
		}
		for _, oc := range offer.Cycles {
			if oc.CycleType == cycleType {
				return oc.CustomPriceBRL, nil
			}
		}
	}

	return planCycle.PriceBRL, nil
}

func (s *planService) UseOffer(ctx context.Context, offerID uuid.UUID) error {
	return s.offerRepo.IncrementUsage(ctx, offerID)
}

func (s *planService) CreatePlan(ctx context.Context, plan *model.Plan) error {
	return s.planRepo.Create(ctx, plan)
}

func (s *planService) UpdatePlan(ctx context.Context, plan *model.Plan) error {
	return s.planRepo.Update(ctx, plan)
}

func (s *planService) CreateOffer(ctx context.Context, offer *model.Offer) error {
	return s.offerRepo.Create(ctx, offer)
}

func (s *planService) ListOffers(ctx context.Context) ([]model.Offer, error) {
	return s.offerRepo.List(ctx)
}

func (s *planService) UpdateOffer(ctx context.Context, offer *model.Offer) error {
	return s.offerRepo.Update(ctx, offer)
}
