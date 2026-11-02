package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/appointment/service"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/auth/middleware"
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
	GetDeletePreview(c *gin.Context)
	Confirm(c *gin.Context)
	Cancel(c *gin.Context)
	Complete(c *gin.Context)
	MarkNoShow(c *gin.Context)
}

// AttendanceAutoRecorder is an optional interface for auto-recording attendance on completion
type AttendanceAutoRecorder interface {
	AutoRecordPresent(ctx context.Context, clinicID, appointmentID uuid.UUID) error
}

// appointmentHandler implements AppointmentHandler
type appointmentHandler struct {
	service            service.AppointmentService
	attendanceRecorder AttendanceAutoRecorder
	auditSvc           audit_service.AuditService
}

// NewAppointmentHandler creates a new appointment handler
func NewAppointmentHandler(service service.AppointmentService, opts ...func(*appointmentHandler)) AppointmentHandler {
	h := &appointmentHandler{service: service}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// WithAuditService wires an audit service so mutating actions leave a trail.
func WithAuditService(svc audit_service.AuditService) func(*appointmentHandler) {
	return func(h *appointmentHandler) {
		h.auditSvc = svc
	}
}

// WithAttendanceRecorder sets the attendance auto-recorder (called on appointment completion)
func WithAttendanceRecorder(recorder AttendanceAutoRecorder) func(*appointmentHandler) {
	return func(h *appointmentHandler) {
		h.attendanceRecorder = recorder
	}
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
		appointments.GET("/:id/delete-preview", h.GetDeletePreview)
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

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntityAppointment, appointment.ID.String(), nil, appointment)
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

	// Check if professional access is forced (professional role can only see their own appointments)
	if forcedProfID, exists := c.Get(middleware.ContextKeyForceProfessionalFilter); exists {
		id := forcedProfID.(uuid.UUID)
		filter.ProfessionalID = &id
	} else if professionalID := c.Query("professional_id"); professionalID != "" {
		// Allow filtering by professional_id for admin/receptionist
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
	// Check if professional access is forced (professional role can only see their own appointments)
	if forcedProfID, exists := c.Get(middleware.ContextKeyForceProfessionalFilter); exists {
		id := forcedProfID.(uuid.UUID)
		professionalID = &id
	} else if profID := c.Query("professional_id"); profID != "" {
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
	// Check if professional access is forced (professional role can only see their own appointments)
	if forcedProfID, exists := c.Get(middleware.ContextKeyForceProfessionalFilter); exists {
		id := forcedProfID.(uuid.UUID)
		professionalID = &id
	} else if profID := c.Query("professional_id"); profID != "" {
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

	// Capture old data before mutation so the audit diff shows both sides.
	oldData, _ := h.service.GetByID(c.Request.Context(), id, clinicID.(uuid.UUID))

	appointment, err := h.service.Update(c.Request.Context(), id, clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityAppointment, id.String(), oldData, appointment)
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

	deletePayments := c.Query("delete_payments") == "true"

	// Capture old data so the audit trail records what was deleted.
	oldData, _ := h.service.GetByID(c.Request.Context(), id, clinicID.(uuid.UUID))

	if err := h.service.Delete(c.Request.Context(), id, clinicID.(uuid.UUID), deletePayments); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityAppointment, id.String(), oldData, nil)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Appointment deleted successfully"})
}

// GetDeletePreview returns the counts the UI needs to show before deleting
// an appointment (procedures to cascade, payments kept by default).
func (h *appointmentHandler) GetDeletePreview(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	preview, err := h.service.GetDeletePreview(c.Request.Context(), id, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, preview)
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

	// Auto-record attendance for session plan appointments (if not already recorded)
	if h.attendanceRecorder != nil {
		if err := h.attendanceRecorder.AutoRecordPresent(c.Request.Context(), clinicID.(uuid.UUID), id); err != nil {
			logger.Printf("[Appointment] Error auto-recording attendance for %s: %v", id, err)
		}
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
