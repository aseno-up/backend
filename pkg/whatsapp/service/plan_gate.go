package service

import (
	"context"

	"github.com/google/uuid"

	account_model "github.com/aseno/backend/pkg/account/model"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	payment_model "github.com/aseno/backend/pkg/payment/model"
	plan_model "github.com/aseno/backend/pkg/plan/model"
)

// PlanGateChecker decides whether the AI agent may serve a given clinic,
// based on the account subscription status and active plan.
//
// Rules:
//  1. If the account subscription is not active (expired trial, canceled, etc.),
//     deny access.
//  2. Trial accounts are granted full access ("tudo do Profissional") while
//     the trial period is valid.
//  3. Subscribers are allowed only when subscriptions.plan_id points to a plan
//     whose slug is in aiAgentAllowedPlanSlugs.
type PlanGateChecker interface {
	IsAIAgentAllowed(ctx context.Context, clinic *clinic_model.Clinic) (bool, error)
}

// SubscriptionFinder is the narrow interface over the subscription repository
// used by the plan gate.
type SubscriptionFinder interface {
	GetByAccountID(ctx context.Context, accountID uuid.UUID) (*payment_model.Subscription, error)
}

// PlanFinder is the narrow interface over the plan repository used by the
// plan gate.
type PlanFinder interface {
	GetByID(ctx context.Context, id uuid.UUID) (*plan_model.Plan, error)
}

var aiAgentAllowedPlanSlugs = map[string]struct{}{
	"professional": {},
	"advanced":     {},
}

type planGateChecker struct {
	subscriptions SubscriptionFinder
	plans         PlanFinder
}

// NewPlanGateChecker returns a PlanGateChecker backed by the given finders.
func NewPlanGateChecker(subscriptions SubscriptionFinder, plans PlanFinder) PlanGateChecker {
	return &planGateChecker{subscriptions: subscriptions, plans: plans}
}

func (c *planGateChecker) IsAIAgentAllowed(ctx context.Context, clinic *clinic_model.Clinic) (bool, error) {
	if clinic == nil || clinic.Account == nil {
		return false, nil
	}

	if !clinic.Account.IsSubscriptionActive() {
		return false, nil
	}

	if clinic.Account.SubscriptionStatus == account_model.SubscriptionStatusTrial {
		return true, nil
	}

	sub, err := c.subscriptions.GetByAccountID(ctx, clinic.AccountID)
	if err != nil || sub == nil || sub.PlanID == nil {
		return false, nil
	}

	plan, err := c.plans.GetByID(ctx, *sub.PlanID)
	if err != nil || plan == nil {
		return false, nil
	}

	_, ok := aiAgentAllowedPlanSlugs[plan.Slug]
	return ok, nil
}
