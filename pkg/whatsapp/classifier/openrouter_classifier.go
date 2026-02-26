package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/whatsapp/model"
)

// openRouterClassifier implements MessageClassifier using OpenRouter API
type openRouterClassifier struct {
	apiKey         string
	apiURL         string
	httpReferer    string
	title          string
	primaryModel   string
	fallbackModels []string
	timeout        time.Duration
	httpClient     *http.Client
}

// NewOpenRouterClassifier creates a new OpenRouter-based classifier
func NewOpenRouterClassifier(cfg Config) (MessageClassifier, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY is required for openrouter classifier")
	}

	model := cfg.Model
	if model == "" {
		model = "google/gemini-2.0-flash-001"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}

	return &openRouterClassifier{
		apiKey:       cfg.APIKey,
		apiURL:       cfg.OpenRouterAPIURL,
		httpReferer:  cfg.OpenRouterHTTPReferer,
		title:        cfg.OpenRouterClassifierTitle,
		primaryModel: model,
		fallbackModels: []string{
			"google/gemini-2.0-flash-lite-001",
			"qwen/qwen-2.5-72b-instruct",
			"meta-llama/llama-3.3-70b-instruct",
		},
		timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (c *openRouterClassifier) Provider() string {
	return "openrouter"
}

// Classify classifies the intent of an incoming message
func (c *openRouterClassifier) Classify(ctx context.Context, content string) (*ClassificationResult, error) {
	prompt := fmt.Sprintf(ClassificationPrompt, content)

	models := append([]string{c.primaryModel}, c.fallbackModels...)
	var lastErr error

	for _, m := range models {
		result, err := c.callCompletion(ctx, m, prompt)
		if err != nil {
			logger.Printf("[OpenRouter Classifier] Model %s failed: %v", m, err)
			lastErr = err
			continue
		}

		// Parse JSON from model response
		text := cleanJSONResponse(result)
		var parsed struct {
			Intent     string  `json:"intent"`
			Confidence float64 `json:"confidence"`
			Reasoning  string  `json:"reasoning"`
			Data       *struct {
				Type      string `json:"tipo"`
				Specialty string `json:"especialidade"`
				Urgency   string `json:"urgencia"`
			} `json:"data,omitempty"`
		}

		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			logger.Printf("[OpenRouter Classifier] Failed to parse response from %s: %v — raw: %q", m, err, text)
			lastErr = err
			continue
		}

		intent := model.MessageIntent(parsed.Intent)
		if !isValidIntent(intent) {
			intent = model.MessageIntentUnknown
		}

		classResult := &ClassificationResult{
			Intent:     intent,
			Confidence: parsed.Confidence,
			Reasoning:  parsed.Reasoning,
			Provider:   "openrouter",
			Model:      m,
		}

		if parsed.Data != nil && intent == model.MessageIntentSchedule {
			classResult.Data = &model.ScheduleIntentData{
				Type:      parsed.Data.Type,
				Specialty: parsed.Data.Specialty,
				Urgency:   parsed.Data.Urgency,
			}
		}

		logger.Printf("[OpenRouter Classifier] Success with model %s: intent=%s confidence=%.2f", m, intent, parsed.Confidence)
		return classResult, nil
	}

	return nil, fmt.Errorf("all openrouter classifier models failed, last error: %w", lastErr)
}

// MatchSlot matches a patient response to a list of available slots
func (c *openRouterClassifier) MatchSlot(ctx context.Context, message string, slots []model.SlotOption) (*SlotMatchResult, error) {
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

	start := time.Now()
	models := append([]string{c.primaryModel}, c.fallbackModels...)
	var lastErr error

	for _, m := range models {
		result, err := c.callCompletion(ctx, m, prompt)
		if err != nil {
			logger.Printf("[OpenRouter SlotMatch] Model %s failed: %v", m, err)
			lastErr = err
			continue
		}

		text := cleanJSONResponse(result)
		var parsed struct {
			SlotIndex  int     `json:"slot_index"`
			Confidence float64 `json:"confidence"`
			Reasoning  string  `json:"reasoning"`
		}

		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			logger.Printf("[OpenRouter SlotMatch] Failed to parse response from %s: %v — raw: %q", m, err, text)
			lastErr = err
			continue
		}

		if parsed.SlotIndex < 0 || parsed.SlotIndex >= len(slots) {
			lastErr = fmt.Errorf("invalid slot index %d (must be 0-%d)", parsed.SlotIndex, len(slots)-1)
			continue
		}

		logger.Printf("[OpenRouter SlotMatch] Success with model %s: slot=%d confidence=%.2f latency=%v",
			m, parsed.SlotIndex, parsed.Confidence, time.Since(start))

		return &SlotMatchResult{
			SlotIndex:  parsed.SlotIndex,
			Confidence: parsed.Confidence,
			Reasoning:  parsed.Reasoning,
			Provider:   "openrouter",
			Model:      m,
			Latency:    time.Since(start),
		}, nil
	}

	return nil, fmt.Errorf("all openrouter slot match models failed, last error: %w", lastErr)
}

// callCompletion sends a chat completion request to OpenRouter and returns the response text
func (c *openRouterClassifier) callCompletion(ctx context.Context, modelName, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
		"max_tokens":  512,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.httpReferer != "" {
		httpReq.Header.Set("HTTP-Referer", c.httpReferer)
	}
	if c.title != "" {
		httpReq.Header.Set("X-Title", c.title)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("openrouter error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty response from openrouter model %s", modelName)
	}

	return apiResp.Choices[0].Message.Content, nil
}
