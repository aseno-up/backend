package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/dashboard/service"
)

type DashboardHandler struct {
	service service.DashboardService
}

func NewDashboardHandler(service service.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

func (h *DashboardHandler) GetMetrics(c *gin.Context) {
	clinicIDVal, exists := c.Get("clinic_id")
	if !exists {
		c.Error(errors.ErrUnauthorizedInstance)
		return
	}
	clinicID := clinicIDVal.(uuid.UUID)

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	now := time.Now()
	// Default to first day of current month to now
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	endDate := now

	if startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			// Set to end of day
			endDate = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, time.Local)
		}
	}

	metrics, err := h.service.GetMetrics(c.Request.Context(), clinicID, startDate, endDate)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, metrics)
}
