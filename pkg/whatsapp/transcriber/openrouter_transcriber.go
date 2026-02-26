package transcriber

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aseno/backend/internal/logger"
)

// AudioTranscriber transcribes audio bytes to text
type AudioTranscriber interface {
	Transcribe(ctx context.Context, audioBytes []byte, mimeType string) (string, error)
}

// OpenRouterTranscriber implements AudioTranscriber using OpenRouter multimodal models
type OpenRouterTranscriber struct {
	apiURL      string
	apiKey      string
	httpReferer string
	title       string
	model       string
	httpClient  *http.Client
}

// NewOpenRouterTranscriber creates a new transcriber using OpenRouter
func NewOpenRouterTranscriber(apiURL, apiKey, model, referer, title string) *OpenRouterTranscriber {
	if model == "" {
		model = "google/gemini-2.0-flash-001"
	}
	if apiURL == "" {
		apiURL = "https://openrouter.ai/api/v1/chat/completions"
	}
	return &OpenRouterTranscriber{
		apiURL:      apiURL,
		apiKey:      apiKey,
		httpReferer: referer,
		title:       title,
		model:       model,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Transcribe sends audio bytes to OpenRouter for transcription and returns the text
func (t *OpenRouterTranscriber) Transcribe(ctx context.Context, audioBytes []byte, mimeType string) (string, error) {
	if mimeType == "" {
		mimeType = "audio/ogg; codecs=opus" // WhatsApp default
	}

	// Encode audio as base64 data URI
	encoded := base64.StdEncoding.EncodeToString(audioBytes)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)

	// Build OpenAI-compatible multimodal request
	reqBody := map[string]interface{}{
		"model": t.model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Transcreva exatamente o áudio a seguir em português. Retorne APENAS o texto transcrito, sem comentários, sem formatação, sem pontuação extra. Se não conseguir entender alguma parte, use [inaudível].",
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": dataURI,
						},
					},
				},
			},
		},
		"temperature": 0.0,
		"max_tokens":  1024,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transcription request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create transcription request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+t.apiKey)
	if t.httpReferer != "" {
		httpReq.Header.Set("HTTP-Referer", t.httpReferer)
	}
	if t.title != "" {
		httpReq.Header.Set("X-Title", t.title)
	}

	resp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("transcription HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read transcription response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter transcription HTTP %d: %s", resp.StatusCode, string(respBytes))
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
		return "", fmt.Errorf("failed to parse transcription response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("openrouter transcription error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty transcription response from openrouter")
	}

	transcribed := strings.TrimSpace(apiResp.Choices[0].Message.Content)
	logger.Printf("[Transcriber] Audio transcribed successfully (%d bytes audio → %d chars text)", len(audioBytes), len(transcribed))
	return transcribed, nil
}
