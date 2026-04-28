package agent

import (
	"context"
	"time"

	"github.com/aseno/backend/pkg/whatsapp/conversation"
)

// AgentRequest represents a request to the AI agent
type AgentRequest struct {
	ConversationContext *conversation.ConversationContext
	UserMessage         string
	ClinicName          string
	ClinicBusinessInfo  string
	ClinicFAQ           string
	MaxTokens           int
	Temperature         float32
	// HistoryTurns limits how many past turns are injected into the prompt.
	// 0 uses the package default (DefaultPromptHistoryTurns = 15).
	HistoryTurns int

	// AllowedTools restricts which tools are exposed to the LLM for this
	// request. When nil/empty, the full registry is exposed (default).
	// Reserved as a primitive — current flow does not set it, the per-clinic
	// gating goes through DisallowedTools instead.
	AllowedTools []string
	// DisallowedTools is a blacklist set per-request from the clinic's
	// AITool* flags (Schedule/Cancel/Reschedule/Confirm). Write tools the
	// clinic disabled are filtered out of the LLM's view, regardless of
	// intent. The LLM falls back to complete_handoff_to_human when it would
	// have called a now-blacklisted tool.
	DisallowedTools []string
	// BlockedIntentReply is the resolved canned reply for this clinic. It is
	// the patient-facing message that complete_handoff_to_human returns when
	// the LLM transfers the conversation to the human team. Plumbed on every
	// request so the tool can render the right wording per clinic.
	BlockedIntentReply string
}

// AgentResponse represents the agent's response
type AgentResponse struct {
	Content      string                  // Generated message content
	ToolCalls    []conversation.ToolCall // Tools called during this turn
	FinishReason string                  // Why the generation stopped
	TokensUsed   int                     // Total tokens consumed
	InputTokens  int                     // Prompt tokens (0 if provider does not report)
	OutputTokens int                     // Completion tokens (0 if provider does not report)
	Latency      time.Duration           // Time taken to generate
	Model        string                  // Model that actually served this response (may be a fallback)
	Provider     string                  // Provider (gemini, claude, openai)

	// PrimaryModelFailed signals that the configured primary model failed and
	// a fallback was used. Set to true when Model != PrimaryModel.
	PrimaryModelFailed bool
	// PrimaryModel is the configured primary model (the one we wanted to use).
	PrimaryModel string
	// PrimaryModelError is a sanitized reason the primary failed (provider error message).
	PrimaryModelError string
}

// Agent defines the interface for AI agents
type Agent interface {
	// Generate generates a response for the given request
	Generate(ctx context.Context, req *AgentRequest) (*AgentResponse, error)

	// Provider returns the provider name (gemini, claude, openai)
	Provider() string

	// Model returns the model being used
	Model() string
}

// Flusher is an optional capability some agents implement to drain
// in-flight observability traces (LangFuse, etc.) before process exit.
// Agents that don't trace (or don't need flushing) just don't implement
// this — main.go uses a type assertion.
type Flusher interface {
	Flush()
}

// Config holds agent configuration
type Config struct {
	Provider       string        // gemini, claude, openai
	APIKey         string        // API key for the provider
	Model          string        // Primary model name
	FallbackModels []string      // List of fallback models to try in order if primary fails
	MaxTokens      int           // Max tokens to generate
	Temperature    float32       // Temperature for generation (0.0-1.0)
	Timeout        time.Duration // Request timeout
}
