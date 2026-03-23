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
)

type PlanService interface {
	ListPlans(ctx context.Context) ([]model.Plan, error)
	GetPlanByID(ctx context.Context, id uuid.UUID) (*model.Plan, error)
	ValidateOffer(ctx context.Context, token string) (*model.Offer, error)
	CalculateBestPrice(ctx context.Context, plan *model.Plan, offer *model.Offer, cycleType model.CycleType) (float64, error)
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
	var bestPrice float64 = -1

	// Check if plan has this cycle
	var planCycle *model.PlanCycle
	for _, c := range plan.Cycles {
		if c.CycleType == cycleType {
			planCycle = &c
			break
		}
	}

	if planCycle == nil {
		return 0, errors.New("plan does not support the requested billing cycle")
	}
	bestPrice = planCycle.PriceBRL

	// Prioridade: Preço customizado da oferta (OfferCycle) -> Preço padrão (PlanCycle)
	if offer != nil && offer.IsValid() {
		// Validate that the offer is for this plan
		if offer.PlanID != plan.ID {
			return 0, ErrOfferPlanMismatch
		}
		for _, oc := range offer.Cycles {
			if oc.CycleType == cycleType {
				return oc.CustomPriceBRL, nil
			}
		}
	}

	return bestPrice, nil
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
