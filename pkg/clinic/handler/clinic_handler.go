package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/clinic/model"
	"github.com/aseno/backend/pkg/clinic/service"
)

// ClinicHandler defines the interface for clinic handlers
type ClinicHandler interface {
	RegisterRoutes(router gin.IRouter)
	GetClinic(c *gin.Context)
	UpdateClinic(c *gin.Context)
}

// clinicHandler implements ClinicHandler
type clinicHandler struct {
	service  service.ClinicService
	auditSvc audit_service.AuditService
}

// NewClinicHandler creates a new clinic handler. The audit service may be nil
// in test wiring; when set, settings changes emit an AuditEntitySettings
// UPDATE record with old and new values so the trail is auditable.
func NewClinicHandler(service service.ClinicService, auditSvc audit_service.AuditService) ClinicHandler {
	return &clinicHandler{
		service:  service,
		auditSvc: auditSvc,
	}
}

// RegisterRoutes registers clinic routes
func (h *clinicHandler) RegisterRoutes(router gin.IRouter) {
	clinic := router.Group("/clinic")
	{
		clinic.GET("", h.GetClinic)
		clinic.PUT("", h.UpdateClinic)
	}
}

// GetClinic returns the current clinic information
func (h *clinicHandler) GetClinic(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	clinic, err := h.service.GetByID(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, clinic)
}

// UpdateClinic updates the current clinic information
func (h *clinicHandler) UpdateClinic(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	// Check if user is admin or owner
	userRole, exists := c.Get("user_role")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	// user_role can be either string or model.UserRole depending on context
	var role string
	switch r := userRole.(type) {
	case string:
		role = r
	default:
		role = fmt.Sprintf("%v", r)
	}

	if role != "admin" && role != "owner" {
		c.JSON(http.StatusForbidden, errors.ToResponse(errors.ErrForbiddenInstance.WithMessage("Only admin or owner can update clinic information")))
		return
	}

	var req model.UpdateClinicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	var oldData interface{}
	if h.auditSvc != nil {
		if old, err := h.service.GetByID(c.Request.Context(), clinicID.(uuid.UUID)); err == nil {
			oldData = old
		}
	}

	clinic, err := h.service.Update(c.Request.Context(), clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		entity := audit_model.AuditEntityClinic
		if req.Settings != nil {
			entity = audit_model.AuditEntitySettings
		}
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, entity, clinicID.(uuid.UUID).String(), oldData, clinic)
	}

	c.JSON(http.StatusOK, clinic)
}
