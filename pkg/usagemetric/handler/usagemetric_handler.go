package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/usagemetric/model"
	"github.com/aseno/backend/pkg/usagemetric/service"
)

// UsageMetricHandler defines the interface for usage metric endpoints
type UsageMetricHandler interface {
	RegisterRoutes(router gin.IRouter)
	List(c *gin.Context)
	Summary(c *gin.Context)
}

type usageMetricHandler struct {
	service service.UsageMetricService
}

// NewUsageMetricHandler creates a new usage metric handler
func NewUsageMetricHandler(service service.UsageMetricService) UsageMetricHandler {
	return &usageMetricHandler{service: service}
}

func (h *usageMetricHandler) RegisterRoutes(router gin.IRouter) {
	group := router.Group("/usage-metrics")
	{
		group.GET("", h.List)
		group.GET("/summary", h.Summary)
	}
}

func (h *usageMetricHandler) List(c *gin.Context) {
	var filter model.UsageMetricFilter
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

func (h *usageMetricHandler) Summary(c *gin.Context) {
	startDate := c.DefaultQuery("start_date", "")
	endDate := c.DefaultQuery("end_date", "")

	result, err := h.service.GetAllClinicsSummary(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}
