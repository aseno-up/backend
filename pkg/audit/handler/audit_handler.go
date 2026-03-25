package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/audit/model"
	"github.com/aseno/backend/pkg/audit/service"
)

// AuditHandler defines the interface for audit endpoints
type AuditHandler interface {
	RegisterRoutes(router gin.IRouter)
	List(c *gin.Context)
}

type auditHandler struct {
	service service.AuditService
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(service service.AuditService) AuditHandler {
	return &auditHandler{service: service}
}

func (h *auditHandler) RegisterRoutes(router gin.IRouter) {
	group := router.Group("/audit-logs")
	{
		group.GET("", h.List)
	}
}

func (h *auditHandler) List(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var filter model.AuditLogFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.List(c.Request.Context(), clinicID.(uuid.UUID), &filter)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}
