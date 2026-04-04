package client

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/whatsapp/model"
	whatsapp_model "github.com/aseno/backend/pkg/whatsapp/model"
)

// Mock client to track calls
type mockClient struct {
	Name string
}

func (m *mockClient) CreateInstance(ctx context.Context, instance string) (*model.QRCodeData, error) {
	return &model.QRCodeData{Instance: m.Name}, nil
}
func (m *mockClient) SendTextMessage(ctx context.Context, instance, phone, message string) (*model.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockClient) SendButtonMessage(ctx context.Context, instance, phone, message string, buttons []model.MessageButton) (*model.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockClient) GetConnectionStatus(ctx context.Context, instance string) (*model.ConnectionStatus, error) {
	return nil, nil
}
func (m *mockClient) GetQRCode(ctx context.Context, instance string) (*model.QRCodeData, error) {
	return nil, nil
}
func (m *mockClient) Logout(ctx context.Context, instance string) error {
	return nil
}
func (m *mockClient) DeleteInstance(ctx context.Context, instance string) error {
	return nil
}
func (m *mockClient) DownloadMedia(ctx context.Context, instance, messageID string) ([]byte, string, error) {
	return nil, "", nil
}

// Mock Connection Repo
type mockConnRepo struct {
	conns map[string]*whatsapp_model.WhatsAppConnection
}

func (m *mockConnRepo) Create(ctx context.Context, conn *whatsapp_model.WhatsAppConnection) error {
	return nil
}
func (m *mockConnRepo) GetActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (*whatsapp_model.WhatsAppConnection, error) {
	return nil, nil
}
func (m *mockConnRepo) GetByInstance(ctx context.Context, instance string) (*whatsapp_model.WhatsAppConnection, error) {
	if conn, ok := m.conns[instance]; ok {
		return conn, nil
	}
	return nil, errors.ErrNotFoundInstance
}
func (m *mockConnRepo) UpdateConnected(ctx context.Context, clinicID uuid.UUID, connected bool) error {
	return nil
}

func (m *mockConnRepo) Update(ctx context.Context, conn *whatsapp_model.WhatsAppConnection) error {
	return nil
}
func (m *mockConnRepo) UpsertInstance(ctx context.Context, clinicID uuid.UUID, instance string) error {
	return nil
}
func (m *mockConnRepo) CountActiveByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error) {
	return 0, nil
}

func ptr(v string) *string {
	return &v
}

func TestProxyClient_Resolve(t *testing.T) {
	repo := &mockConnRepo{
		conns: map[string]*whatsapp_model.WhatsAppConnection{
			"inst-gupshup": {Provider: "GUPSHUP_OFFICIAL", Instance: ptr("inst-gupshup")},
			"inst-waha":    {Provider: "WAHA", Instance: ptr("inst-waha")},
			"inst-evo":     {Provider: "EVOLUTION", Instance: ptr("inst-evo")},
			"inst-null":    {Provider: "", Instance: ptr("inst-null")},
		},
	}

	mockEvo := &mockClient{Name: "evolution"}
	mockWaha := &mockClient{Name: "waha"}
	mockGup := &mockClient{Name: "gupshup"}

	proxy := NewProxyClient(repo, mockEvo, mockWaha, mockGup)

	// Test GUPSHUP_OFFICIAL routing
	res, _ := proxy.CreateInstance(context.Background(), "inst-gupshup")
	assert.Equal(t, "gupshup", res.Instance)

	// Test WAHA routing
	res, _ = proxy.CreateInstance(context.Background(), "inst-waha")
	assert.Equal(t, "waha", res.Instance)

	// Test EVOLUTION routing
	res, _ = proxy.CreateInstance(context.Background(), "inst-evo")
	assert.Equal(t, "evolution", res.Instance)

	// Test fallback for missing connection
	res, _ = proxy.CreateInstance(context.Background(), "inst-desconhecida")
	assert.Equal(t, "evolution", res.Instance)

	// Test fallback for missing provider field
	res, _ = proxy.CreateInstance(context.Background(), "inst-null")
	assert.Equal(t, "evolution", res.Instance)
}
