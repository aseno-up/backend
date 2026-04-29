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

	// PatientContext is pre-fetched per-turn (next pending appointment +
	// active group enrollments). Injected into the system prompt's
	// dynamic tail so the LLM interprets short replies against real
	// state. Nil = pre-fetch disabled or first-contact patient with no
	// patient_id resolved yet.
	PatientContext *PatientContext
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

// PatientContext is pre-fetched data injected into the system prompt so
// the LLM can interpret short/ambiguous patient replies ("sim",
// "imprevisto", "vou hoje") relative to the patient's actual state.
//
// Three data sources, each handling a distinct part of how
// appointments live in the system:
//
//  1. appointments table — already-materialized 1-on-1 appointments
//     (created via quick_schedule / create_appointment). Only the next
//     pending one is included.
//
//  2. session_plans (type=individual) — recurring 1-on-1 plans where
//     each weekly occurrence is virtual until materialized. Common case:
//     fisio 1x/week. "Próxima sessão" comes from RRULE/WeekDays in code,
//     not from a DB row.
//
//  3. session_plan_enrollments (active) — group classes (Pilates etc.)
//     that NEVER materialize. Slots are weekly recurring (Seg 09:00,
//     Qua 09:00...); next class computed in code.
//
// Empty fields ARE meaningful — patient with nothing in the system is
// likely a new client or in transition from another system; the LLM
// should not assume context for short replies.
type PatientContext struct {
	// NextAppointment is the next pending individual appointment
	// already in the appointments table (limit 1 — weekly individual
	// sessions otherwise show appointments 3 weeks out, useless noise).
	NextAppointment *PatientNextAppointment

	// ActivePlans lists active recurring plans the patient has —
	// includes BOTH individual (fisio semanal recorrente) and group
	// (Pilates matrícula). Each entry has a Type field so the LLM can
	// route the right confirm/cancel tool (confirm_appointment for
	// individual, confirm_class_attendance for group).
	ActivePlans []PatientActivePlan
}

// PatientNextAppointment is one pending individual appointment.
type PatientNextAppointment struct {
	StartTime        time.Time
	ProfessionalName string
	ProcedureName    string
}

// PatientActivePlan covers both individual recurring plans and group
// matrículas. Type tells the LLM which confirm/cancel flow to use.
type PatientActivePlan struct {
	Type      string // "individual" or "group"
	PlanTitle string
	Slots     []PlanSlotPreview // weekly grid; same shape for both types
	NextClass *time.Time        // computed next future occurrence
}

// PlanSlotPreview is one weekly recurring slot.
type PlanSlotPreview struct {
	Weekday   int    // 0=Sun..6=Sat
	StartTime string // HH:MM
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
