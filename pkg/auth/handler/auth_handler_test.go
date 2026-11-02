package handler_test

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

	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/auth/handler"
	"github.com/aseno/backend/pkg/auth/model"
	"github.com/aseno/backend/pkg/auth/service"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	user_model "github.com/aseno/backend/pkg/user/model"
)

type mockAuthService struct {
	mock.Mock
}

func (m *mockAuthService) Register(ctx context.Context, req *model.RegisterRequest) (*model.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthService) Login(ctx context.Context, req *model.LoginRequest) (*model.AuthResponse, error) {
	args := m.Called(ctx, req)
	if r := args.Get(0); r != nil {
		return r.(*model.AuthResponse), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockAuthService) GoogleLogin(ctx context.Context, req *model.GoogleLoginRequest) (*model.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*model.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthService) Logout(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}
func (m *mockAuthService) GetCurrentUser(ctx context.Context, userID, currentClinicID uuid.UUID) (*model.CurrentUserResponse, error) {
	return nil, nil
}
func (m *mockAuthService) SwitchClinic(ctx context.Context, userID, clinicID uuid.UUID) (*model.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthService) ChangePassword(ctx context.Context, userID uuid.UUID, req *model.ChangePasswordRequest) error {
	return nil
}
func (m *mockAuthService) RequestPasswordReset(ctx context.Context, req *model.RequestPasswordResetRequest) error {
	return nil
}
func (m *mockAuthService) ResetPassword(ctx context.Context, req *model.ResetPasswordRequest) error {
	return nil
}
func (m *mockAuthService) SendVerificationEmail(ctx context.Context, userID uuid.UUID) error {
	return nil
}
func (m *mockAuthService) VerifyEmail(ctx context.Context, token string) error { return nil }
func (m *mockAuthService) ResendVerificationEmail(ctx context.Context, email string) error {
	return nil
}
func (m *mockAuthService) SetPaymentService(_ service.PaymentService)           {}
func (m *mockAuthService) SetTemplateSeedService(_ service.TemplateSeedService) {}

var _ service.AuthService = (*mockAuthService)(nil)

type mockAuditService struct {
	mock.Mock
}

func (m *mockAuditService) Log(ctx context.Context, entry audit_service.AuditEntry) error {
	args := m.Called(entry)
	return args.Error(0)
}
func (m *mockAuditService) LogFromGin(c *gin.Context, action audit_model.AuditAction, entityType audit_model.AuditEntityType, entityID string, oldData, newData interface{}) error {
	args := m.Called(action, entityType, entityID)
	return args.Error(0)
}
func (m *mockAuditService) List(ctx context.Context, filter *audit_model.AuditLogFilter) (*audit_model.AuditLogResponse, error) {
	return nil, nil
}

func TestLogin_Success_EmitsLoginAudit(t *testing.T) {
	userID := uuid.New()
	clinicID := uuid.New()
	response := &model.AuthResponse{
		User:         &user_model.UserResponse{ID: userID, ClinicID: clinicID, Email: "user@example.com", Role: user_model.UserRoleAdmin},
		Clinic:       &clinic_model.ClinicResponse{ID: clinicID, Name: "Acme"},
		AccessToken:  "access",
		RefreshToken: "refresh",
	}

	authSvc := new(mockAuthService)
	authSvc.On("Login", mock.Anything, mock.Anything).Return(response, nil).Once()

	auditSvc := new(mockAuditService)
	auditSvc.On("Log", mock.MatchedBy(func(e audit_service.AuditEntry) bool {
		return e.Action == audit_model.AuditActionLogin &&
			e.EntityType == audit_model.AuditEntityUser &&
			e.UserID == userID &&
			e.ClinicID == clinicID &&
			e.UserEmail == "user@example.com"
	})).Return(nil).Once()

	gin.SetMode(gin.TestMode)
	h := handler.NewAuthHandler(authSvc, handler.WithAuditService(auditSvc))
	router := gin.New()
	h.RegisterRoutes(router)

	body, _ := json.Marshal(model.LoginRequest{Email: "user@example.com", Password: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	authSvc.AssertExpectations(t)
	auditSvc.AssertExpectations(t)
}

func TestLogin_Failure_DoesNotEmitAudit(t *testing.T) {
	authSvc := new(mockAuthService)
	authSvc.On("Login", mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

	auditSvc := new(mockAuditService)

	gin.SetMode(gin.TestMode)
	h := handler.NewAuthHandler(authSvc, handler.WithAuditService(auditSvc))
	router := gin.New()
	h.RegisterRoutes(router)

	body, _ := json.Marshal(model.LoginRequest{Email: "user@example.com", Password: "bad"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	auditSvc.AssertNotCalled(t, "Log")
}

func TestLogout_EmitsLogoutAudit(t *testing.T) {
	userID := uuid.New()
	clinicID := uuid.New()

	authSvc := new(mockAuthService)
	authSvc.On("Logout", mock.Anything, userID).Return(nil).Once()

	auditSvc := new(mockAuditService)
	auditSvc.On("LogFromGin",
		audit_model.AuditActionLogout,
		audit_model.AuditEntityUser,
		userID.String(),
	).Return(nil).Once()

	gin.SetMode(gin.TestMode)
	h := handler.NewAuthHandler(authSvc, handler.WithAuditService(auditSvc))
	router := gin.New()
	// Mirror what the auth middleware injects on protected routes.
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("clinic_id", clinicID)
		c.Set("email", "user@example.com")
		c.Set("user_role", "admin")
		c.Next()
	})
	router.POST("/auth/logout", h.Logout)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	authSvc.AssertExpectations(t)
	auditSvc.AssertExpectations(t)
}
