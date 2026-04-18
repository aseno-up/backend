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
}

func (f *fakeAgent) Generate(_ context.Context, _ *AgentRequest) (*AgentResponse, error) {
	f.generateCalls++
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
