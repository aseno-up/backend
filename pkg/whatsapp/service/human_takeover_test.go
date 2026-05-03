package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/pkg/whatsapp/conversation"
	"github.com/aseno/backend/pkg/whatsapp/model"
)

// ---------------------------------------------------------------------------
// Stubs specific to human takeover tests
// ---------------------------------------------------------------------------

// stubMessageRepoWithExternalID is a MessageRepository stub that supports
// GetByExternalID lookups and records call counts for assertions.
type stubMessageRepoWithExternalID struct {
	byExternalID       map[string]*model.Message
	getByExternalCalls int
}

func (r *stubMessageRepoWithExternalID) Create(_ context.Context, _ *model.Message) error {
	return nil
}
func (r *stubMessageRepoWithExternalID) GetByID(_ context.Context, _ uuid.UUID) (*model.Message, error) {
	return nil, nil
}
func (r *stubMessageRepoWithExternalID) GetByExternalID(_ context.Context, externalID string) (*model.Message, error) {
	r.getByExternalCalls++
	if m, ok := r.byExternalID[externalID]; ok {
		return m, nil
	}
	return nil, nil
}
func (r *stubMessageRepoWithExternalID) UpdateStatus(_ context.Context, _ uuid.UUID, _ model.MessageStatus) error {
	return nil
}
func (r *stubMessageRepoWithExternalID) GetByAppointmentID(_ context.Context, _ uuid.UUID) ([]*model.Message, error) {
	return nil, nil
}
func (r *stubMessageRepoWithExternalID) GetPendingReminders(_ context.Context, _ time.Time) ([]*model.Message, error) {
	return nil, nil
}

// stubSentMessageCache lets tests configure whether an ExternalID was
// "sent by us" and whether Redis is available.
type stubSentMessageCache struct {
	mu        sync.Mutex
	recorded  map[string]bool
	available bool
}

func newStubSentMessageCache(available bool) *stubSentMessageCache {
	return &stubSentMessageCache{
		recorded:  map[string]bool{},
		available: available,
	}
}

func (s *stubSentMessageCache) RecordSentMessageID(_ context.Context, externalID string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorded[externalID] = true
	return nil
}

func (s *stubSentMessageCache) IsOurSentMessage(_ context.Context, externalID string) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.available {
		return false, false
	}
	return s.recorded[externalID], true
}

// stubTakeoverTracker implements HumanTakeoverTracker.
type stubTakeoverTracker struct {
	mu        sync.Mutex
	takeovers map[string]time.Time // key: clinicID|phone
	markCalls int
	lastPhone string
}

func newStubTakeoverTracker() *stubTakeoverTracker {
	return &stubTakeoverTracker{takeovers: map[string]time.Time{}}
}

func (t *stubTakeoverTracker) key(clinicID uuid.UUID, phone string) string {
	return clinicID.String() + "|" + phone
}

func (t *stubTakeoverTracker) Mark(_ context.Context, clinicID uuid.UUID, phone string, at time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.markCalls++
	t.lastPhone = phone
	t.takeovers[t.key(clinicID, phone)] = at
	return nil
}

func (t *stubTakeoverTracker) GetTakeoverAt(_ context.Context, clinicID uuid.UUID, phone string) (*time.Time, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if at, ok := t.takeovers[t.key(clinicID, phone)]; ok {
		return &at, nil
	}
	return nil, nil
}

func (t *stubTakeoverTracker) preset(clinicID uuid.UUID, phone string, at time.Time) {
	t.takeovers[t.key(clinicID, phone)] = at
}

// stubConversationMgrNoTakeover is a minimal conversation.Manager that only
// tracks AddUserMessage calls (used to assert the takeover gate persists
// messages even while paused).
type stubConversationMgrNoTakeover struct {
	mu                 sync.Mutex
	addUserMessages    []string
	humanAgentMessages []string
	conv               *conversation.ConversationContext
}

func newStubConversationMgrNoTakeover() *stubConversationMgrNoTakeover {
	return &stubConversationMgrNoTakeover{conv: &conversation.ConversationContext{ID: uuid.New()}}
}

func (m *stubConversationMgrNoTakeover) GetOrCreate(_ context.Context, _ uuid.UUID, _, _ string) (*conversation.ConversationContext, error) {
	return m.conv, nil
}
func (m *stubConversationMgrNoTakeover) Get(_ context.Context, _ uuid.UUID) (*conversation.ConversationContext, error) {
	return m.conv, nil
}
func (m *stubConversationMgrNoTakeover) AddUserMessage(_ context.Context, _ uuid.UUID, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addUserMessages = append(m.addUserMessages, content)
	return nil
}
func (m *stubConversationMgrNoTakeover) AddAssistantMessage(_ context.Context, _ uuid.UUID, _ string, _ []conversation.ToolCall) error {
	return nil
}
func (m *stubConversationMgrNoTakeover) AddHumanAgentMessage(_ context.Context, _ uuid.UUID, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.humanAgentMessages = append(m.humanAgentMessages, content)
	return nil
}
func (m *stubConversationMgrNoTakeover) UpdateState(_ context.Context, _ uuid.UUID, _ conversation.ConversationState, _ model.MessageIntent) error {
	return nil
}
func (m *stubConversationMgrNoTakeover) SetPendingAction(_ context.Context, _ uuid.UUID, _ string, _ interface{}) error {
	return nil
}
func (m *stubConversationMgrNoTakeover) GetPendingAction(_ context.Context, _ uuid.UUID, _ string) (interface{}, bool) {
	return nil, false
}
func (m *stubConversationMgrNoTakeover) SetMetadata(_ context.Context, _ uuid.UUID, _ string, _ interface{}) error {
	return nil
}
func (m *stubConversationMgrNoTakeover) SetPatientID(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (m *stubConversationMgrNoTakeover) ExpireConversation(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (m *stubConversationMgrNoTakeover) CompleteConversation(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (m *stubConversationMgrNoTakeover) LogToolCall(_ context.Context, _ uuid.UUID, _ string, _ map[string]interface{}, _ interface{}, _ error, _ time.Duration) error {
	return nil
}

// ---------------------------------------------------------------------------
// HandleOutgoingMessage
// ---------------------------------------------------------------------------

func TestHandleOutgoingMessage_RedisHit_NoTakeover(t *testing.T) {
	cache := newStubSentMessageCache(true)
	_ = cache.RecordSentMessageID(context.Background(), "msg-xyz", time.Minute)
	tracker := newStubTakeoverTracker()

	svc := &whatsAppService{sentMessageCache: cache, takeoverTracker: tracker}
	err := svc.HandleOutgoingMessage(context.Background(), uuid.New(), &model.ParsedIncomingMessage{
		Phone:     "+5511999990000",
		MessageID: "msg-xyz",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, tracker.markCalls, "our own echo must not mark takeover")
}

func TestHandleOutgoingMessage_RedisMiss_MarksTakeover(t *testing.T) {
	cache := newStubSentMessageCache(true)
	tracker := newStubTakeoverTracker()

	svc := &whatsAppService{sentMessageCache: cache, takeoverTracker: tracker}
	err := svc.HandleOutgoingMessage(context.Background(), uuid.New(), &model.ParsedIncomingMessage{
		Phone:     "+5511988880000",
		MessageID: "external-unknown",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, tracker.markCalls, "unknown fromMe ID must mark takeover")
	assert.Equal(t, "+5511988880000", tracker.lastPhone)
}

// TestHandleOutgoingMessage_HumanAgentMessage_PersistedInHistory verifies the
// fix for the "AI asks 'qual horário?' after human confirmed" bug: when a
// clinic attendant types directly to the patient, the message must land in
// the conversation history under RoleHumanAgent so the AI has full context
// when it eventually resumes after the takeover pause.
func TestHandleOutgoingMessage_HumanAgentMessage_PersistedInHistory(t *testing.T) {
	cache := newStubSentMessageCache(true)
	tracker := newStubTakeoverTracker()
	convMgr := newStubConversationMgrNoTakeover()

	svc := &whatsAppService{
		sentMessageCache: cache,
		takeoverTracker:  tracker,
		conversationMgr:  convMgr,
	}
	err := svc.HandleOutgoingMessage(context.Background(), uuid.New(), &model.ParsedIncomingMessage{
		Phone:     "+5511977770001",
		MessageID: "human-typed-msg",
		Content:   "consegui encaixar segunda 14h, tudo certo?",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, tracker.markCalls, "human-typed message must trigger takeover")
	assert.Equal(t, []string{"consegui encaixar segunda 14h, tudo certo?"}, convMgr.humanAgentMessages,
		"attendant message must be persisted under RoleHumanAgent so AI has context when pause ends")
	assert.Empty(t, convMgr.addUserMessages,
		"attendant message must NOT land under RoleUser — that's the bug we're fixing")
}

// TestHandleOutgoingMessage_OurOwnEcho_DoesNotPersist guards against the
// reverse failure: messages WE sent (the AI's own outbound) must not be
// re-persisted under RoleHumanAgent. Detection short-circuits before the
// persistence block — this test pins that ordering.
func TestHandleOutgoingMessage_OurOwnEcho_DoesNotPersist(t *testing.T) {
	cache := newStubSentMessageCache(true)
	_ = cache.RecordSentMessageID(context.Background(), "ours", time.Minute)
	tracker := newStubTakeoverTracker()
	convMgr := newStubConversationMgrNoTakeover()

	svc := &whatsAppService{
		sentMessageCache: cache,
		takeoverTracker:  tracker,
		conversationMgr:  convMgr,
	}
	err := svc.HandleOutgoingMessage(context.Background(), uuid.New(), &model.ParsedIncomingMessage{
		Phone:     "+5511977770002",
		MessageID: "ours",
		Content:   "Olá! Posso ajudar?",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, tracker.markCalls)
	assert.Empty(t, convMgr.humanAgentMessages, "our own echo must not be persisted as human-agent")
}

func TestHandleOutgoingMessage_RedisUnavailable_UsesDBFallback_Hit(t *testing.T) {
	cache := newStubSentMessageCache(false)
	repo := &stubMessageRepoWithExternalID{byExternalID: map[string]*model.Message{
		"db-known": {ExternalID: "db-known", Direction: model.MessageDirectionOutbound},
	}}
	tracker := newStubTakeoverTracker()

	svc := &whatsAppService{sentMessageCache: cache, messageRepo: repo, takeoverTracker: tracker}
	err := svc.HandleOutgoingMessage(context.Background(), uuid.New(), &model.ParsedIncomingMessage{
		Phone:     "+5511977770000",
		MessageID: "db-known",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, tracker.markCalls)
	assert.Equal(t, 1, repo.getByExternalCalls)
}

func TestHandleOutgoingMessage_RedisUnavailable_DBMiss_MarksTakeover(t *testing.T) {
	cache := newStubSentMessageCache(false)
	repo := &stubMessageRepoWithExternalID{byExternalID: map[string]*model.Message{}}
	tracker := newStubTakeoverTracker()

	svc := &whatsAppService{sentMessageCache: cache, messageRepo: repo, takeoverTracker: tracker}
	err := svc.HandleOutgoingMessage(context.Background(), uuid.New(), &model.ParsedIncomingMessage{
		Phone:     "+5511966660000",
		MessageID: "truly-external",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, tracker.markCalls)
}

func TestHandleOutgoingMessage_RedisMiss_DBHit_NoTakeover(t *testing.T) {
	cache := newStubSentMessageCache(true)
	repo := &stubMessageRepoWithExternalID{byExternalID: map[string]*model.Message{
		"race-id": {ExternalID: "race-id", Direction: model.MessageDirectionOutbound},
	}}
	tracker := newStubTakeoverTracker()

	svc := &whatsAppService{sentMessageCache: cache, messageRepo: repo, takeoverTracker: tracker}
	err := svc.HandleOutgoingMessage(context.Background(), uuid.New(), &model.ParsedIncomingMessage{
		Phone:     "+5511955550000",
		MessageID: "race-id",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, tracker.markCalls, "race window: DB has the row before Redis propagates")
}

func TestHandleOutgoingMessage_NoTrackerWired_NoOp(t *testing.T) {
	cache := newStubSentMessageCache(true)
	svc := &whatsAppService{sentMessageCache: cache, takeoverTracker: nil}
	err := svc.HandleOutgoingMessage(context.Background(), uuid.New(), &model.ParsedIncomingMessage{
		MessageID: "unknown",
	})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Gate in handleWithAIAgent
// ---------------------------------------------------------------------------

func takeoverEnabledClinic(pauseHours int) *struct{} { return nil } // placeholder to avoid unused import; real helper below

func TestHandleWithAIAgent_TakeoverActive_SkipsAgent_PersistsMessage(t *testing.T) {
	ai := &stubAIAgentService{}
	svc := newTestServiceWithAIPref(ai, &stubPatientAIPrefChecker{enabled: true})
	clinic := aiEnabledClinic()
	pauseHours := 4
	clinic.Settings.AIHumanTakeoverPauseHours = &pauseHours
	svc.clinicRepo = &stubClinicRepo{clinic: clinic}

	clinicID := uuid.New()
	phone := "+5511999990000"
	recent := time.Now().Add(-1 * time.Hour)

	tracker := newStubTakeoverTracker()
	tracker.preset(clinicID, phone, recent)
	svc.takeoverTracker = tracker

	convMgr := newStubConversationMgrNoTakeover()
	svc.conversationMgr = convMgr

	err := svc.handleWithAIAgent(context.Background(), clinicID, newMsg(phone, "quero remarcar pra sexta"))
	require.NoError(t, err)
	assert.Equal(t, 0, ai.callCount(), "AI must not be called while takeover is active")
	assert.Equal(t, []string{"quero remarcar pra sexta"}, convMgr.addUserMessages,
		"user message during takeover must be persisted so AI has context when it resumes")
}

func TestHandleWithAIAgent_TakeoverExpired_CallsAgent(t *testing.T) {
	ai := &stubAIAgentService{}
	svc := newTestServiceWithAIPref(ai, &stubPatientAIPrefChecker{enabled: true})
	clinic := aiEnabledClinic()
	pauseHours := 4
	clinic.Settings.AIHumanTakeoverPauseHours = &pauseHours
	svc.clinicRepo = &stubClinicRepo{clinic: clinic}

	clinicID := uuid.New()
	phone := "+5511999990000"
	old := time.Now().Add(-6 * time.Hour)

	tracker := newStubTakeoverTracker()
	tracker.preset(clinicID, phone, old)
	svc.takeoverTracker = tracker

	err := svc.handleWithAIAgent(context.Background(), clinicID, newMsg(phone, "oi"))
	require.NoError(t, err)
	assert.Equal(t, 1, ai.callCount(), "expired takeover must NOT block the AI")
}

func TestHandleWithAIAgent_PauseDisabled_CallsAgent(t *testing.T) {
	ai := &stubAIAgentService{}
	svc := newTestServiceWithAIPref(ai, &stubPatientAIPrefChecker{enabled: true})
	clinic := aiEnabledClinic()
	pauseHours := 0
	clinic.Settings.AIHumanTakeoverPauseHours = &pauseHours
	svc.clinicRepo = &stubClinicRepo{clinic: clinic}

	clinicID := uuid.New()
	phone := "+5511999990000"
	recent := time.Now().Add(-30 * time.Minute)

	tracker := newStubTakeoverTracker()
	tracker.preset(clinicID, phone, recent)
	svc.takeoverTracker = tracker

	err := svc.handleWithAIAgent(context.Background(), clinicID, newMsg(phone, "oi"))
	require.NoError(t, err)
	assert.Equal(t, 1, ai.callCount(),
		"pause_hours=0 disables the feature — AI runs even with recent takeover")
}

func TestHandleWithAIAgent_NoTrackerWired_BypassesGate(t *testing.T) {
	ai := &stubAIAgentService{}
	svc := newTestServiceWithAIPref(ai, &stubPatientAIPrefChecker{enabled: true})
	clinic := aiEnabledClinic()
	pauseHours := 4
	clinic.Settings.AIHumanTakeoverPauseHours = &pauseHours
	svc.clinicRepo = &stubClinicRepo{clinic: clinic}
	svc.takeoverTracker = nil

	err := svc.handleWithAIAgent(context.Background(), uuid.New(), newMsg("+5511999990000", "oi"))
	require.NoError(t, err)
	assert.Equal(t, 1, ai.callCount())
}
