package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aseno/backend/internal/logger"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aseno/backend/pkg/whatsapp/conversation"
	"github.com/aseno/backend/pkg/whatsapp/tools"
)

// openRouterAgent implements Agent using OpenRouter API (OpenAI-compatible)
type openRouterAgent struct {
	apiKey         string
	baseURL        string
	model          string
	fallbackModels []string
	timeout        time.Duration
	httpClient     *http.Client
	toolRegistry   *tools.Registry
	toolExecutor   *tools.Executor
	execContext    *tools.ExecutionContext
}

// NewOpenRouterAgent creates a new OpenRouter-based agent
func NewOpenRouterAgent(cfg Config, toolRegistry *tools.Registry, toolExecutor *tools.Executor, execContext *tools.ExecutionContext) (Agent, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY is required")
	}

	// Default model if not specified
	model := cfg.Model
	if model == "" {
		model = "meta-llama/llama-3.1-8b-instruct"
	}

	// Default fallback models
	fallbackModels := cfg.FallbackModels
	if len(fallbackModels) == 0 {
		fallbackModels = []string{
			"mistralai/mistral-7b-instruct",
			"google/gemma-7b-it",
			"qwen/qwen-2.5-7b-instruct",
		}
	}

	// Default timeout
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: timeout,
	}

	logger.Printf("[OpenRouter Agent] Initialized with model: %s (fallbacks: %v)", model, fallbackModels)

	return &openRouterAgent{
		apiKey:         cfg.APIKey,
		baseURL:        "https://openrouter.ai/api/v1",
		model:          model,
		fallbackModels: fallbackModels,
		timeout:        timeout,
		httpClient:     httpClient,
		toolRegistry:   toolRegistry,
		toolExecutor:   toolExecutor,
		execContext:    execContext,
	}, nil
}

func (a *openRouterAgent) Provider() string {
	return "openrouter"
}

func (a *openRouterAgent) Model() string {
	return a.model
}

// Generate generates a response for the given request
func (a *openRouterAgent) Generate(ctx context.Context, req *AgentRequest) (*AgentResponse, error) {
	start := time.Now()

	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// Try with primary model first
	resp, err := a.generateWithModel(ctx, req, a.model)
	if err != nil {
		primaryErr := err
		logger.Printf("[OpenRouter Agent] Primary model (%s) failed: %v", a.model, primaryErr)

		// Try each fallback model in sequence
		for i, fallbackModel := range a.fallbackModels {
			logger.Printf("[OpenRouter Agent] Trying fallback model %d/%d: %s", i+1, len(a.fallbackModels), fallbackModel)
			resp, err = a.generateWithModel(ctx, req, fallbackModel)
			if err == nil {
				// Success with fallback — mark the response so the agent service
				// can emit a system_log warning for observability.
				logger.Printf("[OpenRouter Agent] Success with fallback model: %s", fallbackModel)
				resp.PrimaryModelFailed = true
				resp.PrimaryModel = a.model
				resp.PrimaryModelError = primaryErr.Error()
				resp.Latency = time.Since(start)
				return resp, nil
			}
			logger.Printf("[OpenRouter Agent] Fallback model %s failed: %v", fallbackModel, err)
		}

		// All models failed
		return nil, fmt.Errorf("all models failed (primary + %d fallbacks): %w", len(a.fallbackModels), err)
	}

	resp.Latency = time.Since(start)
	return resp, nil
}

// OpenRouter API types
type openRouterRequest struct {
	Model       string                 `json:"model"`
	Messages    []openRouterMessage    `json:"messages"`
	Tools       []tools.OpenRouterTool `json:"tools,omitempty"`
	Temperature float32                `json:"temperature,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
}

type openRouterMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content,omitempty"`
	ToolCalls  []openRouterToolCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type openRouterToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openRouterFunctionCall `json:"function"`
}

type openRouterFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openRouterResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Choices []openRouterChoice `json:"choices"`
	Usage   openRouterUsage    `json:"usage"`
	Error   *openRouterError   `json:"error,omitempty"`
}

type openRouterChoice struct {
	Index        int               `json:"index"`
	Message      openRouterMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type openRouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openRouterError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// generateWithModel generates a response using a specific model
func (a *openRouterAgent) generateWithModel(ctx context.Context, req *AgentRequest, modelName string) (*AgentResponse, error) {
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
	userPrompt := BuildUserPromptWithLimit(req.ConversationContext, req.UserMessage, req.HistoryTurns)

	// Initialize messages with system prompt
	messages := []openRouterMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	// Track tool calls made during this turn
	var allToolCalls []conversation.ToolCall

	// Get tools in OpenRouter format
	toolDeclarations := a.toolRegistry.GetOpenRouterFunctionDeclarations()

	// Handle multi-turn function calling
	maxIterations := 5
	var finalResponse *openRouterResponse

	for i := 0; i < maxIterations; i++ {
		// Build request
		apiReq := openRouterRequest{
			Model:    modelName,
			Messages: messages,
			Tools:    toolDeclarations,
		}

		// Set temperature
		if req.Temperature > 0 {
			apiReq.Temperature = req.Temperature
		} else {
			apiReq.Temperature = 0.7
		}

		// Set max tokens
		if req.MaxTokens > 0 {
			apiReq.MaxTokens = req.MaxTokens
		} else {
			apiReq.MaxTokens = 2048
		}

		// Call OpenRouter API
		resp, err := a.callAPI(ctx, apiReq)
		if err != nil {
			return nil, fmt.Errorf("openrouter API call failed: %w", err)
		}

		finalResponse = resp

		// Check for errors in response
		if resp.Error != nil {
			return nil, fmt.Errorf("openrouter error: %s (type: %s, code: %s)", resp.Error.Message, resp.Error.Type, resp.Error.Code)
		}

		// Check for empty response
		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("empty response from OpenRouter")
		}

		choice := resp.Choices[0]

		// Check if there are tool calls
		if len(choice.Message.ToolCalls) == 0 {
			// No tool calls, we're done
			break
		}

		// Add assistant message with tool calls to history
		messages = append(messages, choice.Message)

		// Execute tool calls
		for _, toolCall := range choice.Message.ToolCalls {
			if toolCall.Type != "function" {
				continue
			}

			// Parse arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				logger.Printf("[OpenRouter Agent] Failed to parse tool arguments: %v", err)
				continue
			}

			// Execute tool
			result, toolErr := a.executeFunctionCall(ctx, toolCall.Function.Name, args)

			// Track tool call
			tc := conversation.ToolCall{
				ID:        toolCall.ID,
				Name:      toolCall.Function.Name,
				Arguments: args,
				Result:    result,
			}

			if toolErr != nil {
				tc.Error = toolErr.Error()
				logger.Printf("[OpenRouter Agent] Tool %s failed: %v", toolCall.Function.Name, toolErr)

				// Add structured error result
				result = map[string]interface{}{
					"success": false,
					"error":   toolErr.Error(),
					"message": "Tool execution failed - do not invent data",
				}
			}

			now := time.Now()
			tc.ExecutedAt = &now
			allToolCalls = append(allToolCalls, tc)

			// Add tool result to messages
			resultJSON, _ := json.Marshal(map[string]interface{}{
				"result": result,
				"error":  tc.Error,
			})

			messages = append(messages, openRouterMessage{
				Role:       "tool",
				Content:    string(resultJSON),
				ToolCallID: toolCall.ID,
			})
		}

		// After tool execution, continue loop to get final text response
		// The model will respond with text in the next iteration
	}

	// Extract text response
	var responseText string
	if len(finalResponse.Choices) > 0 {
		responseText = finalResponse.Choices[0].Message.Content
	}

	// Validate response is not empty
	trimmedResponse := strings.TrimSpace(responseText)
	if trimmedResponse == "" {
		return nil, fmt.Errorf("empty or whitespace-only response from OpenRouter (tool calls: %d)", len(allToolCalls))
	}

	// Build agent response
	agentResp := &AgentResponse{
		Content:      responseText,
		ToolCalls:    allToolCalls,
		FinishReason: finalResponse.Choices[0].FinishReason,
		TokensUsed:   finalResponse.Usage.TotalTokens,
		InputTokens:  finalResponse.Usage.PromptTokens,
		OutputTokens: finalResponse.Usage.CompletionTokens,
		Model:        modelName,
		Provider:     "openrouter",
	}

	logger.Printf("[OpenRouter Agent] Generated response with %d tool calls, %d tokens, model: %s",
		len(allToolCalls), finalResponse.Usage.TotalTokens, modelName)

	return agentResp, nil
}

// callAPI makes an HTTP request to OpenRouter API
func (a *openRouterAgent) callAPI(ctx context.Context, req openRouterRequest) (*openRouterResponse, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/aseno/backend")
	httpReq.Header.Set("X-Title", "Aseno WhatsApp Agent")

	// Make request
	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		var errResp openRouterResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != nil {
			return &errResp, nil
		}
		return nil, fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Parse response
	var resp openRouterResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// executeFunctionCall executes a single function call
func (a *openRouterAgent) executeFunctionCall(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	logger.Printf("[OpenRouter Agent] Executing tool: %s with args: %v", name, args)

	// Update execution context
	a.execContext.Context = ctx

	// Execute tool
	result, err := a.toolExecutor.Execute(a.execContext, name, args)
	if err != nil {
		logger.Printf("[OpenRouter Agent] Tool %s execution error: %v", name, err)
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, err
	}

	logger.Printf("[OpenRouter Agent] Tool %s executed successfully", name)
	return result, nil
}
