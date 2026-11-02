package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

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

// List returns audit logs. The route is mounted under the dev-only admin
// panel (see devOnlyMiddleware in cmd/api/main.go), so callers are already
// restricted to ClinicTypeDev tenants; results span every tenant unless the
// caller narrows with ?clinic_id=<uuid>.
func (h *auditHandler) List(c *gin.Context) {
	var filter model.AuditLogFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.List(c.Request.Context(), &filter)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}
