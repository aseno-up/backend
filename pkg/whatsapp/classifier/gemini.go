package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/aseno/backend/pkg/whatsapp/model"
)

// Default fallback models in order of preference
// When one model hits rate limit, we rotate to the next
var defaultFallbackModels = []string{
	"gemini-2.0-flash",
	"gemini-2.5-flash-lite",
	"gemini-1.5-flash",
	"gemini-2.5-flash",
	"gemini-3-flash",
	"gemma-3-27b",
	"gemma-3-12b",
	"gemma-3-4b",
}

// modelRotator manages the current model index with thread safety
type modelRotator struct {
	mu           sync.RWMutex
	models       []string
	currentIndex int
}

// Global model rotator instance
var rotator = &modelRotator{
	models:       defaultFallbackModels,
	currentIndex: 0,
}

// GetCurrentModel returns the current model to use
func (r *modelRotator) GetCurrentModel() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.models[r.currentIndex]
}

// RotateToNext moves to the next model in the list
// Returns the new model name
func (r *modelRotator) RotateToNext() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldModel := r.models[r.currentIndex]
	r.currentIndex = (r.currentIndex + 1) % len(r.models)
	newModel := r.models[r.currentIndex]

	log.Printf("[Gemini Rotator] Rotating from '%s' to '%s' (index: %d/%d)",
		oldModel, newModel, r.currentIndex, len(r.models)-1)

	return newModel
}

// SetModels allows configuring custom model list
func (r *modelRotator) SetModels(models []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(models) > 0 {
		r.models = models
		r.currentIndex = 0
		log.Printf("[Gemini Rotator] Models configured: %v", models)
	}
}

// GetAllModels returns the list of all available models
func (r *modelRotator) GetAllModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string{}, r.models...)
}

type geminiClassifier struct {
	client  *genai.Client
	timeout time.Duration
}

// NewGeminiClassifier creates a new Gemini-based classifier
func NewGeminiClassifier(cfg Config) (MessageClassifier, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// If a specific model is configured, put it first in the rotation
	if cfg.Model != "" {
		models := rotator.GetAllModels()
		// Check if configured model is already in the list
		found := false
		for _, m := range models {
			if m == cfg.Model {
				found = true
				break
			}
		}
		// If not found, add it to the beginning
		if !found {
			models = append([]string{cfg.Model}, models...)
			rotator.SetModels(models)
		}
	}

	log.Printf("[Gemini Classifier] Initialized with models: %v", rotator.GetAllModels())
	log.Printf("[Gemini Classifier] Starting with model: %s", rotator.GetCurrentModel())

	return &geminiClassifier{
		client:  client,
		timeout: cfg.Timeout,
	}, nil
}

func (g *geminiClassifier) Provider() string {
	return "gemini"
}

func (g *geminiClassifier) Classify(ctx context.Context, content string) (*ClassificationResult, error) {
	// Try classification with automatic model rotation on rate limit errors
	maxRetries := len(rotator.GetAllModels())
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		currentModel := rotator.GetCurrentModel()

		result, err := g.tryClassify(ctx, content, currentModel)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if it's a rate limit error (429) or resource exhausted
		if isRateLimitError(err) {
			log.Printf("[Gemini Classifier] Rate limit hit on model '%s': %v", currentModel, err)
			rotator.RotateToNext()
			// Small delay before retry to avoid hammering the API
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// For other errors, don't retry
		return nil, err
	}

	// All models exhausted
	return nil, fmt.Errorf("all models exhausted after %d attempts, last error: %w", maxRetries, lastErr)
}

// tryClassify attempts classification with a specific model
func (g *geminiClassifier) tryClassify(ctx context.Context, content string, modelName string) (*ClassificationResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	genModel := g.client.GenerativeModel(modelName)
	genModel.SetTemperature(0.1) // Low temperature for consistent responses

	prompt := fmt.Sprintf(ClassificationPrompt, content)

	resp, err := genModel.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini classification failed with model '%s': %w", modelName, err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini model '%s'", modelName)
	}

	// Extract text from response
	text := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	// Clean the response (remove markdown code blocks if present)
	text = cleanJSONResponse(text)

	// Parse JSON
	var result struct {
		Intent     string  `json:"intent"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response '%s': %w", text, err)
	}

	// Validate intent
	intent := model.MessageIntent(result.Intent)
	if !isValidIntent(intent) {
		intent = model.MessageIntentUnknown
	}

	log.Printf("[Gemini Classifier] Success with model '%s' (latency: %v)", modelName, time.Since(start))

	return &ClassificationResult{
		Intent:     intent,
		Confidence: result.Confidence,
		Reasoning:  result.Reasoning,
		Provider:   "gemini",
		Model:      modelName,
		Latency:    time.Since(start),
	}, nil
}

// isRateLimitError checks if the error is a rate limit (429) or resource exhausted error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "resource exhausted") ||
		strings.Contains(errStr, "quota") ||
		strings.Contains(errStr, "too many requests")
}

// cleanJSONResponse removes markdown code blocks and extra whitespace
func cleanJSONResponse(text string) string {
	// Remove markdown code blocks
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	return text
}

// isValidIntent checks if the intent is a valid MessageIntent
func isValidIntent(intent model.MessageIntent) bool {
	switch intent {
	case model.MessageIntentConfirm,
		model.MessageIntentCancel,
		model.MessageIntentReschedule,
		model.MessageIntentQuestion,
		model.MessageIntentUnknown:
		return true
	default:
		return false
	}
}

// GetCurrentGeminiModel returns the current model being used (for monitoring)
func GetCurrentGeminiModel() string {
	return rotator.GetCurrentModel()
}

// GetGeminiModelsList returns all configured models (for monitoring)
func GetGeminiModelsList() []string {
	return rotator.GetAllModels()
}
