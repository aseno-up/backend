package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/internal/errors"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	"github.com/aseno/backend/pkg/whatsapp/agent"
	"github.com/aseno/backend/pkg/whatsapp/classifier"
	whatsapp_model "github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/service"
	"github.com/aseno/backend/pkg/whatsapp/transcriber"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func ptr[T any](v T) *T { return &v }

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

// stubClinicRepo implementa clinic_repository.ClinicRepository em memória.
type stubClinicRepo struct {
	clinic *clinic_model.Clinic
	err    error
}

func (r *stubClinicRepo) Create(_ context.Context, _ *clinic_model.Clinic) error { return nil }
func (r *stubClinicRepo) GetByID(_ context.Context, _ uuid.UUID) (*clinic_model.Clinic, error) {
	return r.clinic, r.err
}
func (r *stubClinicRepo) GetBySlug(_ context.Context, _ string) (*clinic_model.Clinic, error) {
	return r.clinic, r.err
}
func (r *stubClinicRepo) GetByEmail(_ context.Context, _ string) (*clinic_model.Clinic, error) {
	return r.clinic, r.err
}
func (r *stubClinicRepo) Update(_ context.Context, _ *clinic_model.Clinic) error { return nil }
func (r *stubClinicRepo) Delete(_ context.Context, _ uuid.UUID) error            { return nil }
func (r *stubClinicRepo) List(_ context.Context, _, _ int) ([]*clinic_model.Clinic, int64, error) {
	return nil, 0, nil
}
func (r *stubClinicRepo) ExistsBySlug(_ context.Context, _ string) (bool, error)  { return false, nil }
func (r *stubClinicRepo) ExistsByEmail(_ context.Context, _ string) (bool, error) { return false, nil }

// stubConnRepo implementa WhatsAppConnectionRepository em memória.
type stubConnRepo struct {
	activeConn *whatsapp_model.WhatsAppConnection
	connErr    error
	byInstance map[string]*whatsapp_model.WhatsAppConnection
}

func newStubConnRepo() *stubConnRepo {
	return &stubConnRepo{byInstance: make(map[string]*whatsapp_model.WhatsAppConnection)}
}

func (r *stubConnRepo) Create(_ context.Context, conn *whatsapp_model.WhatsAppConnection) error {
	if conn.ID == uuid.Nil {
		conn.ID = uuid.New()
	}
	cp := *conn
	r.activeConn = &cp
	if conn.Instance != nil {
		r.byInstance[*conn.Instance] = &cp
	}
	return nil
}

func (r *stubConnRepo) GetActiveByClinicID(_ context.Context, _ uuid.UUID) (*whatsapp_model.WhatsAppConnection, error) {
	if r.connErr != nil {
		return nil, r.connErr
	}
	return r.activeConn, nil
}

func (r *stubConnRepo) GetByInstance(_ context.Context, instance string) (*whatsapp_model.WhatsAppConnection, error) {
	if conn, ok := r.byInstance[instance]; ok {
		return conn, nil
	}
	return nil, errors.ErrNotFoundInstance.WithMessage("not found")
}

func (r *stubConnRepo) UpdateConnected(_ context.Context, _ uuid.UUID, connected bool) error {
	if r.activeConn != nil {
		r.activeConn.Connected = connected
	}
	return nil
}

func (r *stubConnRepo) Update(_ context.Context, conn *whatsapp_model.WhatsAppConnection) error {
	if conn == nil {
		return nil
	}
	if conn.Instance != nil {
		r.byInstance[*conn.Instance] = conn
	}
	return nil
}

func (r *stubConnRepo) UpsertInstance(_ context.Context, clinicID uuid.UUID, instance string) error {
	conn := &whatsapp_model.WhatsAppConnection{
		ID:       uuid.New(),
		ClinicID: clinicID,
		Instance: ptr(instance),
		IsActive: true,
	}
	r.activeConn = conn
	r.byInstance[instance] = conn
	return nil
}

// stubWhatsAppSvc implementa service.WhatsAppService completamente.
type stubWhatsAppSvc struct {
	statusState string
	statusErr   error
}

func (s *stubWhatsAppSvc) CreateInstance(_ context.Context, _ string) (*whatsapp_model.QRCodeData, error) {
	return &whatsapp_model.QRCodeData{Base64: "stub-qr"}, nil
}
func (s *stubWhatsAppSvc) SendTextMessage(_ context.Context, _, _, _ string) (*whatsapp_model.SendMessageResponse, error) {
	return &whatsapp_model.SendMessageResponse{MessageID: "stub-id"}, nil
}
func (s *stubWhatsAppSvc) SendConfirmationMessage(_ context.Context, _ string, _ *whatsapp_model.ConfirmationMessage) error {
	return nil
}
func (s *stubWhatsAppSvc) SendReminderMessage(_ context.Context, _ string, _ *whatsapp_model.ReminderMessage) error {
	return nil
}
func (s *stubWhatsAppSvc) HandleIncomingMessage(_ context.Context, _ uuid.UUID, _ *whatsapp_model.ParsedIncomingMessage) error {
	return nil
}
func (s *stubWhatsAppSvc) ProcessDelayedAI(_ context.Context, _ *whatsapp_model.ScheduledMessage) error {
	return nil
}
func (s *stubWhatsAppSvc) ClassifyMessage(_ string) whatsapp_model.MessageIntent {
	return whatsapp_model.MessageIntentUnknown
}
func (s *stubWhatsAppSvc) GetConnectionStatus(_ context.Context, _ string) (*whatsapp_model.ConnectionStatus, error) {
	return &whatsapp_model.ConnectionStatus{State: s.statusState}, s.statusErr
}
func (s *stubWhatsAppSvc) GetQRCode(_ context.Context, _ string) (*whatsapp_model.QRCodeData, error) {
	return nil, nil
}
func (s *stubWhatsAppSvc) Logout(_ context.Context, _ string) error         { return nil }
func (s *stubWhatsAppSvc) DeleteInstance(_ context.Context, _ string) error { return nil }

// Set* methods — no-ops no stub
func (s *stubWhatsAppSvc) SetAppointmentFinder(_ service.AppointmentFinder)             {}
func (s *stubWhatsAppSvc) SetWaitlistHandler(_ service.WaitlistResponseHandler)         {}
func (s *stubWhatsAppSvc) SetMessageLogService(_ service.MessageLogService)             {}
func (s *stubWhatsAppSvc) SetAppointmentUpdater(_ service.AppointmentStatusUpdater)     {}
func (s *stubWhatsAppSvc) SetClassifier(_ classifier.MessageClassifier, _ float64)      {}
func (s *stubWhatsAppSvc) SetSlotSuggester(_ service.SlotSuggester)                     {}
func (s *stubWhatsAppSvc) SetAppointmentCreator(_ service.AppointmentCreator)           {}
func (s *stubWhatsAppSvc) SetSchedulingIntentRepo(_ service.SchedulingIntentRepository) {}
func (s *stubWhatsAppSvc) SetPatientFinderCreator(_ service.PatientFinderCreator)       {}
func (s *stubWhatsAppSvc) SetProfessionalLister(_ service.ProfessionalLister)           {}
func (s *stubWhatsAppSvc) SetSlotFinder(_ service.SlotFinder)                           {}
func (s *stubWhatsAppSvc) SetNewAppointmentCreator(_ service.NewAppointmentCreator)     {}
func (s *stubWhatsAppSvc) SetMessageQueue(_ service.MessageQueue)                       {}
func (s *stubWhatsAppSvc) SetAIAgentService(_ agent.Service)                            {}
func (s *stubWhatsAppSvc) SetClinicRepo(_ service.ClinicRepository)                     {}
func (s *stubWhatsAppSvc) SetScheduledMessageRepo(_ service.ScheduledMessageRepository) {}
func (s *stubWhatsAppSvc) SetDefaultDelayBetweenPatients(_ int)                         {}
func (s *stubWhatsAppSvc) SetDebounceDelay(_ time.Duration)                             {}
func (s *stubWhatsAppSvc) SetAudioTranscriber(_ transcriber.AudioTranscriber)           {}

// newTestRouter cria um router Gin com o handler e clinic_id injetados no contexto.
func newTestRouter(h WhatsAppHandler, clinicID uuid.UUID) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("clinic_id", clinicID)
		c.Next()
	})
	h.RegisterRoutes(r)
	return r
}

// ---------------------------------------------------------------------------
// Testes do GetStatus
// ---------------------------------------------------------------------------

// TestGetStatus_SemConexaoAtiva retorna "disconnected" quando não há conexão
// ativa cadastrada — fluxo de onboarding / clínica recém-criada.
func TestGetStatus_SemConexaoAtiva(t *testing.T) {
	clinicID := uuid.New()
	clinic := &clinic_model.Clinic{ID: clinicID, Name: "Test Clinic"}

	connRepo := newStubConnRepo()
	connRepo.connErr = errors.ErrNotFoundInstance

	h := NewWhatsAppHandler(&stubWhatsAppSvc{}, &stubClinicRepo{clinic: clinic}, connRepo, "", "evolution")
	router := newTestRouter(h, clinicID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/whatsapp/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "disconnected", resp["state"])
}

// TestGetStatus_ComConexaoAtiva retorna o estado real da conexão.
func TestGetStatus_ComConexaoAtiva(t *testing.T) {
	clinicID := uuid.New()
	clinic := &clinic_model.Clinic{ID: clinicID, Name: "Test Clinic"}

	connRepo := newStubConnRepo()
	_ = connRepo.UpsertInstance(context.Background(), clinicID, "test-instance")

	svc := &stubWhatsAppSvc{statusState: "open"}
	h := NewWhatsAppHandler(svc, &stubClinicRepo{clinic: clinic}, connRepo, "", "evolution")
	router := newTestRouter(h, clinicID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/whatsapp/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "open", resp["state"])
}

// ---------------------------------------------------------------------------
// Testes do Webhook
// ---------------------------------------------------------------------------

// TestWebhook_InstanceDesconhecida retorna 200 + status "unknown_instance".
// Crítico: uma instance desconhecida NÃO deve causar panic ou erro 500 —
// o WhatsApp API reenviaria o webhook infinitamente em caso de 5xx.
func TestWebhook_InstanceDesconhecida(t *testing.T) {
	connRepo := newStubConnRepo()
	h := NewWhatsAppHandler(&stubWhatsAppSvc{}, &stubClinicRepo{}, connRepo, "", "evolution")

	payload := map[string]interface{}{
		"event":    "messages.upsert",
		"instance": "instance-desconhecida",
		"data": map[string]interface{}{
			"key": map[string]interface{}{
				"remoteJid": "5511999990000@s.whatsapp.net",
				"fromMe":    false,
				"id":        "MSG-001",
			},
			"message": map[string]interface{}{
				"conversation": "oi",
			},
			"messageTimestamp": 1000000000,
		},
	}
	body, _ := json.Marshal(payload)

	router := gin.New()
	router.POST("/api/v1/whatsapp/webhook", h.Webhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/whatsapp/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "unknown_instance", resp["status"])
}

// TestWebhook_InstanceConhecida processa o payload e retorna "processed".
func TestWebhook_InstanceConhecida(t *testing.T) {
	clinicID := uuid.New()
	instance := "clinica-sp-abc12345"

	connRepo := newStubConnRepo()
	_ = connRepo.UpsertInstance(context.Background(), clinicID, instance)

	h := NewWhatsAppHandler(&stubWhatsAppSvc{}, &stubClinicRepo{}, connRepo, "", "evolution")

	payload := map[string]interface{}{
		"event":    "messages.upsert",
		"instance": instance,
		"data": map[string]interface{}{
			"key": map[string]interface{}{
				"remoteJid": "5511999990001@s.whatsapp.net",
				"fromMe":    false,
				"id":        "MSG-002",
			},
			"message": map[string]interface{}{
				"conversation": "quero agendar",
			},
			"messageTimestamp": 1000000001,
		},
	}
	body, _ := json.Marshal(payload)

	router := gin.New()
	router.POST("/api/v1/whatsapp/webhook", h.Webhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/whatsapp/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "processed", resp["status"])
}

// ---------------------------------------------------------------------------
// Testes do Disconnect
// ---------------------------------------------------------------------------

// TestDisconnect_SemConexaoAtiva retorna 400 quando não há conexão para desconectar.
func TestDisconnect_SemConexaoAtiva(t *testing.T) {
	clinicID := uuid.New()
	clinic := &clinic_model.Clinic{ID: clinicID, Name: "Test Clinic"}

	connRepo := newStubConnRepo()
	connRepo.connErr = errors.ErrNotFoundInstance

	h := NewWhatsAppHandler(&stubWhatsAppSvc{}, &stubClinicRepo{clinic: clinic}, connRepo, "", "evolution")
	router := newTestRouter(h, clinicID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/whatsapp/disconnect", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Testes do SendMessage
// ---------------------------------------------------------------------------

// TestSendMessage_SemWhatsAppConectado retorna 400 quando Connected=false.
func TestSendMessage_SemWhatsAppConectado(t *testing.T) {
	clinicID := uuid.New()
	clinic := &clinic_model.Clinic{ID: clinicID, Name: "Test Clinic"}

	connRepo := newStubConnRepo()
	connRepo.activeConn = &whatsapp_model.WhatsAppConnection{
		ID:        uuid.New(),
		ClinicID:  clinicID,
		Instance:  ptr("test-instance"),
		Connected: false,
		IsActive:  true,
	}

	h := NewWhatsAppHandler(&stubWhatsAppSvc{}, &stubClinicRepo{clinic: clinic}, connRepo, "", "evolution")
	router := newTestRouter(h, clinicID)

	body, _ := json.Marshal(map[string]string{"phone": "5511999990000", "message": "Olá!"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/whatsapp/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestSendMessage_ComWhatsAppConectado envia mensagem com sucesso (200).
func TestSendMessage_ComWhatsAppConectado(t *testing.T) {
	clinicID := uuid.New()
	clinic := &clinic_model.Clinic{ID: clinicID, Name: "Test Clinic"}

	connRepo := newStubConnRepo()
	connRepo.activeConn = &whatsapp_model.WhatsAppConnection{
		ID:        uuid.New(),
		ClinicID:  clinicID,
		Instance:  ptr("test-instance"),
		Connected: true,
		IsActive:  true,
	}

	h := NewWhatsAppHandler(&stubWhatsAppSvc{}, &stubClinicRepo{clinic: clinic}, connRepo, "", "evolution")
	router := newTestRouter(h, clinicID)

	body, _ := json.Marshal(map[string]string{"phone": "5511999990000", "message": "Olá!"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/whatsapp/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "sent", resp["status"])
}
