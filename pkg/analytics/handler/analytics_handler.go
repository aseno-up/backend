package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/analytics/model"
	"github.com/aseno/backend/pkg/analytics/service"
)

// AnalyticsHandler defines the interface for analytics handlers
type AnalyticsHandler interface {
	RegisterRoutes(router gin.IRouter)
	GetSlotAnalytics(c *gin.Context)
	GetBestSlots(c *gin.Context)
	GetWorstSlots(c *gin.Context)
	GetReportMetrics(c *gin.Context)
	GetFinancialReport(c *gin.Context)
}

// analyticsHandler implements AnalyticsHandler
type analyticsHandler struct {
	service service.AnalyticsService
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(service service.AnalyticsService) AnalyticsHandler {
	return &analyticsHandler{service: service}
}

// RegisterRoutes registers analytics routes
func (h *analyticsHandler) RegisterRoutes(router gin.IRouter) {
	analytics := router.Group("/analytics")
	{
		analytics.GET("/slots", h.GetSlotAnalytics)
		analytics.GET("/slots/best", h.GetBestSlots)
		analytics.GET("/slots/worst", h.GetWorstSlots)
		analytics.GET("/reports", h.GetReportMetrics)
		analytics.GET("/financial", h.GetFinancialReport)
	}
}

// GetSlotAnalytics handles getting complete slot analytics
func (h *analyticsHandler) GetSlotAnalytics(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	filter := &model.AnalyticsFilter{
		Period: c.DefaultQuery("period", "6m"),
	}

	// Parse optional professional_id
	if professionalIDStr := c.Query("professional_id"); professionalIDStr != "" {
		professionalID, err := uuid.Parse(professionalIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid professional_id")))
			return
		}
		filter.ProfessionalID = &professionalID
	}

	analytics, err := h.service.GetSlotAnalytics(c.Request.Context(), clinicID.(uuid.UUID), filter)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// GetBestSlots handles getting the best performing time slots
func (h *analyticsHandler) GetBestSlots(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit < 1 || limit > 20 {
		limit = 5
	}

	// Parse optional professional_id
	var professionalID *uuid.UUID
	if professionalIDStr := c.Query("professional_id"); professionalIDStr != "" {
		id, err := uuid.Parse(professionalIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid professional_id")))
			return
		}
		professionalID = &id
	}

	slots, err := h.service.GetBestSlots(c.Request.Context(), clinicID.(uuid.UUID), professionalID, limit)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"best_slots": slots,
		"count":      len(slots),
	})
}

// GetWorstSlots handles getting the worst performing time slots
func (h *analyticsHandler) GetWorstSlots(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit < 1 || limit > 20 {
		limit = 5
	}

	// Parse optional professional_id
	var professionalID *uuid.UUID
	if professionalIDStr := c.Query("professional_id"); professionalIDStr != "" {
		id, err := uuid.Parse(professionalIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid professional_id")))
			return
		}
		professionalID = &id
	}

	slots, err := h.service.GetWorstSlots(c.Request.Context(), clinicID.(uuid.UUID), professionalID, limit)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"worst_slots": slots,
		"count":       len(slots),
	})
}

// GetReportMetrics handles getting the "Relatórios que DOEM" metrics
func (h *analyticsHandler) GetReportMetrics(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	filter := &model.ReportFilter{
		Period: c.DefaultQuery("period", "3m"),
	}

	// Parse optional professional_id
	if professionalIDStr := c.Query("professional_id"); professionalIDStr != "" {
		professionalID, err := uuid.Parse(professionalIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid professional_id")))
			return
		}
		filter.ProfessionalID = &professionalID
	}

	metrics, err := h.service.GetReportMetrics(c.Request.Context(), clinicID.(uuid.UUID), filter)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// GetFinancialReport handles getting the financial report
func (h *analyticsHandler) GetFinancialReport(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	filter := &model.FinancialFilter{
		Month: c.Query("month"),
	}

	if professionalIDStr := c.Query("professional_id"); professionalIDStr != "" {
		professionalID, err := uuid.Parse(professionalIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Indentificador de profissional inválido")))
			return
		}
		filter.ProfessionalID = &professionalID
	}

	report, err := h.service.GetFinancialReport(c.Request.Context(), clinicID.(uuid.UUID), filter)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, report)
}
