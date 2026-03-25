package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/systemlog/model"
	"github.com/aseno/backend/pkg/systemlog/service"
)

// SystemLogHandler defines the interface for system log endpoints
type SystemLogHandler interface {
	RegisterRoutes(router gin.IRouter)
	List(c *gin.Context)
}

type systemLogHandler struct {
	service service.SystemLogService
}

// NewSystemLogHandler creates a new system log handler
func NewSystemLogHandler(service service.SystemLogService) SystemLogHandler {
	return &systemLogHandler{service: service}
}

func (h *systemLogHandler) RegisterRoutes(router gin.IRouter) {
	group := router.Group("/system-logs")
	{
		group.GET("", h.List)
	}
}

func (h *systemLogHandler) List(c *gin.Context) {
	var filter model.SystemLogFilter
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
