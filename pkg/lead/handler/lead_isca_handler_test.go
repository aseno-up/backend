package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	email_service "github.com/aseno/backend/pkg/email/service"
	"github.com/aseno/backend/pkg/lead/model"
)

type mockEmailService struct {
	mock.Mock
}

func (m *mockEmailService) SendInvite(ctx context.Context, data *email_service.InviteEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendWelcome(ctx context.Context, data *email_service.WelcomeEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendPasswordReset(ctx context.Context, data *email_service.ResetPasswordEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendEmailVerification(ctx context.Context, data *email_service.EmailVerificationData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendOwnerWelcome(ctx context.Context, data *email_service.OwnerWelcomeEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendContactForm(ctx context.Context, data *email_service.ContactFormData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendPaymentReminder(ctx context.Context, data *email_service.PaymentReminderEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendOnboardingLink(ctx context.Context, data *email_service.OnboardingLinkEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendBookingConfirmation(ctx context.Context, data *email_service.BookingConfirmationEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendPortalEmailVerification(ctx context.Context, data *email_service.PortalEmailVerificationData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendLeadIsca(ctx context.Context, data *email_service.LeadIscaEmailData) error {
	return m.Called(ctx, data).Error(0)
}

type mockLeadRepo struct {
	mock.Mock
}

func (m *mockLeadRepo) Create(ctx context.Context, lead *model.Lead) error {
	return m.Called(ctx, lead).Error(0)
}
func (m *mockLeadRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Lead, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Lead), args.Error(1)
}
func (m *mockLeadRepo) List(ctx context.Context, filter model.LeadFilter) ([]*model.Lead, int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*model.Lead), args.Get(1).(int64), args.Error(2)
}
func (m *mockLeadRepo) Update(ctx context.Context, lead *model.Lead) error {
	return m.Called(ctx, lead).Error(0)
}
func (m *mockLeadRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockLeadRepo) GetByExternalID(ctx context.Context, externalID string) (*model.Lead, error) {
	args := m.Called(ctx, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Lead), args.Error(1)
}

func setupIscaRouter(repo *mockLeadRepo, emailSvc *mockEmailService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	var svc email_service.EmailService
	if emailSvc != nil {
		svc = emailSvc
	}

	h := NewLeadIscaHandler(repo, svc, "testdata")
	h.RegisterRoutes(r)
	return r
}

func TestIsca_CreateSuccess(t *testing.T) {
	repo := new(mockLeadRepo)
	router := setupIscaRouter(repo, nil)

	repo.On("Create", mock.Anything, mock.MatchedBy(func(l *model.Lead) bool {
		return l.Status == model.LeadStatusIsca &&
			l.ContactName == "João" &&
			l.Email == "joao@example.com" &&
			l.Phone == "(11) 99999-9999"
	})).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"nome":     "João",
		"email":    "joao@example.com",
		"whatsapp": "(11) 99999-9999",
		"origem":   "isca_kit_planilhas",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/isca", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Kit enviado com sucesso", resp["message"])

	repo.AssertExpectations(t)
}

func TestIsca_MissingEmail(t *testing.T) {
	repo := new(mockLeadRepo)
	router := setupIscaRouter(repo, nil)

	body, _ := json.Marshal(map[string]string{
		"nome": "João",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/isca", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestIsca_MissingNome(t *testing.T) {
	repo := new(mockLeadRepo)
	router := setupIscaRouter(repo, nil)

	body, _ := json.Marshal(map[string]string{
		"email": "joao@example.com",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/isca", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestIsca_InvalidEmail(t *testing.T) {
	repo := new(mockLeadRepo)
	router := setupIscaRouter(repo, nil)

	body, _ := json.Marshal(map[string]string{
		"nome":  "João",
		"email": "not-an-email",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/isca", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestIsca_LeadSavedWithIscaStatus(t *testing.T) {
	repo := new(mockLeadRepo)
	router := setupIscaRouter(repo, nil)

	repo.On("Create", mock.Anything, mock.MatchedBy(func(l *model.Lead) bool {
		return l.Status == model.LeadStatusIsca &&
			l.Channel == model.LeadChannelInbound &&
			l.ClinicName == "Landing Page — isca_kit_planilhas"
	})).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"nome":   "Maria",
		"email":  "maria@test.com",
		"origem": "isca_kit_planilhas",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/isca", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
}

func TestIsca_NonKitOrigemDoesNotSendKitEmail(t *testing.T) {
	repo := new(mockLeadRepo)
	emailSvc := new(mockEmailService)
	router := setupIscaRouter(repo, emailSvc)

	repo.On("Create", mock.Anything, mock.MatchedBy(func(l *model.Lead) bool {
		return l.Status == model.LeadStatusIsca &&
			l.ClinicName == "Landing Page — landing_inline_hero"
	})).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"nome":   "Carla",
		"email":  "carla@test.com",
		"origem": "landing_inline_hero",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/isca", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
	// The handler must not try to read the kit_planilhas spreadsheets for
	// non-kit origens. We assert SendLeadIsca was never invoked.
	emailSvc.AssertNotCalled(t, "SendLeadIsca", mock.Anything, mock.Anything)
}

func TestIsca_EngagementModalOrigemDoesNotSendKitEmail(t *testing.T) {
	repo := new(mockLeadRepo)
	emailSvc := new(mockEmailService)
	router := setupIscaRouter(repo, emailSvc)

	repo.On("Create", mock.Anything, mock.MatchedBy(func(l *model.Lead) bool {
		return l.ClinicName == "Landing Page — landing_modal"
	})).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"nome":   "Pedro",
		"email":  "pedro@test.com",
		"origem": "landing_modal",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/isca", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertExpectations(t)
	emailSvc.AssertNotCalled(t, "SendLeadIsca", mock.Anything, mock.Anything)
}

func TestIsca_RepoErrorReturns500(t *testing.T) {
	repo := new(mockLeadRepo)
	router := setupIscaRouter(repo, nil)

	repo.On("Create", mock.Anything, mock.Anything).Return(assert.AnError)

	body, _ := json.Marshal(map[string]string{
		"nome":  "João",
		"email": "joao@example.com",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/isca", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	repo.AssertExpectations(t)
}
