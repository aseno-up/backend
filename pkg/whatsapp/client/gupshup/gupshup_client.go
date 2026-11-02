package gupshup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/whatsapp/model"
	whatsapp_repository "github.com/aseno/backend/pkg/whatsapp/repository"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

var nonDigitRegex = regexp.MustCompile(`[^0-9]`)

// Config holds the Gupshup API configuration
type Config struct {
	BaseURL string // e.g., "https://api.gupshup.io"
	APIKey  string // Global API key (partner mode). Falls back to per-clinic Token in self-serve.
	Timeout time.Duration
}

// gupshupClient implementa service.WhatsAppClient para o provedor Gupshup (Oficial)
type gupshupClient struct {
	config     Config
	httpClient *http.Client
	connRepo   whatsapp_repository.WhatsAppConnectionRepository
}

// NewGupshupClient creates a new Gupshup API client.
// In self-serve mode, the API key comes from whatsapp_connections.token (per-clinic).
// In partner mode, config.APIKey (from env) is used as fallback for all clinics.
func NewGupshupClient(config Config, connRepo whatsapp_repository.WhatsAppConnectionRepository) service.WhatsAppClient {
	if config.Timeout == 0 {
		config.Timeout = 15 * time.Second
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.gupshup.io"
	}

	return &gupshupClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		connRepo: connRepo,
	}
}

// cleanPhone strips @s.whatsapp.net, @c.us, + and any non-digit chars.
// Gupshup expects plain digits like 5511999990000.
func cleanPhone(phone string) string {
	phone = strings.TrimSuffix(phone, "@s.whatsapp.net")
	phone = strings.TrimSuffix(phone, "@c.us")
	phone = nonDigitRegex.ReplaceAllString(phone, "")
	return phone
}

// getCredentials fetches API key and source phone for an instance.
// Priority: per-clinic Token > global config APIKey.
func (c *gupshupClient) getCredentials(ctx context.Context, instance string) (apiKey, sourcePhone string, err error) {
	conn, err := c.connRepo.GetByInstance(ctx, instance)
	if err != nil {
		return "", "", fmt.Errorf("failed to get gupshup connection for instance %s: %w", instance, err)
	}

	// API key: per-clinic Token takes priority, then global config
	if conn.Token != nil && *conn.Token != "" {
		apiKey = *conn.Token
	} else if c.config.APIKey != "" {
		apiKey = c.config.APIKey
	} else {
		return "", "", fmt.Errorf("gupshup API key not configured for instance %s (no token in DB and no GUPSHUP_API_KEY in env)", instance)
	}

	if conn.PhoneNumberID == nil || *conn.PhoneNumberID == "" {
		return "", "", fmt.Errorf("gupshup source phone (phone_number_id) not configured for instance %s", instance)
	}

	return apiKey, *conn.PhoneNumberID, nil
}

// CreateInstance does not generate QR for official API, as it relies on credentials.
func (c *gupshupClient) CreateInstance(ctx context.Context, instance string) (*model.QRCodeData, error) {
	return nil, nil
}

// SendTextMessage sends a text message via Gupshup API
func (c *gupshupClient) SendTextMessage(ctx context.Context, instance, phone, message string) (*model.SendMessageResponse, error) {
	apiKey, sourcePhone, err := c.getCredentials(ctx, instance)
	if err != nil {
		return nil, err
	}

	msgPayload, _ := json.Marshal(map[string]string{
		"type": "text",
		"text": message,
	})

	return c.sendMessage(ctx, apiKey, sourcePhone, instance, cleanPhone(phone), string(msgPayload))
}

// SendButtonMessage sends a message with buttons via Gupshup API
func (c *gupshupClient) SendButtonMessage(ctx context.Context, instance, phone, message string, buttons []model.MessageButton) (*model.SendMessageResponse, error) {
	apiKey, sourcePhone, err := c.getCredentials(ctx, instance)
	if err != nil {
		return nil, err
	}

	// Build interactive quick_reply buttons
	gupshupButtons := make([]map[string]string, len(buttons))
	for i, btn := range buttons {
		gupshupButtons[i] = map[string]string{
			"type":  "text",
			"title": btn.Title,
			"id":    btn.ID,
		}
	}

	msgPayload, _ := json.Marshal(map[string]interface{}{
		"type":    "quick_reply",
		"content": map[string]string{"type": "text", "text": message},
		"options": gupshupButtons,
	})

	dest := cleanPhone(phone)
	resp, err := c.sendMessage(ctx, apiKey, sourcePhone, instance, dest, string(msgPayload))
	if err != nil {
		// Fallback to plain text if interactive message fails
		logger.Printf("[Gupshup] Button message failed, falling back to text: %v", err)
		return c.SendTextMessage(ctx, instance, phone, message)
	}
	return resp, nil
}

// sendMessage performs the actual HTTP POST to Gupshup /wa/api/v1/msg
func (c *gupshupClient) sendMessage(ctx context.Context, apiKey, source, appName, destination, messageJSON string) (*model.SendMessageResponse, error) {
	form := url.Values{}
	form.Set("channel", "whatsapp")
	form.Set("source", source)
	form.Set("destination", destination)
	form.Set("message", messageJSON)
	form.Set("src.name", appName)

	endpoint := c.config.BaseURL + "/wa/api/v1/msg"
	logger.Printf("[Gupshup] POST %s source=%s destination=%s app=%s", endpoint, source, destination, appName)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("apikey", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Printf("[Gupshup] Connection error: %v", err)
		return nil, errors.ErrWhatsAppConnectionInstance.WithError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	logger.Printf("[Gupshup] Response Status: %d Body: %s", resp.StatusCode, truncate(string(respBody), 500))

	if resp.StatusCode >= 400 {
		var errResp struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		errMsg := errResp.Message
		if errMsg == "" {
			errMsg = fmt.Sprintf("Gupshup API error: %d", resp.StatusCode)
		}
		return nil, errors.ErrWhatsAppSendFailedInstance.WithMessage(errMsg)
	}

	var result struct {
		Status    string `json:"status"`
		MessageID string `json:"messageId"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return &model.SendMessageResponse{
		MessageID:  result.MessageID,
		ExternalID: result.MessageID,
		Status:     result.Status,
	}, nil
}

// GetConnectionStatus returns connection status based on whether credentials exist in the DB.
// No external API call — avoids rate limiting with many concurrent clinics.
func (c *gupshupClient) GetConnectionStatus(ctx context.Context, instance string) (*model.ConnectionStatus, error) {
	_, _, err := c.getCredentials(ctx, instance)
	if err != nil {
		return &model.ConnectionStatus{
			Instance:   instance,
			State:      "disconnected",
			StatusCode: 200,
		}, nil
	}

	return &model.ConnectionStatus{
		Instance:   instance,
		State:      "open",
		StatusCode: 200,
	}, nil
}

// GetQRCode is not applicable for the official API
func (c *gupshupClient) GetQRCode(ctx context.Context, instance string) (*model.QRCodeData, error) {
	return nil, nil
}

// Logout is a no-op for the official API
func (c *gupshupClient) Logout(ctx context.Context, instance string) error {
	return nil
}

// DeleteInstance is a no-op for the official API
func (c *gupshupClient) DeleteInstance(ctx context.Context, instance string) error {
	return nil
}

// DownloadMedia downloads media content from Gupshup API
func (c *gupshupClient) DownloadMedia(ctx context.Context, instance, messageID string) ([]byte, string, error) {
	apiKey, _, err := c.getCredentials(ctx, instance)
	if err != nil {
		return nil, "", err
	}

	endpoint := fmt.Sprintf("%s/wa/api/v1/msg/media/%s", c.config.BaseURL, messageID)
	logger.Printf("[Gupshup] GET %s", endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create gupshup download request: %w", err)
	}
	req.Header.Set("apikey", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("gupshup API error downloading media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("gupshup API error downloading media: %d %s", resp.StatusCode, string(respBytes))
	}

	mediaBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read downloaded media: %w", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "audio/ogg"
	}

	return mediaBytes, mimeType, nil
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
