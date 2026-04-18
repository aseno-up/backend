package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	invite_model "github.com/aseno/backend/pkg/invite/model"
	invite_service "github.com/aseno/backend/pkg/invite/service"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/professional/model"
	"github.com/aseno/backend/pkg/professional/service"
	user_model "github.com/aseno/backend/pkg/user/model"
)

// ProfessionalHandler defines the interface for professional handlers
type ProfessionalHandler interface {
	RegisterRoutes(router gin.IRouter)
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	BulkDelete(c *gin.Context)
}

// professionalHandler implements ProfessionalHandler
type professionalHandler struct {
	service   service.ProfessionalService
	inviteSvc invite_service.InviteService
	auditSvc  audit_service.AuditService
}

// NewProfessionalHandler creates a new professional handler
func NewProfessionalHandler(service service.ProfessionalService, inviteSvc invite_service.InviteService, auditSvc audit_service.AuditService) ProfessionalHandler {
	return &professionalHandler{service: service, inviteSvc: inviteSvc, auditSvc: auditSvc}
}

// RegisterRoutes registers professional routes
func (h *professionalHandler) RegisterRoutes(router gin.IRouter) {
	professionals := router.Group("/professionals")
	{
		professionals.POST("", h.Create)
		professionals.GET("", h.List)
		professionals.GET("/:id", h.GetByID)
		professionals.PUT("/:id", h.Update)
	}
}

// createProfessionalRequest represents the new POST /professionals request
// Either email (sends invite) or user_id (links to existing user) must be provided
type createProfessionalRequest struct {
	Email  string     `json:"email" binding:"omitempty,email"`
	UserID *uuid.UUID `json:"user_id"`
}

// Create handles creating a new professional via invite (email) or linking to existing user (user_id)
func (h *professionalHandler) Create(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	var req createProfessionalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if req.Email == "" && req.UserID == nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage(
			"Informe o email para enviar convite ou o user_id para vincular a um usuário existente")))
		return
	}

	ctx := c.Request.Context()

	if req.UserID != nil {
		// Option 2: Link to existing user (owner/admin)
		professional, err := h.service.CreateForExistingUser(ctx, clinicID.(uuid.UUID), *req.UserID)
		if err != nil {
			statusCode := errors.GetStatusCode(err)
			c.JSON(statusCode, errors.ToResponse(err))
			return
		}

		if h.auditSvc != nil {
			_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntityProfessional, professional.ID.String(), nil, professional)
		}

		c.JSON(http.StatusCreated, professional)
		return
	}

	// Option 1: Send invite by email
	inviteReq := &invite_model.CreateInviteRequest{
		Email:               req.Email,
		Role:                user_model.UserRoleProfessional,
		NewProfessionalName: req.Email, // Will be updated when user accepts invite
	}

	invite, err := h.inviteSvc.CreateInvite(ctx, clinicID.(uuid.UUID), userID.(uuid.UUID), inviteReq)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, invite)
}

// GetByID handles getting a professional by ID
func (h *professionalHandler) GetByID(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid professional ID")))
		return
	}

	professional, err := h.service.GetByID(c.Request.Context(), id, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, professional)
}

// List handles listing professionals
func (h *professionalHandler) List(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	var filter pagination.Filter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	// Legacy support for activeOnly (if front-end didn't send ?status=active)
	activeOnly := c.DefaultQuery("active_only", "")
	if activeOnly == "true" && filter.Status == "" {
		filter.Status = "active"
	}

	result, err := h.service.List(c.Request.Context(), clinicID.(uuid.UUID), filter)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// Update handles updating a professional
func (h *professionalHandler) Update(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid professional ID")))
		return
	}

	var req model.UpdateProfessionalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	// Get old data for audit
	var oldData interface{}
	if h.auditSvc != nil {
		old, _ := h.service.GetByID(c.Request.Context(), id, clinicID.(uuid.UUID))
		oldData = old
	}

	professional, err := h.service.Update(c.Request.Context(), id, clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityProfessional, id.String(), oldData, professional)
	}

	c.JSON(http.StatusOK, professional)
}

// Delete handles deleting a professional
func (h *professionalHandler) Delete(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid professional ID")))
		return
	}

	// Get old data for audit
	var oldData interface{}
	if h.auditSvc != nil {
		old, _ := h.service.GetByID(c.Request.Context(), id, clinicID.(uuid.UUID))
		oldData = old
	}

	if err := h.service.Delete(c.Request.Context(), id, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityProfessional, id.String(), oldData, nil)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Professional deleted successfully"})
}

// BulkDelete handles deleting multiple professionals
func (h *professionalHandler) BulkDelete(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.service.BulkDelete(c.Request.Context(), req.IDs, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		for _, id := range req.IDs {
			_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityProfessional, id.String(), nil, nil)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Professionals deleted successfully"})
}
