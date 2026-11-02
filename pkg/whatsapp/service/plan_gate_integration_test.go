package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
)

type stubPlanGate struct {
	allowed bool
	err     error
	calls   int
}

func (s *stubPlanGate) IsAIAgentAllowed(_ context.Context, _ *clinic_model.Clinic) (bool, error) {
	s.calls++
	return s.allowed, s.err
}

func newTestServiceWithPlanGate(ai *stubAIAgentService, gate PlanGateChecker) *whatsAppService {
	svc := NewWhatsAppService(&stubWhatsAppClient{}, &stubMessageRepo{}, nil).(*whatsAppService)
	svc.aiAgentService = ai
	svc.debounceDelay = 0
	svc.clinicRepo = &stubClinicRepo{clinic: aiEnabledClinic()}
	svc.planGateChecker = gate
	return svc
}

func TestHandleWithAIAgent_PlanGateDenies_SkipsAgent(t *testing.T) {
	aiSvc := &stubAIAgentService{}
	gate := &stubPlanGate{allowed: false}
	svc := newTestServiceWithPlanGate(aiSvc, gate)

	err := svc.handleWithAIAgent(context.Background(), uuid.New(), newMsg("+5511999990020", "quero marcar"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if aiSvc.callCount() != 0 {
		t.Errorf("agent should not be called when plan gate denies, got %d calls", aiSvc.callCount())
	}
	if gate.calls != 1 {
		t.Errorf("plan gate should be consulted once, got %d", gate.calls)
	}
}

func TestHandleWithAIAgent_PlanGateAllows_CallsAgent(t *testing.T) {
	aiSvc := &stubAIAgentService{}
	gate := &stubPlanGate{allowed: true}
	svc := newTestServiceWithPlanGate(aiSvc, gate)

	err := svc.handleWithAIAgent(context.Background(), uuid.New(), newMsg("+5511999990021", "quero marcar"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if aiSvc.callCount() != 1 {
		t.Errorf("agent should be called when plan gate allows, got %d calls", aiSvc.callCount())
	}
}

func TestHandleWithAIAgent_PlanGateError_SkipsAgent(t *testing.T) {
	aiSvc := &stubAIAgentService{}
	gate := &stubPlanGate{err: context.DeadlineExceeded}
	svc := newTestServiceWithPlanGate(aiSvc, gate)

	err := svc.handleWithAIAgent(context.Background(), uuid.New(), newMsg("+5511999990022", "quero marcar"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fail-closed: when we cannot determine the plan, skip the agent to avoid
	// consuming AI credits for unauthorized clinics.
	if aiSvc.callCount() != 0 {
		t.Errorf("agent should not be called on plan gate error (fail-closed), got %d calls", aiSvc.callCount())
	}
}

func TestHandleWithAIAgent_NoPlanGate_CallsAgent(t *testing.T) {
	aiSvc := &stubAIAgentService{}
	svc := newTestServiceWithPlanGate(aiSvc, nil)

	err := svc.handleWithAIAgent(context.Background(), uuid.New(), newMsg("+5511999990023", "quero marcar"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Backward compat: if no plan gate is wired, the agent is called.
	if aiSvc.callCount() != 1 {
		t.Errorf("agent should be called when no plan gate is wired, got %d calls", aiSvc.callCount())
	}
}
