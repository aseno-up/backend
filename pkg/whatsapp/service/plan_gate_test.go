package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	account_model "github.com/aseno/backend/pkg/account/model"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	payment_model "github.com/aseno/backend/pkg/payment/model"
	plan_model "github.com/aseno/backend/pkg/plan/model"
)

type stubSubscriptionFinder struct {
	sub *payment_model.Subscription
	err error
}

func (s *stubSubscriptionFinder) GetByAccountID(_ context.Context, _ uuid.UUID) (*payment_model.Subscription, error) {
	return s.sub, s.err
}

type stubPlanFinder struct {
	plan *plan_model.Plan
	err  error
}

func (s *stubPlanFinder) GetByID(_ context.Context, _ uuid.UUID) (*plan_model.Plan, error) {
	return s.plan, s.err
}

func clinicWithAccount(status account_model.SubscriptionStatus, trialEndsAt *time.Time) *clinic_model.Clinic {
	accID := uuid.New()
	return &clinic_model.Clinic{
		AccountID: accID,
		Account: &account_model.Account{
			ID:                 accID,
			SubscriptionStatus: status,
			TrialEndsAt:        trialEndsAt,
		},
	}
}

func TestPlanGate_TrialActive_Allowed(t *testing.T) {
	future := time.Now().Add(7 * 24 * time.Hour)
	clinic := clinicWithAccount(account_model.SubscriptionStatusTrial, &future)

	gate := NewPlanGateChecker(&stubSubscriptionFinder{}, &stubPlanFinder{})
	allowed, err := gate.IsAIAgentAllowed(context.Background(), clinic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("trial with valid TrialEndsAt should be allowed")
	}
}

func TestPlanGate_TrialExpired_Denied(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	clinic := clinicWithAccount(account_model.SubscriptionStatusTrial, &past)

	gate := NewPlanGateChecker(&stubSubscriptionFinder{}, &stubPlanFinder{})
	allowed, err := gate.IsAIAgentAllowed(context.Background(), clinic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Errorf("expired trial should be denied")
	}
}

func TestPlanGate_ActiveProfessional_Allowed(t *testing.T) {
	clinic := clinicWithAccount(account_model.SubscriptionStatusActive, nil)
	planID := uuid.New()
	subFinder := &stubSubscriptionFinder{sub: &payment_model.Subscription{PlanID: &planID}}
	planFinder := &stubPlanFinder{plan: &plan_model.Plan{ID: planID, Slug: "professional"}}

	gate := NewPlanGateChecker(subFinder, planFinder)
	allowed, err := gate.IsAIAgentAllowed(context.Background(), clinic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("active professional plan should be allowed")
	}
}

func TestPlanGate_ActiveAdvanced_Allowed(t *testing.T) {
	clinic := clinicWithAccount(account_model.SubscriptionStatusActive, nil)
	planID := uuid.New()
	subFinder := &stubSubscriptionFinder{sub: &payment_model.Subscription{PlanID: &planID}}
	planFinder := &stubPlanFinder{plan: &plan_model.Plan{ID: planID, Slug: "advanced"}}

	gate := NewPlanGateChecker(subFinder, planFinder)
	allowed, err := gate.IsAIAgentAllowed(context.Background(), clinic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("active advanced plan should be allowed")
	}
}

func TestPlanGate_ActiveBasic_Denied(t *testing.T) {
	clinic := clinicWithAccount(account_model.SubscriptionStatusActive, nil)
	planID := uuid.New()
	subFinder := &stubSubscriptionFinder{sub: &payment_model.Subscription{PlanID: &planID}}
	planFinder := &stubPlanFinder{plan: &plan_model.Plan{ID: planID, Slug: "basic"}}

	gate := NewPlanGateChecker(subFinder, planFinder)
	allowed, err := gate.IsAIAgentAllowed(context.Background(), clinic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Errorf("basic plan should be denied")
	}
}

func TestPlanGate_Canceled_Denied(t *testing.T) {
	clinic := clinicWithAccount(account_model.SubscriptionStatusCanceled, nil)

	gate := NewPlanGateChecker(&stubSubscriptionFinder{}, &stubPlanFinder{})
	allowed, err := gate.IsAIAgentAllowed(context.Background(), clinic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Errorf("canceled subscription should be denied")
	}
}

func TestPlanGate_ActiveWithoutSubscriptionRecord_Denied(t *testing.T) {
	clinic := clinicWithAccount(account_model.SubscriptionStatusActive, nil)
	subFinder := &stubSubscriptionFinder{err: errors.New("not found")}

	gate := NewPlanGateChecker(subFinder, &stubPlanFinder{})
	allowed, err := gate.IsAIAgentAllowed(context.Background(), clinic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Errorf("active status without subscription record should be denied")
	}
}

func TestPlanGate_NilClinic_Denied(t *testing.T) {
	gate := NewPlanGateChecker(&stubSubscriptionFinder{}, &stubPlanFinder{})
	allowed, err := gate.IsAIAgentAllowed(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Errorf("nil clinic should be denied")
	}
}

func TestPlanGate_NilAccount_Denied(t *testing.T) {
	clinic := &clinic_model.Clinic{}
	gate := NewPlanGateChecker(&stubSubscriptionFinder{}, &stubPlanFinder{})
	allowed, err := gate.IsAIAgentAllowed(context.Background(), clinic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Errorf("clinic without account should be denied")
	}
}
