package evolution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// Config holds the Evolution API configuration
type Config struct {
	BaseURL    string
	APIKey     string
	Timeout    time.Duration
	WebhookURL string
}

// evolutionClient implements service.WhatsAppClient
type evolutionClient struct {
	config     Config
	httpClient *http.Client
}

// NewEvolutionClient creates a new Evolution API client
func NewEvolutionClient(config Config) service.WhatsAppClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &evolutionClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// CreateInstance creates a new WhatsApp instance
func (c *evolutionClient) CreateInstance(ctx context.Context, instance string) (*model.QRCodeData, error) {
	payload := map[string]interface{}{
		"instanceName": instance,
		"qrcode":       true,
		"integration":  "WHATSAPP-BAILEYS",
	}

	respBody, err := c.doRequest(ctx, "POST", "/instance/create", payload)
	if err != nil {
		return nil, err
	}

	// Parse the response to get the QR code. The structure is based on Evolution API docs.
	var result struct {
		Instance struct {
			InstanceName string `json:"instanceName"`
		} `json:"instance"`
		Hash   interface{} `json:"hash"` // Can be string or object
		Qrcode struct {
			Code   string `json:"code"`
			Base64 string `json:"base64"`
			Count  int    `json:"count"`
		} `json:"qrcode"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("[Evolution API] Failed to unmarshal CreateInstance response: %v. Body: %s", err, string(respBody))
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// If qrcode is not in the response or count is 0, it may mean the instance already existed
	// or the QR code is not ready yet. The caller should fetch it separately.
	if result.Qrcode.Base64 == "" || result.Qrcode.Count == 0 {
		log.Printf("[Evolution API] CreateInstance response did not contain a QR code for instance %s (count=%d). QR code may need to be fetched separately.", instance, result.Qrcode.Count)
		return nil, nil // Return nil, nil to indicate success but no QR data
	}

	qrData := &model.QRCodeData{
		Instance: result.Instance.InstanceName,
		QRCode:   result.Qrcode.Code,
		Base64:   result.Qrcode.Base64,
	}

	// Configure webhook automatically if WebhookURL is set
	if c.config.WebhookURL != "" {
		if err := c.SetWebhook(ctx, instance); err != nil {
			log.Printf("[Evolution API] Warning: Failed to configure webhook for instance %s: %v", instance, err)
			// Don't fail the instance creation if webhook setup fails
		}
	}

	return qrData, nil
}

// SetWebhook configures the webhook for an instance
func (c *evolutionClient) SetWebhook(ctx context.Context, instance string) error {
	if c.config.WebhookURL == "" {
		return errors.ErrInternalInstance.WithMessage("webhook URL not configured")
	}

	payload := map[string]interface{}{
		"webhook": map[string]interface{}{
			"enabled":          true,
			"url":              c.config.WebhookURL,
			"webhookByEvents":  false,
			"webhookBase64":    false,
			"events": []string{
				"MESSAGES_UPSERT",
			},
		},
	}

	_, err := c.doRequest(ctx, "POST", "/webhook/set/"+instance, payload)
	if err != nil {
		return err
	}

	log.Printf("[Evolution API] Webhook configured successfully for instance %s", instance)
	return nil
}

// SendTextMessage sends a text message via Evolution API
func (c *evolutionClient) SendTextMessage(ctx context.Context, instance, phone, message string) (*model.SendMessageResponse, error) {
	payload := map[string]interface{}{
		"number": phone,
		"text":   message,
	}

	resp, err := c.doRequest(ctx, "POST", "/message/sendText/"+instance, payload)
	if err != nil {
		return nil, err
	}

	return c.parseMessageResponse(resp)
}

// SendButtonMessage sends a message with buttons
func (c *evolutionClient) SendButtonMessage(ctx context.Context, instance, phone, message string, buttons []model.MessageButton) (*model.SendMessageResponse, error) {
	// Evolution API button format
	evolutionButtons := make([]map[string]interface{}, len(buttons))
	for i, btn := range buttons {
		evolutionButtons[i] = map[string]interface{}{
			"buttonId":       btn.ID,
			"buttonText":     map[string]string{"displayText": btn.Title},
			"type":           1,
		}
	}

	payload := map[string]interface{}{
		"number":       phone,
		"title":        "", // Optional title
		"description":  message,
		"footer":       "",
		"buttons":      evolutionButtons,
	}

	resp, err := c.doRequest(ctx, "POST", "/message/sendButtons/"+instance, payload)
	if err != nil {
		// Fallback to text message if buttons are not supported
		return c.SendTextMessage(ctx, instance, phone, message)
	}

	return c.parseMessageResponse(resp)
}

// GetConnectionStatus gets the current connection status
func (c *evolutionClient) GetConnectionStatus(ctx context.Context, instance string) (*model.ConnectionStatus, error) {
	resp, err := c.doRequest(ctx, "GET", "/instance/connectionState/"+instance, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Instance struct {
			InstanceName string `json:"instanceName"`
			State        string `json:"state"`
		} `json:"instance"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return &model.ConnectionStatus{
		Instance:   result.Instance.InstanceName,
		State:      result.Instance.State,
		StatusCode: 200,
	}, nil
}

// GetQRCode gets the QR code for connecting WhatsApp
func (c *evolutionClient) GetQRCode(ctx context.Context, instance string) (*model.QRCodeData, error) {
	resp, err := c.doRequest(ctx, "GET", "/instance/connect/"+instance, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code   string `json:"code"`
		Base64 string `json:"base64"`
		Count  int    `json:"count"` // Evolution API returns {"count":0} when no QR code is available
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// If count is 0, it means no QR code is available (instance might be stuck or already connected)
	if result.Count == 0 && result.Base64 == "" {
		log.Printf("[Evolution API] No QR code available for instance %s (count=0). Instance may need to be restarted.", instance)
	}

	return &model.QRCodeData{
		Instance: instance,
		QRCode:   result.Code,
		Base64:   result.Base64,
	}, nil
}

// Logout disconnects the WhatsApp session
func (c *evolutionClient) Logout(ctx context.Context, instance string) error {
	_, err := c.doRequest(ctx, "DELETE", "/instance/logout/"+instance, nil)
	return err
}

// DeleteInstance removes a WhatsApp instance
func (c *evolutionClient) DeleteInstance(ctx context.Context, instance string) error {
	_, err := c.doRequest(ctx, "DELETE", "/instance/delete/"+instance, nil)
	return err
}

// doRequest performs an HTTP request to Evolution API
func (c *evolutionClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, errors.ErrInternalInstance.WithError(err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := c.config.BaseURL + path
	log.Printf("[Evolution API] Request: %s %s", method, url)

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Evolution API] Connection Error: %v", err)
		return nil, errors.ErrWhatsAppConnectionInstance.WithError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Evolution API] Error reading response body: %v", err)
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	log.Printf("[Evolution API] Response Status: %d", resp.StatusCode)

	if resp.StatusCode >= 400 {
		log.Printf("[Evolution API] Error Response Body: %s", string(respBody))
		
		var errResp struct {
			Message  interface{} `json:"message"` // Can be string or []string
			Error    string      `json:"error"`
			Response struct {
				Message interface{} `json:"message"` // Can be string or []string
			} `json:"response"`
		}
		_ = json.Unmarshal(respBody, &errResp)

		var errMsg string
		if errResp.Message != nil {
			switch v := errResp.Message.(type) {
			case string:
				errMsg = v
			case []interface{}:
				if len(v) > 0 {
					errMsg = fmt.Sprintf("%v", v[0])
				}
			}
		}
		if errMsg == "" && errResp.Response.Message != nil {
			switch v := errResp.Response.Message.(type) {
			case string:
				errMsg = v
			case []interface{}:
				if len(v) > 0 {
					errMsg = fmt.Sprintf("%v", v[0])
				}
			}
		}
		if errMsg == "" {
			errMsg = errResp.Error
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("Evolution API error: %d", resp.StatusCode)
		}

		return nil, errors.ErrWhatsAppSendFailedInstance.WithMessage(errMsg)
	}

	// Limiting success log size to avoid spamming console
	logBody := string(respBody)
	if len(logBody) > 500 {
		logBody = logBody[:500] + "..."
	}
	log.Printf("[Evolution API] Success Response Body: %s", logBody)

	return respBody, nil
}

// parseMessageResponse parses the Evolution API message response
func (c *evolutionClient) parseMessageResponse(resp []byte) (*model.SendMessageResponse, error) {
	var result struct {
		Key struct {
			ID string `json:"id"`
		} `json:"key"`
		Status string `json:"status"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return &model.SendMessageResponse{
		MessageID:  result.Key.ID,
		ExternalID: result.Key.ID,
		Status:     result.Status,
	}, nil
}

// ParseWebhookPayload parses an incoming webhook payload
func ParseWebhookPayload(data []byte) (*model.ParsedIncomingMessage, error) {
	var payload model.EvolutionWebhookPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, errors.ErrInvalidPayloadInstance.WithError(err)
	}

	// Skip messages from ourselves
	if payload.Data.Key.FromMe {
		return nil, nil
	}

	// Extract phone number (remove @s.whatsapp.net)
	phone := payload.Data.Key.RemoteJid
	if len(phone) > 15 {
		phone = phone[:len(phone)-15] // Remove @s.whatsapp.net
	}

	// Extract message content
	var content string
	var buttonID string

	if payload.Data.Message.Conversation != "" {
		content = payload.Data.Message.Conversation
	} else if payload.Data.Message.ExtendedTextMessage != nil {
		content = payload.Data.Message.ExtendedTextMessage.Text
	} else if payload.Data.Message.ButtonsResponseMessage != nil {
		buttonID = payload.Data.Message.ButtonsResponseMessage.SelectedButtonID
		content = payload.Data.Message.ButtonsResponseMessage.SelectedDisplayText
	} else if payload.Data.Message.ListResponseMessage != nil {
		buttonID = payload.Data.Message.ListResponseMessage.SingleSelectReply.SelectedRowID
	}

	return &model.ParsedIncomingMessage{
		Phone:      phone,
		SenderName: payload.Data.PushName,
		MessageID:  payload.Data.Key.ID,
		Content:    content,
		ButtonID:   buttonID,
		IsFromMe:   payload.Data.Key.FromMe,
		Timestamp:  time.Unix(payload.Data.MessageTimestamp, 0),
		Instance:   payload.Instance,
	}, nil
}
