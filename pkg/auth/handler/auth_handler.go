package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/auth/model"
	"github.com/aseno/backend/pkg/auth/service"
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
	authService service.AuthService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService service.AuthService) AuthHandler {
	return &authHandler{
		authService: authService,
	}
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
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	response, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Set HttpOnly cookies
	// MaxAge: 15 minutes for access token, 7 days for refresh token
	// Secure: false (dev), HttpOnly: true
	c.SetCookie("access_token", response.AccessToken, 900, "/", "", false, true)
	c.SetCookie("refresh_token", response.RefreshToken, 604800, "/", "", false, true)

	c.JSON(http.StatusCreated, response)
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
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

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
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

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
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

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
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

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
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

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
