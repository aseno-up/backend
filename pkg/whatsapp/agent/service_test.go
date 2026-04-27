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

func TestShouldUseAIAgent_ClinicOverrideBlocksScheduling(t *testing.T) {
	svc := newSvc(allToolsGlobalOn())
	c := clinicWithTools(boolPtr(false), nil, nil, nil)
	assert.False(t, svc.ShouldUseAIAgent(model.MessageIntentSchedule, c))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentConfirm, c), "other tools unaffected")
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentCancel, c))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentReschedule, c))
}

func TestShouldUseAIAgent_ClinicOverrideBlocksReschedule_IndependentFromCancel(t *testing.T) {
	svc := newSvc(allToolsGlobalOn())
	c := clinicWithTools(nil, nil, nil, boolPtr(false))
	assert.False(t, svc.ShouldUseAIAgent(model.MessageIntentReschedule, c),
		"Reschedule disabled must only block reschedule")
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentCancel, c),
		"Cancel must not be coupled to Reschedule flag (regression of old bug)")
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

func TestShouldUseAIAgent_GlobalAllowClinicBlocksOne(t *testing.T) {
	svc := newSvc(allToolsGlobalOn())
	c := clinicWithTools(boolPtr(true), boolPtr(true), boolPtr(true), boolPtr(false))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentSchedule, c))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentConfirm, c))
	assert.True(t, svc.ShouldUseAIAgent(model.MessageIntentCancel, c))
	assert.False(t, svc.ShouldUseAIAgent(model.MessageIntentReschedule, c),
		"clinic turns reschedule off, global stays on — reschedule must be blocked")
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
	response      *AgentResponse
}

func (f *fakeAgent) Generate(_ context.Context, _ *AgentRequest) (*AgentResponse, error) {
	f.generateCalls++
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
	convID         uuid.UUID
	updateStateErr error
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

func TestProcessMessage_BlockedIntent_ReturnsCannedAndSkipsLLM(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:   true,
			AIToolReschedule: boolPtr(false),
		},
	}
	fa := &fakeAgent{}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	resp, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		PatientName:  "Ana",
		MessageText:  "quero remarcar minha consulta",
		Intent:       model.MessageIntentReschedule,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, fa.generateCalls, "agent.Generate must NOT be called when intent is blocked")
	assert.NotEmpty(t, resp.Segments, "canned response must produce a segment")
	assert.Equal(t, resp.ResponseText, resp.Segments[0].Content)
	assert.Contains(t, resp.ResponseText, "minutinho", "default canned text should be used when clinic has no custom")
	assert.Equal(t, []string{"quero remarcar minha consulta"}, fcm.userMessages,
		"user message must be persisted so later intent classification has context")
	assert.Len(t, fcm.assistantMsgs, 1, "canned assistant reply must be persisted to conversation history")
	assert.Equal(t, resp.ResponseText, fcm.assistantMsgs[0])
}

func TestProcessMessage_BlockedIntent_UsesClinicCustomReply(t *testing.T) {
	clinic := &clinic_model.Clinic{
		ID: uuid.New(),
		Settings: clinic_model.ClinicSettings{
			AIAgentEnabled:       true,
			AIToolReschedule:     boolPtr(false),
			AIBlockedIntentReply: "Aguarde, vamos te ligar.",
		},
	}
	fa := &fakeAgent{}
	fcm := newFakeConvManager()
	svc := buildSvcWithGate(clinic, allToolsGlobalOn(), fa, fcm)

	resp, err := svc.ProcessMessage(context.Background(), &MessageRequest{
		ClinicID:     clinic.ID,
		PatientPhone: "+5511999990000",
		MessageText:  "quero remarcar",
		Intent:       model.MessageIntentReschedule,
	})
	require.NoError(t, err)
	assert.Equal(t, "Aguarde, vamos te ligar.", resp.ResponseText)
	assert.Equal(t, 0, fa.generateCalls)
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

	calls := rec.snapshot()
	require.Len(t, calls, 1, "exactly one Increment call expected per generation")
	assert.Equal(t, clinic.ID, calls[0].clinicID, "tokens must be billed to the request's clinic")
	assert.Equal(t, usagemetric_model.MetricAITokensUsed, calls[0].metric)
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

	assert.Empty(t, rec.snapshot(), "zero-token responses should not produce a row in usage_metrics")
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
	assert.Empty(t, rec.snapshot())
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
	aggCalls := aggRec.snapshot()
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
	assert.Len(t, aggRec.snapshot(), 1, "aggregate path must not depend on granular recorder being set")
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
