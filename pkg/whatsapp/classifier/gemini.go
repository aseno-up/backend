package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/aseno/backend/pkg/whatsapp/model"
)

type geminiClassifier struct {
	client    *genai.Client
	modelName string
	timeout   time.Duration
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

	return &geminiClassifier{
		client:    client,
		modelName: cfg.Model,
		timeout:   cfg.Timeout,
	}, nil
}

func (g *geminiClassifier) Provider() string {
	return "gemini"
}

func (g *geminiClassifier) Classify(ctx context.Context, content string) (*ClassificationResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	genModel := g.client.GenerativeModel(g.modelName)
	genModel.SetTemperature(0.1) // Low temperature for consistent responses

	prompt := fmt.Sprintf(ClassificationPrompt, content)

	resp, err := genModel.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini classification failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
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

	return &ClassificationResult{
		Intent:     intent,
		Confidence: result.Confidence,
		Reasoning:  result.Reasoning,
		Provider:   "gemini",
		Model:      g.modelName,
		Latency:    time.Since(start),
	}, nil
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
