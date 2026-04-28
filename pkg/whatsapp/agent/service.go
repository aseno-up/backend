package agent

import (
	"context"
	"fmt"
	"github.com/aseno/backend/internal/logger"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	systemlog_model "github.com/aseno/backend/pkg/systemlog/model"
	systemlog_service "github.com/aseno/backend/pkg/systemlog/service"
	usagemetric_model "github.com/aseno/backend/pkg/usagemetric/model"
	"github.com/aseno/backend/pkg/whatsapp/conversation"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/tools"
)

// TokenUsageRecorder persists per-clinic AI token consumption.
// Satisfied by usagemetric.Service; defined here as a tiny interface to keep
// this package's dependency surface minimal and to make the dependency mockable.
type TokenUsageRecorder interface {
	Increment(ctx context.Context, clinicID uuid.UUID, metric string, delta float64) error
}

// AITokenUsageEvent describes a single AI generation's resource consumption.
// One event = one call to the LLM, with the model and provider that served it,
// so cost can be attributed per-clinic per-model later.
type AITokenUsageEvent struct {
	ClinicID       uuid.UUID
	ConversationID uuid.UUID
	Provider       string // "openrouter", "gemini", ...
	Model          string // e.g. "openai/gpt-4o-mini"
	InputTokens    int    // prompt tokens (best-effort; 0 if provider does not report)
	OutputTokens  int     // completion tokens (best-effort; 0 if provider does not report)
	TotalTokens    int    // full billable token count
}

// AITokenUsageRecorder persists per-call AI token consumption with full
// granularity (model, provider, in/out split) so we can answer
// "how much did clinic X spend on model Y this month?".
//
// This sits ALONGSIDE TokenUsageRecorder — it does not replace the daily
// aggregate. Implementations should be best-effort and non-blocking;
// failures must not fail the user-facing reply.
type AITokenUsageRecorder interface {
	Record(ctx context.Context, evt AITokenUsageEvent) error
}

// Service handles AI-driven WhatsApp message processing
type Service interface {
	// ProcessMessage processes an incoming message using AI agent
	ProcessMessage(ctx context.Context, req *MessageRequest) (*MessageResponse, error)

	// ShouldUseAIAgent determines if AI agent should handle this message
	// taking into account both the ops-level kill-switch (FeatureFlags) and
	// the per-clinic tool configuration.
	ShouldUseAIAgent(intent model.MessageIntent, clinic *clinic_model.Clinic) bool

	// SetPromptHistoryTurns tunes the history window injected into the LLM
	// prompt. Configured from env (CONVERSATION_PROMPT_HISTORY_TURNS).
	SetPromptHistoryTurns(n int)

	// SetTemperatures configures the LLM sampling temperature for the two
	// paths. Wired from env: AI_AGENT_TEMPERATURE (conversational default,
	// suggested 0.5) and AI_AGENT_HANDOFF_TEMPERATURE (strict, suggested 0.3).
	SetTemperatures(agentTemp, handoffTemp float32)
}

// MessageRequest represents a request to process a WhatsApp message
type MessageRequest struct {
	ClinicID     uuid.UUID
	ClinicName   string
	PatientPhone string
	PatientName  string
	MessageText  string
	Intent       model.MessageIntent
}

// MessageResponse represents the AI agent's response
type MessageResponse struct {
	ResponseText  string
	MessageType   model.MessageType
	Segments      []*model.MessageSegment
	ToolCallCount int
	TokensUsed    int
	Latency       time.Duration
	UsedFallback  bool
}

// aiAgentService implements Service
type aiAgentService struct {
	agent                          Agent
	conversationMgr                conversation.Manager
	toolRegistry                   *tools.Registry
	toolExecutor                   *tools.Executor
	executionContext               *tools.ExecutionContext
	featureFlags                   *FeatureFlags
	clinicRepo                     clinic_repository.ClinicRepository
	systemLogSvc                   systemlog_service.SystemLogService
	usageMetrics                   TokenUsageRecorder
	aiTokenUsage                   AITokenUsageRecorder
	defaultMinDelayBetweenMessages int
	// promptHistoryTurns is how many past turns go into the LLM prompt.
	// 0 means "use package default" (DefaultPromptHistoryTurns).
	promptHistoryTurns int
	// agentTemperature is the conversational default; handoffTemperature is
	// used only when the request is in handoff mode (clinic with action tool
	// disabled, AI gathering context for the team). Lower handoff temp keeps
	// the LLM strict about gather rules (call list_procedures, R$ format,
	// don't call complete_handoff_to_human early). Defaults if unset:
	// 0.5/0.3 — see defaultAgentTemperature/defaultHandoffTemperature.
	agentTemperature   float32
	handoffTemperature float32
}

// Defaults applied when SetTemperatures is not called or passes 0. Same values
// the env vars default to (AI_AGENT_TEMPERATURE / AI_AGENT_HANDOFF_TEMPERATURE)
// so behavior is consistent whether the wiring layer remembers to set them.
const (
	defaultAgentTemperature   = 0.5
	defaultHandoffTemperature = 0.3
)

// SetPromptHistoryTurns configures the history window used when building
// the LLM prompt. Wired from the worker config env var
// CONVERSATION_PROMPT_HISTORY_TURNS in main.go.
// Flush drains any pending observability traces in the underlying agent
// (e.g. LangFuse goroutines). Best-effort — no-op when the agent doesn't
// implement Flusher. Call at graceful shutdown so the dashboard isn't
// missing the last few events when SIGTERM hits.
func (s *aiAgentService) Flush() {
	if f, ok := s.agent.(Flusher); ok {
		f.Flush()
	}
}

func (s *aiAgentService) SetPromptHistoryTurns(n int) {
	s.promptHistoryTurns = n
}

// SetTemperatures configures the LLM sampling temperature for the two paths:
// the conversational default (env AI_AGENT_TEMPERATURE) and the stricter
// handoff path (env AI_AGENT_HANDOFF_TEMPERATURE). Either argument <= 0 keeps
// the package default for that slot.
func (s *aiAgentService) SetTemperatures(agentTemp, handoffTemp float32) {
	if agentTemp > 0 {
		s.agentTemperature = agentTemp
	}
	if handoffTemp > 0 {
		s.handoffTemperature = handoffTemp
	}
}

// resolvedTemperature picks the right temperature for this turn — handoff
// path uses the strict value; the normal path uses the conversational one.
// Falls back to package defaults if SetTemperatures was not called.
func (s *aiAgentService) resolvedTemperature(handoff bool) float32 {
	if handoff {
		if s.handoffTemperature > 0 {
			return s.handoffTemperature
		}
		return defaultHandoffTemperature
	}
	if s.agentTemperature > 0 {
		return s.agentTemperature
	}
	return defaultAgentTemperature
}

// FeatureFlags holds feature enablement flags (ops-level kill-switch).
// Per-clinic overrides can only tighten these, never widen them.
type FeatureFlags struct {
	SchedulingEnabled bool
	ConfirmEnabled    bool
	CancelEnabled     bool
	RescheduleEnabled bool
	RemindersEnabled  bool
}

// NewService creates a new AI agent service.
// Both usageMetrics and aiTokenUsage may be nil — accounting paths are
// best-effort and the user-facing reply must not depend on them.
func NewService(
	agent Agent,
	conversationMgr conversation.Manager,
	toolRegistry *tools.Registry,
	toolExecutor *tools.Executor,
	executionContext *tools.ExecutionContext,
	featureFlags *FeatureFlags,
	clinicRepo clinic_repository.ClinicRepository,
	defaultMinDelayBetweenMessages int,
	systemLogSvc systemlog_service.SystemLogService,
	usageMetrics TokenUsageRecorder,
	aiTokenUsage AITokenUsageRecorder,
) Service {
	return &aiAgentService{
		agent:                          agent,
		conversationMgr:                conversationMgr,
		toolRegistry:                   toolRegistry,
		toolExecutor:                   toolExecutor,
		executionContext:               executionContext,
		featureFlags:                   featureFlags,
		clinicRepo:                     clinicRepo,
		systemLogSvc:                   systemLogSvc,
		usageMetrics:                   usageMetrics,
		aiTokenUsage:                   aiTokenUsage,
		defaultMinDelayBetweenMessages: defaultMinDelayBetweenMessages,
	}
}

// ShouldUseAIAgent determines if AI agent should handle this message.
// Only the ops-level kill-switch (FeatureFlags) blocks here — per-clinic
// configuration is enforced inside ProcessMessage via DisallowedTools, which
// hides write tools from the LLM and routes those flows through
// complete_handoff_to_human. Question/Unknown intents always pass.
func (s *aiAgentService) ShouldUseAIAgent(intent model.MessageIntent, clinic *clinic_model.Clinic) bool {
	switch intent {
	case model.MessageIntentQuestion, model.MessageIntentUnknown:
		return true
	case model.MessageIntentSchedule:
		return s.featureFlags.SchedulingEnabled
	case model.MessageIntentConfirm:
		return s.featureFlags.ConfirmEnabled
	case model.MessageIntentCancel:
		return s.featureFlags.CancelEnabled
	case model.MessageIntentReschedule:
		return s.featureFlags.RescheduleEnabled
	}
	return false
}

// defaultBlockedIntentReply is sent when the AI identifies an intent the clinic
// disabled — the message keeps the patient engaged while the clinic takes over.
const defaultBlockedIntentReply = "Pode aguardar um minutinho? Alguém do nosso time vai te atender em breve."

// disabledToolsForClinic returns the blacklist of write tools the clinic
// opted out of (per AITool* flags). Closes the Question/Unknown bypass:
// without this, the AI receives the full toolset on bypass-intent messages
// and can call e.g. quick_schedule even though AIToolSchedule=false. The
// blacklist is applied to every request, regardless of intent or handoff
// mode, so the LLM physically never sees a disabled tool.
func disabledToolsForClinic(clinic *clinic_model.Clinic) []string {
	if clinic == nil {
		return nil
	}
	var disabled []string
	if !clinic.Settings.IsAIToolScheduleEnabled() {
		disabled = append(disabled, "create_appointment", "quick_schedule")
	}
	if !clinic.Settings.IsAIToolConfirmEnabled() {
		disabled = append(disabled, "confirm_appointment")
	}
	if !clinic.Settings.IsAIToolCancelEnabled() {
		disabled = append(disabled, "cancel_appointment")
	}
	if !clinic.Settings.IsAIToolRescheduleEnabled() {
		disabled = append(disabled, "reschedule_appointment", "quick_reschedule")
	}
	if !clinic.Settings.IsAIToolEnrollClassEnabled() {
		disabled = append(disabled, "enroll_patient_in_class")
	}
	if !clinic.Settings.IsAIToolConfirmClassEnabled() {
		disabled = append(disabled, "confirm_class_attendance")
	}
	return disabled
}

// ResolveBlockedIntentReply returns the clinic-configured canned reply, or the
// built-in default when none was set.
func ResolveBlockedIntentReply(s *clinic_model.ClinicSettings) string {
	if s == nil || strings.TrimSpace(s.AIBlockedIntentReply) == "" {
		return defaultBlockedIntentReply
	}
	return s.AIBlockedIntentReply
}

// ProcessMessage processes an incoming message using the AI agent
func (s *aiAgentService) ProcessMessage(ctx context.Context, req *MessageRequest) (*MessageResponse, error) {
	start := time.Now()

	// Get clinic configuration for AI settings
	clinic, err := s.clinicRepo.GetByID(ctx, req.ClinicID)
	if err != nil {
		logger.Printf("[AI Agent Service] Warning: failed to fetch clinic settings, using defaults: %v", err)
		clinic = nil // Will use default settings
	}

	// Get or create conversation context — done before the intent gate so the
	// canned-reply branch can still persist the exchange to conversation
	// history (otherwise subsequent messages would arrive without context).
	conv, err := s.conversationMgr.GetOrCreate(ctx, req.ClinicID, req.PatientPhone, req.PatientName)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create conversation: %w", err)
	}

	// Count AI-handled conversations once per session-start (empty history
	// means GetOrCreate just created it). Both LLM and canned-reply paths
	// count, since both represent a session the AI system handled. The metric
	// powers the "AI conversations" KPI on the admin dashboard.
	if len(conv.History) == 0 && s.usageMetrics != nil {
		_ = s.usageMetrics.Increment(ctx, req.ClinicID, usagemetric_model.MetricAIConversations, 1)
	}

	// Resolve canned reply per-clinic (used by complete_handoff_to_human as
	// the patient-facing message when the LLM transfers to humans). Always
	// computed — the tool can be invoked regardless of intent. Compute the
	// blacklist of tools the clinic disabled too: applied on EVERY request
	// so write tools the clinic opted out of are physically invisible to
	// the LLM, regardless of classified intent.
	var settings *clinic_model.ClinicSettings
	if clinic != nil {
		settings = &clinic.Settings
	}
	blockedReply := ResolveBlockedIntentReply(settings)
	disallowed := disabledToolsForClinic(clinic)
	if len(disallowed) > 0 {
		logger.Printf("[AI Agent Service] Clinic %s has tools disabled: %v — agent runs with blacklist + complete_handoff_to_human fallback", req.ClinicID, disallowed)
	}

	// If the conversation has no patient linked yet, try to resolve it now.
	// The patient may already exist (created by the ensurer on first contact).
	if conv.PatientID == nil && s.executionContext.PatientFinderCreator != nil {
		patient, findErr := s.executionContext.PatientFinderCreator.FindOrCreateByPhone(
			ctx, req.ClinicID, req.PatientPhone, req.PatientName,
		)
		if findErr != nil {
			logger.Printf("[AI Agent Service] Warning: failed to resolve patient for %s: %v", req.PatientPhone, findErr)
		} else if patient != nil {
			conv.PatientID = &patient.ID
			if err := s.conversationMgr.SetPatientID(ctx, conv.ID, patient.ID); err != nil {
				logger.Printf("[AI Agent Service] Warning: failed to persist patient ID on conversation: %v", err)
			}
		}
	}

	// Update execution context with current request details
	s.executionContext.Context = ctx
	s.executionContext.ClinicID = req.ClinicID
	s.executionContext.PatientPhone = req.PatientPhone
	s.executionContext.PatientID = conv.PatientID
	s.executionContext.PatientName = req.PatientName
	s.executionContext.ConversationID = conv.ID
	// list_procedures uses this to persist the active filter snapshot
	// (last_procedure_filter), which the prompt builder reads next turn to
	// hint the LLM about the recent topic — fixes "AI ignores RPG context
	// when patient says 'quero agendar avaliação' right after".
	s.executionContext.ConversationMetadata = s.conversationMgr
	// BlockedIntentReply is read by complete_handoff_to_human when it returns
	// the patient-facing message; it stays empty in the normal flow.
	s.executionContext.BlockedIntentReply = blockedReply

	// Add user message to conversation history
	if err := s.conversationMgr.AddUserMessage(ctx, conv.ID, req.MessageText); err != nil {
		logger.Printf("[AI Agent Service] Warning: failed to add user message to history: %v", err)
	}

	// Update conversation state if intent is identified
	if req.Intent != model.MessageIntentUnknown && req.Intent != "" {
		state := s.mapIntentToState(req.Intent)
		if err := s.conversationMgr.UpdateState(ctx, conv.ID, state, req.Intent); err != nil {
			logger.Printf("[AI Agent Service] Warning: failed to update conversation state: %v", err)
		}
	}

	// Prepare agent request. Temperature is stricter when any clinic tool is
	// disabled — those clinics rely on the LLM following the handoff fallback
	// rule (call complete_handoff_to_human instead of write tools), which
	// needs lower temperature to be reliable.
	agentReq := &AgentRequest{
		ConversationContext: conv,
		UserMessage:         req.MessageText,
		ClinicName:          req.ClinicName,
		MaxTokens:           2048,
		Temperature:         s.resolvedTemperature(len(disallowed) > 0),
		HistoryTurns:        s.promptHistoryTurns,
		BlockedIntentReply:  blockedReply,
		DisallowedTools:     disallowed,
	}
	if clinic != nil {
		if agentReq.ClinicName == "" {
			agentReq.ClinicName = clinic.Name
		}
		agentReq.ClinicBusinessInfo = clinic.BusinessInfo
		agentReq.ClinicFAQ = clinic.FAQ
	}

	// Generate AI response
	agentResp, err := s.agent.Generate(ctx, agentReq)
	if err != nil {
		// Persist generation failure to system_logs so the failure mode is
		// debuggable from the DB even after the backend container restarts
		// and stdout logs are lost. The error string from openrouter_agent
		// already includes finish_reason / tool_call count / token usage —
		// preserving it whole is more useful than rewriting.
		if s.systemLogSvc != nil {
			_ = s.systemLogSvc.Log(ctx, systemlog_service.SystemLogEntry{
				ClinicID: &req.ClinicID,
				Service:  "ai_agent",
				Event:    "ai_agent.generation_failed",
				Status:   systemlog_model.LogStatusFailed,
				Error:    err.Error(),
				Payload: map[string]interface{}{
					"intent":          string(req.Intent),
					"patient_phone":   req.PatientPhone,
					"conversation_id": conv.ID.String(),
					"provider":        s.agent.Provider(),
					"model":           s.agent.Model(),
					"history_turns":   len(conv.History),
					"disallowed_tools": disallowed,
				},
			})
		}
		return nil, fmt.Errorf("AI agent generation failed: %w", err)
	}

	// Persist PatientID to conversation if it was resolved during tool execution
	if s.executionContext.PatientID != nil && conv.PatientID == nil {
		if err := s.conversationMgr.SetPatientID(ctx, conv.ID, *s.executionContext.PatientID); err != nil {
			logger.Printf("[AI Agent Service] Warning: failed to set patient ID on conversation: %v", err)
		}
	}

	// Add assistant message to conversation history
	if err := s.conversationMgr.AddAssistantMessage(ctx, conv.ID, agentResp.Content, agentResp.ToolCalls); err != nil {
		logger.Printf("[AI Agent Service] Warning: failed to add assistant message to history: %v", err)
	}

	// Hallucination warning: when the assistant message includes
	// matrícula-style fields (Horários: / Mensalidade: / *Modalidade:*)
	// without having called present_enrollment_summary in this turn, the
	// LLM is composing the resumé as free-text and probably inventing
	// data. Surface as a warning to system_logs so an admin sees the
	// alert without having to read transcripts. Detection-only — does
	// NOT block the message; production safety is on the tool layer.
	s.maybeLogResumeHallucinationWarning(ctx, req.ClinicID, conv.ID, agentResp.Content, agentResp.ToolCalls)

	// Log tool calls for monitoring
	for _, toolCall := range agentResp.ToolCalls {
		duration := time.Duration(0)
		if toolCall.ExecutedAt != nil {
			duration = time.Since(*toolCall.ExecutedAt)
		}

		var toolErr error
		if toolCall.Error != "" {
			toolErr = fmt.Errorf("%s", toolCall.Error)
		}

		if logErr := s.conversationMgr.LogToolCall(
			ctx,
			conv.ID,
			toolCall.Name,
			toolCall.Arguments,
			toolCall.Result,
			toolErr,
			duration,
		); logErr != nil {
			logger.Printf("[AI Agent Service] Warning: failed to log tool call: %v", logErr)

			// Log to system_logs so failures are visible
			if s.systemLogSvc != nil {
				_ = s.systemLogSvc.Log(ctx, systemlog_service.SystemLogEntry{
					ClinicID: &req.ClinicID,
					Service:  "ai_agent",
					Event:    "tool_call_log_failed",
					Status:   "failed",
					Payload: map[string]interface{}{
						"tool_name":       toolCall.Name,
						"conversation_id": conv.ID.String(),
						"log_error":       logErr.Error(),
					},
					Error: logErr.Error(),
				})
			}
		}

		// Also log tool execution errors to system_logs
		if toolErr != nil && s.systemLogSvc != nil {
			_ = s.systemLogSvc.Log(ctx, systemlog_service.SystemLogEntry{
				ClinicID: &req.ClinicID,
				Service:  "ai_agent",
				Event:    "tool_call_error",
				Status:   "failed",
				Payload: map[string]interface{}{
					"tool_name":       toolCall.Name,
					"conversation_id": conv.ID.String(),
					"arguments":       toolCall.Arguments,
				},
				Error:    toolCall.Error,
				Duration: int(duration.Milliseconds()),
			})
		}
	}

	// Segment the response for natural message delivery
	segments := s.segmentResponse(agentResp.Content, clinic)

	// Build message response
	msgResp := &MessageResponse{
		ResponseText:  agentResp.Content,
		MessageType:   model.MessageTypeText,
		Segments:      segments,
		ToolCallCount: len(agentResp.ToolCalls),
		TokensUsed:    agentResp.TokensUsed,
		Latency:       time.Since(start),
		UsedFallback:  false, // TODO: Track fallback usage
	}

	// Persist per-clinic AI token consumption. Both recorders are best-effort
	// and must not fail the user-facing reply. The aggregate (usage_metrics)
	// powers daily KPIs; the granular event (ai_token_usage) breaks consumption
	// down by model/provider so we can attribute cost per clinic per model later.
	if msgResp.TokensUsed > 0 {
		if s.usageMetrics != nil {
			_ = s.usageMetrics.Increment(ctx, req.ClinicID, usagemetric_model.MetricAITokensUsed, float64(msgResp.TokensUsed))
		}
		if s.aiTokenUsage != nil {
			_ = s.aiTokenUsage.Record(ctx, AITokenUsageEvent{
				ClinicID:       req.ClinicID,
				ConversationID: conv.ID,
				Provider:       agentResp.Provider,
				Model:          agentResp.Model,
				InputTokens:    agentResp.InputTokens,
				OutputTokens:   agentResp.OutputTokens,
				TotalTokens:    msgResp.TokensUsed,
			})
		}
	}

	// Surface primary-model fallbacks as a warning in system_logs so we can
	// detect silent degradation (e.g. gpt-4o-mini failing and traffic quietly
	// shifting to a Gemini fallback). Status=Warning because the user still
	// got an answer — only the cost/quality profile changed.
	if agentResp.PrimaryModelFailed && s.systemLogSvc != nil {
		clinicID := req.ClinicID
		_ = s.systemLogSvc.Log(ctx, systemlog_service.SystemLogEntry{
			ClinicID: &clinicID,
			Service:  "ai_agent",
			Event:    "ai_agent.primary_model_fallback",
			Status:   systemlog_model.LogStatusWarning,
			Error:    agentResp.PrimaryModelError,
			Payload: map[string]interface{}{
				"primary_model":   agentResp.PrimaryModel,
				"fallback_model":  agentResp.Model,
				"provider":        agentResp.Provider,
				"conversation_id": conv.ID.String(),
				"tokens_used":     msgResp.TokensUsed,
			},
		})
	}

	logger.Printf("[AI Agent Service] Processed message in %v, %d tools called, %d tokens used",
		msgResp.Latency, msgResp.ToolCallCount, msgResp.TokensUsed)

	return msgResp, nil
}

// mapIntentToState maps a message intent to a conversation state
func (s *aiAgentService) mapIntentToState(intent model.MessageIntent) conversation.ConversationState {
	switch intent {
	case model.MessageIntentSchedule:
		return conversation.ConversationStateScheduling
	case model.MessageIntentConfirm:
		return conversation.ConversationStateAwaitingConfirm
	case model.MessageIntentCancel:
		return conversation.ConversationStateCancelling
	case model.MessageIntentReschedule:
		return conversation.ConversationStateRescheduling
	case model.MessageIntentQuestion:
		return conversation.ConversationStateQuestionAnswering
	default:
		return conversation.ConversationStateIntentIdentified
	}
}

// segmentResponse segments a long AI response into natural message chunks
// This creates a more human-like conversation flow with appropriate delays
func (s *aiAgentService) segmentResponse(response string, clinic *clinic_model.Clinic) []*model.MessageSegment {
	segments := []*model.MessageSegment{}

	// Split response into lines
	lines := strings.Split(strings.TrimSpace(response), "\n")
	if len(lines) == 0 {
		return segments
	}

	var greetingLines []string
	var contentLines []string
	var questionLines []string

	// Phase 1: Detect greeting at the beginning
	inGreeting := true
	inContent := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Empty lines
		if trimmed == "" {
			if inGreeting && len(greetingLines) > 0 {
				// End of greeting section
				inGreeting = false
				inContent = true
			} else if inContent {
				contentLines = append(contentLines, line)
			}
			continue
		}

		// Check if line is a greeting (first non-empty lines)
		if inGreeting {
			// Greetings are usually short (< 50 chars) and at the start
			// Examples: "Olá!", "Bom dia!", "Olá! Como posso ajudar?"
			if len(greetingLines) == 0 && len(trimmed) < 80 && s.looksLikeGreeting(trimmed) {
				greetingLines = append(greetingLines, line)
			} else {
				// Not a greeting, start content
				inGreeting = false
				inContent = true
				contentLines = append(contentLines, line)
			}
		} else if inContent {
			contentLines = append(contentLines, line)
		}
	}

	// Phase 2: Extract questions from the end of content
	for len(contentLines) > 0 {
		lastLine := strings.TrimSpace(contentLines[len(contentLines)-1])

		// Check if last line is a question
		if lastLine == "" {
			// Remove empty line and continue
			contentLines = contentLines[:len(contentLines)-1]
			continue
		}

		if s.looksLikeQuestion(lastLine) {
			// Move to question section
			questionLines = append([]string{contentLines[len(contentLines)-1]}, questionLines...)
			contentLines = contentLines[:len(contentLines)-1]
		} else {
			// No more questions
			break
		}
	}

	// Phase 3: Build segments

	// 1. Greeting segment (if exists)
	if len(greetingLines) > 0 {
		greetingText := strings.Join(greetingLines, "\n")
		segments = append(segments, &model.MessageSegment{
			Content:    greetingText,
			Type:       model.SegmentTypeGreeting,
			DelayAfter: 3 * time.Second, // Fixed 3s delay after greeting
		})
	}

	// 2. Content segment (if exists)
	if len(contentLines) > 0 {
		contentText := strings.TrimSpace(strings.Join(contentLines, "\n"))
		if contentText != "" {
			segments = append(segments, &model.MessageSegment{
				Content:    contentText,
				Type:       model.SegmentTypeContent,
				DelayAfter: s.calculateDelayByLength(len(contentText), clinic),
			})
		}
	}

	// 3. Question segment (if exists)
	if len(questionLines) > 0 {
		questionText := strings.TrimSpace(strings.Join(questionLines, "\n"))
		if questionText != "" {
			segments = append(segments, &model.MessageSegment{
				Content:    questionText,
				Type:       model.SegmentTypeQuestion,
				DelayAfter: 0, // No delay after question (last message)
			})
		}
	}

	// If no segments were created, return the whole response as content
	if len(segments) == 0 {
		segments = append(segments, &model.MessageSegment{
			Content:    response,
			Type:       model.SegmentTypeContent,
			DelayAfter: 0,
		})
	}

	return segments
}

// looksLikeGreeting checks if a line looks like a greeting
func (s *aiAgentService) looksLikeGreeting(line string) bool {
	lower := strings.ToLower(line)

	// Common Portuguese greetings
	greetingPrefixes := []string{
		"olá", "oi", "bom dia", "boa tarde", "boa noite",
		"seja bem-vindo", "bem-vindo", "tudo bem",
	}

	for _, prefix := range greetingPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	return false
}

// looksLikeQuestion checks if a line looks like a question
func (s *aiAgentService) looksLikeQuestion(line string) bool {
	trimmed := strings.TrimSpace(line)

	// Ends with question mark
	if strings.HasSuffix(trimmed, "?") {
		return true
	}

	// Common question starters in Portuguese
	lower := strings.ToLower(trimmed)
	questionStarters := []string{
		"qual", "quais", "quando", "onde", "como", "por que", "porque",
		"gostaria", "prefere", "deseja", "quer", "precisa",
		"posso ajudar", "posso auxiliar",
	}

	for _, starter := range questionStarters {
		if strings.HasPrefix(lower, starter) {
			return true
		}
	}

	return false
}

// calculateDelayByLength calculates delay based on message length
// Formula: baseDelay (from clinic or env) + 1s per 100 characters, Max: 10s
func (s *aiAgentService) calculateDelayByLength(messageLength int, clinic *clinic_model.Clinic) time.Duration {
	// Use delay from clinic settings (overrides env default)
	baseDelaySeconds := s.defaultMinDelayBetweenMessages
	if clinic != nil {
		baseDelaySeconds = clinic.GetMinDelayBetweenMessages(s.defaultMinDelayBetweenMessages)
	}
	baseDelay := time.Duration(baseDelaySeconds) * time.Second

	const maxDelay = 10 * time.Second
	const charsPerSecond = 100

	additionalDelay := time.Duration(messageLength/charsPerSecond) * time.Second
	totalDelay := baseDelay + additionalDelay

	if totalDelay > maxDelay {
		return maxDelay
	}
	return totalDelay
}

// resumeFieldPattern matches the matrícula-style bold field labels the AI
// is supposed to emit ONLY via present_enrollment_summary. If we see the
// pattern in free-text content without a corresponding tool call this turn,
// it's a hallucination signal worth surfacing on the dashboard.
var resumeFieldPattern = regexp.MustCompile(`(?i)\*?(Modalidade|Frequência|Horários|Mensalidade)\*?:`)

// maybeLogResumeHallucinationWarning checks whether the assistant just
// produced a matrícula resumé without going through present_enrollment_summary.
// Best-effort detection — never blocks the message, just emits a warning.
func (s *aiAgentService) maybeLogResumeHallucinationWarning(
	ctx context.Context,
	clinicID, conversationID uuid.UUID,
	content string,
	toolCalls []conversation.ToolCall,
) {
	if s.systemLogSvc == nil || content == "" {
		return
	}
	if !resumeFieldPattern.MatchString(content) {
		return
	}
	// If the same turn called present_enrollment_summary, the resumé is
	// legit (LLM is echoing the server-built summary_text). No warning.
	for _, tc := range toolCalls {
		if tc.Name == "present_enrollment_summary" {
			return
		}
	}
	cid := clinicID
	_ = s.systemLogSvc.Log(ctx, systemlog_service.SystemLogEntry{
		ClinicID: &cid,
		Service:  "ai_agent_service",
		Event:    "resume_hallucination_suspected",
		Status:   systemlog_model.LogStatusWarning,
		Payload: map[string]any{
			"conversation_id": conversationID.String(),
			"snippet":         truncateForLog(content, 240),
		},
	})
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
