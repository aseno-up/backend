package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/auth/model"
	"github.com/saas/backend/pkg/auth/service"
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
	ChangePassword(c *gin.Context)
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
	}
}

// RegisterProtectedRoutes registers protected authentication routes
func (h *authHandler) RegisterProtectedRoutes(router gin.IRouter) {
	auth := router.Group("/auth")
	{
		auth.POST("/logout", h.Logout)
		auth.GET("/me", h.GetCurrentUser)
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	response, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

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

	user, err := h.authService.GetCurrentUser(c.Request.Context(), userID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, user)
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
