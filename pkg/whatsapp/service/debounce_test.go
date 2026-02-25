package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/whatsapp/agent"
	"github.com/aseno/backend/pkg/whatsapp/model"
)

// ---------------------------------------------------------------------------
// Helpers / stubs
// ---------------------------------------------------------------------------

// stubAIAgentService implementa agent.Service para testes.
// Registra cada chamada a ProcessMessage e retorna uma resposta vazia mas válida.
type stubAIAgentService struct {
	mu    sync.Mutex
	calls []agent.MessageRequest
}

func (s *stubAIAgentService) ProcessMessage(_ context.Context, req *agent.MessageRequest) (*agent.MessageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, *req)
	return &agent.MessageResponse{
		ResponseText: "resposta stub",
		MessageType:  model.MessageTypeText,
		Segments: []*model.MessageSegment{
			{Content: "resposta stub", Type: model.SegmentTypeContent},
		},
	}, nil
}

func (s *stubAIAgentService) ShouldUseAIAgent(_ model.MessageIntent) bool {
	return true
}

func (s *stubAIAgentService) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *stubAIAgentService) lastRequest() *agent.MessageRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return nil
	}
	r := s.calls[len(s.calls)-1]
	return &r
}

// stubWhatsAppClient descarta envios silenciosamente.
type stubWhatsAppClient struct{}

func (s *stubWhatsAppClient) CreateInstance(_ context.Context, _ string) (*model.QRCodeData, error) {
	return nil, nil
}
func (s *stubWhatsAppClient) SendTextMessage(_ context.Context, _, _, _ string) (*model.SendMessageResponse, error) {
	return &model.SendMessageResponse{MessageID: "stub"}, nil
}
func (s *stubWhatsAppClient) SendButtonMessage(_ context.Context, _, _, _ string, _ []model.MessageButton) (*model.SendMessageResponse, error) {
	return &model.SendMessageResponse{MessageID: "stub"}, nil
}
func (s *stubWhatsAppClient) GetConnectionStatus(_ context.Context, _ string) (*model.ConnectionStatus, error) {
	return nil, nil
}
func (s *stubWhatsAppClient) GetQRCode(_ context.Context, _ string) (*model.QRCodeData, error) {
	return nil, nil
}
func (s *stubWhatsAppClient) Logout(_ context.Context, _ string) error         { return nil }
func (s *stubWhatsAppClient) DeleteInstance(_ context.Context, _ string) error { return nil }

// stubMessageRepo descarta persistência.
type stubMessageRepo struct{}

func (r *stubMessageRepo) Create(_ context.Context, _ *model.Message) error { return nil }
func (r *stubMessageRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Message, error) {
	return nil, nil
}
func (r *stubMessageRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ model.MessageStatus) error {
	return nil
}
func (r *stubMessageRepo) GetByAppointmentID(_ context.Context, _ uuid.UUID) ([]*model.Message, error) {
	return nil, nil
}
func (r *stubMessageRepo) GetPendingReminders(_ context.Context, _ time.Time) ([]*model.Message, error) {
	return nil, nil
}

// newTestService cria um WhatsAppService pronto para teste com o agente stub injetado.
func newTestService(ai agent.Service, debounce time.Duration) WhatsAppService {
	svc := NewWhatsAppService(&stubWhatsAppClient{}, &stubMessageRepo{}, nil)
	svc.SetAIAgentService(ai)
	svc.SetDebounceDelay(debounce)
	return svc
}

// newMsg cria um ParsedIncomingMessage mínimo para os testes.
func newMsg(phone, content string) *model.ParsedIncomingMessage {
	return &model.ParsedIncomingMessage{
		MessageID: uuid.New().String(),
		Phone:     phone,
		Instance:  "test-instance",
		Content:   content,
	}
}

// ---------------------------------------------------------------------------
// Testes unitários do messageDebouncer
// ---------------------------------------------------------------------------

// TestDebouncer_FiresAfterIdle garante que o callback é chamado exatamente
// uma vez após o período de idle expirar.
func TestDebouncer_FiresAfterIdle(t *testing.T) {
	d := newMessageDebouncer()
	clinicID := uuid.New()

	var called int32
	d.schedule(clinicID, newMsg("+5511999990001", "oi"), 50*time.Millisecond, func(_ *model.ParsedIncomingMessage) {
		atomic.AddInt32(&called, 1)
	})

	time.Sleep(120 * time.Millisecond)

	if got := atomic.LoadInt32(&called); got != 1 {
		t.Errorf("esperava 1 chamada após idle, recebeu %d", got)
	}
}

// TestDebouncer_ResetsOnNewMessage garante que cada nova mensagem reinicia o
// timer e o callback é disparado apenas uma vez ao final do silence window.
func TestDebouncer_ResetsOnNewMessage(t *testing.T) {
	d := newMessageDebouncer()
	clinicID := uuid.New()
	phone := "+5511999990002"

	var called int32
	delay := 80 * time.Millisecond

	// 3 mensagens em sequência rápida — cada uma reseta o timer.
	for i, text := range []string{"Oi", "quero marcar", "consulta"} {
		d.schedule(clinicID, newMsg(phone, text), delay, func(_ *model.ParsedIncomingMessage) {
			atomic.AddInt32(&called, 1)
		})
		if i < 2 {
			time.Sleep(30 * time.Millisecond) // menor que o delay
		}
	}

	// Aguarda o timer da última mensagem disparar.
	time.Sleep(delay + 60*time.Millisecond)

	if got := atomic.LoadInt32(&called); got != 1 {
		t.Errorf("esperava exatamente 1 disparo após reset, recebeu %d", got)
	}
}

// TestDebouncer_NaoDisparaAntesDoIdle verifica que o callback NÃO é chamado
// antes do período de idle terminar.
func TestDebouncer_NaoDisparaAntesDoIdle(t *testing.T) {
	d := newMessageDebouncer()
	clinicID := uuid.New()

	var called int32
	delay := 100 * time.Millisecond

	d.schedule(clinicID, newMsg("+5511999990003", "oi"), delay, func(_ *model.ParsedIncomingMessage) {
		atomic.AddInt32(&called, 1)
	})

	// Verifica antes do delay expirar.
	time.Sleep(40 * time.Millisecond)

	if got := atomic.LoadInt32(&called); got != 0 {
		t.Errorf("callback não deveria ter sido chamado antes do idle expirar, mas foi chamado %d vez(es)", got)
	}

	// Aguarda completar para não vazar goroutines.
	time.Sleep(delay + 40*time.Millisecond)
}

// TestDebouncer_IndependentPerPhone garante que contatos diferentes
// possuem timers totalmente independentes.
func TestDebouncer_IndependentPerPhone(t *testing.T) {
	d := newMessageDebouncer()
	clinicID := uuid.New()

	var calledA, calledB int32
	delay := 60 * time.Millisecond

	d.schedule(clinicID, newMsg("+5511000000001", "msg A"), delay, func(_ *model.ParsedIncomingMessage) {
		atomic.AddInt32(&calledA, 1)
	})
	d.schedule(clinicID, newMsg("+5511000000002", "msg B"), delay, func(_ *model.ParsedIncomingMessage) {
		atomic.AddInt32(&calledB, 1)
	})

	time.Sleep(delay + 50*time.Millisecond)

	if got := atomic.LoadInt32(&calledA); got != 1 {
		t.Errorf("telefone A: esperava 1 disparo, recebeu %d", got)
	}
	if got := atomic.LoadInt32(&calledB); got != 1 {
		t.Errorf("telefone B: esperava 1 disparo, recebeu %d", got)
	}
}

// TestDebouncer_IndependentPerClinic garante que o mesmo telefone em clínicas
// diferentes não compartilha timer — invariante INV-001 (isolamento multi-tenancy).
func TestDebouncer_IndependentPerClinic(t *testing.T) {
	d := newMessageDebouncer()
	clinicA := uuid.New()
	clinicB := uuid.New()
	phone := "+5511999900000"

	var calledA, calledB int32
	delay := 60 * time.Millisecond

	d.schedule(clinicA, newMsg(phone, "msg A"), delay, func(_ *model.ParsedIncomingMessage) {
		atomic.AddInt32(&calledA, 1)
	})
	d.schedule(clinicB, newMsg(phone, "msg B"), delay, func(_ *model.ParsedIncomingMessage) {
		atomic.AddInt32(&calledB, 1)
	})

	time.Sleep(delay + 50*time.Millisecond)

	if got := atomic.LoadInt32(&calledA); got != 1 {
		t.Errorf("clínica A: esperava 1 disparo, recebeu %d", got)
	}
	if got := atomic.LoadInt32(&calledB); got != 1 {
		t.Errorf("clínica B: esperava 1 disparo, recebeu %d", got)
	}
}

// ---------------------------------------------------------------------------
// Testes de integração do serviço com debounce
// ---------------------------------------------------------------------------

// TestHandleIncomingMessage_AINotCalledImmediately verifica que o AI Agent
// NÃO é acionado imediatamente após receber uma mensagem.
func TestHandleIncomingMessage_AINotCalledImmediately(t *testing.T) {
	ai := &stubAIAgentService{}
	svc := newTestService(ai, 80*time.Millisecond)

	clinicID := uuid.New()
	if err := svc.HandleIncomingMessage(context.Background(), clinicID, newMsg("+5511999991111", "Oi")); err != nil {
		t.Fatalf("HandleIncomingMessage retornou erro: %v", err)
	}

	// O AI não deve ter sido acionado ainda.
	if got := ai.callCount(); got != 0 {
		t.Errorf("AI não deveria ter sido chamado imediatamente, mas foi chamado %d vez(es)", got)
	}
}

// TestHandleIncomingMessage_AICalledAfterDebounce verifica que o AI Agent
// É acionado após o período de idle expirar.
func TestHandleIncomingMessage_AICalledAfterDebounce(t *testing.T) {
	ai := &stubAIAgentService{}
	debounce := 80 * time.Millisecond
	svc := newTestService(ai, debounce)

	clinicID := uuid.New()
	phone := "+5511999992222"

	if err := svc.HandleIncomingMessage(context.Background(), clinicID, newMsg(phone, "quero agendar")); err != nil {
		t.Fatalf("HandleIncomingMessage retornou erro: %v", err)
	}

	// Aguarda idle expirar.
	time.Sleep(debounce + 100*time.Millisecond)

	if got := ai.callCount(); got != 1 {
		t.Errorf("AI deveria ter sido chamado 1 vez após debounce, recebeu %d", got)
	}
}

// TestHandleIncomingMessage_OnlyLastMessageProcessed verifica que várias
// mensagens em sequência resultam em apenas UMA chamada ao AI, com o texto
// da última mensagem enviada.
func TestHandleIncomingMessage_OnlyLastMessageProcessed(t *testing.T) {
	ai := &stubAIAgentService{}
	debounce := 80 * time.Millisecond
	svc := newTestService(ai, debounce)

	clinicID := uuid.New()
	phone := "+5511999993333"
	msgs := []string{"Oi", "quero marcar", "consulta pediátrica"}

	for _, text := range msgs {
		if err := svc.HandleIncomingMessage(context.Background(), clinicID, newMsg(phone, text)); err != nil {
			t.Fatalf("HandleIncomingMessage(%q) retornou erro: %v", text, err)
		}
		time.Sleep(20 * time.Millisecond) // mais rápido que o debounce
	}

	// Aguarda idle expirar.
	time.Sleep(debounce + 100*time.Millisecond)

	if got := ai.callCount(); got != 1 {
		t.Errorf("esperava 1 chamada ao AI (só a última mensagem), recebeu %d", got)
	}

	req := ai.lastRequest()
	if req == nil {
		t.Fatal("lastRequest retornou nil")
	}
	ultimaMensagem := msgs[len(msgs)-1]
	if req.MessageText != ultimaMensagem {
		t.Errorf("AI deveria ter recebido %q, recebeu %q", ultimaMensagem, req.MessageText)
	}
}
