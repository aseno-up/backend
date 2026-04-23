package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/auth/model"
	"github.com/aseno/backend/pkg/auth/service"
	systemlog_model "github.com/aseno/backend/pkg/systemlog/model"
	systemlog_service "github.com/aseno/backend/pkg/systemlog/service"
)

// AuthHandler defines the interface for authentication handlers
type AuthHandler interface {
	RegisterRoutes(router gin.IRouter)
	Register(c *gin.Context)
	Login(c *gin.Context)
	GoogleLogin(c *gin.Context)
	RefreshToken(c *gin.Context)
	Logout(c *gin.Context)
	GetCurrentUser(c *gin.Context)
	SwitchClinic(c *gin.Context)
	ChangePassword(c *gin.Context)
	RequestPasswordReset(c *gin.Context)
	ResetPassword(c *gin.Context)
	VerifyEmail(c *gin.Context)
	ResendVerificationEmail(c *gin.Context)
}

// authHandler implements AuthHandler
type authHandler struct {
	authService  service.AuthService
	auditSvc     audit_service.AuditService
	systemLogSvc systemlog_service.SystemLogService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService service.AuthService, opts ...func(*authHandler)) AuthHandler {
	h := &authHandler{authService: authService}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// WithAuditService wires an audit service so login/logout leave a trail.
func WithAuditService(svc audit_service.AuditService) func(*authHandler) {
	return func(h *authHandler) {
		h.auditSvc = svc
	}
}

// WithSystemLogService wires a system log service so anonymous auth failures
// (failed register / failed reset-password) can be inspected without tying
// them to a tenant. AuditLog cannot be used for these because it requires a
// clinic_id/user_id, which don't exist for these events.
func WithSystemLogService(svc systemlog_service.SystemLogService) func(*authHandler) {
	return func(h *authHandler) {
		h.systemLogSvc = svc
	}
}

// recordAuthSystemEvent logs an auth event to the system log (used for anonymous
// or failed events that cannot be tied to a clinic_id in the audit log).
func (h *authHandler) recordAuthSystemEvent(c *gin.Context, event string, status systemlog_model.LogStatus, payload interface{}, errMsg string) {
	if h.systemLogSvc == nil {
		return
	}
	_ = h.systemLogSvc.Log(c.Request.Context(), systemlog_service.SystemLogEntry{
		Service: "auth",
		Event:   event,
		Status:  status,
		Payload: payload,
		Error:   errMsg,
	})
}

// recordAuthEvent emits an audit entry for a login/logout. Login endpoints are
// public, so we can't rely on the gin context's clinic_id/user_id — we pull
// those from the auth response instead.
func (h *authHandler) recordAuthEvent(c *gin.Context, action audit_model.AuditAction, response *model.AuthResponse) {
	if h.auditSvc == nil || response == nil || response.User == nil || response.Clinic == nil {
		return
	}
	_ = h.auditSvc.Log(c.Request.Context(), audit_service.AuditEntry{
		ClinicID:   response.Clinic.ID,
		UserID:     response.User.ID,
		UserEmail:  response.User.Email,
		UserRole:   string(response.User.Role),
		Action:     action,
		EntityType: audit_model.AuditEntityUser,
		EntityID:   response.User.ID.String(),
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	})
}

// RegisterRoutes registers authentication routes
func (h *authHandler) RegisterRoutes(router gin.IRouter) {
	auth := router.Group("/auth")
	{
		// Public routes
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/google", h.GoogleLogin)
		auth.POST("/refresh", h.RefreshToken)
		auth.POST("/forgot-password", h.RequestPasswordReset)
		auth.POST("/reset-password", h.ResetPassword)
		auth.POST("/verify-email", h.VerifyEmail)
		auth.POST("/resend-verification", h.ResendVerificationEmail)
	}
}

// RegisterProtectedRoutes registers protected authentication routes
func (h *authHandler) RegisterProtectedRoutes(router gin.IRouter) {
	auth := router.Group("/auth")
	{
		auth.POST("/logout", h.Logout)
		auth.GET("/me", h.GetCurrentUser)
		auth.POST("/switch-clinic", h.SwitchClinic)
		auth.POST("/change-password", h.ChangePassword)
	}
}

// Register handles user registration
// @Summary Register a new clinic and user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.RegisterRequest true "Registration data"
// @Success 201 {object} model.AuthResponse
// @Failure 400 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Router /auth/register [post]
func (h *authHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.recordAuthSystemEvent(c, "register_validation_failed", systemlog_model.LogStatusFailed, gin.H{
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}, err.Error())
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	response, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		// Capture failed registration so admins can inspect attempts that never
		// produced a clinic (duplicate email, invalid plan, etc).
		h.recordAuthSystemEvent(c, "register_failed", systemlog_model.LogStatusFailed, gin.H{
			"email":       req.Email,
			"name":        req.Name,
			"clinic_name": req.ClinicName,
			"plan":        req.Plan,
			"billing":     req.Billing,
			"variant":     req.Variant,
			"phone":       req.Phone,
			"ip":          c.ClientIP(),
			"user_agent":  c.Request.UserAgent(),
		}, err.Error())
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Successful registration is an auditable tenant event.
	h.recordRegisterAudit(c, response, &req)

	// Set HttpOnly cookies
	// MaxAge: 15 minutes for access token, 7 days for refresh token
	// Secure: false (dev), HttpOnly: true
	c.SetCookie("access_token", response.AccessToken, 900, "/", "", false, true)
	c.SetCookie("refresh_token", response.RefreshToken, 604800, "/", "", false, true)

	c.JSON(http.StatusCreated, response)
}

// recordRegisterAudit writes the successful registration to the audit log.
// We deliberately omit the password and document (CPF/CNPJ) from new_data —
// email, name, clinic name, plan and phone are enough to trace who signed up.
func (h *authHandler) recordRegisterAudit(c *gin.Context, response *model.AuthResponse, req *model.RegisterRequest) {
	if h.auditSvc == nil || response == nil || response.User == nil || response.Clinic == nil {
		return
	}
	_ = h.auditSvc.Log(c.Request.Context(), audit_service.AuditEntry{
		ClinicID:   response.Clinic.ID,
		UserID:     response.User.ID,
		UserEmail:  response.User.Email,
		UserRole:   string(response.User.Role),
		Action:     audit_model.AuditActionCreate,
		EntityType: audit_model.AuditEntityUser,
		EntityID:   response.User.ID.String(),
		NewData: gin.H{
			"email":       response.User.Email,
			"name":        response.User.Name,
			"phone":       req.Phone,
			"clinic_name": req.ClinicName,
			"plan":        req.Plan,
			"billing":     req.Billing,
			"variant":     req.Variant,
			"source":      "register",
		},
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
}

// Login handles user login
// @Summary Login with email and password
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.LoginRequest true "Login credentials"
// @Success 200 {object} model.AuthResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /auth/login [post]
func (h *authHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	response, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		h.recordAuthSystemEvent(c, "login_failed", systemlog_model.LogStatusFailed, gin.H{
			"email":      req.Email,
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}, err.Error())
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	h.recordAuthEvent(c, audit_model.AuditActionLogin, response)

	// Set HttpOnly cookies
	// MaxAge: 15 minutes for access token, 7 days for refresh token
	// Secure: false (dev), HttpOnly: true
	c.SetCookie("access_token", response.AccessToken, 900, "/", "", false, true)
	c.SetCookie("refresh_token", response.RefreshToken, 604800, "/", "", false, true)

	c.JSON(http.StatusOK, response)
}

// GoogleLogin handles Google OAuth login
// @Summary Login with Google OAuth
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.GoogleAuthRequest true "Google ID token"
// @Success 200 {object} model.AuthResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /auth/google [post]
func (h *authHandler) GoogleLogin(c *gin.Context) {
	var req model.GoogleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	response, err := h.authService.GoogleLogin(c.Request.Context(), &req)
	if err != nil {
		h.recordAuthSystemEvent(c, "google_login_failed", systemlog_model.LogStatusFailed, gin.H{
			"plan":       req.Plan,
			"billing":    req.Billing,
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}, err.Error())
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// If the user was created in the last 10 seconds, this Google call was the
	// registration — emit a CREATE audit on top of the LOGIN event so the
	// sign-up is visible in the tenant audit trail.
	if response != nil && response.User != nil && response.Clinic != nil {
		if time.Since(response.User.CreatedAt) < 10*time.Second {
			_ = h.auditSvc // guard nil check happens inside
			if h.auditSvc != nil {
				_ = h.auditSvc.Log(c.Request.Context(), audit_service.AuditEntry{
					ClinicID:   response.Clinic.ID,
					UserID:     response.User.ID,
					UserEmail:  response.User.Email,
					UserRole:   string(response.User.Role),
					Action:     audit_model.AuditActionCreate,
					EntityType: audit_model.AuditEntityUser,
					EntityID:   response.User.ID.String(),
					NewData: gin.H{
						"email":   response.User.Email,
						"name":    response.User.Name,
						"plan":    req.Plan,
						"billing": req.Billing,
						"source":  "google",
					},
					IP:        c.ClientIP(),
					UserAgent: c.Request.UserAgent(),
				})
			}
		}
	}

	h.recordAuthEvent(c, audit_model.AuditActionLogin, response)

	// Set HttpOnly cookies
	c.SetCookie("access_token", response.AccessToken, 900, "/", "", false, true)
	c.SetCookie("refresh_token", response.RefreshToken, 604800, "/", "", false, true)

	c.JSON(http.StatusOK, response)
}

// RefreshToken handles token refresh
// @Summary Refresh access token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.RefreshRequest true "Refresh token"
// @Success 200 {object} model.AuthResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /auth/refresh [post]
func (h *authHandler) RefreshToken(c *gin.Context) {
	var req model.RefreshRequest

	// Check cookie first
	cookieToken, err := c.Cookie("refresh_token")
	if err == nil && cookieToken != "" {
		req.RefreshToken = cookieToken
	} else {
		// Fallback to JSON body
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
			return
		}
	}

	response, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Set HttpOnly cookies
	c.SetCookie("access_token", response.AccessToken, 900, "/", "", false, true)
	c.SetCookie("refresh_token", response.RefreshToken, 604800, "/", "", false, true)

	c.JSON(http.StatusOK, response)
}

// Logout handles user logout
// @Summary Logout and invalidate refresh token
// @Tags Auth
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} errors.AppError
// @Router /auth/logout [post]
func (h *authHandler) Logout(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	if err := h.authService.Logout(c.Request.Context(), userID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Logout lives on the protected group, so the auth middleware has already
	// populated clinic_id/user_id/email/user_role on the gin context, which is
	// exactly what LogFromGin expects.
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionLogout, audit_model.AuditEntityUser, userID.(uuid.UUID).String(), nil, nil)
	}

	// Clear cookies
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}

// GetCurrentUser handles getting current user data
// @Summary Get current user data
// @Tags Auth
// @Security BearerAuth
// @Success 200 {object} model.CurrentUserResponse
// @Failure 401 {object} errors.AppError
// @Router /auth/me [get]
func (h *authHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	// Pass the clinic_id from the JWT (set by auth middleware) so that after
	// SwitchClinic the active clinic — not the user's owned clinic — is
	// returned. Falls back to uuid.Nil (default) if the middleware didn't set
	// it for some reason.
	var currentClinicID uuid.UUID
	if v, ok := c.Get("clinic_id"); ok {
		if cid, ok := v.(uuid.UUID); ok {
			currentClinicID = cid
		}
	}

	user, err := h.authService.GetCurrentUser(c.Request.Context(), userID.(uuid.UUID), currentClinicID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, user)
}

// SwitchClinic handles switching the user's active clinic
func (h *authHandler) SwitchClinic(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req model.SwitchClinicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	response, err := h.authService.SwitchClinic(c.Request.Context(), userID.(uuid.UUID), req.ClinicID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Set new cookies
	c.SetCookie("access_token", response.AccessToken, 900, "/", "", false, true)
	c.SetCookie("refresh_token", response.RefreshToken, 604800, "/", "", false, true)

	c.JSON(http.StatusOK, response)
}

// ChangePassword handles password change
// @Summary Change user password
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body model.ChangePasswordRequest true "Password change data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /auth/change-password [post]
func (h *authHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), userID.(uuid.UUID), &req); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Authenticated endpoint — middleware has populated user context.
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityUser, userID.(uuid.UUID).String(), nil, gin.H{"field": "password"})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// RequestPasswordReset handles password reset request
// @Summary Request password reset
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.RequestPasswordResetRequest true "Email"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Router /auth/forgot-password [post]
func (h *authHandler) RequestPasswordReset(c *gin.Context) {
	var req model.RequestPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.authService.RequestPasswordReset(c.Request.Context(), &req); err != nil {
		h.recordAuthSystemEvent(c, "password_reset_requested_failed", systemlog_model.LogStatusFailed, gin.H{
			"email":      req.Email,
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}, err.Error())
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Public endpoint, no user context in gin — record to system log so admins
	// can still inspect who requested resets, even when the email doesn't match
	// any account (the service returns success either way for privacy).
	h.recordAuthSystemEvent(c, "password_reset_requested", systemlog_model.LogStatusSuccess, gin.H{
		"email":      req.Email,
		"ip":         c.ClientIP(),
		"user_agent": c.Request.UserAgent(),
	}, "")

	c.JSON(http.StatusOK, gin.H{"message": "If an account exists with this email, instructions have been sent."})
}

// ResetPassword handles password reset completion
// @Summary Reset password
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.ResetPasswordRequest true "Token and new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /auth/reset-password [post]
func (h *authHandler) ResetPassword(c *gin.Context) {
	var req model.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.authService.ResetPassword(c.Request.Context(), &req); err != nil {
		h.recordAuthSystemEvent(c, "password_reset_failed", systemlog_model.LogStatusFailed, gin.H{
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}, err.Error())
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	h.recordAuthSystemEvent(c, "password_reset_completed", systemlog_model.LogStatusSuccess, gin.H{
		"ip":         c.ClientIP(),
		"user_agent": c.Request.UserAgent(),
	}, "")

	c.JSON(http.StatusOK, gin.H{"message": "Password successfully reset."})
}

// VerifyEmail handles email verification
// @Summary Verify email with token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.VerifyEmailRequest true "Verification token"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Router /auth/verify-email [post]
func (h *authHandler) VerifyEmail(c *gin.Context) {
	var req model.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.authService.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		h.recordAuthSystemEvent(c, "email_verification_failed", systemlog_model.LogStatusFailed, gin.H{
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}, err.Error())
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	h.recordAuthSystemEvent(c, "email_verified", systemlog_model.LogStatusSuccess, gin.H{
		"ip":         c.ClientIP(),
		"user_agent": c.Request.UserAgent(),
	}, "")

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully."})
}

// ResendVerificationEmail handles resending verification email
// @Summary Resend verification email
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.ResendVerificationRequest true "Email"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Router /auth/resend-verification [post]
func (h *authHandler) ResendVerificationEmail(c *gin.Context) {
	var req model.ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.authService.ResendVerificationEmail(c.Request.Context(), req.Email); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If an account exists with this email and is not verified, a verification email has been sent."})
}
