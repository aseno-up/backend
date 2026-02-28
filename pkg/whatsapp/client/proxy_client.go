package client

import (
	"context"

	"github.com/aseno/backend/pkg/whatsapp/model"
	whatsapp_repository "github.com/aseno/backend/pkg/whatsapp/repository"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// ProxyClient implements service.WhatsAppClient and routes requests to the correct
// underlying provider instance based on the clinic's connection configuration.
type ProxyClient struct {
	connRepo  whatsapp_repository.WhatsAppConnectionRepository
	evolution service.WhatsAppClient
	waha      service.WhatsAppClient
	gupshup   service.WhatsAppClient
}

// NewProxyClient creates a new ProxyClient
func NewProxyClient(
	connRepo whatsapp_repository.WhatsAppConnectionRepository,
	evolution service.WhatsAppClient,
	waha service.WhatsAppClient,
	gupshup service.WhatsAppClient,
) service.WhatsAppClient {
	return &ProxyClient{
		connRepo:  connRepo,
		evolution: evolution,
		waha:      waha,
		gupshup:   gupshup,
	}
}

// resolveClient helper to find the correct client based on the instance
func (p *ProxyClient) resolveClient(ctx context.Context, instance string) (service.WhatsAppClient, error) {
	conn, err := p.connRepo.GetByInstance(ctx, instance)
	if err != nil {
		// If instance is not found, fallback to evolution (legacy behavior)
		if p.evolution != nil {
			return p.evolution, nil
		}
		return nil, err
	}

	switch conn.Provider {
	case "GUPSHUP_OFFICIAL":
		if p.gupshup != nil {
			return p.gupshup, nil
		}
	case "WAHA":
		if p.waha != nil {
			return p.waha, nil
		}
	case "EVOLUTION":
		if p.evolution != nil {
			return p.evolution, nil
		}
	}

	// Fallback to whatever is available
	if p.evolution != nil {
		return p.evolution, nil
	}
	if p.waha != nil {
		return p.waha, nil
	}
	return p.gupshup, nil
}

// CreateInstance delegates to the appropriate client
func (p *ProxyClient) CreateInstance(ctx context.Context, instance string) (*model.QRCodeData, error) {
	client, err := p.resolveClient(ctx, instance)
	if err != nil || client == nil {
		return nil, err
	}
	return client.CreateInstance(ctx, instance)
}

// SendTextMessage delegates to the appropriate client
func (p *ProxyClient) SendTextMessage(ctx context.Context, instance, phone, message string) (*model.SendMessageResponse, error) {
	client, err := p.resolveClient(ctx, instance)
	if err != nil || client == nil {
		return nil, err
	}
	return client.SendTextMessage(ctx, instance, phone, message)
}

// SendButtonMessage delegates to the appropriate client
func (p *ProxyClient) SendButtonMessage(ctx context.Context, instance, phone, message string, buttons []model.MessageButton) (*model.SendMessageResponse, error) {
	client, err := p.resolveClient(ctx, instance)
	if err != nil || client == nil {
		return nil, err
	}
	return client.SendButtonMessage(ctx, instance, phone, message, buttons)
}

// GetConnectionStatus delegates to the appropriate client
func (p *ProxyClient) GetConnectionStatus(ctx context.Context, instance string) (*model.ConnectionStatus, error) {
	client, err := p.resolveClient(ctx, instance)
	if err != nil || client == nil {
		return nil, err
	}
	return client.GetConnectionStatus(ctx, instance)
}

// GetQRCode delegates to the appropriate client
func (p *ProxyClient) GetQRCode(ctx context.Context, instance string) (*model.QRCodeData, error) {
	client, err := p.resolveClient(ctx, instance)
	if err != nil || client == nil {
		return nil, err
	}
	return client.GetQRCode(ctx, instance)
}

// Logout delegates to the appropriate client
func (p *ProxyClient) Logout(ctx context.Context, instance string) error {
	client, err := p.resolveClient(ctx, instance)
	if err != nil || client == nil {
		return err
	}
	return client.Logout(ctx, instance)
}

// DeleteInstance delegates to the appropriate client
func (p *ProxyClient) DeleteInstance(ctx context.Context, instance string) error {
	client, err := p.resolveClient(ctx, instance)
	if err != nil || client == nil {
		return err
	}
	return client.DeleteInstance(ctx, instance)
}

// DownloadMedia delegates to the appropriate client
func (p *ProxyClient) DownloadMedia(ctx context.Context, instance, messageID string) ([]byte, string, error) {
	client, err := p.resolveClient(ctx, instance)
	if err != nil || client == nil {
		return nil, "", err
	}
	return client.DownloadMedia(ctx, instance, messageID)
}
