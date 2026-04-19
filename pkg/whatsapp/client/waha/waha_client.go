package waha

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

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// Config holds the WAHA API configuration
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// wahaClient implements service.WhatsAppClient
type wahaClient struct {
	config     Config
	httpClient *http.Client
}

// normalizeSessionName converts any instance name to "default" for WAHA Core compatibility
// WAHA Core (free version) only supports "default" as session name
func normalizeSessionName(instance string) string {
	return "default"
}

// NewWahaClient creates a new WAHA API client
func NewWahaClient(config Config) service.WhatsAppClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &wahaClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// CreateInstance creates a new WhatsApp session (instance) in WAHA
func (c *wahaClient) CreateInstance(ctx context.Context, instance string) (*model.QRCodeData, error) {
	sessionName := normalizeSessionName(instance)

	payload := map[string]interface{}{
		"name":  sessionName,
		"start": true, // Start session immediately
	}

	respBody, err := c.doRequest(ctx, "POST", "/api/sessions", payload)
	if err != nil {
		return nil, err
	}

	// Parse the response to get session info
	var result struct {
		Name   string `json:"name"`
		Status string `json:"status"` // STOPPED, STARTING, SCAN_QR_CODE, WORKING, FAILED
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		logger.Printf("[WAHA] Failed to unmarshal CreateInstance response: %v. Body: %s", err, string(respBody))
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	logger.Printf("[WAHA] Session created with status: %s", result.Status)

	// If status is SCAN_QR_CODE, try to get QR code
	if result.Status == "SCAN_QR_CODE" {
		// Wait a moment for QR code to be ready
		time.Sleep(1 * time.Second)
		return c.GetQRCode(ctx, instance)
	}

	// Session created but QR not ready yet
	return nil, nil
}

// SendTextMessage sends a text message via WAHA API
func (c *wahaClient) SendTextMessage(ctx context.Context, instance, phone, message string) (*model.SendMessageResponse, error) {
	sessionName := normalizeSessionName(instance)

	// Format phone as chat ID (phone@c.us for individual chats)
	chatId := phone
	if len(phone) > 0 && phone[len(phone)-5:] != "@c.us" {
		chatId = phone + "@c.us"
	}

	payload := map[string]interface{}{
		"session": sessionName,
		"chatId":  chatId,
		"text":    message,
	}

	resp, err := c.doRequest(ctx, "POST", "/api/sendText", payload)
	if err != nil {
		return nil, err
	}

	return c.parseMessageResponse(resp)
}

// SendButtonMessage sends a message with buttons via WAHA API
func (c *wahaClient) SendButtonMessage(ctx context.Context, instance, phone, message string, buttons []model.MessageButton) (*model.SendMessageResponse, error) {
	sessionName := normalizeSessionName(instance)

	// Format phone as chat ID
	chatId := phone
	if len(phone) > 0 && phone[len(phone)-5:] != "@c.us" {
		chatId = phone + "@c.us"
	}

	// WAHA button format
	wahaButtons := make([]map[string]interface{}, len(buttons))
	for i, btn := range buttons {
		wahaButtons[i] = map[string]interface{}{
			"id":   btn.ID,
			"text": btn.Title,
		}
	}

	payload := map[string]interface{}{
		"session": sessionName,
		"chatId":  chatId,
		"text":    message,
		"buttons": wahaButtons,
	}

	resp, err := c.doRequest(ctx, "POST", "/api/sendButtons", payload)
	if err != nil {
		// Fallback to text message if buttons are not supported
		return c.SendTextMessage(ctx, instance, phone, message)
	}

	return c.parseMessageResponse(resp)
}

// GetConnectionStatus gets the current connection status
func (c *wahaClient) GetConnectionStatus(ctx context.Context, instance string) (*model.ConnectionStatus, error) {
	sessionName := normalizeSessionName(instance)

	resp, err := c.doRequest(ctx, "GET", "/api/sessions/"+sessionName, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Name   string `json:"name"`
		Status string `json:"status"` // STOPPED, STARTING, SCAN_QR_CODE, WORKING, FAILED
		Me     *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"me"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Map WAHA status to Evolution-like status
	state := "disconnected"
	switch result.Status {
	case "WORKING":
		state = "open"
	case "SCAN_QR_CODE":
		state = "connecting"
	case "STARTING":
		state = "connecting"
	case "STOPPED":
		state = "disconnected"
	case "FAILED":
		state = "failed"
	}

	return &model.ConnectionStatus{
		Instance:   instance, // Return original instance name for tracking
		State:      state,
		StatusCode: 200,
	}, nil
}

// GetQRCode gets the QR code for connecting WhatsApp
func (c *wahaClient) GetQRCode(ctx context.Context, instance string) (*model.QRCodeData, error) {
	sessionName := normalizeSessionName(instance)

	// WAHA endpoint: GET /api/{session}/auth/qr with Accept: application/json for base64
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.BaseURL+"/api/"+sessionName+"/auth/qr", nil)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", c.config.APIKey)

	logger.Printf("[WAHA] Request: GET %s", req.URL.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Printf("[WAHA] Connection Error: %v", err)
		return nil, errors.ErrWhatsAppConnectionInstance.WithError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Printf("[WAHA] Error reading response body: %v", err)
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	logger.Printf("[WAHA] Response Status: %d", resp.StatusCode)

	if resp.StatusCode >= 400 {
		logger.Printf("[WAHA] Error Response Body: %s", string(respBody))
		return nil, errors.ErrWhatsAppSendFailedInstance.WithMessage(fmt.Sprintf("WAHA API error: %d", resp.StatusCode))
	}

	// Parse QR code response
	var result struct {
		Value  string `json:"value"`  // Raw QR code value
		Base64 string `json:"base64"` // Base64 encoded image
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		logger.Printf("[WAHA] Failed to unmarshal QR response: %v. Body: %s", err, string(respBody))
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	if result.Base64 == "" && result.Value == "" {
		logger.Printf("[WAHA] No QR code available for session %s", sessionName)
		return nil, nil
	}

	logger.Printf("[WAHA] Success Response Body: %s", string(respBody)[:min(len(respBody), 200)])

	return &model.QRCodeData{
		Instance: instance, // Return original instance name for tracking
		QRCode:   result.Value,
		Base64:   result.Base64,
	}, nil
}

// Logout disconnects the WhatsApp session
func (c *wahaClient) Logout(ctx context.Context, instance string) error {
	sessionName := normalizeSessionName(instance)
	_, err := c.doRequest(ctx, "POST", "/api/sessions/"+sessionName+"/logout", nil)
	return err
}

// DeleteInstance removes a WhatsApp session
func (c *wahaClient) DeleteInstance(ctx context.Context, instance string) error {
	sessionName := normalizeSessionName(instance)
	_, err := c.doRequest(ctx, "DELETE", "/api/sessions/"+sessionName, nil)
	return err
}

// doRequest performs an HTTP request to WAHA API
func (c *wahaClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, errors.ErrInternalInstance.WithError(err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := c.config.BaseURL + path
	logger.Printf("[WAHA] Request: %s %s", method, url)

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Printf("[WAHA] Connection Error: %v", err)
		return nil, errors.ErrWhatsAppConnectionInstance.WithError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Printf("[WAHA] Error reading response body: %v", err)
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	logger.Printf("[WAHA] Response Status: %d", resp.StatusCode)

	if resp.StatusCode >= 400 {
		logger.Printf("[WAHA] Error Response Body: %s", string(respBody))

		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(respBody, &errResp)

		errMsg := errResp.Message
		if errMsg == "" {
			errMsg = errResp.Error
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("WAHA API error: %d", resp.StatusCode)
		}

		return nil, errors.ErrWhatsAppSendFailedInstance.WithMessage(errMsg)
	}

	// Limiting success log size to avoid spamming console
	logBody := string(respBody)
	if len(logBody) > 500 {
		logBody = logBody[:500] + "..."
	}
	logger.Printf("[WAHA] Success Response Body: %s", logBody)

	return respBody, nil
}

// DownloadMedia downloads audio/media content from WAHA API
func (c *wahaClient) DownloadMedia(ctx context.Context, instance, messageID string) ([]byte, string, error) {
	// WAHA API: GET /api/{session}/messages/{messageId}/download
	endpoint := fmt.Sprintf("%s/api/%s/messages/%s/download", c.config.BaseURL, instance, messageID)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create waha download request: %w", err)
	}

	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("waha API error downloading media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("waha API error downloading media: %d %s", resp.StatusCode, string(respBytes))
	}

	mediaBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read downloaded media: %w", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "audio/ogg" // Fallback mostly for whatsapp voice notes
	}

	return mediaBytes, mimeType, nil
}

// parseMessageResponse parses the WAHA message response
func (c *wahaClient) parseMessageResponse(resp []byte) (*model.SendMessageResponse, error) {
	var result struct {
		ID        string `json:"id"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return &model.SendMessageResponse{
		MessageID:  result.ID,
		ExternalID: result.ID,
		Status:     "sent",
	}, nil
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ParseWebhookPayload parses an incoming webhook payload from WAHA
func ParseWebhookPayload(data []byte) (*model.ParsedIncomingMessage, error) {
	var payload struct {
		Event   string `json:"event"`
		Session string `json:"session"`
		Payload struct {
			ID        string `json:"id"`
			Timestamp int64  `json:"timestamp"`
			From      string `json:"from"` // e.g. "5511999999999@c.us"
			To        string `json:"to"`   // e.g. "5511999999999@c.us"
			Body      string `json:"body"` // Message text
			FromMe    bool   `json:"fromMe"`
			HasMedia  bool   `json:"hasMedia"`
			ReplyTo   string `json:"replyTo"`
			Ack       int    `json:"ack"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, errors.ErrInvalidPayloadInstance.WithError(err)
	}

	// Only process message events
	if payload.Event != "message" && payload.Event != "message.any" {
		return nil, nil
	}

	// fromMe is returned to the service so it can distinguish our own echoes
	// (cache hit) from a human takeover via the WhatsApp app.

	// Detect group chats by the @g.us JID suffix. The handler uses this flag
	// to skip processing entirely — the service never sees group messages.
	isGroup := strings.HasSuffix(payload.Payload.From, "@g.us")

	// Extract phone number (remove @c.us suffix)
	phone := payload.Payload.From
	if len(phone) > 5 && phone[len(phone)-5:] == "@c.us" {
		phone = phone[:len(phone)-5]
	}

	// Get sender name (not provided in WAHA webhook, use phone)
	senderName := phone

	// Detect audio: WAHA marks media messages with hasMedia=true and empty body
	isAudio := payload.Payload.HasMedia && payload.Payload.Body == ""

	return &model.ParsedIncomingMessage{
		Phone:      phone,
		SenderName: senderName,
		MessageID:  payload.Payload.ID,
		Content:    payload.Payload.Body,
		ButtonID:   "", // WAHA doesn't provide button responses in same format
		IsFromMe:   payload.Payload.FromMe,
		Timestamp:  time.Unix(payload.Payload.Timestamp, 0),
		Instance:   payload.Session,
		IsGroup:    isGroup,
		IsAudio:    isAudio,
	}, nil
}
