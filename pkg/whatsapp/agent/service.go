package agent

import (
	"context"
	"fmt"
	"github.com/aseno/backend/internal/logger"
	"strings"
	"time"

	"github.com/google/uuid"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/whatsapp/conversation"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/tools"
)

// Service handles AI-driven WhatsApp message processing
type Service interface {
	// ProcessMessage processes an incoming message using AI agent
	ProcessMessage(ctx context.Context, req *MessageRequest) (*MessageResponse, error)

	// ShouldUseAIAgent determines if AI agent should handle this message
	ShouldUseAIAgent(intent model.MessageIntent) bool
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
	defaultMinDelayBetweenMessages int
}

// FeatureFlags holds feature enablement flags
type FeatureFlags struct {
	SchedulingEnabled bool
	ConfirmEnabled    bool
	CancelEnabled     bool
	RemindersEnabled  bool
}

// NewService creates a new AI agent service
func NewService(
	agent Agent,
	conversationMgr conversation.Manager,
	toolRegistry *tools.Registry,
	toolExecutor *tools.Executor,
	executionContext *tools.ExecutionContext,
	featureFlags *FeatureFlags,
	clinicRepo clinic_repository.ClinicRepository,
	defaultMinDelayBetweenMessages int,
) Service {
	return &aiAgentService{
		agent:                          agent,
		conversationMgr:                conversationMgr,
		toolRegistry:                   toolRegistry,
		toolExecutor:                   toolExecutor,
		executionContext:               executionContext,
		featureFlags:                   featureFlags,
		clinicRepo:                     clinicRepo,
		defaultMinDelayBetweenMessages: defaultMinDelayBetweenMessages,
	}
}

// ShouldUseAIAgent determines if AI agent should handle this message based on feature flags
func (s *aiAgentService) ShouldUseAIAgent(intent model.MessageIntent) bool {
	switch intent {
	case model.MessageIntentSchedule:
		return s.featureFlags.SchedulingEnabled
	case model.MessageIntentConfirm:
		return s.featureFlags.ConfirmEnabled
	case model.MessageIntentCancel:
		return s.featureFlags.CancelEnabled
	case model.MessageIntentReschedule:
		return s.featureFlags.CancelEnabled // Use same flag as cancel
	case model.MessageIntentQuestion:
		return true // Always use AI for questions if agent is enabled
	case model.MessageIntentUnknown:
		return true // Let AI figure out what the user wants
	default:
		return false
	}
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

	// Get or create conversation context
	conv, err := s.conversationMgr.GetOrCreate(ctx, req.ClinicID, req.PatientPhone, req.PatientName)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create conversation: %w", err)
	}

	// Update execution context with current request details
	s.executionContext.Context = ctx
	s.executionContext.ClinicID = req.ClinicID
	s.executionContext.PatientPhone = req.PatientPhone
	s.executionContext.PatientID = conv.PatientID
	s.executionContext.PatientName = req.PatientName
	s.executionContext.ConversationID = conv.ID

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

	// Prepare agent request
	agentReq := &AgentRequest{
		ConversationContext: conv,
		UserMessage:         req.MessageText,
		ClinicName:          req.ClinicName,
		MaxTokens:           2048,
		Temperature:         0.7,
	}

	// Generate AI response
	agentResp, err := s.agent.Generate(ctx, agentReq)
	if err != nil {
		return nil, fmt.Errorf("AI agent generation failed: %w", err)
	}

	// Add assistant message to conversation history
	if err := s.conversationMgr.AddAssistantMessage(ctx, conv.ID, agentResp.Content, agentResp.ToolCalls); err != nil {
		logger.Printf("[AI Agent Service] Warning: failed to add assistant message to history: %v", err)
	}

	// Log tool calls for monitoring
	for _, toolCall := range agentResp.ToolCalls {
		duration := time.Duration(0)
		if toolCall.ExecutedAt != nil {
			duration = time.Since(*toolCall.ExecutedAt)
		}

		var err error
		if toolCall.Error != "" {
			err = fmt.Errorf("%s", toolCall.Error)
		}

		if logErr := s.conversationMgr.LogToolCall(
			ctx,
			conv.ID,
			toolCall.Name,
			toolCall.Arguments,
			toolCall.Result,
			err,
			duration,
		); logErr != nil {
			logger.Printf("[AI Agent Service] Warning: failed to log tool call: %v", logErr)
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
