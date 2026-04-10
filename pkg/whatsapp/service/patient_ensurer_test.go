package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

// stubPatientEnsurer records calls to EnsureByPhone.
type stubPatientEnsurer struct {
	mu    sync.Mutex
	calls []ensurerCall
	err   error
}

type ensurerCall struct {
	ClinicID   uuid.UUID
	Phone      string
	SenderName string
}

func (s *stubPatientEnsurer) EnsureByPhone(_ context.Context, clinicID uuid.UUID, phone, senderName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, ensurerCall{ClinicID: clinicID, Phone: phone, SenderName: senderName})
	return s.err
}

func (s *stubPatientEnsurer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *stubPatientEnsurer) lastCall() *ensurerCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return nil
	}
	c := s.calls[len(s.calls)-1]
	return &c
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestHandleIncomingMessage_EnsuresPatientBeforeAI verifica que o paciente é
// criado/encontrado ANTES do AI processar a mensagem. Isso permite que a
// clínica desabilite a IA para aquele contato antes da próxima resposta.
func TestHandleIncomingMessage_EnsuresPatientBeforeAI(t *testing.T) {
	ai := &stubAIAgentService{}
	ensurer := &stubPatientEnsurer{}
	debounce := 50 * time.Millisecond

	svc := NewWhatsAppService(&stubWhatsAppClient{}, &stubMessageRepo{}, nil).(*whatsAppService)
	svc.aiAgentService = ai
	svc.debounceDelay = debounce
	svc.clinicRepo = &stubClinicRepo{clinic: aiEnabledClinic()}
	svc.patientEnsurer = ensurer

	clinicID := uuid.New()
	phone := "+5511999998888"
	msg := newMsg(phone, "Oi, quero agendar")
	msg.SenderName = "João Silva"

	if err := svc.HandleIncomingMessage(context.Background(), clinicID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage returned error: %v", err)
	}

	// EnsureByPhone deve ter sido chamado imediatamente (antes do debounce)
	if ensurer.callCount() != 1 {
		t.Fatalf("expected 1 call to EnsureByPhone, got %d", ensurer.callCount())
	}

	call := ensurer.lastCall()
	if call.ClinicID != clinicID {
		t.Errorf("expected clinicID %s, got %s", clinicID, call.ClinicID)
	}
	if call.Phone != phone {
		t.Errorf("expected phone %s, got %s", phone, call.Phone)
	}
	if call.SenderName != "João Silva" {
		t.Errorf("expected senderName 'João Silva', got %q", call.SenderName)
	}
}

// TestHandleIncomingMessage_EnsurePatient_NoEnsurer verifica que quando
// o PatientEnsurer não está configurado, o fluxo continua normalmente.
func TestHandleIncomingMessage_EnsurePatient_NoEnsurer(t *testing.T) {
	ai := &stubAIAgentService{}
	debounce := 50 * time.Millisecond

	svc := NewWhatsAppService(&stubWhatsAppClient{}, &stubMessageRepo{}, nil).(*whatsAppService)
	svc.aiAgentService = ai
	svc.debounceDelay = debounce
	svc.clinicRepo = &stubClinicRepo{clinic: aiEnabledClinic()}
	// patientEnsurer NOT set

	clinicID := uuid.New()
	msg := newMsg("+5511999997777", "Oi")

	if err := svc.HandleIncomingMessage(context.Background(), clinicID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage returned error: %v", err)
	}

	// Wait for debounce
	time.Sleep(debounce + 80*time.Millisecond)

	// AI should still be called
	if ai.callCount() != 1 {
		t.Errorf("AI should have been called even without ensurer, got %d calls", ai.callCount())
	}
}

// TestHandleIncomingMessage_EnsurePatient_ErrorDoesNotBlock verifica que
// um erro no EnsureByPhone não impede o fluxo normal (fail-open).
func TestHandleIncomingMessage_EnsurePatient_ErrorDoesNotBlock(t *testing.T) {
	ai := &stubAIAgentService{}
	ensurer := &stubPatientEnsurer{err: context.DeadlineExceeded}
	debounce := 50 * time.Millisecond

	svc := NewWhatsAppService(&stubWhatsAppClient{}, &stubMessageRepo{}, nil).(*whatsAppService)
	svc.aiAgentService = ai
	svc.debounceDelay = debounce
	svc.clinicRepo = &stubClinicRepo{clinic: aiEnabledClinic()}
	svc.patientEnsurer = ensurer

	clinicID := uuid.New()
	msg := newMsg("+5511999996666", "Oi")

	if err := svc.HandleIncomingMessage(context.Background(), clinicID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage returned error: %v", err)
	}

	// Wait for debounce
	time.Sleep(debounce + 80*time.Millisecond)

	// AI should still be called even though ensurer failed
	if ai.callCount() != 1 {
		t.Errorf("AI should have been called despite ensurer error, got %d calls", ai.callCount())
	}
}

// TestHandleIncomingMessage_EnsurePatient_AIDisabled verifica que o
// paciente é criado mesmo quando a clínica tem a IA desabilitada.
// Isso é importante para que o contato apareça no painel.
func TestHandleIncomingMessage_EnsurePatient_AIDisabled(t *testing.T) {
	ai := &stubAIAgentService{}
	ensurer := &stubPatientEnsurer{}

	svc := NewWhatsAppService(&stubWhatsAppClient{}, &stubMessageRepo{}, nil).(*whatsAppService)
	svc.aiAgentService = ai
	svc.debounceDelay = 50 * time.Millisecond
	svc.clinicRepo = &stubClinicRepo{clinic: aiDisabledClinic()}
	svc.patientEnsurer = ensurer

	clinicID := uuid.New()
	msg := newMsg("+5511999995555", "Oi")
	msg.SenderName = "Maria"

	if err := svc.HandleIncomingMessage(context.Background(), clinicID, msg); err != nil {
		t.Fatalf("HandleIncomingMessage returned error: %v", err)
	}

	// Patient should still be ensured even with AI disabled
	if ensurer.callCount() != 1 {
		t.Errorf("expected 1 call to EnsureByPhone even with AI disabled, got %d", ensurer.callCount())
	}
}
