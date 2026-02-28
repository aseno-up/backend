package gupshup

import (
	"context"
	"net/http"
	"time"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// Config holds the Gupshup/Meta API configuration
type Config struct {
	BaseURL string // e.g., "https://api.gupshup.io/wa/api/v1"
	APIKey  string
	Timeout time.Duration
}

// gupshupClient implementa service.WhatsAppClient para o provedor Gupshup/Meta (Oficial)
type gupshupClient struct {
	config     Config
	httpClient *http.Client
}

// NewGupshupClient creates a new Gupshup/Meta API client
func NewGupshupClient(config Config) service.WhatsAppClient {
	if config.Timeout == 0 {
		config.Timeout = 15 * time.Second
	}

	return &gupshupClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// CreateInstance does not generate QR for official API, as it relies on credentials.
func (c *gupshupClient) CreateInstance(ctx context.Context, instance string) (*model.QRCodeData, error) {
	// For the official API, the connection is configured via credentials (WabaID, Token, etc.).
	// We don't generate a QR code. Return nil to indicate it's ready.
	return nil, nil
}

// SendTextMessage sends a text message via Gupshup API
func (c *gupshupClient) SendTextMessage(ctx context.Context, instance, phone, message string) (*model.SendMessageResponse, error) {
	// STUB: This will be implemented with actual Gupshup API requests
	// usually hitting POST /msg with headers like apikey, etc.
	return &model.SendMessageResponse{
		MessageID: "gupshup-msg-id-stub",
	}, nil
}

// SendButtonMessage sends a message with buttons via Gupshup API
func (c *gupshupClient) SendButtonMessage(ctx context.Context, instance, phone, message string, buttons []model.MessageButton) (*model.SendMessageResponse, error) {
	// STUB: This will be implemented with actual Gupshup template/interactive messages
	return &model.SendMessageResponse{
		MessageID: "gupshup-btn-msg-id-stub",
	}, nil
}

// GetConnectionStatus gets the current connection status
func (c *gupshupClient) GetConnectionStatus(ctx context.Context, instance string) (*model.ConnectionStatus, error) {
	// For official API, if credentials are set in the DB, we consider it connected ("open").
	// We can add logic to optionally verify the token against the Meta/Gupshup API.
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

// Logout is essentially a no-op for official API, we just remove credentials in standard logic
func (c *gupshupClient) Logout(ctx context.Context, instance string) error {
	return nil
}

// DeleteInstance removes a WhatsApp instance (no-op or deregister webhook)
func (c *gupshupClient) DeleteInstance(ctx context.Context, instance string) error {
	return nil
}

// DownloadMedia downloads audio/media content from the WhatsApp provider
func (c *gupshupClient) DownloadMedia(ctx context.Context, instance, messageID string) ([]byte, string, error) {
	// STUB for downloading media via Official API
	return nil, "", errors.ErrInternalInstance.WithMessage("Media download not implemented for Gupshup API yet")
}
