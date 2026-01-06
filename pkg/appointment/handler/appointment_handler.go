package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/appointment/service"
)

// AppointmentHandler defines the interface for appointment handlers
type AppointmentHandler interface {
	RegisterRoutes(router gin.IRouter)
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	List(c *gin.Context)
	GetByDay(c *gin.Context)
	GetByWeek(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Confirm(c *gin.Context)
	Cancel(c *gin.Context)
	Complete(c *gin.Context)
	MarkNoShow(c *gin.Context)
}

// appointmentHandler implements AppointmentHandler
type appointmentHandler struct {
	service service.AppointmentService
}

// NewAppointmentHandler creates a new appointment handler
func NewAppointmentHandler(service service.AppointmentService) AppointmentHandler {
	return &appointmentHandler{service: service}
}

// RegisterRoutes registers appointment routes
func (h *appointmentHandler) RegisterRoutes(router gin.IRouter) {
	appointments := router.Group("/appointments")
	{
		appointments.POST("", h.Create)
		appointments.GET("", h.List)
		appointments.GET("/day/:date", h.GetByDay)
		appointments.GET("/week/:date", h.GetByWeek)
		appointments.GET("/:id", h.GetByID)
		appointments.PUT("/:id", h.Update)
		appointments.DELETE("/:id", h.Delete)
		appointments.POST("/:id/confirm", h.Confirm)
		appointments.POST("/:id/cancel", h.Cancel)
		appointments.POST("/:id/complete", h.Complete)
		appointments.POST("/:id/no-show", h.MarkNoShow)
	}
}

// Create handles creating a new appointment
func (h *appointmentHandler) Create(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	var req model.CreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	appointment, err := h.service.Create(c.Request.Context(), clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, appointment)
}

// GetByID handles getting an appointment by ID
func (h *appointmentHandler) GetByID(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	appointment, err := h.service.GetByID(c.Request.Context(), id, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, appointment)
}

// List handles listing appointments with filters
func (h *appointmentHandler) List(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	filter := &model.AppointmentFilter{}

	// Parse query parameters
	filter.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	filter.PerPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if professionalID := c.Query("professional_id"); professionalID != "" {
		if id, err := uuid.Parse(professionalID); err == nil {
			filter.ProfessionalID = &id
		}
	}

	if patientID := c.Query("patient_id"); patientID != "" {
		if id, err := uuid.Parse(patientID); err == nil {
			filter.PatientID = &id
		}
	}

	if status := c.Query("status"); status != "" {
		filter.Status = model.AppointmentStatus(status)
	}

	if startDate := c.Query("start_date"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			filter.StartDate = &t
		}
	}

	if endDate := c.Query("end_date"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Second)
			filter.EndDate = &endOfDay
		}
	}

	result, err := h.service.List(c.Request.Context(), clinicID.(uuid.UUID), filter)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetByDay handles getting appointments for a specific day
func (h *appointmentHandler) GetByDay(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	dateParam := c.Param("date")
	date, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid date format. Use YYYY-MM-DD")))
		return
	}

	var professionalID *uuid.UUID
	if profID := c.Query("professional_id"); profID != "" {
		if id, err := uuid.Parse(profID); err == nil {
			professionalID = &id
		}
	}

	appointments, err := h.service.GetByDay(c.Request.Context(), clinicID.(uuid.UUID), date, professionalID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": appointments, "date": dateParam})
}

// GetByWeek handles getting appointments for a week
func (h *appointmentHandler) GetByWeek(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	dateParam := c.Param("date")
	date, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid date format. Use YYYY-MM-DD")))
		return
	}

	var professionalID *uuid.UUID
	if profID := c.Query("professional_id"); profID != "" {
		if id, err := uuid.Parse(profID); err == nil {
			professionalID = &id
		}
	}

	appointments, err := h.service.GetByWeek(c.Request.Context(), clinicID.(uuid.UUID), date, professionalID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": appointments, "week_of": dateParam})
}

// Update handles updating an appointment
func (h *appointmentHandler) Update(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	var req model.UpdateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	appointment, err := h.service.Update(c.Request.Context(), id, clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, appointment)
}

// Delete handles deleting an appointment
func (h *appointmentHandler) Delete(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	if err := h.service.Delete(c.Request.Context(), id, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Appointment deleted successfully"})
}

// Confirm handles confirming an appointment
func (h *appointmentHandler) Confirm(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	var req model.ConfirmAppointmentRequest
	_ = c.ShouldBindJSON(&req) // Optional body

	appointment, err := h.service.Confirm(c.Request.Context(), id, clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, appointment)
}

// Cancel handles canceling an appointment
func (h *appointmentHandler) Cancel(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	var req model.CancelAppointmentRequest
	_ = c.ShouldBindJSON(&req) // Optional body

	appointment, err := h.service.Cancel(c.Request.Context(), id, clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, appointment)
}

// Complete handles marking an appointment as completed
func (h *appointmentHandler) Complete(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	appointment, err := h.service.Complete(c.Request.Context(), id, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, appointment)
}

// MarkNoShow handles marking an appointment as no-show
func (h *appointmentHandler) MarkNoShow(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	appointment, err := h.service.MarkNoShow(c.Request.Context(), id, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, appointment)
}
