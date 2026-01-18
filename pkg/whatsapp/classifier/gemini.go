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
		Data       *struct {
			Type      string `json:"tipo"`
			Specialty string `json:"especialidade"`
			Urgency   string `json:"urgencia"`
		} `json:"data,omitempty"`
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

	// Build classification result
	classResult := &ClassificationResult{
		Intent:     intent,
		Confidence: result.Confidence,
		Reasoning:  result.Reasoning,
		Provider:   "gemini",
		Model:      modelName,
		Latency:    time.Since(start),
	}

	// Add schedule data if present
	if result.Data != nil && intent == model.MessageIntentSchedule {
		classResult.Data = &model.ScheduleIntentData{
			Type:      result.Data.Type,
			Specialty: result.Data.Specialty,
			Urgency:   result.Data.Urgency,
		}
	}

	return classResult, nil
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
	case model.MessageIntentSchedule,
		model.MessageIntentConfirm,
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

// MatchSlot matches a patient's natural language response to a time slot
func (g *geminiClassifier) MatchSlot(ctx context.Context, message string, slots []model.SlotOption) (*SlotMatchResult, error) {
	// Try matching with automatic model rotation on rate limit errors
	maxRetries := len(rotator.GetAllModels())
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		currentModel := rotator.GetCurrentModel()

		result, err := g.tryMatchSlot(ctx, message, slots, currentModel)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if it's a rate limit error (429) or resource exhausted
		if isRateLimitError(err) {
			log.Printf("[Gemini Slot Match] Rate limit hit on model '%s': %v", currentModel, err)
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

// tryMatchSlot attempts to match a slot with a specific model
func (g *geminiClassifier) tryMatchSlot(ctx context.Context, message string, slots []model.SlotOption, modelName string) (*SlotMatchResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// Format slots for the prompt
	var slotsText strings.Builder
	for i, slot := range slots {
		weekday := translateWeekday(slot.StartTime.Weekday())
		date := slot.StartTime.Format("02/01")
		timeStr := slot.StartTime.Format("15:04")
		slotsText.WriteString(fmt.Sprintf("Slot %d: %s %s as %s - %s\n",
			i, weekday, date, timeStr, slot.ProfessionalName))
	}

	prompt := fmt.Sprintf(`Voce e um assistente de agendamento de consultas medicas.

O paciente recebeu as seguintes opcoes de horario:

%s
O paciente respondeu: "%s"

Identifique qual slot o paciente escolheu. Se nao houver correspondencia clara, retorne confidence < 0.5.

Considere:
- Numeros diretos (1, 2, 3, etc) correspondem aos slots numerados
- Dias da semana (segunda, terca, quarta, quinta, sexta, sabado, domingo)
- Horarios (14h, 14:00, 2pm, de manha, de tarde, de noite)
- Nomes de profissionais
- Posicoes (primeiro, segundo, ultimo)
- Referencias de data (amanha, hoje, proxima semana)

Responda APENAS com JSON valido (sem markdown, sem crases):
{"slot_index": 0, "confidence": 0.95, "reasoning": "Explicacao de por que este slot foi escolhido"}`, slotsText.String(), message)

	genModel := g.client.GenerativeModel(modelName)
	genModel.SetTemperature(0.1) // Low temperature for consistent responses

	resp, err := genModel.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini slot matching failed with model '%s': %w", modelName, err)
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
		SlotIndex  int     `json:"slot_index"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini slot match response '%s': %w", text, err)
	}

	// Validate slot index
	if result.SlotIndex < 0 || result.SlotIndex >= len(slots) {
		return nil, fmt.Errorf("invalid slot index %d (must be 0-%d)", result.SlotIndex, len(slots)-1)
	}

	log.Printf("[Gemini Slot Match] Success with model '%s' (latency: %v)", modelName, time.Since(start))

	return &SlotMatchResult{
		SlotIndex:  result.SlotIndex,
		Confidence: result.Confidence,
		Reasoning:  result.Reasoning,
		Provider:   "gemini",
		Model:      modelName,
		Latency:    time.Since(start),
	}, nil
}

// translateWeekday translates Go's time.Weekday to Portuguese
func translateWeekday(wd time.Weekday) string {
	switch wd {
	case time.Sunday:
		return "Domingo"
	case time.Monday:
		return "Segunda"
	case time.Tuesday:
		return "Terca"
	case time.Wednesday:
		return "Quarta"
	case time.Thursday:
		return "Quinta"
	case time.Friday:
		return "Sexta"
	case time.Saturday:
		return "Sabado"
	default:
		return ""
	}
}
