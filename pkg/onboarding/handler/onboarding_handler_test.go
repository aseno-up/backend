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
	"github.com/stretchr/testify/mock"

	auth_model "github.com/aseno/backend/pkg/auth/model"
	"github.com/aseno/backend/pkg/onboarding/model"
	onboarding_service "github.com/aseno/backend/pkg/onboarding/service"
)

// mockOnboardingService stands in for the real onboarding service in handler
// tests. Each method records expectations via testify/mock.
type mockOnboardingService struct {
	mock.Mock
}

func (m *mockOnboardingService) GenerateForLead(ctx context.Context, email, name, whatsapp, origem string) (string, *model.OnboardingToken, error) {
	args := m.Called(ctx, email, name, whatsapp, origem)
	tok, _ := args.Get(1).(*model.OnboardingToken)
	return args.String(0), tok, args.Error(2)
}

func (m *mockOnboardingService) Validate(ctx context.Context, token string) (*model.OnboardingToken, error) {
	args := m.Called(ctx, token)
	tok, _ := args.Get(0).(*model.OnboardingToken)
	return tok, args.Error(1)
}

func (m *mockOnboardingService) Complete(ctx context.Context, token string, req onboarding_service.CompleteRequest) (*auth_model.AuthResponse, error) {
	args := m.Called(ctx, token, req)
	resp, _ := args.Get(0).(*auth_model.AuthResponse)
	return resp, args.Error(1)
}

func setupOnboardingRouter(svc *mockOnboardingService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewOnboardingHandler(svc, nil)
	h.RegisterRoutes(r)
	return r
}

func TestCreateLandingLink_Success(t *testing.T) {
	svc := new(mockOnboardingService)
	router := setupOnboardingRouter(svc)

	svc.On("GenerateForLead", mock.Anything, "carla@test.com", "Carla", "(11) 99999-9999", "landing_inline_hero").
		Return("token-123", &model.OnboardingToken{ID: uuid.New()}, nil)

	body, _ := json.Marshal(map[string]string{
		"nome":     "Carla",
		"email":    "carla@test.com",
		"whatsapp": "(11) 99999-9999",
		"origem":   "landing_inline_hero",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/landing-link", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestCreateLandingLink_InvalidEmailReturns400(t *testing.T) {
	svc := new(mockOnboardingService)
	router := setupOnboardingRouter(svc)

	body, _ := json.Marshal(map[string]string{
		"email": "not-an-email",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads/landing-link", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "GenerateForLead", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetToken_Valid(t *testing.T) {
	svc := new(mockOnboardingService)
	router := setupOnboardingRouter(svc)

	exp := time.Now().Add(24 * time.Hour)
	svc.On("Validate", mock.Anything, "abc-token").Return(&model.OnboardingToken{
		Email:     "carla@test.com",
		Name:      "Carla",
		WhatsApp:  "(11) 99999-9999",
		ExpiresAt: exp,
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/onboarding-tokens/abc-token", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp tokenResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "carla@test.com", resp.Email)
	assert.Equal(t, "Carla", resp.Name)
}

func TestComplete_Success(t *testing.T) {
	svc := new(mockOnboardingService)
	router := setupOnboardingRouter(svc)

	svc.On("Complete", mock.Anything, "tok-1", onboarding_service.CompleteRequest{
		Name:       "Carla Silva",
		Password:   "longenough123",
		ClinicName: "Clínica Bem-Estar",
		Phone:      "",
	}).Return(&auth_model.AuthResponse{}, nil)

	body, _ := json.Marshal(map[string]string{
		"name":        "Carla Silva",
		"password":    "longenough123",
		"clinic_name": "Clínica Bem-Estar",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/onboarding-tokens/tok-1/complete", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestComplete_RejectsShortPassword(t *testing.T) {
	svc := new(mockOnboardingService)
	router := setupOnboardingRouter(svc)

	body, _ := json.Marshal(map[string]string{
		"name":     "Carla Silva",
		"password": "short",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/onboarding-tokens/tok-1/complete", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Complete", mock.Anything, mock.Anything, mock.Anything)
}
