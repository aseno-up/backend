package conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/internal/phone"

	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/google/uuid"
)

// Manager handles conversation lifecycle and state management
type Manager interface {
	// GetOrCreate retrieves an active conversation or creates a new one
	GetOrCreate(ctx context.Context, clinicID uuid.UUID, phone, patientName string) (*ConversationContext, error)

	// Get retrieves a conversation by ID
	Get(ctx context.Context, id uuid.UUID) (*ConversationContext, error)

	// AddUserMessage adds a user message to the conversation
	AddUserMessage(ctx context.Context, conversationID uuid.UUID, content string) error

	// AddAssistantMessage adds an assistant message to the conversation
	AddAssistantMessage(ctx context.Context, conversationID uuid.UUID, content string, toolCalls []ToolCall) error

	// UpdateState updates the conversation state
	UpdateState(ctx context.Context, conversationID uuid.UUID, state ConversationState, intent model.MessageIntent) error

	// SetPendingAction sets a pending action in the conversation
	SetPendingAction(ctx context.Context, conversationID uuid.UUID, key string, value interface{}) error

	// GetPendingAction retrieves a pending action from the conversation
	GetPendingAction(ctx context.Context, conversationID uuid.UUID, key string) (interface{}, bool)

	// SetMetadata sets metadata in the conversation
	SetMetadata(ctx context.Context, conversationID uuid.UUID, key string, value interface{}) error

	// SetPatientID sets the patient ID once the patient is identified
	SetPatientID(ctx context.Context, conversationID uuid.UUID, patientID uuid.UUID) error

	// ExpireConversation marks a conversation as expired
	ExpireConversation(ctx context.Context, conversationID uuid.UUID) error

	// CompleteConversation marks a conversation as completed
	CompleteConversation(ctx context.Context, conversationID uuid.UUID) error

	// LogToolCall logs a tool execution for monitoring
	LogToolCall(ctx context.Context, conversationID uuid.UUID, toolName string, parameters map[string]interface{}, result interface{}, err error, duration time.Duration) error
}

// conversationManager implements Manager
type conversationManager struct {
	repo        Repository
	defaultTTL  time.Duration
	extendedTTL time.Duration
	maxTurns    int
}

// NewManager creates a new conversation manager
func NewManager(repo Repository, defaultTTL, extendedTTL time.Duration, maxTurns int) Manager {
	if maxTurns <= 0 {
		maxTurns = 30 // Safety default
	}
	return &conversationManager{
		repo:        repo,
		defaultTTL:  defaultTTL,
		extendedTTL: extendedTTL,
		maxTurns:    maxTurns,
	}
}

// GetOrCreate retrieves an active conversation or creates a new one
func (m *conversationManager) GetOrCreate(ctx context.Context, clinicID uuid.UUID, phone, patientName string) (*ConversationContext, error) {
	// Try to get existing active conversation
	conversation, err := m.repo.GetActiveByPhone(ctx, clinicID, phone)
	if err == nil && !conversation.IsExpired() {
		logger.Printf("[Conversation Manager] Found active conversation %s for phone %s", conversation.ID, phone)
		return conversation, nil
	}

	// Create new conversation. ExpiresAt uses the extended (Postgres) TTL —
	// the short defaultTTL is for Redis cache validity only and treating it
	// as the row expiry causes every conversation to expire within 30
	// minutes, leading to duplicate rows per patient and loss of context.
	conversation = &ConversationContext{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		PatientPhone:   phone,
		PatientName:    patientName,
		State:          ConversationStateNew,
		History:        []ConversationTurn{},
		PendingActions: map[string]interface{}{},
		Metadata:       map[string]interface{}{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(m.extendedTTL),
	}

	if err := m.repo.Create(ctx, conversation); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	logger.Printf("[Conversation Manager] Created new conversation %s for phone %s", conversation.ID, phone)
	return conversation, nil
}

// Get retrieves a conversation by ID
func (m *conversationManager) Get(ctx context.Context, id uuid.UUID) (*ConversationContext, error) {
	return m.repo.GetByID(ctx, id)
}

// AddUserMessage adds a user message to the conversation
func (m *conversationManager) AddUserMessage(ctx context.Context, conversationID uuid.UUID, content string) error {
	conversation, err := m.repo.GetByID(ctx, conversationID)
	if err != nil {
		return err
	}

	conversation.AddUserTurn(content, m.maxTurns)

	return m.repo.Update(ctx, conversation)
}

// AddAssistantMessage adds an assistant message to the conversation
func (m *conversationManager) AddAssistantMessage(ctx context.Context, conversationID uuid.UUID, content string, toolCalls []ToolCall) error {
	conversation, err := m.repo.GetByID(ctx, conversationID)
	if err != nil {
		return err
	}

	conversation.AddAssistantTurn(content, toolCalls, m.maxTurns)

	return m.repo.Update(ctx, conversation)
}

// UpdateState updates the conversation state
func (m *conversationManager) UpdateState(ctx context.Context, conversationID uuid.UUID, state ConversationState, intent model.MessageIntent) error {
	conversation, err := m.repo.GetByID(ctx, conversationID)
	if err != nil {
		return err
	}

	conversation.State = state
	if intent != "" {
		conversation.Intent = intent
	}

	logger.Printf("[Conversation Manager] Updated conversation %s state to %s (intent: %s)", conversationID, state, intent)

	return m.repo.Update(ctx, conversation)
}

// CanonicalizePhone is a thin wrapper around internal/phone.Canonicalize kept
// here so existing callers inside this package don't churn. New code should
// import internal/phone directly.
func CanonicalizePhone(p string) string {
	return phone.Canonicalize(p)
}

// SetPendingAction sets a pending action in the conversation
func (m *conversationManager) SetPendingAction(ctx context.Context, conversationID uuid.UUID, key string, value interface{}) error {
	conversation, err := m.repo.GetByID(ctx, conversationID)
	if err != nil {
		return err
	}

	if conversation.PendingActions == nil {
		conversation.PendingActions = map[string]interface{}{}
	}

	conversation.PendingActions[key] = value

	return m.repo.Update(ctx, conversation)
}

// GetPendingAction retrieves a pending action from the conversation
func (m *conversationManager) GetPendingAction(ctx context.Context, conversationID uuid.UUID, key string) (interface{}, bool) {
	conversation, err := m.repo.GetByID(ctx, conversationID)
	if err != nil {
		return nil, false
	}

	value, exists := conversation.PendingActions[key]
	return value, exists
}

// SetMetadata sets metadata in the conversation
func (m *conversationManager) SetMetadata(ctx context.Context, conversationID uuid.UUID, key string, value interface{}) error {
	conversation, err := m.repo.GetByID(ctx, conversationID)
	if err != nil {
		return err
	}

	if conversation.Metadata == nil {
		conversation.Metadata = map[string]interface{}{}
	}

	conversation.Metadata[key] = value

	return m.repo.Update(ctx, conversation)
}

// SetPatientID sets the patient ID once the patient is identified
func (m *conversationManager) SetPatientID(ctx context.Context, conversationID uuid.UUID, patientID uuid.UUID) error {
	conversation, err := m.repo.GetByID(ctx, conversationID)
	if err != nil {
		return err
	}

	conversation.PatientID = &patientID

	logger.Printf("[Conversation Manager] Set patient ID %s for conversation %s", patientID, conversationID)

	return m.repo.Update(ctx, conversation)
}

// ExpireConversation marks a conversation as expired
func (m *conversationManager) ExpireConversation(ctx context.Context, conversationID uuid.UUID) error {
	conversation, err := m.repo.GetByID(ctx, conversationID)
	if err != nil {
		return err
	}

	conversation.State = ConversationStateExpired
	conversation.ExpiresAt = time.Now()

	logger.Printf("[Conversation Manager] Expired conversation %s", conversationID)

	return m.repo.Update(ctx, conversation)
}

// CompleteConversation marks a conversation as completed
func (m *conversationManager) CompleteConversation(ctx context.Context, conversationID uuid.UUID) error {
	conversation, err := m.repo.GetByID(ctx, conversationID)
	if err != nil {
		return err
	}

	conversation.State = ConversationStateCompleted

	logger.Printf("[Conversation Manager] Completed conversation %s", conversationID)

	return m.repo.Update(ctx, conversation)
}

// LogToolCall logs a tool execution for monitoring
func (m *conversationManager) LogToolCall(ctx context.Context, conversationID uuid.UUID, toolName string, parameters map[string]interface{}, result interface{}, err error, duration time.Duration) error {
	log := &ToolCallLog{
		ConversationID: conversationID,
		ToolName:       toolName,
		Parameters:     parameters,
		Result:         NewToolCallResult(result),
		DurationMS:     int(duration.Milliseconds()),
	}

	if err != nil {
		log.Error = err.Error()
	}

	return m.repo.LogToolCall(ctx, log)
}
