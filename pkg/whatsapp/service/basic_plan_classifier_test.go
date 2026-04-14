package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// stubAppointmentFinder com AppointmentInfo configuravel.
type stubAppointmentFinder struct {
	apt *AppointmentInfo
}

func (s *stubAppointmentFinder) GetPendingByPhone(_ context.Context, _ uuid.UUID, _ string) (*AppointmentInfo, error) {
	return s.apt, nil
}

// stubAppointmentUpdater conta confirmacoes/cancelamentos.
type stubAppointmentUpdater struct {
	mu        sync.Mutex
	confirmed []uuid.UUID
	canceled  []uuid.UUID
}

func (s *stubAppointmentUpdater) ConfirmByPatient(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.confirmed = append(s.confirmed, id)
	return nil
}

func (s *stubAppointmentUpdater) CancelByPatient(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canceled = append(s.canceled, id)
	return nil
}

func (s *stubAppointmentUpdater) confirmCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.confirmed)
}

func (s *stubAppointmentUpdater) cancelCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.canceled)
}

// newBasicPlanService monta um whatsAppService SEM aiAgentService (simulando
// plano basico), com appointmentFinder/updater mockados.
func newBasicPlanService(finder AppointmentFinder, updater AppointmentStatusUpdater) *whatsAppService {
	svc := NewWhatsAppService(&stubWhatsAppClient{}, &stubMessageRepo{}, updater).(*whatsAppService)
	svc.appointmentFinder = finder
	return svc
}

func newPendingApt(future time.Duration) *AppointmentInfo {
	expires := time.Now().Add(future)
	return &AppointmentInfo{
		ID:                    uuid.New(),
		StartTime:             time.Now().Add(2 * time.Hour),
		EndTime:               time.Now().Add(3 * time.Hour),
		Status:                "pending",
		ConfirmationExpiresAt: &expires,
	}
}

// TestBasicPlan_ConfirmationWindow_SimConfirma: no plano basico (sem AI agent),
// quando o paciente esta na janela de confirmacao e responde "sim", o
// appointment e confirmado via classifier (regex fallback). Diferenciacao com
// Pro: aqui NAO ha conversa livre — so a confirmacao reativa ao reminder.
func TestBasicPlan_ConfirmationWindow_SimConfirma(t *testing.T) {
	finder := &stubAppointmentFinder{apt: newPendingApt(24 * time.Hour)}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "sim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 1 {
		t.Errorf("expected appointment to be confirmed, got %d confirmations", updater.confirmCount())
	}
}

// Variantes de resposta que regex pega: "confirmo", "ok", "vou".
func TestBasicPlan_ConfirmationWindow_VariantesConfirma(t *testing.T) {
	cases := []string{"confirmo", "ok", "vou", "Sim!", "CONFIRMADO"}
	for _, content := range cases {
		t.Run(content, func(t *testing.T) {
			finder := &stubAppointmentFinder{apt: newPendingApt(24 * time.Hour)}
			updater := &stubAppointmentUpdater{}
			svc := newBasicPlanService(finder, updater)

			_ = svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", content))
			if updater.confirmCount() != 1 {
				t.Errorf("%q deveria confirmar, mas ConfirmByPatient foi chamado %d vezes", content, updater.confirmCount())
			}
		})
	}
}

// TestBasicPlan_ConfirmationWindow_Cancela: "cancelar" durante janela de
// confirmacao deve cancelar via classifier. (Nota: o fallback regex tem
// ambiguidades conhecidas — ex: "nao vou" casa "vou" do confirm antes de
// chegar no cancel. Em producao o classifier AI trata isso; aqui usamos uma
// palavra nao-ambigua para testar o caminho cancel sem depender do AI.)
func TestBasicPlan_ConfirmationWindow_Cancela(t *testing.T) {
	finder := &stubAppointmentFinder{apt: newPendingApt(24 * time.Hour)}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "cancelar"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.cancelCount() != 1 {
		t.Errorf("expected appointment to be canceled, got %d cancellations", updater.cancelCount())
	}
}

// TestBasicPlan_SemAppointmentEmAguardo_Ignora: sem appointment pendente,
// texto livre continua sendo ignorado — comportamento do plano basico antigo.
// Isso preserva a fronteira com o Pro: o basico nao inicia conversa, so
// responde a lembretes que o proprio sistema enviou.
func TestBasicPlan_SemAppointmentEmAguardo_Ignora(t *testing.T) {
	finder := &stubAppointmentFinder{apt: nil}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "sim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 0 || updater.cancelCount() != 0 {
		t.Errorf("texto livre fora da janela nao pode disparar acao: confirm=%d cancel=%d",
			updater.confirmCount(), updater.cancelCount())
	}
}

// TestBasicPlan_JanelaExpirada_Ignora: appointment existe mas confirmation_expires_at
// ja passou — janela fechada, trata como se nao houvesse appointment em aguardo.
func TestBasicPlan_JanelaExpirada_Ignora(t *testing.T) {
	finder := &stubAppointmentFinder{apt: newPendingApt(-1 * time.Hour)} // ja expirou
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "sim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 0 {
		t.Errorf("janela expirada deve ignorar, mas ConfirmByPatient foi chamado %d vezes", updater.confirmCount())
	}
}

// TestBasicPlan_ConfirmationNilExpiresAt_Ignora: appointment sem
// ConfirmationExpiresAt (reminder nao foi enviado ainda) tambem ignora.
func TestBasicPlan_ConfirmationNilExpiresAt_Ignora(t *testing.T) {
	apt := newPendingApt(24 * time.Hour)
	apt.ConfirmationExpiresAt = nil
	finder := &stubAppointmentFinder{apt: apt}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "sim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 0 {
		t.Errorf("sem ConfirmationExpiresAt deve ignorar, got %d", updater.confirmCount())
	}
}

// TestBasicPlan_AIAgentEnabledFalse_NaoClassifica: clinica optou por desligar
// IA (AIAgentEnabled=false). Nesse caso NADA de IA roda — nem o classifier
// reativo, porque ClassifyMessage usa um provider AI (regex e apenas
// fallback). A clinica paga plano basico apenas pelo envio automatico de
// lembretes; entrada do paciente e totalmente ignorada.
func TestBasicPlan_AIAgentEnabledFalse_NaoClassifica(t *testing.T) {
	finder := &stubAppointmentFinder{apt: newPendingApt(24 * time.Hour)}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	// Clinic com AI desligada.
	disabledClinic := aiEnabledClinic()
	disabledClinic.Settings.AIAgentEnabled = false
	svc.clinicRepo = &stubClinicRepo{clinic: disabledClinic}

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "sim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 0 || updater.cancelCount() != 0 {
		t.Errorf("AIAgentEnabled=false deve ignorar texto livre: confirm=%d cancel=%d",
			updater.confirmCount(), updater.cancelCount())
	}
}

// TestBasicPlan_AIAgentEnabledFalse_ButtonAindaFunciona: botoes (deterministicos,
// sem IA) continuam funcionando mesmo com AIAgentEnabled=false. A setting
// desliga IA, nao desliga confirmacao por UI.
func TestBasicPlan_AIAgentEnabledFalse_ButtonAindaFunciona(t *testing.T) {
	finder := &stubAppointmentFinder{apt: newPendingApt(24 * time.Hour)}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	disabledClinic := aiEnabledClinic()
	disabledClinic.Settings.AIAgentEnabled = false
	svc.clinicRepo = &stubClinicRepo{clinic: disabledClinic}

	msg := newMsg("+5511988887777", "")
	msg.ButtonID = "confirm"
	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 1 {
		t.Errorf("botao confirm deve funcionar mesmo com AIAgentEnabled=false, got %d", updater.confirmCount())
	}
}

// TestBasicPlan_StatusConfirmed_Ignora: apos o paciente confirmar, a janela
// fecha — novas mensagens de texto livre nao devem reprocessar. Sem isso, um
// "obrigado" posterior poderia trigar o classifier de novo (e no pior caso,
// um "cancelar" desfaz uma confirmacao do paciente sem ele pretender).
func TestBasicPlan_StatusConfirmed_Ignora(t *testing.T) {
	apt := newPendingApt(24 * time.Hour)
	apt.Status = "confirmed" // ja confirmou antes
	finder := &stubAppointmentFinder{apt: apt}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "sim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 0 || updater.cancelCount() != 0 {
		t.Errorf("status=confirmed deve fechar a janela: confirm=%d cancel=%d",
			updater.confirmCount(), updater.cancelCount())
	}
}

// TestBasicPlan_StatusNoResponse_ContinuaAceitando: timeout marcou o appt como
// no_response — o paciente ainda pode confirmar ou cancelar tardiamente.
func TestBasicPlan_StatusNoResponse_ContinuaAceitando(t *testing.T) {
	apt := newPendingApt(24 * time.Hour)
	apt.Status = "no_response"
	finder := &stubAppointmentFinder{apt: apt}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "sim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 1 {
		t.Errorf("status=no_response ainda aceita confirmacao tardia, got %d confirm", updater.confirmCount())
	}
}

// TestBasicPlan_IntentSchedule_Ignora: mesmo com paciente em aguardo, se a
// mensagem for "quero marcar", isso e tarefa do AI Agent (Pro) — ignora no
// basico para preservar a fronteira de plano.
func TestBasicPlan_IntentSchedule_Ignora(t *testing.T) {
	finder := &stubAppointmentFinder{apt: newPendingApt(24 * time.Hour)}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "quero marcar uma consulta"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "quero marcar" e classificado como schedule; no basico, schedule nao e
	// processado — so Confirm/Cancel.
	if updater.confirmCount() != 0 || updater.cancelCount() != 0 {
		t.Errorf("intent schedule nao pode disparar acao no basico: confirm=%d cancel=%d",
			updater.confirmCount(), updater.cancelCount())
	}
}

// TestBasicPlan_IntentReschedule_Ignora: "prefiro outro dia" e reschedule —
// tambem ignorado no basico (requer conversa para sugerir novo horario).
func TestBasicPlan_IntentReschedule_Ignora(t *testing.T) {
	finder := &stubAppointmentFinder{apt: newPendingApt(24 * time.Hour)}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "prefiro remarcar"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 0 || updater.cancelCount() != 0 {
		t.Errorf("intent reschedule nao pode disparar acao no basico: confirm=%d cancel=%d",
			updater.confirmCount(), updater.cancelCount())
	}
}

// TestBasicPlan_ButtonID_ContinuaFuncionando: mensagens vindas de botoes de
// template continuam sendo processadas — regressao garante que a mudanca no
// caminho de texto livre nao quebrou o caminho do ButtonID.
func TestBasicPlan_ButtonID_ContinuaFuncionando(t *testing.T) {
	// Note: sem janela de aguardo, mas com ButtonID explicito.
	finder := &stubAppointmentFinder{apt: &AppointmentInfo{
		ID:        uuid.New(),
		StartTime: time.Now().Add(2 * time.Hour),
		EndTime:   time.Now().Add(3 * time.Hour),
		// ConfirmationExpiresAt nil — janela nao importa para ButtonID.
	}}
	updater := &stubAppointmentUpdater{}
	svc := newBasicPlanService(finder, updater)

	msg := newMsg("+5511988887777", "")
	msg.ButtonID = "confirm"
	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.confirmCount() != 1 {
		t.Errorf("ButtonID=confirm deve confirmar independente de janela, got %d", updater.confirmCount())
	}
}

// TestBasicPlan_PlanGateAllows_UsaAIAgentPeloDebouncer: quando o plan gate
// permite (Pro), o debouncer dispara o AI Agent. O classifier reativo NAO
// roda porque o AI Agent cuida de toda a conversa.
func TestBasicPlan_PlanGateAllows_UsaAIAgentPeloDebouncer(t *testing.T) {
	finder := &stubAppointmentFinder{apt: newPendingApt(24 * time.Hour)}
	updater := &stubAppointmentUpdater{}
	ai := &stubAIAgentService{}
	gate := &stubPlanGate{allowed: true}
	svc := newBasicPlanService(finder, updater)
	svc.aiAgentService = ai
	svc.clinicRepo = &stubClinicRepo{clinic: aiEnabledClinic()}
	svc.planGateChecker = gate
	svc.debounceDelay = 30 * time.Millisecond

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "sim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Aguarda o debouncer disparar.
	time.Sleep(100 * time.Millisecond)

	if ai.callCount() != 1 {
		t.Errorf("plan gate permite — AI Agent deve ser chamado: got %d calls", ai.callCount())
	}
	if updater.confirmCount() != 0 {
		t.Errorf("classifier reativo NAO deve rodar quando AI Agent e chamado: got %d", updater.confirmCount())
	}
}

// TestBasicPlan_PlanGateDenies_UsaClassifierReativoPeloDebouncer: quando o
// plan gate nega (basico), o debouncer dispara o classifier reativo em vez do
// AI Agent. Mensagens rapidas ainda sao coalescidas pelo debouncer.
func TestBasicPlan_PlanGateDenies_UsaClassifierReativoPeloDebouncer(t *testing.T) {
	finder := &stubAppointmentFinder{apt: newPendingApt(24 * time.Hour)}
	updater := &stubAppointmentUpdater{}
	ai := &stubAIAgentService{}
	gate := &stubPlanGate{allowed: false}
	svc := newBasicPlanService(finder, updater)
	svc.aiAgentService = ai
	svc.clinicRepo = &stubClinicRepo{clinic: aiEnabledClinic()}
	svc.planGateChecker = gate
	svc.debounceDelay = 30 * time.Millisecond

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "sim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Aguarda o debouncer disparar.
	time.Sleep(100 * time.Millisecond)

	if ai.callCount() != 0 {
		t.Errorf("plan gate nega — AI Agent NAO deve ser chamado: got %d calls", ai.callCount())
	}
	if updater.confirmCount() != 1 {
		t.Errorf("classifier reativo deve confirmar quando plan gate nega + janela aberta: got %d", updater.confirmCount())
	}
}

// TestBasicPlan_PlanGateDenies_AlemDaJanelaIgnora: plan gate nega + mensagem
// fora de janela. Classifier nao dispara nada (mantem boundary do plano).
func TestBasicPlan_PlanGateDenies_AlemDaJanelaIgnora(t *testing.T) {
	finder := &stubAppointmentFinder{apt: nil}
	updater := &stubAppointmentUpdater{}
	ai := &stubAIAgentService{}
	gate := &stubPlanGate{allowed: false}
	svc := newBasicPlanService(finder, updater)
	svc.aiAgentService = ai
	svc.clinicRepo = &stubClinicRepo{clinic: aiEnabledClinic()}
	svc.planGateChecker = gate
	svc.debounceDelay = 30 * time.Millisecond

	err := svc.HandleIncomingMessage(context.Background(), uuid.New(), newMsg("+5511988887777", "quero marcar"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if ai.callCount() != 0 || updater.confirmCount() != 0 || updater.cancelCount() != 0 {
		t.Errorf("sem janela, plano basico ignora texto livre: ai=%d confirm=%d cancel=%d",
			ai.callCount(), updater.confirmCount(), updater.cancelCount())
	}
}
