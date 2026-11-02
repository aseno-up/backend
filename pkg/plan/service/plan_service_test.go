package service

import (
	"context"
	"testing"
	"time"

	"github.com/aseno/backend/pkg/plan/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPlanService_CalculateBestPrice(t *testing.T) {
	s := &planService{}

	plan := &model.Plan{
		ID:   uuid.New(),
		Name: "Professional",
		Cycles: []model.PlanCycle{
			{
				CycleType: model.CycleTypeMonthly,
				PriceBRL:  199.0,
			},
			{
				CycleType: model.CycleTypeYearly,
				PriceBRL:  1990.0,
			},
		},
	}

	validOffer := &model.Offer{
		ID:       uuid.New(),
		PlanID:   plan.ID,
		IsActive: true,
		Cycles: []model.OfferCycle{
			{
				CycleType:      model.CycleTypeYearly,
				CustomPriceBRL: 990.0,
			},
		},
	}
	validOffer.ValidUntil = func() *time.Time { t := time.Now().Add(24 * time.Hour); return &t }()

	ctx := context.Background()

	t.Run("Return plan standard price when no offer", func(t *testing.T) {
		price, err := s.CalculateBestPrice(ctx, plan, nil, model.CycleTypeMonthly)
		assert.NoError(t, err)
		assert.Equal(t, 199.0, price)
	})

	t.Run("Return offer cycle custom price over plan cycle price", func(t *testing.T) {
		price, err := s.CalculateBestPrice(ctx, plan, validOffer, model.CycleTypeYearly)
		assert.NoError(t, err)
		assert.Equal(t, 990.0, price)
	})

	t.Run("Fallback to plan price if offer doesn't have the requested cycle", func(t *testing.T) {
		price, err := s.CalculateBestPrice(ctx, plan, validOffer, model.CycleTypeMonthly)
		assert.NoError(t, err)
		assert.Equal(t, 199.0, price)
	})

	t.Run("Return error if plan does not support requested cycle", func(t *testing.T) {
		_, err := s.CalculateBestPrice(ctx, plan, nil, "weekly")
		assert.ErrorContains(t, err, "plan does not support")
	})

	t.Run("Fallback to plan price if offer is invalid", func(t *testing.T) {
		invalidOffer := &model.Offer{
			IsActive: false,
			Cycles: []model.OfferCycle{
				{
					CycleType:      model.CycleTypeYearly,
					CustomPriceBRL: 990.0,
				},
			},
		}

		price, err := s.CalculateBestPrice(ctx, plan, invalidOffer, model.CycleTypeYearly)
		assert.NoError(t, err)
		assert.Equal(t, 1990.0, price)
	})
}

func TestPlanService_ResolvePrice_Solo(t *testing.T) {
	s := &planService{}
	soloMonthly := 55.0
	soloYearly := 550.0
	plan := &model.Plan{
		ID:   uuid.New(),
		Name: "Professional",
		Cycles: []model.PlanCycle{
			{CycleType: model.CycleTypeMonthly, PriceBRL: 199.0, SoloPriceBRL: &soloMonthly},
			{CycleType: model.CycleTypeYearly, PriceBRL: 1990.0, SoloPriceBRL: &soloYearly},
		},
	}
	ctx := context.Background()

	t.Run("solo returns solo monthly price", func(t *testing.T) {
		price, err := s.ResolvePrice(ctx, plan, nil, model.CycleTypeMonthly, model.PlanVariantSolo)
		assert.NoError(t, err)
		assert.Equal(t, 55.0, price)
	})

	t.Run("solo returns solo yearly price", func(t *testing.T) {
		price, err := s.ResolvePrice(ctx, plan, nil, model.CycleTypeYearly, model.PlanVariantSolo)
		assert.NoError(t, err)
		assert.Equal(t, 550.0, price)
	})

	t.Run("solo errors when cycle has no solo price", func(t *testing.T) {
		planNoSolo := &model.Plan{
			ID: uuid.New(),
			Cycles: []model.PlanCycle{
				{CycleType: model.CycleTypeMonthly, PriceBRL: 99.0},
			},
		}
		_, err := s.ResolvePrice(ctx, planNoSolo, nil, model.CycleTypeMonthly, model.PlanVariantSolo)
		assert.ErrorIs(t, err, ErrSoloNotAvailable)
	})

	t.Run("solo ignores a valid offer", func(t *testing.T) {
		offer := &model.Offer{
			ID:       uuid.New(),
			PlanID:   plan.ID,
			IsActive: true,
			Cycles: []model.OfferCycle{
				{CycleType: model.CycleTypeMonthly, CustomPriceBRL: 20.0},
			},
		}
		offer.ValidUntil = func() *time.Time { tm := time.Now().Add(24 * time.Hour); return &tm }()
		price, err := s.ResolvePrice(ctx, plan, offer, model.CycleTypeMonthly, model.PlanVariantSolo)
		assert.NoError(t, err)
		assert.Equal(t, 55.0, price)
	})

	t.Run("clinic variant preserves standard pricing", func(t *testing.T) {
		price, err := s.ResolvePrice(ctx, plan, nil, model.CycleTypeMonthly, model.PlanVariantClinic)
		assert.NoError(t, err)
		assert.Equal(t, 199.0, price)
	})

	t.Run("CalculateBestPrice keeps backward compatibility", func(t *testing.T) {
		price, err := s.CalculateBestPrice(ctx, plan, nil, model.CycleTypeMonthly)
		assert.NoError(t, err)
		assert.Equal(t, 199.0, price)
	})
}

func TestNormalizeVariant(t *testing.T) {
	assert.Equal(t, model.PlanVariantSolo, model.NormalizeVariant("solo"))
	assert.Equal(t, model.PlanVariantClinic, model.NormalizeVariant(""))
	assert.Equal(t, model.PlanVariantClinic, model.NormalizeVariant("clinic"))
	assert.Equal(t, model.PlanVariantClinic, model.NormalizeVariant("garbage"))
	assert.True(t, model.PlanVariantSolo.IsSolo())
	assert.False(t, model.PlanVariantClinic.IsSolo())
}
