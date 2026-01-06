package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/invite/model"
	"github.com/saas/backend/pkg/invite/service"
)

// InviteHandler defines the interface for invite handlers
type InviteHandler interface {
	RegisterPublicRoutes(router gin.IRouter)
	RegisterProtectedRoutes(router gin.IRouter)
	CreateInvite(c *gin.Context)
	ListInvites(c *gin.Context)
	CancelInvite(c *gin.Context)
	ResendInvite(c *gin.Context)
	GetInviteByToken(c *gin.Context)
	AcceptInvite(c *gin.Context)
}

// inviteHandler implements InviteHandler
type inviteHandler struct {
	inviteService service.InviteService
}

// NewInviteHandler creates a new invite handler
func NewInviteHandler(inviteService service.InviteService) InviteHandler {
	return &inviteHandler{
		inviteService: inviteService,
	}
}

// RegisterPublicRoutes registers public invite routes
func (h *inviteHandler) RegisterPublicRoutes(router gin.IRouter) {
	auth := router.Group("/auth")
	{
		auth.GET("/invite/:token", h.GetInviteByToken)
		auth.POST("/accept-invite", h.AcceptInvite)
	}
}

// RegisterProtectedRoutes registers protected invite routes (Admin only)
func (h *inviteHandler) RegisterProtectedRoutes(router gin.IRouter) {
	invites := router.Group("/invites")
	{
		invites.POST("", h.CreateInvite)
		invites.GET("", h.ListInvites)
		invites.DELETE("/:id", h.CancelInvite)
		invites.POST("/:id/resend", h.ResendInvite)
	}
}

// CreateInvite handles creating a new invite
// @Summary Create a new invite
// @Tags Invites
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body model.CreateInviteRequest true "Invite data"
// @Success 201 {object} model.InviteResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Router /invites [post]
func (h *inviteHandler) CreateInvite(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req model.CreateInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	response, err := h.inviteService.CreateInvite(c.Request.Context(), clinicID.(uuid.UUID), userID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, response)
}

// ListInvites handles listing invites for a clinic
// @Summary List invites
// @Tags Invites
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} model.InviteListResponse
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Router /invites [get]
func (h *inviteHandler) ListInvites(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	response, err := h.inviteService.ListInvites(c.Request.Context(), clinicID.(uuid.UUID), page, perPage)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// CancelInvite handles canceling an invite
// @Summary Cancel an invite
// @Tags Invites
// @Security BearerAuth
// @Param id path string true "Invite ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /invites/{id} [delete]
func (h *inviteHandler) CancelInvite(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	inviteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Invalid invite ID")))
		return
	}

	if err := h.inviteService.CancelInvite(c.Request.Context(), inviteID, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invite canceled successfully"})
}

// ResendInvite handles resending an invite
// @Summary Resend an invite
// @Tags Invites
// @Security BearerAuth
// @Param id path string true "Invite ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /invites/{id}/resend [post]
func (h *inviteHandler) ResendInvite(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	inviteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Invalid invite ID")))
		return
	}

	if err := h.inviteService.ResendInvite(c.Request.Context(), inviteID, clinicID.(uuid.UUID), userID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invite resent successfully"})
}

// GetInviteByToken handles getting invite details by token (public)
// @Summary Get invite details by token
// @Tags Auth
// @Produce json
// @Param token path string true "Invite token"
// @Success 200 {object} model.InviteDetailsResponse
// @Failure 404 {object} errors.AppError
// @Failure 410 {object} errors.AppError
// @Router /auth/invite/{token} [get]
func (h *inviteHandler) GetInviteByToken(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Token is required")))
		return
	}

	response, err := h.inviteService.GetInviteByToken(c.Request.Context(), token)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// AcceptInvite handles accepting an invite and creating a user (public)
// @Summary Accept an invite
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.AcceptInviteRequest true "Accept invite data"
// @Success 200 {object} auth_model.AuthResponse
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Failure 410 {object} errors.AppError
// @Router /auth/accept-invite [post]
func (h *inviteHandler) AcceptInvite(c *gin.Context) {
	var req model.AcceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	response, err := h.inviteService.AcceptInvite(c.Request.Context(), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, response)
}
