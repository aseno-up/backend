package evolution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// Config holds the Evolution API configuration
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
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
func (c *evolutionClient) CreateInstance(ctx context.Context, instance string) error {
	payload := map[string]interface{}{
		"instanceName": instance,
		"qrcode":       true,
	}

	_, err := c.doRequest(ctx, "POST", "/instance/create", payload)
	return err
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
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
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
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.ErrWhatsAppConnectionInstance.WithError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	if resp.StatusCode >= 400 {
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
			errMsg = fmt.Sprintf("Evolution API error: %d", resp.StatusCode)
		}

		return nil, errors.ErrWhatsAppSendFailedInstance.WithMessage(errMsg)
	}

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
