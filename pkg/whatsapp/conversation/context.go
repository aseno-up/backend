package conversation

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/google/uuid"
)

// ConversationState represents the current state in the conversation flow
type ConversationState string

const (
	ConversationStateNew               ConversationState = "NEW"
	ConversationStateIntentIdentified  ConversationState = "INTENT_IDENTIFIED"
	ConversationStateScheduling        ConversationState = "SCHEDULING"
	ConversationStateAwaitingSlot      ConversationState = "AWAITING_SLOT"
	ConversationStateAwaitingConfirm   ConversationState = "AWAITING_CONFIRM"
	ConversationStateCancelling        ConversationState = "CANCELLING"
	ConversationStateRescheduling      ConversationState = "RESCHEDULING"
	ConversationStateQuestionAnswering ConversationState = "QUESTION_ANSWERING"
	ConversationStateCompleted         ConversationState = "COMPLETED"
	ConversationStateExpired           ConversationState = "EXPIRED"
)

// Conversation turn roles. "user" and "assistant" are the OpenAI/OpenRouter
// wire-format names and must stay as such for compatibility with provider
// payloads. "human_agent" is internal-only — set when a clinic attendant
// replies to the patient directly via the WhatsApp app, so the prompt builder
// can render those turns under a distinct label and the LLM doesn't mistake
// them for the patient nor for itself.
const (
	RoleUser       = "user"
	RoleAssistant  = "assistant"
	RoleHumanAgent = "human_agent"
)

// ToolCall represents a function/tool call made by the AI agent
type ToolCall struct {
	ID         string                 `json:"id"`               // Unique ID for this tool call
	Name       string                 `json:"name"`             // Tool name (e.g., "get_available_slots")
	Arguments  map[string]interface{} `json:"arguments"`        // Tool arguments
	Result     interface{}            `json:"result,omitempty"` // Tool execution result
	Error      string                 `json:"error,omitempty"`  // Error if tool execution failed
	ExecutedAt *time.Time             `json:"executed_at,omitempty"`
}

// ConversationTurn represents a single turn in the conversation
type ConversationTurn struct {
	Role      string     `json:"role"`                 // RoleUser, RoleAssistant or RoleHumanAgent
	Content   string     `json:"content"`              // Message content
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // Tool calls made in this turn
	Timestamp time.Time  `json:"timestamp"`            // When this turn occurred
}

// ConversationHistory is a slice of conversation turns
type ConversationHistory []ConversationTurn

// Value implements driver.Valuer for ConversationHistory
func (h ConversationHistory) Value() (driver.Value, error) {
	var v interface{} = h
	if h == nil {
		v = []ConversationTurn{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for ConversationHistory
func (h *ConversationHistory) Scan(value interface{}) error {
	if value == nil {
		*h = []ConversationTurn{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for ConversationHistory")
	}

	return json.Unmarshal(data, h)
}

// PendingActions is a map of pending actions/data
type PendingActions map[string]interface{}

// Value implements driver.Valuer for PendingActions
func (p PendingActions) Value() (driver.Value, error) {
	var v interface{} = p
	if p == nil {
		v = map[string]interface{}{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for PendingActions
func (p *PendingActions) Scan(value interface{}) error {
	if value == nil {
		*p = map[string]interface{}{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for PendingActions")
	}

	return json.Unmarshal(data, p)
}

// ConversationMetadata is a map of metadata
type ConversationMetadata map[string]interface{}

// Value implements driver.Valuer for ConversationMetadata
func (m ConversationMetadata) Value() (driver.Value, error) {
	var v interface{} = m
	if m == nil {
		v = map[string]interface{}{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for ConversationMetadata
func (m *ConversationMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = map[string]interface{}{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for ConversationMetadata")
	}

	return json.Unmarshal(data, m)
}

// ConversationContext represents a conversation session with a patient
type ConversationContext struct {
	ID             uuid.UUID            `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID       uuid.UUID            `json:"clinic_id" gorm:"type:uuid;not null;index:idx_conversation_phone"`
	PatientPhone   string               `json:"patient_phone" gorm:"type:varchar(50);not null;index:idx_conversation_phone"`
	PatientID      *uuid.UUID           `json:"patient_id,omitempty" gorm:"type:uuid;index"`
	PatientName    string               `json:"patient_name" gorm:"type:varchar(255)"`
	State          ConversationState    `json:"state" gorm:"type:varchar(50);not null;default:'NEW'"`
	Intent         model.MessageIntent  `json:"intent" gorm:"type:varchar(50)"`
	History        ConversationHistory  `json:"history" gorm:"type:jsonb;not null;default:'[]'"`
	PendingActions PendingActions       `json:"pending_actions" gorm:"type:jsonb"`
	Metadata       ConversationMetadata `json:"metadata" gorm:"type:jsonb"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	ExpiresAt      time.Time            `json:"expires_at" gorm:"index"`
}

// TableName returns the table name for GORM
func (ConversationContext) TableName() string {
	return "conversation_contexts"
}

// IsExpired checks if the conversation has expired
func (c *ConversationContext) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// AddUserTurn adds a user message to the conversation history
func (c *ConversationContext) AddUserTurn(content string, maxTurns int) {
	turn := ConversationTurn{
		Role:      RoleUser,
		Content:   content,
		Timestamp: time.Now(),
	}
	c.History = append(c.History, turn)
	c.trimHistory(maxTurns)
}

// AddAssistantTurn adds an assistant message to the conversation history
func (c *ConversationContext) AddAssistantTurn(content string, toolCalls []ToolCall, maxTurns int) {
	turn := ConversationTurn{
		Role:      RoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
		Timestamp: time.Now(),
	}
	c.History = append(c.History, turn)
	c.trimHistory(maxTurns)
}

// AddHumanAgentTurn records a message that a clinic attendant typed directly
// to the patient via the WhatsApp app while the AI was paused. Stored under
// RoleHumanAgent so the prompt builder can render it under a distinct label.
func (c *ConversationContext) AddHumanAgentTurn(content string, maxTurns int) {
	turn := ConversationTurn{
		Role:      RoleHumanAgent,
		Content:   content,
		Timestamp: time.Now(),
	}
	c.History = append(c.History, turn)
	c.trimHistory(maxTurns)
}

// trimHistory keeps only the last N turns to prevent unbounded growth
func (c *ConversationContext) trimHistory(maxTurns int) {
	if maxTurns <= 0 {
		maxTurns = 30 // Safety fallback
	}
	if len(c.History) > maxTurns {
		// Keep the last N turns
		c.History = c.History[len(c.History)-maxTurns:]
	}
}

// GetLastUserMessage returns the last user message content
func (c *ConversationContext) GetLastUserMessage() string {
	for i := len(c.History) - 1; i >= 0; i-- {
		if c.History[i].Role == RoleUser {
			return c.History[i].Content
		}
	}
	return ""
}

// ToolCallResult is a custom type for JSONB tool result
type ToolCallResult struct {
	data interface{}
}

// NewToolCallResult wraps a result value for JSONB storage
func NewToolCallResult(v interface{}) ToolCallResult {
	return ToolCallResult{data: v}
}

// Value implements driver.Valuer — serializa para JSON antes de inserir no PostgreSQL
func (r ToolCallResult) Value() (driver.Value, error) {
	if r.data == nil {
		return "null", nil
	}
	b, err := json.Marshal(r.data)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner
func (r *ToolCallResult) Scan(value interface{}) error {
	if value == nil {
		r.data = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for ToolCallResult")
	}
	return json.Unmarshal(data, &r.data)
}

// MarshalJSON implements json.Marshaler
func (r ToolCallResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.data)
}

// ToolCallLog represents a log of tool execution for monitoring
type ToolCallLog struct {
	ID             uuid.UUID          `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ConversationID uuid.UUID          `json:"conversation_id" gorm:"type:uuid;not null;index"`
	ToolName       string             `json:"tool_name" gorm:"type:varchar(100);not null;index"`
	Parameters     ToolCallParameters `json:"parameters" gorm:"type:jsonb"`
	Result         ToolCallResult     `json:"result" gorm:"type:jsonb"`
	Error          string             `json:"error" gorm:"type:text"`
	DurationMS     int                `json:"duration_ms" gorm:"type:int"`
	CreatedAt      time.Time          `json:"created_at" gorm:"index"`
}

// TableName returns the table name for GORM
func (ToolCallLog) TableName() string {
	return "tool_call_logs"
}

// ToolCallParameters is a custom type for JSONB tool parameters
type ToolCallParameters map[string]interface{}

// Value implements driver.Valuer for ToolCallParameters
func (p ToolCallParameters) Value() (driver.Value, error) {
	var v interface{} = p
	if p == nil {
		v = map[string]interface{}{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for ToolCallParameters
func (p *ToolCallParameters) Scan(value interface{}) error {
	if value == nil {
		*p = map[string]interface{}{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for ToolCallParameters")
	}

	return json.Unmarshal(data, p)
}
