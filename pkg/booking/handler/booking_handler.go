package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/booking/model"
	"github.com/aseno/backend/pkg/booking/service"
)

// BookingHandler defines the interface for public booking endpoints
type BookingHandler interface {
	RegisterRoutes(router gin.IRouter)
	GetClinicInfo(c *gin.Context)
	GetProfessionals(c *gin.Context)
	GetSpecialties(c *gin.Context)
	GetProcedures(c *gin.Context)
	GetAvailableSlots(c *gin.Context)
	CreateBooking(c *gin.Context)
}

// bookingHandler implements BookingHandler
type bookingHandler struct {
	service service.BookingService
}

// NewBookingHandler creates a new booking handler
func NewBookingHandler(svc service.BookingService) BookingHandler {
	return &bookingHandler{
		service: svc,
	}
}

// RegisterRoutes registers public booking routes (no auth required)
func (h *bookingHandler) RegisterRoutes(router gin.IRouter) {
	book := router.Group("/book")
	{
		book.GET("/:slug", h.GetClinicInfo)
		book.GET("/:slug/professionals", h.GetProfessionals)
		book.GET("/:slug/specialties", h.GetSpecialties)
		book.GET("/:slug/procedures", h.GetProcedures)
		book.GET("/:slug/slots", h.GetAvailableSlots)
		book.POST("/:slug/appointments", h.CreateBooking)
	}
}

// GetClinicInfo returns public clinic information for booking page
func (h *bookingHandler) GetClinicInfo(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	info, err := h.service.GetClinicInfo(c.Request.Context(), slug)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, info)
}

// GetProfessionals returns list of active professionals for booking
func (h *bookingHandler) GetProfessionals(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	professionals, err := h.service.GetProfessionals(c.Request.Context(), slug)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, professionals)
}

// GetSpecialties returns active specialties for a clinic
func (h *bookingHandler) GetSpecialties(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	specialties, err := h.service.GetSpecialties(c.Request.Context(), slug)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, specialties)
}

// GetProcedures returns procedures for a clinic, optionally filtered by specialty
func (h *bookingHandler) GetProcedures(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	var specialtyID *uuid.UUID
	if specIDStr := c.Query("specialty_id"); specIDStr != "" {
		parsed, err := uuid.Parse(specIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid specialty_id"})
			return
		}
		specialtyID = &parsed
	}

	procedures, err := h.service.GetProcedures(c.Request.Context(), slug, specialtyID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, procedures)
}

// GetAvailableSlots returns available time slots for a professional on a given date
func (h *bookingHandler) GetAvailableSlots(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	professionalIDStr := c.Query("professional_id")
	if professionalIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "professional_id is required"})
		return
	}

	professionalID, err := uuid.Parse(professionalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid professional_id"})
		return
	}

	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date is required"})
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, use YYYY-MM-DD"})
		return
	}

	// Optional procedure_id for duration-based slot calculation
	var procedureID *uuid.UUID
	if procIDStr := c.Query("procedure_id"); procIDStr != "" {
		parsed, err := uuid.Parse(procIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid procedure_id"})
			return
		}
		procedureID = &parsed
	}

	slots, err := h.service.GetAvailableSlots(c.Request.Context(), slug, professionalID, date, procedureID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, slots)
}

// CreateBooking creates a new booking from public form
func (h *bookingHandler) CreateBooking(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	var req model.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	booking, err := h.service.CreateBooking(c.Request.Context(), slug, &req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, booking)
}

// handleError handles errors and returns appropriate HTTP status
func handleError(c *gin.Context, err error) {
	if errors.IsNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": map[string]interface{}{
				"type":    "NOT_FOUND",
				"message": err.Error(),
			},
		})
		return
	}

	if errors.IsValidation(err) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"type":    "VALIDATION_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	if errors.IsSlotUnavailable(err) {
		c.JSON(http.StatusConflict, gin.H{
			"error": map[string]interface{}{
				"type":    "SLOT_UNAVAILABLE",
				"message": "Este horario nao esta mais disponivel",
			},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"error": map[string]interface{}{
			"type":    "INTERNAL_ERROR",
			"message": "Erro interno do servidor",
		},
	})
}
