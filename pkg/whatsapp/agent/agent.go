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
}

// AgentResponse represents the agent's response
type AgentResponse struct {
	Content      string                  // Generated message content
	ToolCalls    []conversation.ToolCall // Tools called during this turn
	FinishReason string                  // Why the generation stopped
	TokensUsed   int                     // Total tokens consumed
	Latency      time.Duration           // Time taken to generate
	Model        string                  // Model used
	Provider     string                  // Provider (gemini, claude, openai)
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
