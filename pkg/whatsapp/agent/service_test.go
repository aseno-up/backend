package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	systemlog_model "github.com/aseno/backend/pkg/systemlog/model"
	systemlog_service "github.com/aseno/backend/pkg/systemlog/service"
	usagemetric_model "github.com/aseno/backend/pkg/usagemetric/model"
	"github.com/aseno/backend/pkg/whatsapp/conversation"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/tools"
)

func boolPtr(b bool) *bool {
	return &b
}

// allToolsGlobalOn represents the ops-level kill-switch fully open.
func allToolsGlobalOn() *FeatureFlags {
	return &FeatureFlags{
		SchedulingEnabled: true,
		ConfirmEnabled:    true,
		CancelEnabled:     true,
		RescheduleEnabled: true,
		RemindersEnabled:  true,
	}
}

func newSvc(flags *FeatureFlags) *aiAgentService {
	return &aiAgentService{featureFlags: flags}
}

func clinicWithTools(schedule, confirm, cancel, reschedule *bool) *clinic_model.Clinic {
	return &clinic_model.Clinic{
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolSchedule:   schedule,
			AIToolConfirm:    confirm,
			AIToolCancel:     cancel,
			AIToolReschedule: reschedule,
		},
	}
}

func TestShouldUseAIAgent_QuestionAlwaysAllowedWhenAgentOn(t *testing.T) {
	svc := newSvc(allToolsGlobalOn())
	c := clinicWithTools(boolPtr(false), boolPtr(false), boolPtr(false), boolPtr(false))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentQuestion, c),
		"Question is FAQ-answering and must stay on even with all tools disabled")
}

func TestShouldUseAIAgent_UnknownAlwaysAllowedWhenAgentOn(t *testing.T) {
	svc := newSvc(allToolsGlobalOn())
	c := clinicWithTools(boolPtr(false), boolPtr(false), boolPtr(false), boolPtr(false))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentUnknown, c),
		"Unknown must reach the AI so it can reclassify or ask clarification")
}

func TestShouldUseAIAgent_ClinicConfigDoesNotBlockGate(t *testing.T) {
	// Unification: per-clinic AITool* flags no longer block ShouldUseAIAgent.
	// They are enforced INSIDE the agent (via DisallowedTools) so the agent
	// still runs for blocked intents and routes through complete_handoff_to_human
	// instead of returning a canned reply. ShouldUseAIAgent only enforces the
	// global kill-switch.
	svc := newSvc(allToolsGlobalOn())
	c := clinicWithTools(boolPtr(false), boolPtr(false), boolPtr(false), boolPtr(false))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentSchedule, c),
		"clinic-disabled tools must NOT block the agent — DisallowedTools handles tool-level filtering inside ProcessMessage")
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentReschedule, c))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentCancel, c))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentConfirm, c))
}

func TestShouldUseAIAgent_NilClinicDefaultsAllowed(t *testing.T) {
	svc := newSvc(allToolsGlobalOn())
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentSchedule, nil))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentConfirm, nil))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentCancel, nil))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentReschedule, nil))
}

func TestShouldUseAIAgent_GlobalKillSwitchBlocksEvenIfClinicAllows(t *testing.T) {
	svc := newSvc(&FeatureFlags{
		SchedulingEnabled: false,
		ConfirmEnabled:    false,
		CancelEnabled:     false,
		RescheduleEnabled: false,
	})
	c := clinicWithTools(boolPtr(true), boolPtr(true), boolPtr(true), boolPtr(true))
	assert.False(t, svc.ShouldUseAIAgent(model.MessageIntentSchedule, c),
		"global kill-switch must win over clinic config")
	assert.False(t, svc.ShouldUseAIAgent(model.MessageIntentConfirm, c))
	assert.False(t, svc.ShouldUseAIAgent(model.MessageIntentCancel, c))
	assert.False(t, svc.ShouldUseAIAgent(model.MessageIntentReschedule, c))
}

func TestShouldUseAIAgent_GlobalAllowClinicBlocksOne_NoLongerBlocks(t *testing.T) {
	// Post-unification: clinic-level Reschedule=false no longer blocks the
	// gate (DisallowedTools handles it). All four return true here because
	// the global kill-switch is on.
	svc := newSvc(allToolsGlobalOn())
	c := clinicWithTools(boolPtr(true), boolPtr(true), boolPtr(true), boolPtr(false))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentSchedule, c))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentConfirm, c))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentCancel, c))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentReschedule, c),
		"clinic-level config no longer affects ShouldUseAIAgent — DisallowedTools enforces it inside the agent")
}

func TestBlockedIntentReply_DefaultWhenEmpty(t *testing.T) {
	s := clinic_model.ClinicSettings{}
	got := ResolveBlockedIntentReply(&s)
	assert.NotEmpty(t, got, "must fall back to a non-empty default canned reply")
}

func TestBlockedIntentReply_UsesClinicCustomWhenSet(t *testing.T) {
	custom := "Podemos te atender em instantes, aguarde!"
	s := clinic_model.ClinicSettings{AIBlockedIntentReply: custom}
	got := ResolveBlockedIntentReply(&s)
	assert.Equal(t, custom, got)
}

func TestBlockedIntentReply_NilClinicUsesDefault(t *testing.T) {
	got := ResolveBlockedIntentReply(nil)
	assert.NotEmpty(t, got)
}

// ---------------------------------------------------------------------------
// ProcessMessage integration test — gate must skip agent.Generate entirely
// ---------------------------------------------------------------------------

type fakeAgent struct {
	generateCalls int
	lastRequest   *AgentRequest
	response      *AgentResponse
}

func (f *fakeAgent) Generate(_ context.Context, req *AgentRequest) (*AgentResponse, error) {
	f.generateCalls++
	f.lastRequest = req
	if f.response != nil {
		return f.response, nil
	}
	return &AgentResponse{Content: "should not be reached"}, nil
}
func (f *fakeAgent) Provider() string { return "fake" }
func (f *fakeAgent) Model() string    { return "fake-model" }

type fakeConvManager struct {
	mu             sync.Mutex
	userMessages   []string
	assistantMsgs  []string
	humanAgentMsgs []string
	convID         uuid.UUID
	updateStateErr error
	preloadHistory []conversation.ConversationTurn
}

func newFakeConvManager() *fakeConvManager {
	return &fakeConvManager{convID: uuid.New()}
}

func (f *fakeConvManager) GetOrCreate(_ context.Context, clinicID uuid.UUID, phone, patientName string) (*conversation.ConversationContext, error) {
	return &conversation.ConversationContext{
		ID:           f.convID,
		ClinicID:     clinicID,
		PatientPhone: phone,
		PatientName:  patientName,
		History:      append(conversation.ConversationHistory(nil), f.preloadHistory...),
	}, nil
}
func (f *fakeConvManager) Get(_ context.Context, id uuid.UUID) (*conversation.ConversationContext, error) {
	return &conversation.ConversationContext{ID: id}, nil
}
func (f *fakeConvManager) AddUserMessage(_ context.Context, _ uuid.UUID, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.userMessages = append(f.userMessages, content)
	return nil
}
func (f *fakeConvManager) AddAssistantMessage(_ context.Context, _ uuid.UUID, content string, _ []conversation.ToolCall) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assistantMsgs = append(f.assistantMsgs, content)
	return nil
}
func (f *fakeConvManager) AddHumanAgentMessage(_ context.Context, _ uuid.UUID, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.humanAgentMsgs = append(f.humanAgentMsgs, content)
	return nil
}
func (f *fakeConvManager) UpdateState(_ context.Context, _ uuid.UUID, _ conversation.ConversationState, _ model.MessageIntent) error {
	return f.updateStateErr
}
func (f *fakeConvManager) SetPendingAction(_ context.Context, _ uuid.UUID, _ string, _ interface{}) error {
	return nil
}
func (f *fakeConvManager) GetPendingAction(_ context.Context, _ uuid.UUID, _ string) (interface{}, bool) {
	return nil, false
}
func (f *fakeConvManager) SetMetadata(_ context.Context, _ uuid.UUID, _ string, _ interface{}) error {
	return nil
}
func (f *fakeConvManager) SetPatientID(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (f *fakeConvManager) ExpireConversation(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (f *fakeConvManager) CompleteConversation(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (f *fakeConvManager) LogToolCall(_ context.Context, _ uuid.UUID, _ string, _ map[string]interface{}, _ interface{}, _ error, _ time.Duration) error {
	return nil
}

type fakeClinicRepo struct {
	clinic *clinic_model.Clinic
	err    error
}

func (r *fakeClinicRepo) Create(_ context.Context, _ *clinic_model.Clinic) error { return nil }
func (r *fakeClinicRepo) GetByID(_ context.Context, _ uuid.UUID) (*clinic_model.Clinic, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.clinic, nil
}
func (r *fakeClinicRepo) GetBySlug(_ context.Context, _ string) (*clinic_model.Clinic, error) {
	return nil, errors.New("not used")
}
func (r *fakeClinicRepo) GetByEmail(_ context.Context, _ string) (*clinic_model.Clinic, error) {
	return nil, errors.New("not used")
}
func (r *fakeClinicRepo) Update(_ context.Context, _ *clinic_model.Clinic) error { return nil }
func (r *fakeClinicRepo) Delete(_ context.Context, _ uuid.UUID) error            { return nil }
func (r *fakeClinicRepo) List(_ context.Context, _, _ int) ([]*clinic_model.Clinic, int64, error) {
	return nil, 0, nil
}
func (r *fakeClinicRepo) ExistsBySlug(_ context.Context, _ string) (bool, error)  { return false, nil }
func (r *fakeClinicRepo) ExistsByEmail(_ context.Context, _ string) (bool, error) { return false, nil }
func (r *fakeClinicRepo) CountByAccountID(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func buildSvcWithGate(clinic *clinic_model.Clinic, flags *FeatureFlags, fa *fakeAgent, fcm *fakeConvManager) *aiAgentService {
	return &aiAgentService{
		agent:            fa,
		conversationMgr:  fcm,
		toolRegistry:     tools.NewRegistry(),
		toolExecutor:     nil,
		executionContext: &tools.ExecutionContext{},
		featureFlags:     flags,
		clinicRepo:       &fakeClinicRepo{clinic: clinic},
	}
}

// Blocked intent path: instead of returning the canned reply directly, the
// service must run the LLM in handoff mode so it can gather context (tipo /
// dia-horário / profissional) before triggering the existing human-takeover
// gate. The team takes over with the conversation pre-loaded.

func TestProcessMessage_BlockedIntent_CallsGenerateInHandoffMode(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolReschedule: boolPtr(false),
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "Que tipo de consulta? E para quando?"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		PatientName:  "Ana",
		MessageText:  "quero remarcar minha consulta",
		Intent:       model.MessageIntentReschedule,
	})

	require.NoError(t, err)
	require.Equal(t, 1, fa.generateCalls, "Generate must run — clinic-disabled tools no longer block the agent, only filter the toolset")
	require.NotNil(t, fa.lastRequest)

	// Unified flow: write tools the clinic disabled appear in DisallowedTools
	// (filtered at registry level). The LLM falls back to
	// complete_handoff_to_human via the prompt rule.
	disallowed := fa.lastRequest.DisallowedTools
	bannedWrite := []string{"reschedule_appointment", "quick_reschedule"}
	for _, banned := range bannedWrite {
		assert.Contains(t, disallowed, banned, "write tool %q must be blacklisted because clinic disabled AIToolReschedule", banned)
	}
	assert.NotEmpty(t, fa.lastRequest.BlockedIntentReply, "the canned reply must be plumbed so complete_handoff_to_human can return it as the patient-facing message")
}

func TestProcessMessage_BlockedIntent_PassesClinicConfiguredCannedReply(t *testing.T) {
	custom := "Aguarde, vamos te ligar."
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:       true,
			AIToolReschedule:     boolPtr(false),
			AIBlockedIntentReply: custom,
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero remarcar",
		Intent:       model.MessageIntentReschedule,
	})
	require.NoError(t, err)
	require.NotNil(t, fa.lastRequest)
	assert.Equal(t, custom, fa.lastRequest.BlockedIntentReply,
		"clinic-configured canned reply must reach the agent so the handoff tool returns the right text")
}

// TestProcessMessage_SilentHandoff_SuppressesReplyAndPersistence covers the
// "AI decides not to respond" feature: when complete_handoff_to_human is
// invoked with mode=silent, the executor returns status=silenced. The agent
// service must (a) NOT persist the AI's text reply to history (otherwise
// future turns inherit a "ghost" message the patient never saw) and (b)
// return a MessageResponse with no segments so the WhatsApp service layer
// short-circuits the send path.
func TestProcessMessage_SilentHandoff_SuppressesReplyAndPersistence(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled: true,
		},
	}
	silentToolCall := conversation.ToolCall{
		Name:      "complete_handoff_to_human",
		Arguments: map[string]interface{}{"mode": "silent"},
		Result:    map[string]interface{}{"status": "silenced"},
	}
	fa := &fakeAgent{response: &AgentResponse{
		Content:   "would have replied but staying silent",
		ToolCalls: []conversation.ToolCall{silentToolCall},
	}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	resp, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "sim",
		Intent:       model.MessageIntentUnknown,
	})

	require.NoError(t, err)
	require.NotNil(t, resp, "service must still return a response struct so callers can read tokens/latency")
	assert.Empty(t, resp.Segments, "silent mode must produce zero segments — the WhatsApp service layer skips send when len(Segments)==0")
	assert.Empty(t, fcm.assistantMsgs,
		"silent mode must NOT persist the AI's draft text — that ghost message would mislead future turns")
	assert.Equal(t, []string{"sim"}, fcm.userMessages,
		"patient message is still persisted (separate path that runs before agent.Generate)")
}

// TestProcessMessage_HandoffMode_AlsoSuppresses pins the post-2026-05
// behavior change: handoff mode no longer sends a canned reply (the
// "aguarde, alguém te atende em breve" wording was tacky and broke flow
// mid-conversation). Both handoff and silent now suppress the patient-facing
// reply; handoff additionally pauses the conversation, silent doesn't.
func TestProcessMessage_HandoffMode_AlsoSuppresses(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled: true,
		},
	}
	handoffToolCall := conversation.ToolCall{
		Name:      "complete_handoff_to_human",
		Arguments: map[string]interface{}{},
		Result:    map[string]interface{}{"status": "handoff_completed"},
	}
	fa := &fakeAgent{response: &AgentResponse{
		Content:   "Vou te transferir pro time, só um minuto.",
		ToolCalls: []conversation.ToolCall{handoffToolCall},
	}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	resp, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero remarcar",
		Intent:       model.MessageIntentReschedule,
	})

	require.NoError(t, err)
	assert.Empty(t, resp.Segments, "handoff mode must produce zero segments — no canned reply goes to the patient anymore")
	assert.Empty(t, fcm.assistantMsgs,
		"handoff mode suppresses the assistant text the same way silent does — only difference is the takeover mark")
}

func TestProcessMessage_BlockedIntent_PersistsUserMessageBeforeHandoff(t *testing.T) {
	// The team needs to see the patient's original message when they take
	// over. Persisting it early means even if the LLM call later fails, the
	// message is already in conversation history.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled: true,
			AIToolCancel:   boolPtr(false),
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "qual aula você quer cancelar?"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero cancelar",
		Intent:       model.MessageIntentCancel,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"quero cancelar"}, fcm.userMessages,
		"patient message must be persisted exactly once even when the path goes through handoff mode")
}

func TestProcessMessage_BlockedIntent_UsesHandoffTemperature(t *testing.T) {
	// Handoff temperature must be lower than the conversational default
	// because handoff mode requires strict rule-following (call list_procedures
	// when type is vague, format R$ X,XX, never call complete_handoff_to_human
	// before gathering). Higher temp = more likely to skip rules.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolReschedule: boolPtr(false),
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "que tipo?"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.SetTemperatures(0.5, 0.3)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero remarcar",
		Intent:       model.MessageIntentReschedule,
	})
	require.NoError(t, err)
	require.NotNil(t, fa.lastRequest)
	assert.InDelta(t, 0.3, fa.lastRequest.Temperature, 0.001,
		"handoff mode must use the configured handoff temperature so the LLM follows strict gather rules")
}

func TestProcessMessage_NormalIntent_UsesConfiguredAgentTemperature(t *testing.T) {
	// Regression: env-configured temperature must reach the request. Before
	// the fix, service.go had Temperature:0.7 hardcoded and the env var was
	// silently dropped on the floor.
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.SetTemperatures(0.5, 0.3)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "tudo certo?",
		Intent:       model.MessageIntentConfirm,
	})
	require.NoError(t, err)
	require.NotNil(t, fa.lastRequest)
	assert.InDelta(t, 0.5, fa.lastRequest.Temperature, 0.001,
		"normal flow must use the configured agent temperature, not a hardcoded value")
}

// Real-world bug: clinic Renascer had AIToolSchedule=false, AIToolCancel=false,
// AIToolReschedule=false, AIToolConfirm=false. AI in middle of multi-turn
// scheduling conversation classified one message as Unknown (e.g. "com
// hamilton"), which makes ShouldUseAIAgent return true regardless of clinic
// flags — AI got full toolset including quick_schedule and CREATED an
// appointment. The fix: per-clinic disabled tools must be filtered out of
// the toolset on EVERY request, not just when intent is Schedule/Cancel/etc.

func TestProcessMessage_DisabledScheduleTools_BlacklistedEvenForQuestionIntent(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled: true,
			AIToolSchedule: boolPtr(false),
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "vocês fazem fisioterapia?",
		Intent:       model.MessageIntentQuestion, // bypass intent — but tools must still be filtered
	})
	require.NoError(t, err)
	require.NotNil(t, fa.lastRequest)

	// Schedule write tools must be blacklisted because clinic disabled them.
	disallowed := fa.lastRequest.DisallowedTools
	assert.Contains(t, disallowed, "create_appointment", "AIToolSchedule=false must blacklist create_appointment even on Question intent")
	assert.Contains(t, disallowed, "quick_schedule", "AIToolSchedule=false must blacklist quick_schedule even on Question intent")
}

func TestProcessMessage_DisabledTools_AllFlagsRespected(t *testing.T) {
	// All four tool flags off (Renascer setup): every write tool blacklisted.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolSchedule:   boolPtr(false),
			AIToolConfirm:    boolPtr(false),
			AIToolCancel:     boolPtr(false),
			AIToolReschedule: boolPtr(false),
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "ok",
		Intent:       model.MessageIntentUnknown,
	})
	require.NoError(t, err)

	disallowed := fa.lastRequest.DisallowedTools
	for _, tool := range []string{"create_appointment", "quick_schedule", "confirm_appointment", "cancel_appointment", "reschedule_appointment", "quick_reschedule"} {
		assert.Contains(t, disallowed, tool, "all-disabled clinic must blacklist write tool %q", tool)
	}
}

func TestProcessMessage_DisablingEnrollClass_AddsToDisallowedTools(t *testing.T) {
	// When the clinic disables AI-driven group-class enrollment via the UI,
	// enroll_patient_in_class must be physically removed from the toolset
	// so the LLM cannot call it. The Pilates flow then falls back to
	// complete_handoff_to_human after gathering the matrícula context.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:    true,
			AIToolEnrollClass: boolPtr(false),
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "queria me matricular no pilates",
		Intent:       model.MessageIntentUnknown,
	})
	require.NoError(t, err)
	assert.Contains(t, fa.lastRequest.DisallowedTools, "enroll_patient_in_class",
		"disabling AIToolEnrollClass must add enroll_patient_in_class to DisallowedTools so the LLM never calls it directly")
}

func TestProcessMessage_DisablingConfirmClass_AddsToDisallowedTools(t *testing.T) {
	// AIToolConfirmClass gates confirm_class_attendance specifically (the
	// group-class confirmation tool). AIToolConfirm continues to gate
	// confirm_appointment (1-on-1). They must be independent toggles.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:     true,
			AIToolConfirmClass: boolPtr(false),
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "vou confirmar minha próxima aula",
		Intent:       model.MessageIntentUnknown,
	})
	require.NoError(t, err)
	assert.Contains(t, fa.lastRequest.DisallowedTools, "confirm_class_attendance",
		"disabling AIToolConfirmClass must blacklist confirm_class_attendance specifically")
	assert.NotContains(t, fa.lastRequest.DisallowedTools, "confirm_appointment",
		"AIToolConfirmClass must NOT touch confirm_appointment — that one is gated by AIToolConfirm")
}

func TestProcessMessage_EnrollClass_DefaultEnabled_NotInDisallowedTools(t *testing.T) {
	// Clinics that never touched the flag (nil) must keep enroll available —
	// flipping the default would silently break the Renascer flow.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled: true,
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "ok",
		Intent:       model.MessageIntentUnknown,
	})
	require.NoError(t, err)
	assert.NotContains(t, fa.lastRequest.DisallowedTools, "enroll_patient_in_class",
		"nil/default AIToolEnrollClass must keep enroll_patient_in_class available so existing clinics aren't silently affected")
}

func TestProcessMessage_AllToolsEnabled_NoBlacklist(t *testing.T) {
	// Clinic with everything enabled: no blacklist, AI has full toolset.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolSchedule:   boolPtr(true),
			AIToolConfirm:    boolPtr(true),
			AIToolCancel:     boolPtr(true),
			AIToolReschedule: boolPtr(true),
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero marcar",
		Intent:       model.MessageIntentSchedule,
	})
	require.NoError(t, err)
	assert.Empty(t, fa.lastRequest.DisallowedTools, "all-enabled clinic must not blacklist anything")
}

func TestProcessMessage_ScheduleEnabled_NoBlacklist(t *testing.T) {
	// Regression: clinics with scheduling tool enabled keep the full toolset.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled: true,
			AIToolSchedule: boolPtr(true),
		},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "que dia?"}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero agendar",
		Intent:       model.MessageIntentSchedule,
	})
	require.NoError(t, err)
	require.Equal(t, 1, fa.generateCalls)
	require.NotNil(t, fa.lastRequest)
	assert.Empty(t, fa.lastRequest.AllowedTools, "no tool whitelist on the unified path — only DisallowedTools is used")
	assert.NotContains(t, fa.lastRequest.DisallowedTools, "create_appointment", "scheduling-enabled clinic must not blacklist write tools")
	assert.NotContains(t, fa.lastRequest.DisallowedTools, "quick_schedule", "scheduling-enabled clinic must not blacklist write tools")
}

// ---------------------------------------------------------------------------
// Per-clinic AI token usage tracking
// ---------------------------------------------------------------------------

type usageRecorderCall struct {
	clinicID uuid.UUID
	metric   string
	delta    float64
}

type fakeUsageRecorder struct {
	mu    sync.Mutex
	calls []usageRecorderCall
}

func (r *fakeUsageRecorder) Increment(_ context.Context, clinicID uuid.UUID, metric string, delta float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, usageRecorderCall{clinicID: clinicID, metric: metric, delta: delta})
	return nil
}

func (r *fakeUsageRecorder) snapshot() []usageRecorderCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]usageRecorderCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// callsForMetric filters the recorded increments to a single metric so tests
// targeting one specific counter (e.g. ai_tokens_used) are not coupled to
// unrelated counters that ProcessMessage may also emit (e.g. ai_conversations).
func (r *fakeUsageRecorder) callsForMetric(metric string) []usageRecorderCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []usageRecorderCall
	for _, c := range r.calls {
		if c.metric == metric {
			out = append(out, c)
		}
	}
	return out
}

func TestProcessMessage_RecordsAITokenUsagePerClinic(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{
		Content:    "Sua consulta esta confirmada para amanha as 10h.",
		TokensUsed: 1234,
	}}
	fcm := newFakeConvManager()
	rec := &fakeUsageRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.usageMetrics = rec

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		PatientName:  "Ana",
		MessageText:  "Tudo certo com a consulta?",
		Intent:       model.MessageIntentConfirm,
	})
	require.NoError(t, err)

	calls := rec.callsForMetric(usagemetric_model.MetricAITokensUsed)
	require.Len(t, calls, 1, "exactly one ai_tokens_used Increment expected per generation")
	assert.Equal(t, clinic.ID, calls[0].clinicID, "tokens must be billed to the request's clinic")
	assert.InDelta(t, 1234.0, calls[0].delta, 0.001)
}

func TestProcessMessage_DoesNotRecordWhenZeroTokens(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok", TokensUsed: 0}}
	fcm := newFakeConvManager()
	rec := &fakeUsageRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.usageMetrics = rec

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "?",
		Intent:       model.MessageIntentQuestion,
	})
	require.NoError(t, err)

	assert.Empty(t, rec.callsForMetric(usagemetric_model.MetricAITokensUsed),
		"zero-token responses should not increment ai_tokens_used")
}

func TestProcessMessage_NilUsageRecorderIsSafe(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok", TokensUsed: 999}}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	// svc.usageMetrics intentionally left nil — older wiring must not panic.

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "?",
		Intent:       model.MessageIntentQuestion,
	})
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// MetricAIConversations: count NEW conversations (one per session-start),
// not every message. Matches what the existing `ai_conversations` constant
// in usagemetric/model implies and what an admin would expect when reading
// the dashboard.
// ---------------------------------------------------------------------------

func TestProcessMessage_IncrementsAIConversations_OnNewConversation(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok", TokensUsed: 10}}
	fcm := newFakeConvManager() // GetOrCreate returns a new convo with empty History
	rec := &fakeUsageRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.usageMetrics = rec

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "oi",
		Intent:       model.MessageIntentQuestion,
	})
	require.NoError(t, err)

	calls := rec.snapshot()
	var convCalls []usageRecorderCall
	for _, c := range calls {
		if c.metric == usagemetric_model.MetricAIConversations {
			convCalls = append(convCalls, c)
		}
	}
	require.Len(t, convCalls, 1, "must increment ai_conversations exactly once for a fresh conversation")
	assert.Equal(t, clinic.ID, convCalls[0].clinicID, "metric must be scoped to the request's clinic (multi-tenancy)")
	assert.InDelta(t, 1.0, convCalls[0].delta, 0.001, "increment by exactly 1 per new conversation")
}

func TestProcessMessage_DoesNotIncrementAIConversations_OnExistingConversation(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok", TokensUsed: 10}}
	fcm := newFakeConvManager()
	// Simulate an existing conversation by giving the fake a non-empty History.
	fcm.preloadHistory = []conversation.ConversationTurn{
		{Role: "user", Content: "previous message"},
	}
	rec := &fakeUsageRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.usageMetrics = rec

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "second message",
		Intent:       model.MessageIntentQuestion,
	})
	require.NoError(t, err)

	for _, c := range rec.snapshot() {
		assert.NotEqual(t, usagemetric_model.MetricAIConversations, c.metric,
			"existing conversations must NOT re-increment the conversations counter")
	}
}

func TestProcessMessage_IncrementsAIConversations_EvenForBlockedIntent(t *testing.T) {
	// Blocked-intent path returns a canned reply (no LLM call), but the
	// conversation still exists and was handled by the AI service.
	// Counting it keeps the metric a simple "how many sessions did the AI
	// system handle?" instead of a fragile "did the LLM actually fire?".
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolReschedule: boolPtr(false),
		},
	}
	fa := &fakeAgent{}
	fcm := newFakeConvManager()
	rec := &fakeUsageRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.usageMetrics = rec

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero remarcar",
		Intent:       model.MessageIntentReschedule,
	})
	require.NoError(t, err)

	var found bool
	for _, c := range rec.snapshot() {
		if c.metric == usagemetric_model.MetricAIConversations {
			found = true
			break
		}
	}
	assert.True(t, found, "blocked-intent canned reply still represents one new AI-handled conversation")
}

func TestProcessMessage_BlockedIntentDoesNotRecordTokens(t *testing.T) {
	// Canned reply path bypasses the LLM; no tokens are spent, nothing to charge.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolReschedule: boolPtr(false),
		},
	}
	fa := &fakeAgent{}
	fcm := newFakeConvManager()
	rec := &fakeUsageRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.usageMetrics = rec

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero remarcar",
		Intent:       model.MessageIntentReschedule,
	})
	require.NoError(t, err)
	assert.Empty(t, rec.callsForMetric(usagemetric_model.MetricAITokensUsed),
		"blocked-intent path bypasses the LLM, so ai_tokens_used must not be incremented")
}

// ---------------------------------------------------------------------------
// Granular per-call AI token usage (model + provider + in/out split)
// ---------------------------------------------------------------------------

type fakeAITokenRecorder struct {
	mu     sync.Mutex
	events []AITokenUsageEvent
}

func (r *fakeAITokenRecorder) Record(_ context.Context, evt AITokenUsageEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, evt)
	return nil
}

func (r *fakeAITokenRecorder) snapshot() []AITokenUsageEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AITokenUsageEvent, len(r.events))
	copy(out, r.events)
	return out
}

func TestProcessMessage_RecordsGranularTokenUsage(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{
		Content:      "ok, anotado.",
		TokensUsed:   1234,
		InputTokens:  900,
		OutputTokens: 334,
		Model:        "openai/gpt-4o-mini",
		Provider:     "openrouter",
	}}
	fcm := newFakeConvManager()
	aggRec := &fakeUsageRecorder{}
	granRec := &fakeAITokenRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.usageMetrics = aggRec
	svc.aiTokenUsage = granRec

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		PatientName:  "Ana",
		MessageText:  "tudo certo?",
		Intent:       model.MessageIntentConfirm,
	})
	require.NoError(t, err)

	events := granRec.snapshot()
	require.Len(t, events, 1, "exactly one granular event per generation")
	evt := events[0]
	assert.Equal(t, clinic.ID, evt.ClinicID, "must be billed to the request's clinic (multi-tenancy)")
	assert.Equal(t, fcm.convID, evt.ConversationID, "must reference the active conversation")
	assert.Equal(t, "openrouter", evt.Provider)
	assert.Equal(t, "openai/gpt-4o-mini", evt.Model)
	assert.Equal(t, 900, evt.InputTokens)
	assert.Equal(t, 334, evt.OutputTokens)
	assert.Equal(t, 1234, evt.TotalTokens)

	// Regression: the daily aggregate (usage_metrics.ai_tokens_used) MUST keep
	// receiving the increment — the granular table sits alongside, not instead.
	aggCalls := aggRec.callsForMetric(usagemetric_model.MetricAITokensUsed)
	require.Len(t, aggCalls, 1, "aggregate increment must still happen")
	assert.InDelta(t, 1234.0, aggCalls[0].delta, 0.001)
}

func TestProcessMessage_GranularRecorderNilIsSafe(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{
		Content: "ok", TokensUsed: 100, InputTokens: 80, OutputTokens: 20,
		Model: "x", Provider: "y",
	}}
	fcm := newFakeConvManager()
	aggRec := &fakeUsageRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.usageMetrics = aggRec
	// svc.aiTokenUsage left nil — must not panic, aggregate must still fire.

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "?",
		Intent:       model.MessageIntentQuestion,
	})
	require.NoError(t, err)
	assert.Len(t, aggRec.callsForMetric(usagemetric_model.MetricAITokensUsed), 1,
		"aggregate ai_tokens_used path must not depend on granular recorder being set")
}

func TestProcessMessage_NoGranularEventForZeroTokens(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{Content: "ok", TokensUsed: 0}}
	fcm := newFakeConvManager()
	granRec := &fakeAITokenRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.aiTokenUsage = granRec

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "?",
		Intent:       model.MessageIntentQuestion,
	})
	require.NoError(t, err)
	assert.Empty(t, granRec.snapshot(), "no granular row when nothing was consumed")
}

// ---------------------------------------------------------------------------
// Primary model fallback emits a system_log warning so we can detect when
// gpt-4o-mini (or whatever AI_AGENT_MODEL is) starts silently degrading.
// ---------------------------------------------------------------------------

type fakeSystemLogSvc struct {
	mu      sync.Mutex
	entries []systemlog_service.SystemLogEntry
}

func (f *fakeSystemLogSvc) Log(_ context.Context, entry systemlog_service.SystemLogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeSystemLogSvc) List(_ context.Context, _ *systemlog_model.SystemLogFilter) (*systemlog_model.SystemLogResponse, error) {
	return nil, nil
}

func (f *fakeSystemLogSvc) snapshot() []systemlog_service.SystemLogEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]systemlog_service.SystemLogEntry, len(f.entries))
	copy(out, f.entries)
	return out
}

func (f *fakeSystemLogSvc) findEvent(event string) *systemlog_service.SystemLogEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.entries {
		if f.entries[i].Event == event {
			return &f.entries[i]
		}
	}
	return nil
}

func TestProcessMessage_LogsSystemLogOnPrimaryModelFallback(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{
		Content:            "ok",
		TokensUsed:         500,
		Model:              "google/gemini-2.0-flash-001", // what actually served
		Provider:           "openrouter",
		PrimaryModelFailed: true,
		PrimaryModel:       "openai/gpt-4o-mini",
		PrimaryModelError:  "rate limit exceeded",
	}}
	fcm := newFakeConvManager()
	logSvc := &fakeSystemLogSvc{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.systemLogSvc = logSvc

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "oi",
		Intent:       model.MessageIntentQuestion,
	})
	require.NoError(t, err)

	entry := logSvc.findEvent("ai_agent.primary_model_fallback")
	require.NotNil(t, entry, "must emit ai_agent.primary_model_fallback when fallback is used")
	require.NotNil(t, entry.ClinicID)
	assert.Equal(t, clinic.ID, *entry.ClinicID, "must scope the alert to the clinic affected (multi-tenancy)")
	assert.Equal(t, "ai_agent", entry.Service)
	assert.Equal(t, systemlog_model.LogStatusWarning, entry.Status,
		"primary failure with successful fallback is a warning, not a failure — service still worked")
	assert.Equal(t, "rate limit exceeded", entry.Error)

	payload, ok := entry.Payload.(map[string]interface{})
	require.True(t, ok, "payload must be a map for queryable fields")
	assert.Equal(t, "openai/gpt-4o-mini", payload["primary_model"])
	assert.Equal(t, "google/gemini-2.0-flash-001", payload["fallback_model"])
	assert.Equal(t, "openrouter", payload["provider"])
}

func TestProcessMessage_NoSystemLogWhenPrimaryModelSucceeds(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID:       uuid.New(),
		Settings: clinic_model.ClinicSettings{AIAgentEnabled: true},
	}
	fa := &fakeAgent{response: &AgentResponse{
		Content:            "ok",
		TokensUsed:         500,
		Model:              "openai/gpt-4o-mini",
		Provider:           "openrouter",
		PrimaryModelFailed: false,
	}}
	fcm := newFakeConvManager()
	logSvc := &fakeSystemLogSvc{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.systemLogSvc = logSvc

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "oi",
		Intent:       model.MessageIntentQuestion,
	})
	require.NoError(t, err)

	assert.Nil(t, logSvc.findEvent("ai_agent.primary_model_fallback"),
		"happy path must not emit fallback event")
}

func TestProcessMessage_BlockedIntentDoesNotRecordGranular(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolReschedule: boolPtr(false),
		},
	}
	fa := &fakeAgent{}
	fcm := newFakeConvManager()
	granRec := &fakeAITokenRecorder{}
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)
	svc.aiTokenUsage = granRec

	_, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero remarcar",
		Intent:       model.MessageIntentReschedule,
	})
	require.NoError(t, err)
	assert.Empty(t, granRec.snapshot())
}

func TestProcessMessage_QuestionPassesGateEvenWithAllToolsOff(t *testing.T) {
	// This is the FAQ-only lead scenario: everything off except Question.
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolSchedule:   boolPtr(false),
			AIToolConfirm:    boolPtr(false),
			AIToolCancel:     boolPtr(false),
			AIToolReschedule: boolPtr(false),
		},
	}
	fa := &fakeAgent{}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	// Use Question intent — should reach the agent.
	// The test stops before calling Generate because executionContext.PatientFinderCreator is nil
	// and the stub agent would be reached; we assert specifically that the gate did not fire.
	_, _ = svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quais sao os horarios de atendimento?",
		Intent:       model.MessageIntentQuestion,
	})

	assert.Equal(t, 1, fa.generateCalls,
		"Question intent must reach the LLM even when all scheduling tools are disabled")
}
