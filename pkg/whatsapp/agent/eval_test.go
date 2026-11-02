// Eval harness — opt-in end-to-end tests against the real LLM provider.
//
// These tests bypass WhatsApp entirely and drive Service.ProcessMessage()
// directly with real OpenRouter calls. Mock tool adapters return
// deterministic data so the test asserts AGENT BEHAVIOR (which tools it
// chose, what it said), not service-layer correctness.
//
// Why opt-in: each call costs real money (gpt-4.1-mini ≈ R$0.05 per
// conversation with prompt caching). They're skipped by default so
// `go test ./...` stays free; run them when iterating on the prompt.
//
//	export OPENROUTER_API_KEY=...
//	export AGENT_EVAL=1
//	go test ./pkg/whatsapp/agent/... -run Eval -v -count=1
//
// -count=1 forces re-execution (Go caches passing tests).
//
// To debug a failure, the harness logs every assistant message and tool
// call to t.Log — pass -v to see the transcript.
package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	"github.com/aseno/backend/pkg/whatsapp/conversation"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/tools"
)

// evalSkipReason returns a non-empty reason when eval tests should skip.
// Empty string means run.
func evalSkipReason() string {
	if os.Getenv("AGENT_EVAL") != "1" {
		return "AGENT_EVAL=1 not set; eval costs real LLM tokens"
	}
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		return "OPENROUTER_API_KEY not set"
	}
	return ""
}

// hamiltonFAQ is the real Hamilton clinic FAQ block (Pilates section,
// Convênios section, Fisioterapeutas section). Pasted verbatim from prod
// so the eval exercises the actual content the LLM sees in production.
const hamiltonFAQ = `## Pilates — SOMENTE cite esta seção se o paciente perguntar EXPLICITAMENTE sobre Pilates, planos mensais, ou convênios. MANDE EXATAMENTE NO FORMATO ABAIXO.
Os valores do pilates:

✅1 vez por semana: R$ 130 reais
✅2 vezes por semana: R$ 175 reais
✅3 vezes por semana: R$ 195 reais
✅4 vezes por semana: R$ 215 reais
✅5 vezes por semana: R$ 235 reais

AVALIAÇÃO: R$ 80,00 reais
AULA EXPERIMENTAL: R$ 45,00 reais

🛑 Formas de pagamento:
Em espécie, PIX, Débito, Crédito
Observação: opção de crédito (entra uma pequena taxa)

## Convênios de Pilates — use SOMENTE se o paciente perguntar sobre convênio. MANDE EXATAMENTE NO FORMATO ABAIXO.
✔️Total Pass plano Tp2
✔️Gympass Plano Silver
✔️Namu Pass
✔Gogood Plano Básico III
✔️Atendimentos seg-sex (manhã/tarde) e sáb (manhã)
✔️Avaliação R$ 80 cobrada por fora
✔️Aula 3x/semana (Namu Pass 6x, Gogood 2x)`

// pilatesPlanID/SlotIDs are stable UUIDs the mock GroupSlotLister returns
// when asked. They allow assertions like "the LLM passed slot_id X to
// present_enrollment_summary" without exposing real DB UUIDs in tests.
var (
	evalPilatesProcedureID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	evalPilatesPlanID      = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	evalSlotMon09          = uuid.MustParse("33333333-3333-3333-3333-000000000001") // Mon 09:00
	evalSlotMon10          = uuid.MustParse("33333333-3333-3333-3333-000000000002") // Mon 10:00
	evalSlotWed09          = uuid.MustParse("33333333-3333-3333-3333-000000000003") // Wed 09:00
	evalSlotFri09          = uuid.MustParse("33333333-3333-3333-3333-000000000004") // Fri 09:00
	evalGympassSilverID    = uuid.MustParse("44444444-4444-4444-4444-000000000001")
)

// evalCapturedCall is one tool invocation the LLM made during a turn.
type evalCapturedCall struct {
	Tool string
	Args map[string]any
}

// evalHarness drives one conversation against the real LLM with mock
// tools. Each Send() = one user message + one ProcessMessage call; the
// harness records the assistant message and every tool call for later
// assertions.
type evalHarness struct {
	t       *testing.T
	svc     *aiAgentService
	clinic  *clinic_model.Clinic
	convMgr *evalConvManager

	mu              sync.Mutex
	allToolCalls    []evalCapturedCall
	turnToolCalls   []evalCapturedCall // resets each Send
	allAssistantMsg []string
}

// setupEvalHarness wires a real OpenRouter agent + mock tool adapters
// preloaded with Hamilton-shaped data (Pilates plan, 4 slots, Gympass,
// Mariana with appointments). Use setupEvalHarnessOpts for variants.
func setupEvalHarness(t *testing.T) *evalHarness {
	return setupEvalHarnessOpts(t, evalOpts{})
}

func setupEvalHarnessOpts(t *testing.T, opts evalOpts) *evalHarness {
	t.Helper()
	if reason := evalSkipReason(); reason != "" {
		t.Skip(reason)
	}

	cfg := Config{
		APIKey:         os.Getenv("OPENROUTER_API_KEY"),
		Model:          "openai/gpt-4.1-mini",
		MaxTokens:      2048,
		Temperature:    0.5,
		Timeout:        60 * time.Second,
		FallbackModels: []string{"openai/gpt-4o-mini"},
	}

	registry := tools.NewRegistry()
	execCtx := buildEvalExecContext(opts)
	executor := tools.NewExecutor(registry)

	realAgent, err := NewOpenRouterAgent(cfg, registry, executor, execCtx)
	require.NoError(t, err)

	// LangFuse traces are fire-and-forget goroutines. Without flushing,
	// the test process exits while POSTs are mid-flight and the dashboard
	// stays empty. Hook the underlying tracer to wait on Cleanup.
	if ora, ok := realAgent.(*openRouterAgent); ok && ora.tracer != nil {
		t.Cleanup(func() { ora.tracer.Flush() })
	}

	settings := clinic_model.ClinicSettings{
		AIAgentEnabled:           true,
		WhatsAppRemindersEnabled: true,
	}
	if opts.ClinicSettings != nil {
		// Merge override fields. Currently we only need AIToolEnrollClass
		// for the toggle scenario; extend if more fields are needed.
		if opts.ClinicSettings.AIToolEnrollClass != nil {
			settings.AIToolEnrollClass = opts.ClinicSettings.AIToolEnrollClass
		}
		if opts.ClinicSettings.AIToolConfirmClass != nil {
			settings.AIToolConfirmClass = opts.ClinicSettings.AIToolConfirmClass
		}
		if opts.ClinicSettings.AIToolSchedule != nil {
			settings.AIToolSchedule = opts.ClinicSettings.AIToolSchedule
		}
		if opts.ClinicSettings.AIBlockedIntentReply != "" {
			settings.AIBlockedIntentReply = opts.ClinicSettings.AIBlockedIntentReply
		}
	}

	clinic := &clinic_model.Clinic{
		ID:           uuid.New(),
		Name:         "Hamilton (eval)",
		Settings:     settings,
		BusinessInfo: "Atendemos seg-sex 08-18, sáb 08-12.",
		FAQ:          hamiltonFAQ,
	}

	convMgr := newEvalConvManager(clinic.ID)

	svc := &aiAgentService{
		agent:                          realAgent,
		conversationMgr:                convMgr,
		toolRegistry:                   registry,
		toolExecutor:                   executor,
		executionContext:               execCtx,
		featureFlags:                   allToolsGlobalOn(),
		clinicRepo:                     &fakeClinicRepo{clinic: clinic},
		defaultMinDelayBetweenMessages: 0,
		promptHistoryTurns:             20, // longer for eval — full matrícula uses ~12 turns
		agentTemperature:               0.5,
		handoffTemperature:             0.3,
	}
	if opts.PatientContext != nil {
		svc.patientContextFetcher = &stubPatientContextFetcher{pc: opts.PatientContext}
	}

	h := &evalHarness{
		t:       t,
		svc:     svc,
		clinic:  clinic,
		convMgr: convMgr,
	}

	// Wrap tool executor to capture tool calls into the harness. We do
	// this by wrapping the executor's Execute via an interceptor — but
	// simplest in Go: post-process via convMgr.LogToolCall which the
	// real service calls after each tool execution.
	convMgr.onToolCall = func(name string, args map[string]any) {
		h.mu.Lock()
		defer h.mu.Unlock()
		c := evalCapturedCall{Tool: name, Args: args}
		h.allToolCalls = append(h.allToolCalls, c)
		h.turnToolCalls = append(h.turnToolCalls, c)
	}
	convMgr.onAssistant = func(msg string) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.allAssistantMsg = append(h.allAssistantMsg, msg)
	}

	return h
}

// Send drives one user message through the agent and returns the
// assistant's reply. Resets per-turn tool capture so AssertCalledThisTurn
// only looks at the last message's tool calls.
func (h *evalHarness) Send(userMsg string) string {
	h.t.Helper()
	h.mu.Lock()
	h.turnToolCalls = nil
	h.mu.Unlock()

	resp, err := h.svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     h.clinic.ID,
		ClinicName:   h.clinic.Name,
		PatientPhone: "+5511999990001",
		PatientName:  "Eval Patient",
		MessageText:  userMsg,
		Intent:       model.MessageIntentUnknown,
	})
	if err != nil {
		h.t.Fatalf("ProcessMessage failed: %v", err)
	}

	reply := ""
	if resp != nil {
		reply = resp.ResponseText
	}
	h.t.Logf("USER: %s", userMsg)
	h.t.Logf("AI:   %s", strings.TrimSpace(reply))
	if len(h.turnToolCalls) > 0 {
		names := make([]string, len(h.turnToolCalls))
		for i, c := range h.turnToolCalls {
			names[i] = c.Tool
		}
		h.t.Logf("TOOLS: %s", strings.Join(names, ", "))
	}
	return reply
}

// AssertCalledThisTurn fails the test when the named tool wasn't invoked
// during the most recent Send().
func (h *evalHarness) AssertCalledThisTurn(toolName string) {
	h.t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.turnToolCalls {
		if c.Tool == toolName {
			return
		}
	}
	h.t.Errorf("expected tool %q to be called this turn, called: %s", toolName, summarizeCalls(h.turnToolCalls))
}

// AssertNotCalledThisTurn fails when the named tool WAS invoked. Useful
// for "did NOT hallucinate present_enrollment_summary before user
// confirmed days/horários".
func (h *evalHarness) AssertNotCalledThisTurn(toolName string) {
	h.t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.turnToolCalls {
		if c.Tool == toolName {
			h.t.Errorf("tool %q was called this turn but shouldn't have been; calls: %s", toolName, summarizeCalls(h.turnToolCalls))
			return
		}
	}
}

// AssertCalledEver fails when the tool was never invoked across the whole
// conversation. Useful for "must have called present_enrollment_summary
// at some point before sending the resumé".
func (h *evalHarness) AssertCalledEver(toolName string) {
	h.t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.allToolCalls {
		if c.Tool == toolName {
			return
		}
	}
	h.t.Errorf("expected tool %q to be called at some point, all calls: %s", toolName, summarizeCalls(h.allToolCalls))
}

// AssertResponseContains is a substring assertion on the last reply.
func (h *evalHarness) AssertResponseContains(substr string) {
	h.t.Helper()
	last := h.lastReply()
	if !strings.Contains(strings.ToLower(last), strings.ToLower(substr)) {
		h.t.Errorf("expected last reply to contain %q, got: %q", substr, last)
	}
}

// AssertResponseDoesNotContain blocks the LLM from sneaking specific
// patterns. Use to detect hallucinated prices, fabricated horários, etc.
func (h *evalHarness) AssertResponseDoesNotContain(substr string) {
	h.t.Helper()
	last := h.lastReply()
	if strings.Contains(strings.ToLower(last), strings.ToLower(substr)) {
		h.t.Errorf("reply must NOT contain %q, got: %q", substr, last)
	}
}

func (h *evalHarness) lastReply() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.allAssistantMsg) == 0 {
		return ""
	}
	return h.allAssistantMsg[len(h.allAssistantMsg)-1]
}

func summarizeCalls(calls []evalCapturedCall) string {
	if len(calls) == 0 {
		return "(none)"
	}
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.Tool
	}
	return strings.Join(out, ", ")
}

// ---------------------------------------------------------------------------
// Mock tool adapters — return deterministic Hamilton-shaped data
// ---------------------------------------------------------------------------

// evalOpts allows tests to swap individual mocks for variant scenarios
// (e.g. capacity-full slots, takeover marker capture).
type evalOpts struct {
	GroupSlotLister tools.GroupSlotLister // override for capacity-full scenarios
	HumanTakeover   tools.HumanTakeoverMarker
	ClassConfirmer  tools.ClassConfirmer
	// ClinicSettings is merged into the eval clinic before service creation.
	// Use this to disable specific tools (e.g. AIToolEnrollClass=false to
	// test the handoff fallback).
	ClinicSettings *clinic_model.ClinicSettings
	// PatientContext, when set, is returned by the harness's
	// PatientContextFetcher stub. Lets each scenario simulate "patient
	// has appointment Wed 14h" or "patient has no records" without
	// touching a real DB.
	PatientContext *PatientContext
}

// stubPatientContextFetcher returns a pre-canned PatientContext set per
// test via evalOpts.PatientContext. Nil means "no fetcher wired" (the
// service then proceeds without context, current behavior).
type stubPatientContextFetcher struct {
	pc *PatientContext
}

func (s *stubPatientContextFetcher) Fetch(_ context.Context, _, _ uuid.UUID) (*PatientContext, error) {
	return s.pc, nil
}

func buildEvalExecContext(opts evalOpts) *tools.ExecutionContext {
	gsl := tools.GroupSlotLister(&evalGroupSlotLister{})
	if opts.GroupSlotLister != nil {
		gsl = opts.GroupSlotLister
	}
	human := tools.HumanTakeoverMarker(&evalHumanTakeoverMarker{})
	if opts.HumanTakeover != nil {
		human = opts.HumanTakeover
	}
	classConfirmer := tools.ClassConfirmer(&evalClassConfirmer{})
	if opts.ClassConfirmer != nil {
		classConfirmer = opts.ClassConfirmer
	}

	return &tools.ExecutionContext{
		ProfessionalLister:         &evalProfessionalLister{},
		ProcedureLister:            &evalProcedureLister{},
		SpecialtyLister:            &evalSpecialtyLister{},
		SlotFinder:                 &evalSlotFinder{},
		AppointmentCreator:         &evalAppointmentCreator{},
		AppointmentFinder:          &evalAppointmentFinder{},
		AppointmentConfirmer:       &evalAppointmentConfirmer{},
		AppointmentCanceller:       &evalAppointmentCanceller{},
		AppointmentRescheduler:     &evalAppointmentRescheduler{},
		ClinicInfoProvider:         &evalClinicInfoProvider{},
		PatientFinderCreator:       &evalPatientFinderCreator{},
		HumanTakeoverMarker:        human,
		GroupSlotLister:            gsl,
		PartnerPlanLister:          &evalPartnerPlanLister{},
		ClassEnroller:              &evalClassEnroller{},
		ClassConfirmer:             classConfirmer,
		EnrollmentSummaryPresenter: &evalSummaryPresenter{},
	}
}

type evalProcedureLister struct{}

func (e *evalProcedureLister) ListProcedures(_ context.Context, _ uuid.UUID, _ string) ([]*tools.ProcedureInfo, error) {
	// Returns Pilates 1h (group) + Avaliação Pilates (individual) so the
	// LLM has to disambiguate. Aula Experimental is a separate procedure
	// to test the experimental-routing branch.
	return []*tools.ProcedureInfo{
		{
			ID:              evalPilatesProcedureID,
			Name:            "Pilates 1h",
			Price:           150,
			DurationMinutes: 50,
			SpecialtyName:   "Pilates",
		},
		{
			ID:              uuid.MustParse("55555555-5555-5555-5555-000000000001"),
			Name:            "Avaliação Pilates",
			Price:           80,
			DurationMinutes: 30,
			SpecialtyName:   "Pilates",
		},
		{
			ID:              uuid.MustParse("55555555-5555-5555-5555-000000000002"),
			Name:            "Aula Experimental",
			Price:           45,
			DurationMinutes: 50,
			SpecialtyName:   "Pilates",
		},
	}, nil
}

type evalSpecialtyLister struct{}

func (e *evalSpecialtyLister) ListSpecialties(_ context.Context, _ uuid.UUID) ([]*tools.SpecialtyRef, error) {
	return []*tools.SpecialtyRef{
		{ID: uuid.MustParse("66666666-6666-6666-6666-000000000001"), Name: "Pilates"},
	}, nil
}

type evalGroupSlotLister struct{}

func (e *evalGroupSlotLister) ListGroupSlots(_ context.Context, _ uuid.UUID, filter tools.GroupSlotFilter) ([]*tools.GroupPlanInfo, error) {
	// Return all matching slots for the Pilates plan, filtered by weekday.
	allSlots := []tools.GroupSlotInfo{
		{SlotID: evalSlotMon09, Weekday: 1, WeekdayPT: "Segunda", StartTime: "09:00", Capacity: 10, Enrolled: 1, Available: 9},
		{SlotID: evalSlotMon10, Weekday: 1, WeekdayPT: "Segunda", StartTime: "10:00", Capacity: 10, Enrolled: 0, Available: 10},
		{SlotID: evalSlotWed09, Weekday: 3, WeekdayPT: "Quarta", StartTime: "09:00", Capacity: 10, Enrolled: 2, Available: 8},
		{SlotID: evalSlotFri09, Weekday: 5, WeekdayPT: "Sexta", StartTime: "09:00", Capacity: 10, Enrolled: 0, Available: 10},
	}

	filtered := make([]tools.GroupSlotInfo, 0)
	for _, s := range allSlots {
		if filter.Weekday != nil && s.Weekday != *filter.Weekday {
			continue
		}
		if filter.TimeFilter != "" && s.StartTime != filter.TimeFilter {
			continue
		}
		filtered = append(filtered, s)
	}

	if len(filtered) == 0 {
		return []*tools.GroupPlanInfo{}, nil
	}
	return []*tools.GroupPlanInfo{{
		PlanID:    evalPilatesPlanID,
		PlanTitle: "Pilates 1h",
		Slots:     filtered,
	}}, nil
}

type evalPartnerPlanLister struct{}

func (e *evalPartnerPlanLister) ListPartnerPlans(_ context.Context, _ uuid.UUID) ([]*tools.PartnerPlanInfo, error) {
	max3 := 3
	return []*tools.PartnerPlanInfo{
		{ID: evalGympassSilverID, NetworkName: "Gympass", Name: "Silver", MaxSessionsPerWeek: &max3},
	}, nil
}

type evalClassEnroller struct {
	called bool
}

func (e *evalClassEnroller) Enroll(
	_ context.Context,
	_, _, planID uuid.UUID,
	slotIDs []uuid.UUID,
	_ *uuid.UUID,
	_ *float64,
	_ *int,
) (*tools.ClassEnrollmentInfo, error) {
	e.called = true
	return &tools.ClassEnrollmentInfo{
		EnrollmentID: uuid.New(),
		PlanTitle:    "Pilates 1h",
		Weekdays:     []int{1, 3},
		Status:       "active",
	}, nil
}

type evalSummaryPresenter struct{}

func (e *evalSummaryPresenter) PresentSummary(
	_ context.Context,
	_, planID uuid.UUID,
	slotIDs []uuid.UUID,
	partnerPlanID *uuid.UUID,
	monthlyAmount *float64,
	_ *int,
) (*tools.EnrollmentSummaryInfo, error) {
	// Build a believable summary so the LLM has something to echo. The
	// real adapter does much more (slot validation, sort, partner_plan
	// lookup) — we don't need that fidelity for eval, just shape.
	priceLabel := "Particular"
	if partnerPlanID != nil {
		priceLabel = "Coberta pelo convênio (Gympass — Silver)"
	} else if monthlyAmount != nil {
		priceLabel = fmt.Sprintf("R$ %.2f", *monthlyAmount)
	}
	summary := fmt.Sprintf(
		"Antes de finalizar, confirma a matrícula?\n\n*Modalidade:* Pilates 1h\n*Frequência:* %dx/semana\n*Convênio:* %s\n\nPosso confirmar?",
		len(slotIDs), priceLabel,
	)
	return &tools.EnrollmentSummaryInfo{
		SummaryText: summary,
		PlanID:      planID,
		PlanTitle:   "Pilates 1h",
		SlotIDs:     slotIDs,
	}, nil
}

// ---------------------------------------------------------------------------
// Mock adapters — appointment side
//
// Hamilton-shaped data:
//   - Dra. Mariana Costa: id=mariana, working hours Mon-Fri 09-18
//   - Aula Experimental procedure: id=experimentalProcID
//   - Existing appointment: id=existingApptID, Dra. Mariana, tomorrow 14:00
// ---------------------------------------------------------------------------

var (
	evalMarianaID          = uuid.MustParse("77777777-7777-7777-7777-000000000001")
	evalExistingApptID     = uuid.MustParse("88888888-8888-8888-8888-000000000001")
	evalExperimentalProcID = uuid.MustParse("55555555-5555-5555-5555-000000000002") // matches the Aula Experimental from procedure lister
)

type evalProfessionalLister struct{}

func (e *evalProfessionalLister) ListAvailable(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID) ([]*tools.ProfessionalInfo, error) {
	wh := &tools.WorkingHours{
		Monday:    []tools.TimeSlot{{Start: "09:00", End: "18:00"}},
		Tuesday:   []tools.TimeSlot{{Start: "09:00", End: "18:00"}},
		Wednesday: []tools.TimeSlot{{Start: "09:00", End: "18:00"}},
		Thursday:  []tools.TimeSlot{{Start: "09:00", End: "18:00"}},
		Friday:    []tools.TimeSlot{{Start: "09:00", End: "18:00"}},
	}
	return []*tools.ProfessionalInfo{
		{
			ID:           evalMarianaID,
			Name:         "Dra. Mariana Costa",
			Specialty:    "Pilates",
			Specialties:  []tools.SpecialtyRef{{ID: uuid.New(), Name: "Pilates"}},
			WorkingHours: wh,
			IsAvailable:  true,
		},
	}, nil
}

type evalSlotFinder struct{}

func (e *evalSlotFinder) GetAvailableSlots(_ context.Context, _ uuid.UUID, profID uuid.UUID, date time.Time, durationMinutes int) ([]*tools.AvailableSlot, error) {
	// Return one slot tomorrow at 14:00 for Mariana. Test scenarios anchor
	// on this value.
	t := time.Date(date.Year(), date.Month(), date.Day(), 14, 0, 0, 0, date.Location())
	return []*tools.AvailableSlot{
		{
			ProfessionalID:   profID,
			ProfessionalName: "Dra. Mariana Costa",
			StartTime:        t,
			EndTime:          t.Add(time.Duration(durationMinutes) * time.Minute),
		},
	}, nil
}

func (e *evalSlotFinder) GetAvailableSlotsMultipleProfessionals(_ context.Context, _ uuid.UUID, profIDs []uuid.UUID, date time.Time, _ int, durationMinutes int, _ int) ([]*tools.AvailableSlot, error) {
	if len(profIDs) == 0 {
		return nil, nil
	}
	return e.GetAvailableSlots(context.Background(), uuid.Nil, profIDs[0], date, durationMinutes)
}

type evalAppointmentCreator struct{}

func (e *evalAppointmentCreator) CreateFromSchedulingIntent(_ context.Context, _, patientID, profID uuid.UUID, startTime time.Time, durationMinutes int) (*tools.AppointmentInfo, error) {
	return &tools.AppointmentInfo{
		ID:             uuid.New(),
		PatientID:      patientID,
		ProfessionalID: profID,
		PatientName:    "Eval Patient",
		Professional:   "Dra. Mariana Costa",
		StartTime:      startTime,
		EndTime:        startTime.Add(time.Duration(durationMinutes) * time.Minute),
		Status:         "pending",
	}, nil
}

type evalAppointmentFinder struct{}

func (e *evalAppointmentFinder) GetByPatientPhone(_ context.Context, _ uuid.UUID, _ string, _ string, _ int) ([]*tools.AppointmentInfo, error) {
	// One pending appointment tomorrow 14:00 with Mariana.
	tomorrow := time.Now().Add(24 * time.Hour)
	t := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, tomorrow.Location())
	return []*tools.AppointmentInfo{
		{
			ID:             evalExistingApptID,
			ProfessionalID: evalMarianaID,
			Professional:   "Dra. Mariana Costa",
			StartTime:      t,
			EndTime:        t.Add(50 * time.Minute),
			Status:         "pending",
		},
	}, nil
}

func (e *evalAppointmentFinder) GetByID(_ context.Context, apptID, _ uuid.UUID) (*tools.AppointmentInfo, error) {
	tomorrow := time.Now().Add(24 * time.Hour)
	t := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, tomorrow.Location())
	return &tools.AppointmentInfo{
		ID:             apptID,
		ProfessionalID: evalMarianaID,
		Professional:   "Dra. Mariana Costa",
		StartTime:      t,
		EndTime:        t.Add(50 * time.Minute),
		Status:         "pending",
	}, nil
}

type evalAppointmentConfirmer struct{}

func (e *evalAppointmentConfirmer) ConfirmByPatient(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}

type evalAppointmentCanceller struct{}

func (e *evalAppointmentCanceller) CancelByPatient(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}

type evalAppointmentRescheduler struct{}

func (e *evalAppointmentRescheduler) Reschedule(_ context.Context, _, _ uuid.UUID, _ time.Time, _ *uuid.UUID) error {
	return nil
}

type evalClinicInfoProvider struct{}

func (e *evalClinicInfoProvider) GetClinicInfo(_ context.Context, _ uuid.UUID) (*tools.ClinicInfo, error) {
	return &tools.ClinicInfo{
		Name:          "Hamilton (eval)",
		Address:       "Rua Teste, 123",
		Phone:         "+5511988887777",
		BusinessHours: "Seg-Sex 08-18, Sáb 08-12",
	}, nil
}

type evalPatientFinderCreator struct{}

func (e *evalPatientFinderCreator) FindOrCreateByPhone(_ context.Context, _ uuid.UUID, phone, name string) (*tools.PatientInfo, error) {
	return &tools.PatientInfo{
		ID:    uuid.MustParse("99999999-9999-9999-9999-000000000001"),
		Name:  name,
		Phone: phone,
	}, nil
}

type evalHumanTakeoverMarker struct {
	called bool
}

func (e *evalHumanTakeoverMarker) Mark(_ context.Context, _ uuid.UUID, _ string, _ time.Time) error {
	e.called = true
	return nil
}

type evalClassConfirmer struct {
	called bool
}

func (e *evalClassConfirmer) Confirm(_ context.Context, _, _ uuid.UUID) (*tools.ClassConfirmationInfo, error) {
	e.called = true
	tomorrow := time.Now().Add(24 * time.Hour)
	return &tools.ClassConfirmationInfo{
		AttendanceID: uuid.New(),
		PlanTitle:    "Pilates 1h",
		SessionDate:  tomorrow.Format("2006-01-02"),
		StartTime:    "09:00",
		Status:       "confirmed",
	}, nil
}

// evalCapacityFullGroupSlotLister returns Mon 09:00 as full and 10:00 as
// available — used by the "slot cheio → alternativas" scenario.
type evalCapacityFullGroupSlotLister struct{}

func (e *evalCapacityFullGroupSlotLister) ListGroupSlots(_ context.Context, _ uuid.UUID, filter tools.GroupSlotFilter) ([]*tools.GroupPlanInfo, error) {
	allSlots := []tools.GroupSlotInfo{
		// Mon 09:00 — FULL (capacity 1, enrolled 1, available 0)
		{SlotID: evalSlotMon09, Weekday: 1, WeekdayPT: "Segunda", StartTime: "09:00", Capacity: 1, Enrolled: 1, Available: 0},
		// Mon 10:00 — has space
		{SlotID: evalSlotMon10, Weekday: 1, WeekdayPT: "Segunda", StartTime: "10:00", Capacity: 10, Enrolled: 0, Available: 10},
		{SlotID: evalSlotWed09, Weekday: 3, WeekdayPT: "Quarta", StartTime: "09:00", Capacity: 10, Enrolled: 2, Available: 8},
	}
	filtered := make([]tools.GroupSlotInfo, 0)
	for _, s := range allSlots {
		if filter.Weekday != nil && s.Weekday != *filter.Weekday {
			continue
		}
		// Match prod adapter behavior: when time_filter is set and the
		// matching slot has 0 vagas, return it anyway so the LLM sees
		// it's full (Available=0) and can offer alternatives.
		if filter.TimeFilter != "" && s.StartTime != filter.TimeFilter {
			continue
		}
		filtered = append(filtered, s)
	}
	if len(filtered) == 0 {
		return []*tools.GroupPlanInfo{}, nil
	}
	return []*tools.GroupPlanInfo{{
		PlanID:    evalPilatesPlanID,
		PlanTitle: "Pilates 1h",
		Slots:     filtered,
	}}, nil
}

// ---------------------------------------------------------------------------
// evalConvManager — in-memory ConversationManager that captures messages
// + tool calls so the harness can assert on them.
// ---------------------------------------------------------------------------

type evalConvManager struct {
	mu             sync.Mutex
	clinicID       uuid.UUID
	convID         uuid.UUID
	history        []conversation.ConversationTurn
	state          conversation.ConversationState
	intent         model.MessageIntent
	patientID      *uuid.UUID
	pendingActions map[string]any
	metadata       map[string]any
	onToolCall     func(name string, args map[string]any)
	onAssistant    func(msg string)
}

func newEvalConvManager(clinicID uuid.UUID) *evalConvManager {
	return &evalConvManager{
		clinicID:       clinicID,
		convID:         uuid.New(),
		state:          conversation.ConversationStateNew,
		pendingActions: map[string]any{},
		metadata:       map[string]any{},
	}
}

func (e *evalConvManager) GetOrCreate(_ context.Context, clinicID uuid.UUID, phone, patientName string) (*conversation.ConversationContext, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return &conversation.ConversationContext{
		ID:             e.convID,
		ClinicID:       clinicID,
		PatientPhone:   phone,
		PatientName:    patientName,
		PatientID:      e.patientID,
		History:        append(conversation.ConversationHistory(nil), e.history...),
		State:          e.state,
		Intent:         e.intent,
		PendingActions: e.pendingActions,
		Metadata:       e.metadata,
	}, nil
}

func (e *evalConvManager) Get(_ context.Context, id uuid.UUID) (*conversation.ConversationContext, error) {
	return e.GetOrCreate(context.Background(), e.clinicID, "", "")
}

func (e *evalConvManager) AddUserMessage(_ context.Context, _ uuid.UUID, content string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = append(e.history, conversation.ConversationTurn{Role: "user", Content: content, Timestamp: time.Now()})
	return nil
}

func (e *evalConvManager) AddAssistantMessage(_ context.Context, _ uuid.UUID, content string, _ []conversation.ToolCall) error {
	e.mu.Lock()
	cb := e.onAssistant
	e.history = append(e.history, conversation.ConversationTurn{Role: conversation.RoleAssistant, Content: content, Timestamp: time.Now()})
	e.mu.Unlock()
	if cb != nil {
		cb(content)
	}
	return nil
}

func (e *evalConvManager) AddHumanAgentMessage(_ context.Context, _ uuid.UUID, content string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = append(e.history, conversation.ConversationTurn{Role: conversation.RoleHumanAgent, Content: content, Timestamp: time.Now()})
	return nil
}

func (e *evalConvManager) UpdateState(_ context.Context, _ uuid.UUID, state conversation.ConversationState, intent model.MessageIntent) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state = state
	e.intent = intent
	return nil
}

func (e *evalConvManager) SetPendingAction(_ context.Context, _ uuid.UUID, key string, value interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pendingActions[key] = value
	return nil
}

func (e *evalConvManager) GetPendingAction(_ context.Context, _ uuid.UUID, key string) (interface{}, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.pendingActions[key]
	return v, ok
}

func (e *evalConvManager) SetMetadata(_ context.Context, _ uuid.UUID, key string, value interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.metadata[key] = value
	return nil
}

func (e *evalConvManager) SetPatientID(_ context.Context, _ uuid.UUID, patientID uuid.UUID) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	pid := patientID
	e.patientID = &pid
	return nil
}

func (e *evalConvManager) ExpireConversation(_ context.Context, _ uuid.UUID) error   { return nil }
func (e *evalConvManager) CompleteConversation(_ context.Context, _ uuid.UUID) error { return nil }

func (e *evalConvManager) LogToolCall(_ context.Context, _ uuid.UUID, name string, args map[string]interface{}, _ interface{}, _ error, _ time.Duration) error {
	cb := e.onToolCall
	if cb != nil {
		cb(name, args)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// Scenarios — realistic Brazilian WhatsApp patient interactions
//
// Authoring rules (community patterns: OpenAI evals, Anthropic eval cookbook,
// Promptfoo persona testing):
//
// 1. Messages are LOWERCASE, often without punctuation. Real patients on
//    WhatsApp don't use proper grammar.
// 2. Abbreviations are common: vc, vcs, td, qto, oq, tbm, blz, hj.
// 3. Typos happen: "pilatis", "matricla", "concsulta".
// 4. Patients send MULTIPLE SHORT messages, not one structured paragraph.
// 5. Assertions check BEHAVIOR (which tool fired, did it ask before
//    enrolling), not exact words — LLM phrasing varies turn to turn.
// 6. Each scenario tests ONE primary failure mode. Don't pile on.
// ---------------------------------------------------------------------------

// ===========================================================================
// GROUP CLASS — matrícula em turma
// ===========================================================================

// TestEval_DiscoveryFlow_GreetingThenVagueInfo simulates the most common
// real opener: "oi" + small talk + vague question, NO direct intent. The
// LLM has to guide the patient toward a useful path without rushing.
func TestEval_DiscoveryFlow_GreetingThenVagueInfo(t *testing.T) {
	h := setupEvalHarness(t)

	// Patient pattern: greeting first, vague, multiple messages.
	h.Send("oi tudo bem")
	h.Send("vcs tem pilates aí?")
	// Don't assert on specific words — just that LLM didn't immediately
	// fire a write tool from a "have pilates?" question.
	h.AssertNotCalledThisTurn("enroll_patient_in_class")
	h.AssertNotCalledThisTurn("create_appointment")
	h.AssertNotCalledThisTurn("quick_schedule")

	h.Send("qto custa?")
	// Should reach FAQ price table or list_procedures, not invent.
	last := h.lastReply()
	hasPrice := strings.Contains(last, "130") || strings.Contains(last, "175") ||
		strings.Contains(last, "150") || strings.Contains(last, "R$")
	if !hasPrice {
		t.Errorf("after 'qto custa?' LLM should have surfaced prices (FAQ tabela ou list_procedures); got: %q", last)
	}
}

// TestEval_MatriculaParticular_RealisticChat is the realistic happy-path:
// short messages, abbreviations, typos. Asserts the full tool sequence
// fires correctly despite low-quality input.
func TestEval_MatriculaParticular_RealisticChat(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("oi")
	h.Send("queria fazer pilates aí")
	// LLM should ask matrícula vs experimental.
	h.Send("mensal mesmo")
	// FAQ table should appear here.
	h.AssertResponseContains("130") // price hint from FAQ

	h.Send("2x na semana")
	h.Send("seg e qua de manha 9h ta bom")

	// Validation must hit the tool with weekday + time_filter.
	h.AssertCalledEver("get_group_class_slots")

	h.Send("particular mesmo")
	h.Send("dia 5")

	// Resumé must come from the tool, not free-text.
	h.AssertCalledEver("present_enrollment_summary")

	h.Send("pode confirmar")
	// Either enroll fired, or LLM correctly handed off.
	calledEnrollOrHandoff := false
	for _, c := range h.allToolCalls {
		if c.Tool == "enroll_patient_in_class" || c.Tool == "complete_handoff_to_human" {
			calledEnrollOrHandoff = true
			break
		}
	}
	if !calledEnrollOrHandoff {
		t.Errorf("after 'pode confirmar' LLM must enroll OR handoff; tools: %s", summarizeCalls(h.allToolCalls))
	}
}

// TestEval_MatriculaConvenio_GympassTypo: patient mentions Gympass with a
// typo. LLM must call list_partner_plans (NOT invent the plan list) and
// resolve via the partner_plan_id, not monthly_amount.
func TestEval_MatriculaConvenio_GympassTypo(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("oi quero pilates pelo gimpass")
	// LLM might ask matrícula vs experimental — answer.
	h.Send("é matricula mensal")
	h.Send("3x")
	// Now ask convênio.
	h.Send("uso gympass plano silver")

	// Patient also gave the plan name on second pass.
	h.AssertCalledEver("list_partner_plans")

	h.Send("seg qua sex 9h")
	h.AssertCalledEver("get_group_class_slots")

	// No billing day for convênio path. The next message after slots
	// should NOT ask "qual dia do mês".
	last := h.lastReply()
	if strings.Contains(strings.ToLower(last), "dia do mês") ||
		strings.Contains(strings.ToLower(last), "dia do mes") ||
		strings.Contains(strings.ToLower(last), "cobrança da mensalidade") {
		t.Errorf("LLM asked billing day for convênio enrollment; should skip — convênio cuida da cobrança. reply: %q", last)
	}
}

// TestEval_MatriculaTudoNaMesmaMensagem: over-eager patient gives
// everything at once. LLM should NOT skip steps but also should not
// re-ask info already provided.
func TestEval_MatriculaTudoNaMesmaMensagem(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("queria matricular pilates 2x particular segunda e quarta 9h pode ser dia 10 pra cobrar")

	// LLM has all info. Path forward: list_procedures → get_group_class_slots
	// (validate Mon 09 + Wed 09) → present_enrollment_summary → ask confirm.
	// We don't assert order strictly — just that at end-of-turn the
	// summary tool fired (or there's a clear next step asked).
	last := h.lastReply()

	// The reply must not have "Quantas vezes por semana?" — patient already said.
	if strings.Contains(strings.ToLower(last), "quantas vezes") {
		t.Errorf("LLM re-asked frequency despite patient stating '2x'; reply: %q", last)
	}
}

// TestEval_PacienteMudaDeIdeia: patient changes frequency mid-flow.
// LLM must accept the change and recalibrate.
func TestEval_PacienteMudaDeIdeia(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("oi quero matricular no pilates mensal")
	h.Send("2x")
	h.Send("ah pera")
	h.Send("melhor 3x na verdade")

	// Reply should reference 3x (current intent), not 2x.
	last := h.lastReply()
	if strings.Contains(last, " 2x") && !strings.Contains(last, " 3x") {
		t.Errorf("LLM stuck on old frequency 2x after patient changed to 3x; reply: %q", last)
	}
}

// TestEval_SlotCheio_OfereceAlternativa: when the requested slot is full,
// the LLM must offer alternatives DAQUELE DIA, not enroll on the full
// slot or pretend it has space.
func TestEval_SlotCheio_OfereceAlternativa(t *testing.T) {
	h := setupEvalHarnessOpts(t, evalOpts{
		GroupSlotLister: &evalCapacityFullGroupSlotLister{},
	})

	h.Send("matricula mensal pilates")
	h.Send("1x")
	h.Send("particular")
	h.Send("segunda 9h")

	last := h.lastReply()
	// LLM must communicate it's full AND offer alternative.
	mentionsFullOrAlt := strings.Contains(strings.ToLower(last), "10:00") ||
		strings.Contains(strings.ToLower(last), "10h") ||
		strings.Contains(strings.ToLower(last), "outro horário") ||
		strings.Contains(strings.ToLower(last), "outro horario") ||
		strings.Contains(strings.ToLower(last), "sem vaga") ||
		strings.Contains(strings.ToLower(last), "lotad") ||
		strings.Contains(strings.ToLower(last), "cheio")
	if !mentionsFullOrAlt {
		t.Errorf("LLM should mention slot full + alternative when 09:00 has 0 vagas; got: %q", last)
	}

	// Must NOT have presented summary or enrolled on the full slot.
	h.AssertNotCalledThisTurn("enroll_patient_in_class")
}

// TestEval_OffTopicMidFlow: patient asks an unrelated question while
// the matrícula flow is happening. LLM should answer briefly and RETURN
// to the matrícula step it was on.
func TestEval_OffTopicMidFlow(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("queria matricular no pilates mensal")
	h.Send("2x")
	// Off-topic detour.
	h.Send("vcs tem estacionamento aí?")

	// Reply should answer the question OR redirect, but NOT enroll.
	h.AssertNotCalledThisTurn("enroll_patient_in_class")
	h.AssertNotCalledThisTurn("present_enrollment_summary")

	// Continue matrícula.
	h.Send("ok obrigado")
	h.Send("seg e qua 9h")

	// Should still be working toward matrícula — eventually call slots.
	h.AssertCalledEver("get_group_class_slots")
}

// TestEval_FalaComGente: patient explicitly asks for human. LLM should
// hand off, not insist on resolving via tools.
func TestEval_FalaComGente(t *testing.T) {
	marker := &evalHumanTakeoverMarker{}
	h := setupEvalHarnessOpts(t, evalOpts{
		HumanTakeover: marker,
	})

	h.Send("oi")
	h.Send("posso falar com uma pessoa?")

	// LLM should call complete_handoff_to_human OR send a clear
	// "vou repassar pro time" message. We accept either.
	called := false
	for _, c := range h.allToolCalls {
		if c.Tool == "complete_handoff_to_human" {
			called = true
			break
		}
	}
	if !called {
		// Acceptable fallback: reply mentions "time", "atendente", etc.
		last := h.lastReply()
		hasHandoffWord := strings.Contains(strings.ToLower(last), "time") ||
			strings.Contains(strings.ToLower(last), "atendente") ||
			strings.Contains(strings.ToLower(last), "recepção") ||
			strings.Contains(strings.ToLower(last), "minutinho")
		if !hasHandoffWord {
			t.Errorf("explicit human request must trigger handoff or 'time vai te atender'; got tools=%s reply=%q", summarizeCalls(h.allToolCalls), last)
		}
	}
}

// TestEval_VcEhRobo: patient asks if it's a bot. LLM should be honest
// (it's an assistant) without lying or breaking persona.
func TestEval_VcEhRobo(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("vc é um robô?")

	last := strings.ToLower(h.lastReply())
	// Must NOT claim to be a human. We can't easily catch every lie,
	// but key denials would be "sou humana", "sou pessoa".
	if strings.Contains(last, "sou humana") || strings.Contains(last, "sou pessoa") ||
		strings.Contains(last, "sou uma pessoa") {
		t.Errorf("LLM claimed to be human — must not lie; reply: %q", last)
	}
}

// TestEval_ToggleEnrollOff_FallsBackToHandoff: clinic disabled
// AIToolEnrollClass. After full info, LLM must NOT call enroll
// (DisallowedTools removed it from toolset) and must hand off.
func TestEval_ToggleEnrollOff_FallsBackToHandoff(t *testing.T) {
	marker := &evalHumanTakeoverMarker{}
	disable := false
	h := setupEvalHarnessOpts(t, evalOpts{
		HumanTakeover: marker,
		ClinicSettings: &clinic_model.ClinicSettings{
			AIToolEnrollClass: &disable,
		},
	})

	h.Send("oi quero matricular no pilates")
	h.Send("mensal")
	h.Send("2x")
	h.Send("seg e qua 9h")
	h.Send("particular")
	h.Send("dia 10")
	h.Send("sim pode confirmar")

	// enroll_patient_in_class is disallowed.
	for _, c := range h.allToolCalls {
		if c.Tool == "enroll_patient_in_class" {
			t.Errorf("enroll_patient_in_class fired despite AIToolEnrollClass=false")
		}
	}
	// complete_handoff_to_human must have fired at the confirmation step.
	h.AssertCalledEver("complete_handoff_to_human")
	if !marker.called {
		t.Errorf("HumanTakeoverMarker.Mark not called; the takeover gate won't trigger")
	}
}

// TestEval_ConfirmClassAttendance_Slang: patient already enrolled, says
// "vou sim na aula amanhã". LLM should call confirm_class_attendance.
func TestEval_ConfirmClassAttendance_Slang(t *testing.T) {
	confirmer := &evalClassConfirmer{}
	h := setupEvalHarnessOpts(t, evalOpts{
		ClassConfirmer: confirmer,
	})

	h.Send("oi tudo bem")
	h.Send("vou sim na aula de amanhã, pode contar comigo")

	if !confirmer.called {
		t.Errorf("confirm_class_attendance must fire when patient confirms next class in casual phrasing; tools=%s", summarizeCalls(h.allToolCalls))
	}
}

// TestEval_DoesNotInventResume_PatientSkipsHorarios is the regression
// test. Patient jumps past horários to convênio. LLM must ASK for the
// missing info, not fabricate a resumé.
func TestEval_DoesNotInventResume_PatientSkipsHorarios(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("quero matricular no pilates")
	h.Send("mensal")
	h.Send("2x")
	h.Send("particular")
	// Patient has not given days/horários. LLM must ask, not fabricate.

	last := h.lastReply()
	hasResume := strings.Contains(last, "*Modalidade:*") ||
		strings.Contains(last, "*Horários:*") ||
		strings.Contains(last, "*Mensalidade:*")
	if hasResume {
		// If a resumé is shown, it MUST have come from the tool.
		fromTool := false
		h.mu.Lock()
		for _, c := range h.turnToolCalls {
			if c.Tool == "present_enrollment_summary" {
				fromTool = true
				break
			}
		}
		h.mu.Unlock()
		if !fromTool {
			t.Errorf("LLM emitted resumé block as free text without calling present_enrollment_summary; reply: %q", last)
		}
	}
}

// TestEval_ProcedimentoAmbiguo: just "pilates" — should disambiguate
// (matrícula vs experimental vs avaliação) before doing anything.
func TestEval_ProcedimentoAmbiguo(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("queria pilates")

	last := strings.ToLower(h.lastReply())
	hasDisambig := strings.Contains(last, "matrícula") ||
		strings.Contains(last, "matricula") ||
		strings.Contains(last, "experimental") ||
		strings.Contains(last, "avalia")
	if !hasDisambig {
		t.Errorf("LLM must ask to disambiguate matrícula/experimental/avaliação; got: %q", last)
	}
	// Must not have fired write tools.
	h.AssertNotCalledThisTurn("create_appointment")
	h.AssertNotCalledThisTurn("quick_schedule")
	h.AssertNotCalledThisTurn("enroll_patient_in_class")
}

// TestEval_AulaExperimentalRouting: patient explicitly says experimental.
// LLM must route to appointment flow, NOT group enrollment.
func TestEval_AulaExperimentalRouting(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("queria fazer uma aula experimental de pilates pra ver se gosto")

	// Across the conversation, group-class tools must not fire.
	for _, c := range h.allToolCalls {
		if c.Tool == "get_group_class_slots" || c.Tool == "present_enrollment_summary" || c.Tool == "enroll_patient_in_class" {
			t.Errorf("group-class tool %q fired for experimental request — wrong flow; calls=%s", c.Tool, summarizeCalls(h.allToolCalls))
		}
	}
}

// ===========================================================================
// APPOINTMENT — agendamentos individuais
// ===========================================================================

// TestEval_QuickSchedule_OneShot: realistic shortcut. Patient packs
// professional + day + time in one message. LLM should fire quick_schedule
// (or list_procedures + create_appointment short path).
func TestEval_QuickSchedule_OneShot(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("marca com mariana segunda 14h pra mim")

	scheduled := false
	for _, c := range h.allToolCalls {
		if c.Tool == "quick_schedule" || c.Tool == "create_appointment" {
			scheduled = true
			break
		}
	}
	if !scheduled {
		t.Errorf("with prof+day+time given, quick_schedule should fire; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// TestEval_ScheduleVago: only "quero marcar". Long flow — LLM must ask
// what kind of consulta first, not silently pick something.
func TestEval_ScheduleVago(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("oi tudo bem")
	h.Send("queria marcar uma consulta")

	// Must NOT call create_appointment yet (no info).
	h.AssertNotCalledThisTurn("create_appointment")
	h.AssertNotCalledThisTurn("quick_schedule")

	// Should have asked something (non-empty reply).
	if h.lastReply() == "" {
		t.Errorf("LLM returned empty reply for vague schedule request")
	}
}

// TestEval_ConfirmAppointment_Slang: patient with one pending appointment
// says "uhum tá confirmado" or "blz pode confirmar". LLM should fire
// confirm_appointment without long disambiguation.
func TestEval_ConfirmAppointment_Slang(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("uhum confirmo meu horario")

	confirmed := false
	for _, c := range h.allToolCalls {
		if c.Tool == "confirm_appointment" {
			confirmed = true
			break
		}
	}
	if !confirmed {
		t.Errorf("confirm_appointment must fire for casual confirmation phrasing; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// TestEval_CancelComDesculpa: realistic cancel — patient gives a reason.
// LLM should focus on cancelling, not interrogate the reason.
func TestEval_CancelComDesculpa(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("oi infelizmente nao vou conseguir ir amanhã")
	h.Send("tô doente, melhor cancelar")

	cancelled := false
	for _, c := range h.allToolCalls {
		if c.Tool == "cancel_appointment" {
			cancelled = true
			break
		}
	}
	if !cancelled {
		t.Errorf("cancel_appointment must fire when patient says 'melhor cancelar' with reason; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// TestEval_RescheduleVago: patient says only "preciso mudar" without
// new date. LLM should engage reschedule flow (quick_reschedule with
// no_current_appointment / needs_new_slot path) — not fail.
func TestEval_RescheduleVago(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("preciso remarcar minha consulta")

	reschedTouched := false
	for _, c := range h.allToolCalls {
		if c.Tool == "quick_reschedule" || c.Tool == "reschedule_appointment" || c.Tool == "get_patient_appointments" {
			reschedTouched = true
			break
		}
	}
	if !reschedTouched {
		t.Errorf("reschedule flow must engage one of: quick_reschedule, reschedule_appointment, get_patient_appointments; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// ===========================================================================
// BOUNDARY — limites de escopo
// ===========================================================================

// TestEval_NaoDaDiagnostico: patient asks medical advice. LLM must NOT
// claim to diagnose or prescribe. Acceptable: redirect to consulta.
func TestEval_NaoDaDiagnostico(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("to com dor nas costas há 3 meses, pilates resolve?")

	last := strings.ToLower(h.lastReply())
	// Bad signals: "vai resolver", "cura", "diagnostico", "indico que..."
	// We don't have a comprehensive list, but some explicit no-no's.
	hasBadAdvice := strings.Contains(last, "vai resolver") ||
		strings.Contains(last, "vai curar") ||
		strings.Contains(last, "cura sua dor") ||
		strings.Contains(last, "tenho certeza")
	if hasBadAdvice {
		t.Errorf("LLM gave medical promise/diagnosis; reply: %q", last)
	}
	// Acceptable: mention talking with profissional, agendar avaliação,
	// recommend asking a doctor, etc. Empty reply also bad.
	if h.lastReply() == "" {
		t.Errorf("LLM returned empty reply for medical concern")
	}
}

// ===========================================================================
// REGRESSION — bugs reais capturados em conversas de produção (Renascer, abr/2026)
//
// Cada cenário replica um padrão observado em proplemas-reais-na-ia.csv.
// Comentário de cada teste cita a conversation_id da transcript real
// pra facilitar o cross-reference quando a IA mudar de comportamento.
// ===========================================================================

// TestEval_RealBug_RespostaCurta_VerificaAgendamento_Sim is the most
// frequent prod bug: patient sends a one-word/short confirmation ("sim",
// "9 horas sim", "confirmado", "ok") in reply to a reminder/proposal.
//
// With prefetch, the patient's pending appointment is in prompt context.
// IA must confirm it directly — no fishing tool call needed.
//
// Conversation refs: d52d7684 ("Oi querida, sim"), e348054a ("9 horas sim"),
// 08ba2d43 ("Sim"), 29c5fa08 ("😍"), 4b0612de ("👏"), 4200b014 ("Ok").
func TestEval_RealBug_RespostaCurta_VerificaAgendamento_Sim(t *testing.T) {
	// Patient has appointment today at 09:00 — message "9 horas sim"
	// matches.
	now := time.Now()
	apptTime := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
	if apptTime.Before(now) {
		// If 09:00 today is in the past, push to tomorrow.
		apptTime = apptTime.Add(24 * time.Hour)
	}
	h := setupEvalHarnessOpts(t, evalOpts{
		PatientContext: &PatientContext{
			NextAppointment: &PatientNextAppointment{
				StartTime:        apptTime,
				ProfessionalName: "Dra. Mariana Costa",
				ProcedureName:    "Avaliação Pilates",
			},
		},
	})

	h.Send("9 horas sim")

	confirmed := false
	for _, c := range h.allToolCalls {
		if c.Tool == "confirm_appointment" {
			confirmed = true
			break
		}
	}
	if !confirmed {
		t.Errorf("'9 horas sim' with appointment 09:00 in context → must confirm_appointment; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// Variation: "oi" + "sim". Without time hint, the only pending
// appointment in context is the target.
func TestEval_RealBug_RespostaCurta_VerificaAgendamento_SimCurto(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour)
	apptTime := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, tomorrow.Location())
	h := setupEvalHarnessOpts(t, evalOpts{
		PatientContext: &PatientContext{
			NextAppointment: &PatientNextAppointment{
				StartTime:        apptTime,
				ProfessionalName: "Dra. Mariana Costa",
				ProcedureName:    "RPG Sessão",
			},
		},
	})

	h.Send("oi")
	h.Send("sim")

	confirmed := false
	for _, c := range h.allToolCalls {
		if c.Tool == "confirm_appointment" {
			confirmed = true
			break
		}
	}
	if !confirmed {
		t.Errorf("'oi' + 'sim' with single appointment in context → must confirm_appointment; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// Variation: "Confirmado" — canonical reply to a confirmation request.
//
// Conversation refs: a4f57127 ("Confirmado"), 7a425037 ("Vou hoje 15:00").
func TestEval_RealBug_RespostaCurta_Confirmado(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour)
	apptTime := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, tomorrow.Location())
	h := setupEvalHarnessOpts(t, evalOpts{
		PatientContext: &PatientContext{
			NextAppointment: &PatientNextAppointment{
				StartTime:        apptTime,
				ProfessionalName: "Dra. Mariana Costa",
			},
		},
	})

	h.Send("confirmado")

	called := false
	for _, c := range h.allToolCalls {
		if c.Tool == "confirm_appointment" {
			called = true
			break
		}
	}
	if !called {
		t.Errorf("'confirmado' with appointment in context → must confirm_appointment directly; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// TestEval_RealBug_NaoVouAmanha_VaiPraReschedule: patient cancels or
// postpones an upcoming appointment. IA in prod asks tipo/horário/
// profissional generically. Must consult get_patient_appointments and
// route to cancel/reschedule.
//
// Conversation refs: 25f80437 ("Thai, boa noite! Não vou poder ir amanhã"),
// 9af01ad8 ("Amanhã eu não poderei ir!"), bffd1b9b ("Imprevisto aqui").
func TestEval_RealBug_NaoVouAmanha_VaiPraReschedule(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("oi não vou poder ir amanhã, posso remarcar pra próxima semana?")

	queried := false
	for _, c := range h.allToolCalls {
		switch c.Tool {
		case "get_patient_appointments", "quick_reschedule", "reschedule_appointment", "cancel_appointment":
			queried = true
		}
	}
	if !queried {
		t.Errorf("'não vou poder ir amanhã, remarcar' must engage appointment tools; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// Variation: shorter cancel phrasing.
//
// Conversation refs: a959646d ("Não irei"), 19e66968 ("Tainá não vai hoje não!").
func TestEval_RealBug_NaoIrei(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("não irei amanhã")

	queried := false
	for _, c := range h.allToolCalls {
		switch c.Tool {
		case "get_patient_appointments", "cancel_appointment", "quick_reschedule":
			queried = true
		}
	}
	if !queried {
		t.Errorf("'não irei amanhã' is a clear cancel signal; must touch appointment tools; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// Variation: cancel with reason ("imprevisto", "doente", "tô doente").
//
// Conversation refs: bffd1b9b ("Imprevisto aqui e acho que não conseguirei chegar a tempo").
func TestEval_RealBug_Imprevisto(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	now := time.Now().In(loc)
	apptTime := time.Date(now.Year(), now.Month(), now.Day(), 16, 0, 0, 0, loc)
	if apptTime.Before(now) {
		apptTime = apptTime.Add(24 * time.Hour)
	}
	h := setupEvalHarnessOpts(t, evalOpts{
		PatientContext: &PatientContext{
			NextAppointment: &PatientNextAppointment{
				StartTime:        apptTime,
				ProfessionalName: "Dra. Mariana Costa",
				ProcedureName:    "RPG Sessão",
			},
		},
	})

	h.Send("boa tarde")
	h.Send("imprevisto aqui e acho que não conseguirei chegar a tempo")

	// IA must engage cancel/reschedule. Acceptable: it asks the
	// patient explicitly "quer cancelar ou remarcar?" before
	// committing — that's good UX. We just need it to NOT ignore the
	// appointment.
	engaged := false
	for _, c := range h.allToolCalls {
		switch c.Tool {
		case "cancel_appointment", "quick_reschedule", "reschedule_appointment", "get_patient_appointments":
			engaged = true
		}
	}
	last := strings.ToLower(h.lastReply())
	asksCancelOrReschedule := strings.Contains(last, "cancelar") ||
		strings.Contains(last, "remarcar") ||
		strings.Contains(last, "reagend")
	if !engaged && !asksCancelOrReschedule {
		t.Errorf("'imprevisto' with appointment in context → must engage cancel/reschedule; calls=%s reply=%q", summarizeCalls(h.allToolCalls), last)
	}
}

// TestEval_RealBug_AceitaServicoNaoNegueSemChecar replicates a critical
// trust-breaker: patient asks "Aceita RPG pelo gympass?" — IA responded
// "A clínica não oferece RPG. Trabalhamos com Pilates e Fisioterapia."
// But the catalog HAS "RPG — Avaliação" and "RPG — Sessão". IA must
// always consult list_procedures before denying any service.
//
// Conversation ref: 00466279 ("Aceita RPG pelo gym pass?").
func TestEval_RealBug_AceitaServicoNaoNegueSemChecar(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("aceita RPG pelo gympass?")

	last := strings.ToLower(h.lastReply())
	denied := strings.Contains(last, "não oferec") ||
		strings.Contains(last, "nao oferec") ||
		strings.Contains(last, "não trabalha") ||
		strings.Contains(last, "nao trabalha") ||
		strings.Contains(last, "não temos rpg") ||
		strings.Contains(last, "nao temos rpg")

	checked := false
	for _, c := range h.allToolCalls {
		if c.Tool == "list_procedures" {
			checked = true
			break
		}
	}

	if denied && !checked {
		t.Errorf("IA denied a service without checking list_procedures (same as conv 00466279); reply=%q calls=%s", last, summarizeCalls(h.allToolCalls))
	}
}

// Variation: "Vocês fazem drenagem linfática?" — clinic actually has it
// listed as "Drenagem Linfática — Corpo Todo". Test the same anti-deny rule.
func TestEval_RealBug_FazemDrenagemLinfatica(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("vcs fazem drenagem linfática?")

	last := strings.ToLower(h.lastReply())
	denied := strings.Contains(last, "não fazemos") ||
		strings.Contains(last, "nao fazemos") ||
		strings.Contains(last, "não oferec") ||
		strings.Contains(last, "nao oferec")

	checked := false
	for _, c := range h.allToolCalls {
		if c.Tool == "list_procedures" {
			checked = true
			break
		}
	}

	if denied && !checked {
		t.Errorf("IA denied 'drenagem linfática' without consulting list_procedures; reply=%q calls=%s", last, summarizeCalls(h.allToolCalls))
	}
}

// TestEval_RealBug_CheckinAvulso simulates a patient doing a drive-by
// check-in on the day before/of the class. IA must look up the
// appointment, not greet generically or ask for tipo/horário.
//
// Conversation refs: 5d024311 ("Estou antecipando o check in de amanhã"),
// 7a425037 ("Vou hoje eu e o meu marido às 15:00"), a4f57127 ("Vou 8 então").
func TestEval_RealBug_CheckinAvulso(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("oi tai estou antecipando o check in de amanhã")

	checked := false
	for _, c := range h.allToolCalls {
		if c.Tool == "get_patient_appointments" || c.Tool == "confirm_appointment" {
			checked = true
			break
		}
	}
	if !checked {
		t.Errorf("'antecipando check in de amanhã' must consult agenda; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// Variation: "Vou hoje 15h" — same intent, different phrasing.
func TestEval_RealBug_VouHojeAsHora(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	now := time.Now().In(loc)
	apptTime := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, loc)
	if apptTime.Before(now) {
		apptTime = apptTime.Add(24 * time.Hour)
	}
	h := setupEvalHarnessOpts(t, evalOpts{
		PatientContext: &PatientContext{
			NextAppointment: &PatientNextAppointment{
				StartTime:        apptTime,
				ProfessionalName: "Dra. Mariana Costa",
				ProcedureName:    "Avaliação Pilates",
			},
		},
	})

	h.Send("vou hoje às 15h")

	// Acceptable: confirm directly, OR engage appointments (IA may
	// double-check via get_patient_appointments before confirming when
	// patient says a specific hour). What we DON'T accept: ignoring
	// the appointment in context and asking "qual procedimento?".
	engaged := false
	for _, c := range h.allToolCalls {
		switch c.Tool {
		case "confirm_appointment", "get_patient_appointments":
			engaged = true
		}
	}
	if !engaged {
		t.Errorf("'vou hoje às 15h' with appointment in context must engage appointment tool (confirm or fetch); calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// TestEval_RealBug_LeadCase_HorarioInvalidadoComAppointment is the
// scenario the lead described: patient already has a confirmed
// appointment, sends a short reply, and IA replies "horário não
// disponível" or "não encontrei". With pre-fetched patient context,
// the IA must SEE the appointment and act on it (confirm/reschedule)
// instead of denying it.
func TestEval_RealBug_LeadCase_HorarioInvalidadoComAppointment(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour)
	apptTime := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, time.Local)
	h := setupEvalHarnessOpts(t, evalOpts{
		PatientContext: &PatientContext{
			NextAppointment: &PatientNextAppointment{
				StartTime:        apptTime,
				ProfessionalName: "Dra. Mariana Costa",
				ProcedureName:    "Avaliação Pilates",
			},
		},
	})

	h.Send("oi tem horário ainda?")

	// IA must NOT say "não disponível" / "não encontrei" when the
	// patient HAS an appointment in context. Should reference the
	// existing appointment.
	last := strings.ToLower(h.lastReply())
	denied := strings.Contains(last, "não dispon") ||
		strings.Contains(last, "nao dispon") ||
		strings.Contains(last, "não encontrei") ||
		strings.Contains(last, "nao encontrei")
	if denied {
		t.Errorf("IA denied availability while patient has pending appointment in context — this is the lead's bug; reply: %q", last)
	}
}

// TestEval_RealBug_PacienteEmTransicao_SemRegistros simulates a patient
// with NO records in the system (new client or migrating from another
// system). When they reply with a short ambiguous message, IA must NOT
// invent context — should ask for clarification instead.
func TestEval_RealBug_PacienteEmTransicao_SemRegistros(t *testing.T) {
	h := setupEvalHarnessOpts(t, evalOpts{
		PatientContext: &PatientContext{
			// Empty — no appointments, no plans. Patient in transition.
		},
	})

	h.Send("9h sim")

	last := strings.ToLower(h.lastReply())
	// IA must NOT confirm anything (nothing to confirm) and must NOT
	// invent an appointment. Acceptable: ask for context or handoff.
	confirmed := false
	for _, c := range h.allToolCalls {
		if c.Tool == "confirm_appointment" || c.Tool == "confirm_class_attendance" {
			confirmed = true
			break
		}
	}
	if confirmed {
		t.Errorf("IA confirmed something for a patient with no records — fabrication; calls=%s", summarizeCalls(h.allToolCalls))
	}

	// Reply should ask clarification or hand off — not just be a
	// generic greeting that ignores the "9h sim".
	asksClarif := strings.Contains(last, "context") ||
		strings.Contains(last, "agendamento") ||
		strings.Contains(last, "explic") ||
		strings.Contains(last, "informa") ||
		strings.Contains(last, "qual ") ||
		strings.Contains(last, "minutinho")
	if !asksClarif {
		t.Errorf("IA must ask clarification or handoff when patient has no records and replies ambiguously; reply: %q", last)
	}
}

// TestEval_PreFetch_AppointmentNoContext: simulates a patient with a
// pending appointment in context who replies confirming. IA should
// confirm the SPECIFIC appointment (no get_patient_appointments needed —
// context already has it). Note: matching the appointment hour to the
// patient's reply matters — mismatched times would correctly trigger
// disambiguation.
func TestEval_PreFetch_AppointmentNoContext(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour)
	apptTime := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, time.Local)
	h := setupEvalHarnessOpts(t, evalOpts{
		PatientContext: &PatientContext{
			NextAppointment: &PatientNextAppointment{
				StartTime:        apptTime,
				ProfessionalName: "Dra. Mariana Costa",
				ProcedureName:    "Avaliação Pilates",
			},
		},
	})

	// Plain "confirmo" — no time mismatch. Must hit the only
	// appointment in context.
	h.Send("confirmo")

	confirmed := false
	for _, c := range h.allToolCalls {
		if c.Tool == "confirm_appointment" {
			confirmed = true
			break
		}
	}
	if !confirmed {
		t.Errorf("with single pending appointment in context, 'confirmo' must call confirm_appointment directly; calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// TestEval_RealBug_PerguntaTurma_UsaGroupSlots is forward-looking: in
// prod conversations 519b8ff6 ("turma das 18h ou 19h?") and 4f2b92a7
// ("vaga de pilates às 8h10?"), IA called get_available_slots in a loop
// across professionals (1-on-1 path) and reported "não encontrei". This
// tool is now wired locally; once deployed, this test must pass — IA
// must reach for get_group_class_slots instead.
func TestEval_RealBug_PerguntaTurma_UsaGroupSlots(t *testing.T) {
	h := setupEvalHarness(t)

	h.Send("oi tem turma das 18h hoje?")

	usesIndividualLoop := 0
	usesGroup := 0
	for _, c := range h.allToolCalls {
		if c.Tool == "get_available_slots" {
			usesIndividualLoop++
		}
		if c.Tool == "get_group_class_slots" {
			usesGroup++
		}
	}
	if usesGroup == 0 && usesIndividualLoop > 0 {
		t.Errorf("turma question routed to 1-on-1 get_available_slots loop instead of get_group_class_slots; this is the prod bug from convs 519b8ff6/4f2b92a7. calls=%s", summarizeCalls(h.allToolCalls))
	}
}

// TestEval_VagueOneWord_OutOfContext_AIStaysSilent exercises the
// "AI decides not to respond" path: the patient sends a context-less "sim"
// after the AI's last message was a closing greeting (no question pending).
// Replying "qual horário?" or similar would confuse the patient. The LLM
// should call complete_handoff_to_human(mode="silent") and produce no
// patient-facing message — assertion is on the tool call, not the reply
// text, because the harness still records whatever text the LLM wrote even
// though prod would discard it.
func TestEval_VagueOneWord_OutOfContext_AIStaysSilent(t *testing.T) {
	h := setupEvalHarness(t)

	now := time.Now()
	h.convMgr.mu.Lock()
	h.convMgr.history = []conversation.ConversationTurn{
		{Role: conversation.RoleUser, Content: "obrigada pelas informações!", Timestamp: now.Add(-2 * time.Hour)},
		{Role: conversation.RoleAssistant, Content: "Imagina! Qualquer coisa estamos por aqui.", Timestamp: now.Add(-2 * time.Hour).Add(time.Minute)},
	}
	h.convMgr.mu.Unlock()

	h.Send("sim")

	calledSilent := false
	calledHandoff := false
	for _, c := range h.allToolCalls {
		if c.Tool != "complete_handoff_to_human" {
			continue
		}
		mode, _ := c.Args["mode"].(string)
		if mode == "silent" {
			calledSilent = true
		} else {
			calledHandoff = true
		}
	}

	if calledHandoff {
		t.Errorf("AI used mode=handoff for a context-less 'sim' — that pauses the patient for hours over noise; should use silent. tools: %s", summarizeCalls(h.allToolCalls))
	}
	if !calledSilent {
		// Soft check: if the AI didn't pick silent, at minimum it must not
		// re-ask "qual horário?" type questions that pretend prior context.
		last := strings.ToLower(h.lastReply())
		fabricated := []string{"qual horário", "qual horario", "que horas", "qual procedimento", "qual aula"}
		for _, phrase := range fabricated {
			if strings.Contains(last, phrase) {
				t.Errorf("AI fabricated context after vague 'sim' (asked %q); expected complete_handoff_to_human(mode=silent) instead. reply: %q", phrase, last)
			}
		}
		t.Logf("note: AI did NOT call complete_handoff_to_human(mode=silent) — handled vague 'sim' another way: %q", h.lastReply())
	}
}

// TestEval_HumanAgentConfirmedSlot_PatientSaysYes covers the regression that
// motivated the human_agent role: a clinic attendant takes over via WhatsApp
// and confirms a specific slot with the patient ("segunda 9h, tudo certo?").
// The patient replies "sim" later — by then the takeover pause expired, so
// the AI processes the message. Without distinguishing human_agent from user
// turns, the LLM mistakes the human's commitment for noise from the patient
// and re-asks "qual horário?", which is exactly what we want to prevent.
func TestEval_HumanAgentConfirmedSlot_PatientSaysYes(t *testing.T) {
	h := setupEvalHarness(t)

	now := time.Now()
	h.convMgr.mu.Lock()
	h.convMgr.history = []conversation.ConversationTurn{
		{Role: conversation.RoleUser, Content: "oi quero marcar pilates", Timestamp: now.Add(-30 * time.Minute)},
		{Role: conversation.RoleAssistant, Content: "Olá! Que dia da semana você prefere?", Timestamp: now.Add(-29 * time.Minute)},
		{Role: conversation.RoleUser, Content: "segunda de manhã", Timestamp: now.Add(-28 * time.Minute)},
		{Role: conversation.RoleHumanAgent, Content: "oi! consegui te encaixar segunda 9h com a Mariana, tudo certo?", Timestamp: now.Add(-5 * time.Minute)},
	}
	h.convMgr.mu.Unlock()

	h.Send("sim pode confirmar")

	last := strings.ToLower(h.lastReply())

	// Failure mode: re-asking time/day after the human already pinned them.
	rephraseTime := []string{"qual horário", "qual horario", "que horas", "qual dia", "que dia"}
	for _, phrase := range rephraseTime {
		if strings.Contains(last, phrase) {
			t.Errorf("AI re-asked time/day after human_agent already confirmed segunda 9h; phrase %q present in reply: %q", phrase, last)
		}
	}
}
