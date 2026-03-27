package gupshup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConnRepo implements a minimal mock for WhatsAppConnectionRepository
type mockConnRepo struct {
	conn *model.WhatsAppConnection
	err  error
}

func (m *mockConnRepo) GetByInstance(ctx context.Context, instance string) (*model.WhatsAppConnection, error) {
	return m.conn, m.err
}

func (m *mockConnRepo) Create(ctx context.Context, conn *model.WhatsAppConnection) error { return nil }
func (m *mockConnRepo) GetActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.WhatsAppConnection, error) {
	return nil, nil
}
func (m *mockConnRepo) Update(ctx context.Context, conn *model.WhatsAppConnection) error { return nil }
func (m *mockConnRepo) UpdateConnected(ctx context.Context, clinicID uuid.UUID, connected bool) error {
	return nil
}
func (m *mockConnRepo) UpsertInstance(ctx context.Context, clinicID uuid.UUID, instance string) error {
	return nil
}

func strPtr(s string) *string { return &s }

func TestSendTextMessage_SelfServe(t *testing.T) {
	// Self-serve: API key comes from per-clinic Token
	clinicAPIKey := "clinic-api-key-abc"
	sourcePhone := "5511999990000"
	conn := &model.WhatsAppConnection{
		Token:         &clinicAPIKey,
		PhoneNumberID: &sourcePhone,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/wa/api/v1/msg", r.URL.Path)
		assert.Equal(t, clinicAPIKey, r.Header.Get("apikey"), "should use per-clinic Token as API key")
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		params := string(body)
		assert.Contains(t, params, "channel=whatsapp")
		assert.Contains(t, params, "source="+sourcePhone)
		assert.Contains(t, params, "destination=5511888880000")
		assert.Contains(t, params, "src.name=ClinicaDraMaria")

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "submitted",
			"messageId": "gs-msg-id-001",
		})
	}))
	defer server.Close()

	repo := &mockConnRepo{conn: conn}
	client := NewGupshupClient(Config{BaseURL: server.URL}, repo)

	resp, err := client.SendTextMessage(context.Background(), "ClinicaDraMaria", "5511888880000", "Olá, tudo bem?")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "gs-msg-id-001", resp.MessageID)
	assert.Equal(t, "submitted", resp.Status)
}

func TestSendTextMessage_PartnerFallback(t *testing.T) {
	// Partner mode: no Token per-clinic, falls back to global config APIKey
	sourcePhone := "5511999990000"
	conn := &model.WhatsAppConnection{
		PhoneNumberID: &sourcePhone,
		// Token is nil — no per-clinic key
	}

	partnerKey := "partner-global-key-xyz"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, partnerKey, r.Header.Get("apikey"), "should fall back to global partner API key")

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "submitted",
			"messageId": "gs-msg-id-002",
		})
	}))
	defer server.Close()

	repo := &mockConnRepo{conn: conn}
	client := NewGupshupClient(Config{BaseURL: server.URL, APIKey: partnerKey}, repo)

	resp, err := client.SendTextMessage(context.Background(), "MyApp", "5511888880000", "Test")

	require.NoError(t, err)
	assert.Equal(t, "gs-msg-id-002", resp.MessageID)
}

func TestSendTextMessage_NoAPIKey(t *testing.T) {
	// No Token, no global key — should fail
	sourcePhone := "5511999990000"
	conn := &model.WhatsAppConnection{
		PhoneNumberID: &sourcePhone,
	}

	repo := &mockConnRepo{conn: conn}
	client := NewGupshupClient(Config{BaseURL: "http://unused"}, repo)

	resp, err := client.SendTextMessage(context.Background(), "MyApp", "5511888880000", "Test")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "API key not configured")
}

func TestSendTextMessage_APIError(t *testing.T) {
	conn := &model.WhatsAppConnection{
		Token:         strPtr("key"),
		PhoneNumberID: strPtr("5511999990000"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid API key"})
	}))
	defer server.Close()

	repo := &mockConnRepo{conn: conn}
	client := NewGupshupClient(Config{BaseURL: server.URL}, repo)

	resp, err := client.SendTextMessage(context.Background(), "MyApp", "5511888880000", "Test")
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSendButtonMessage(t *testing.T) {
	conn := &model.WhatsAppConnection{
		Token:         strPtr("clinic-key"),
		PhoneNumberID: strPtr("5511999990000"),
	}

	t.Run("sends interactive button message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			params := string(body)
			assert.Contains(t, params, "message=")

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    "submitted",
				"messageId": "gs-btn-msg-001",
			})
		}))
		defer server.Close()

		repo := &mockConnRepo{conn: conn}
		client := NewGupshupClient(Config{BaseURL: server.URL}, repo)

		buttons := []model.MessageButton{
			{ID: "confirm", Title: "Confirmar"},
			{ID: "cancel", Title: "Cancelar"},
		}
		resp, err := client.SendButtonMessage(context.Background(), "MyApp", "5511888880000", "Deseja confirmar?", buttons)

		require.NoError(t, err)
		assert.Equal(t, "gs-btn-msg-001", resp.MessageID)
	})

	t.Run("falls back to text when buttons fail", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"message": "Interactive not supported"})
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    "submitted",
				"messageId": "gs-text-fallback-001",
			})
		}))
		defer server.Close()

		repo := &mockConnRepo{conn: conn}
		client := NewGupshupClient(Config{BaseURL: server.URL}, repo)

		buttons := []model.MessageButton{{ID: "ok", Title: "OK"}}
		resp, err := client.SendButtonMessage(context.Background(), "MyApp", "5511888880000", "Escolha", buttons)

		require.NoError(t, err)
		assert.Equal(t, "gs-text-fallback-001", resp.MessageID)
		assert.Equal(t, 2, callCount)
	})
}

func TestDownloadMedia(t *testing.T) {
	conn := &model.WhatsAppConnection{
		Token:         strPtr("clinic-key"),
		PhoneNumberID: strPtr("5511999990000"),
	}

	t.Run("downloads media successfully", func(t *testing.T) {
		audioBytes := []byte("fake-audio-data")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/wa/api/v1/msg/media/msg-123")
			assert.Equal(t, "clinic-key", r.Header.Get("apikey"))

			w.Header().Set("Content-Type", "audio/ogg")
			w.Write(audioBytes)
		}))
		defer server.Close()

		repo := &mockConnRepo{conn: conn}
		client := NewGupshupClient(Config{BaseURL: server.URL}, repo)

		data, mimeType, err := client.DownloadMedia(context.Background(), "MyApp", "msg-123")

		require.NoError(t, err)
		assert.Equal(t, audioBytes, data)
		assert.Equal(t, "audio/ogg", mimeType)
	})

	t.Run("returns error on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}))
		defer server.Close()

		repo := &mockConnRepo{conn: conn}
		client := NewGupshupClient(Config{BaseURL: server.URL}, repo)

		data, _, err := client.DownloadMedia(context.Background(), "MyApp", "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, data)
	})
}

func TestCleanPhone(t *testing.T) {
	assert.Equal(t, "5511999990000", cleanPhone("5511999990000@s.whatsapp.net"))
	assert.Equal(t, "5511999990000", cleanPhone("5511999990000@c.us"))
	assert.Equal(t, "5511999990000", cleanPhone("+5511999990000"))
	assert.Equal(t, "5511999990000", cleanPhone("55 11 99999-0000"))
	assert.Equal(t, "5511999990000", cleanPhone("5511999990000"))
}

func TestGetConnectionStatus_WithCredentials(t *testing.T) {
	conn := &model.WhatsAppConnection{
		Token:         strPtr("valid-key"),
		PhoneNumberID: strPtr("5511999990000"),
	}
	repo := &mockConnRepo{conn: conn}
	client := NewGupshupClient(Config{BaseURL: "http://unused"}, repo)

	status, err := client.GetConnectionStatus(context.Background(), "MyApp")
	require.NoError(t, err)
	assert.Equal(t, "open", status.State)
}

func TestGetConnectionStatus_NoCredentials(t *testing.T) {
	repo := &mockConnRepo{conn: nil, err: assert.AnError}
	client := NewGupshupClient(Config{BaseURL: "http://unused"}, repo)

	status, err := client.GetConnectionStatus(context.Background(), "MyApp")
	require.NoError(t, err)
	assert.Equal(t, "disconnected", status.State)
}

func TestSendTextMessage_NoConnection(t *testing.T) {
	repo := &mockConnRepo{conn: nil, err: assert.AnError}
	client := NewGupshupClient(Config{BaseURL: "http://unused"}, repo)

	resp, err := client.SendTextMessage(context.Background(), "UnknownApp", "5511888880000", "Test")
	assert.Error(t, err)
	assert.Nil(t, resp)
}
