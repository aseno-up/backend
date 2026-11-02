package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

// stubPatientAIPrefChecker retorna um valor fixo para IsAISchedulingEnabledByPhone.
type stubPatientAIPrefChecker struct {
	enabled bool
	err     error
}

func (s *stubPatientAIPrefChecker) IsAISchedulingEnabledByPhone(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return s.enabled, s.err
}

// stubClinicRepo retorna uma clínica com AIAgentEnabled configurável.
type stubClinicRepo struct {
	clinic *clinic_model.Clinic
	err    error
}

func (r *stubClinicRepo) GetByID(_ context.Context, _ uuid.UUID) (*clinic_model.Clinic, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.clinic, nil
}

// aiEnabledClinic retorna uma clínica com AIAgentEnabled=true e horário 24h.
func aiEnabledClinic() *clinic_model.Clinic {
	allDay := &clinic_model.DaySchedule{Enabled: true, StartTime: "00:00", EndTime: "23:59"}
	return &clinic_model.Clinic{
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled: true,
			AIBusinessHours: &clinic_model.BusinessHoursConfig{
				Monday:    allDay,
				Tuesday:   allDay,
				Wednesday: allDay,
				Thursday:  allDay,
				Friday:    allDay,
				Saturday:  allDay,
				Sunday:    allDay,
			},
		},
	}
}

func newTestServiceWithAIPref(ai *stubAIAgentService, prefChecker PatientAIPreferenceChecker) *whatsAppService {
	svc := NewWhatsAppService(&stubWhatsAppClient{}, &stubMessageRepo{}, nil).(*whatsAppService)
	svc.aiAgentService = ai
	svc.debounceDelay = 0
	svc.clinicRepo = &stubClinicRepo{clinic: aiEnabledClinic()}
	svc.patientAIPrefChecker = prefChecker
	return svc
}

// ---------------------------------------------------------------------------
// Testes
// ---------------------------------------------------------------------------

func TestHandleWithAIAgent_PatientAIDisabled_SkipsAgent(t *testing.T) {
	aiSvc := &stubAIAgentService{}
	prefChecker := &stubPatientAIPrefChecker{enabled: false}
	svc := newTestServiceWithAIPref(aiSvc, prefChecker)

	err := svc.handleWithAIAgent(context.Background(), uuid.New(), newMsg("+5511999990010", "quero marcar consulta"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if aiSvc.callCount() != 0 {
		t.Errorf("AI agent should NOT have been called when patient has AI disabled, got %d calls", aiSvc.callCount())
	}
}

func TestHandleWithAIAgent_PatientAIEnabled_CallsAgent(t *testing.T) {
	aiSvc := &stubAIAgentService{}
	prefChecker := &stubPatientAIPrefChecker{enabled: true}
	svc := newTestServiceWithAIPref(aiSvc, prefChecker)

	err := svc.handleWithAIAgent(context.Background(), uuid.New(), newMsg("+5511999990011", "quero marcar consulta"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if aiSvc.callCount() != 1 {
		t.Errorf("AI agent should have been called when patient has AI enabled, got %d calls", aiSvc.callCount())
	}
}

func TestHandleWithAIAgent_NoPrefChecker_CallsAgent(t *testing.T) {
	aiSvc := &stubAIAgentService{}
	svc := newTestServiceWithAIPref(aiSvc, nil)

	err := svc.handleWithAIAgent(context.Background(), uuid.New(), newMsg("+5511999990012", "quero marcar consulta"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if aiSvc.callCount() != 1 {
		t.Errorf("AI agent should have been called when no pref checker is set (backward compat), got %d calls", aiSvc.callCount())
	}
}

func TestHandleWithAIAgent_PrefCheckerError_CallsAgent(t *testing.T) {
	aiSvc := &stubAIAgentService{}
	prefChecker := &stubPatientAIPrefChecker{enabled: false, err: context.DeadlineExceeded}
	svc := newTestServiceWithAIPref(aiSvc, prefChecker)

	err := svc.handleWithAIAgent(context.Background(), uuid.New(), newMsg("+5511999990013", "quero marcar consulta"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// On error, should default to calling AI (fail-open)
	if aiSvc.callCount() != 1 {
		t.Errorf("AI agent should have been called on preference check error (fail-open), got %d calls", aiSvc.callCount())
	}
}

// Garantir que o import de time é utilizado (evita erro de compilação)
var _ = time.Now
