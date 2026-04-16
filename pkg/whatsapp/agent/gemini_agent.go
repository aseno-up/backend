package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aseno/backend/internal/logger"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/aseno/backend/pkg/whatsapp/conversation"
	"github.com/aseno/backend/pkg/whatsapp/tools"
)

// geminiAgent implements Agent using Google's Gemini API
type geminiAgent struct {
	client         *genai.Client
	model          string
	fallbackModels []string
	timeout        time.Duration
	toolRegistry   *tools.Registry
	toolExecutor   *tools.Executor
	execContext    *tools.ExecutionContext
}

// NewGeminiAgent creates a new Gemini-based agent
func NewGeminiAgent(cfg Config, toolRegistry *tools.Registry, toolExecutor *tools.Executor, execContext *tools.ExecutionContext) (Agent, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Default model if not specified
	model := cfg.Model
	if model == "" {
		model = "gemini-2.0-flash-exp"
	}

	// Default fallback models
	fallbackModels := cfg.FallbackModels
	if len(fallbackModels) == 0 {
		fallbackModels = []string{
			"gemini-2.0-flash",
			"gemini-2.5-flash-lite",
			"gemini-1.5-flash",
			"gemini-2.5-flash",
		}
	}

	// Default timeout
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	logger.Printf("[Gemini Agent] Initialized with model: %s (fallbacks: %v)", model, fallbackModels)

	return &geminiAgent{
		client:         client,
		model:          model,
		fallbackModels: fallbackModels,
		timeout:        timeout,
		toolRegistry:   toolRegistry,
		toolExecutor:   toolExecutor,
		execContext:    execContext,
	}, nil
}

func (a *geminiAgent) Provider() string {
	return "gemini"
}

func (a *geminiAgent) Model() string {
	return a.model
}

// Generate generates a response for the given request
func (a *geminiAgent) Generate(ctx context.Context, req *AgentRequest) (*AgentResponse, error) {
	start := time.Now()

	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// Try with primary model first
	resp, err := a.generateWithModel(ctx, req, a.model)
	if err != nil {
		logger.Printf("[Gemini Agent] Primary model (%s) failed: %v", a.model, err)

		// Try each fallback model in sequence
		for i, fallbackModel := range a.fallbackModels {
			logger.Printf("[Gemini Agent] Trying fallback model %d/%d: %s", i+1, len(a.fallbackModels), fallbackModel)
			resp, err = a.generateWithModel(ctx, req, fallbackModel)
			if err == nil {
				// Success with fallback
				logger.Printf("[Gemini Agent] Success with fallback model: %s", fallbackModel)
				resp.Latency = time.Since(start)
				return resp, nil
			}
			logger.Printf("[Gemini Agent] Fallback model %s failed: %v", fallbackModel, err)
		}

		// All models failed
		return nil, fmt.Errorf("all models failed (primary + %d fallbacks): %w", len(a.fallbackModels), err)
	}

	resp.Latency = time.Since(start)
	return resp, nil
}

// generateWithModel generates a response using a specific model
func (a *geminiAgent) generateWithModel(ctx context.Context, req *AgentRequest, modelName string) (*AgentResponse, error) {
	// Create generative model
	genModel := a.client.GenerativeModel(modelName)

	// Configure model
	genModel.SetTemperature(req.Temperature)
	if req.Temperature == 0 {
		genModel.SetTemperature(0.7) // Default temperature
	}

	if req.MaxTokens > 0 {
		genModel.SetMaxOutputTokens(int32(req.MaxTokens))
	} else {
		genModel.SetMaxOutputTokens(2048) // Default max tokens
	}

	// Configure function calling (tools)
	functionDeclarations := a.toolRegistry.GetGeminiFunctionDeclarations()
	genModel.Tools = []*genai.Tool{
		{
			FunctionDeclarations: functionDeclarations,
		},
	}

	// Build system prompt
	promptBuilder := NewSystemPromptBuilder(
		req.ClinicName,
		req.ConversationContext.PatientName,
		req.ConversationContext,
	).
		WithBusinessInfo(req.ClinicBusinessInfo).
		WithFAQ(req.ClinicFAQ)
	systemPrompt := promptBuilder.Build()

	// Build user prompt with conversation history
	userPrompt := BuildUserPrompt(req.ConversationContext, req.UserMessage)

	// Start chat session
	chat := genModel.StartChat()
	chat.History = []*genai.Content{}

	// Add system instruction via first message
	chat.History = append(chat.History, &genai.Content{
		Parts: []genai.Part{
			genai.Text(systemPrompt),
		},
		Role: "model",
	})

	// Track tool calls made during this turn
	var allToolCalls []conversation.ToolCall

	// Send message and handle tool calls in a loop (multi-turn function calling)
	currentPrompt := userPrompt
	maxIterations := 5 // Prevent infinite loops
	var finalResponse *genai.GenerateContentResponse

	for i := 0; i < maxIterations; i++ {
		resp, err := chat.SendMessage(ctx, genai.Text(currentPrompt))
		if err != nil {
			return nil, fmt.Errorf("gemini generation failed: %w", err)
		}

		finalResponse = resp

		// Check if there are function calls
		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			return nil, fmt.Errorf("empty response from Gemini")
		}

		// Look for function calls in the response
		var functionCalls []*genai.FunctionCall
		for _, part := range resp.Candidates[0].Content.Parts {
			if fc, ok := part.(genai.FunctionCall); ok {
				functionCalls = append(functionCalls, &fc)
			}
		}

		// If no function calls, we're done
		if len(functionCalls) == 0 {
			break
		}

		// Execute function calls and prepare responses
		var functionResponses []genai.Part
		for _, fc := range functionCalls {
			result, toolErr := a.executeFunctionCall(ctx, fc)

			// Track tool call
			toolCall := conversation.ToolCall{
				ID:        fc.Name, // Use function name as ID for now
				Name:      fc.Name,
				Arguments: fc.Args,
				Result:    result,
			}

			if toolErr != nil {
				toolCall.Error = toolErr.Error()
				logger.Printf("[Gemini Agent] Tool %s failed: %v", fc.Name, toolErr)
			}

			now := time.Now()
			toolCall.ExecutedAt = &now
			allToolCalls = append(allToolCalls, toolCall)

			// Create function response for Gemini
			// Convert result to JSON-compatible format for Gemini protobuf
			var resultData interface{} = result
			if result != nil {
				// Marshal to JSON and back to convert complex types to simple maps/slices
				jsonData, jsonErr := json.Marshal(result)
				if jsonErr == nil {
					var simpleResult interface{}
					if err := json.Unmarshal(jsonData, &simpleResult); err == nil {
						resultData = simpleResult
					}
				}
			}

			functionResponse := genai.FunctionResponse{
				Name: fc.Name,
				Response: map[string]interface{}{
					"result": resultData,
					"error":  toolCall.Error,
				},
			}

			functionResponses = append(functionResponses, functionResponse)
		}

		// Send function responses back to model and get the response in one call
		currentPrompt = "" // Clear prompt for subsequent iterations
		finalResp, err := chat.SendMessage(ctx, functionResponses...)
		if err != nil {
			return nil, fmt.Errorf("failed to send function responses: %w", err)
		}

		finalResponse = finalResp

		// Check if there are more function calls (iterative)
		hasMoreCalls := false
		for _, part := range finalResponse.Candidates[0].Content.Parts {
			if _, ok := part.(genai.FunctionCall); ok {
				hasMoreCalls = true
				break
			}
		}

		if !hasMoreCalls {
			break
		}
	}

	// Extract text response
	var responseText string
	if len(finalResponse.Candidates) > 0 && len(finalResponse.Candidates[0].Content.Parts) > 0 {
		for _, part := range finalResponse.Candidates[0].Content.Parts {
			if text, ok := part.(genai.Text); ok {
				responseText += string(text)
			}
		}
	}

	// Validate response is not empty (trim whitespace to check)
	trimmedResponse := strings.TrimSpace(responseText)
	if trimmedResponse == "" {
		return nil, fmt.Errorf("empty or whitespace-only response from Gemini (tool calls: %d)", len(allToolCalls))
	}

	// Calculate tokens used (approximate)
	tokensUsed := 0
	if finalResponse.UsageMetadata != nil {
		tokensUsed = int(finalResponse.UsageMetadata.TotalTokenCount)
	}

	// Build agent response
	agentResp := &AgentResponse{
		Content:      responseText,
		ToolCalls:    allToolCalls,
		FinishReason: string(finalResponse.Candidates[0].FinishReason),
		TokensUsed:   tokensUsed,
		Model:        modelName,
		Provider:     "gemini",
	}

	logger.Printf("[Gemini Agent] Generated response with %d tool calls, %d tokens, model: %s",
		len(allToolCalls), tokensUsed, modelName)

	return agentResp, nil
}

// executeFunctionCall executes a single function call
func (a *geminiAgent) executeFunctionCall(ctx context.Context, fc *genai.FunctionCall) (interface{}, error) {
	logger.Printf("[Gemini Agent] Executing tool: %s with args: %v", fc.Name, fc.Args)

	// Convert Args (map[string]any) to map[string]interface{}
	params := make(map[string]interface{})
	for k, v := range fc.Args {
		params[k] = v
	}

	// Update execution context
	a.execContext.Context = ctx

	// Execute tool
	result, err := a.toolExecutor.Execute(a.execContext, fc.Name, params)
	if err != nil {
		logger.Printf("[Gemini Agent] Tool %s execution error: %v", fc.Name, err)
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, err
	}

	logger.Printf("[Gemini Agent] Tool %s executed successfully", fc.Name)
	return result, nil
}

// Close closes the Gemini client
func (a *geminiAgent) Close() error {
	if a.client != nil {
		return a.client.Close()
	}
	return nil
}

// Helper function to format tool results for logging
func formatToolResult(result interface{}) string {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("%v", result)
	}
	return string(data)
}
