package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/service"
)

// SessionPlanHandler defines the interface for session plan handlers
type SessionPlanHandler interface {
	RegisterRoutes(router gin.IRouter)
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	List(c *gin.Context)
	GetByPatient(c *gin.Context)
	Update(c *gin.Context)
	Pause(c *gin.Context)
	Resume(c *gin.Context)
	Cancel(c *gin.Context)
	Delete(c *gin.Context)
	BulkCancel(c *gin.Context)
	GetCalendarEvents(c *gin.Context)
	Materialize(c *gin.Context)
	CancelOccurrence(c *gin.Context)
	RescheduleOccurrence(c *gin.Context)
}

// sessionPlanHandler implements SessionPlanHandler
type sessionPlanHandler struct {
	service        service.SessionPlanService
	calendarFacade service.CalendarFacade
	materializeSvc service.MaterializeService
}

// NewSessionPlanHandler creates a new session plan handler
func NewSessionPlanHandler(
	svc service.SessionPlanService,
	calendarFacade service.CalendarFacade,
	materializeSvc service.MaterializeService,
) SessionPlanHandler {
	return &sessionPlanHandler{
		service:        svc,
		calendarFacade: calendarFacade,
		materializeSvc: materializeSvc,
	}
}

// RegisterRoutes registers session plan routes
func (h *sessionPlanHandler) RegisterRoutes(router gin.IRouter) {
	plans := router.Group("/session-plans")
	{
		plans.POST("", h.Create)
		plans.GET("", h.List)
		plans.GET("/:id", h.GetByID)
		plans.PUT("/:id", h.Update)
		plans.DELETE("/:id", h.Delete)
		plans.PUT("/:id/pause", h.Pause)
		plans.PUT("/:id/resume", h.Resume)
		plans.PUT("/:id/cancel", h.Cancel)
		plans.POST("/bulk-cancel", h.BulkCancel)
		plans.POST("/:id/materialize", h.Materialize)
		plans.POST("/:id/cancel-occurrence", h.CancelOccurrence)
		plans.POST("/:id/reschedule-occurrence", h.RescheduleOccurrence)
		plans.GET("/patient/:patient_id", h.GetByPatient)
	}

	// Calendar endpoint — returns merged virtual + real events
	calendar := router.Group("/calendar")
	{
		calendar.GET("/events", h.GetCalendarEvents)
	}
}

// Create creates a new session plan
func (h *sessionPlanHandler) Create(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req model.CreateSessionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	plan, err := h.service.Create(c.Request.Context(), clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, plan.ToResponse())
}

// GetByID retrieves a session plan by ID
func (h *sessionPlanHandler) GetByID(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	plan, err := h.service.GetByID(c.Request.Context(), clinicID.(uuid.UUID), id)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, plan.ToResponse())
}

// List retrieves all session plans for a clinic
func (h *sessionPlanHandler) List(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var filter pagination.Filter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	resp, err := h.service.List(c.Request.Context(), clinicID.(uuid.UUID), filter)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetByPatient retrieves all session plans for a patient
func (h *sessionPlanHandler) GetByPatient(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	patientID, err := uuid.Parse(c.Param("patient_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	plans, err := h.service.GetByPatient(c.Request.Context(), clinicID.(uuid.UUID), patientID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Convert to response
	responses := make([]*model.SessionPlanResponse, len(plans))
	for i, plan := range plans {
		responses[i] = plan.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"data": responses,
	})
}

// Update updates a session plan
func (h *sessionPlanHandler) Update(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	var req model.UpdateSessionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	plan, err := h.service.Update(c.Request.Context(), clinicID.(uuid.UUID), id, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, plan.ToResponse())
}

// Pause pauses a session plan
func (h *sessionPlanHandler) Pause(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	if err := h.service.Pause(c.Request.Context(), clinicID.(uuid.UUID), id); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "paused",
		"message": "Session plan paused successfully",
	})
}

// Resume resumes a paused session plan
func (h *sessionPlanHandler) Resume(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	if err := h.service.Resume(c.Request.Context(), clinicID.(uuid.UUID), id); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "active",
		"message": "Session plan resumed successfully",
	})
}

// Cancel cancels a session plan
func (h *sessionPlanHandler) Cancel(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	if err := h.service.Cancel(c.Request.Context(), clinicID.(uuid.UUID), id); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "cancelled",
		"message": "Session plan cancelled successfully",
	})
}

// Delete deletes a session plan
func (h *sessionPlanHandler) Delete(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	if err := h.service.Delete(c.Request.Context(), clinicID.(uuid.UUID), id); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Session plan deleted successfully",
	})
}

// GetCalendarEvents returns merged virtual + real appointments for the calendar
func (h *sessionPlanHandler) GetCalendarEvents(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req model.CalendarEventsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	rangeStart, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid start_date format")))
		return
	}
	rangeEnd, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid end_date format")))
		return
	}
	// Include the full end day
	rangeEnd = rangeEnd.Add(24*time.Hour - time.Second)

	var professionalID *uuid.UUID
	if req.ProfessionalID != "" {
		id, err := uuid.Parse(req.ProfessionalID)
		if err == nil {
			professionalID = &id
		}
	}

	events, err := h.calendarFacade.GetCalendarEvents(c.Request.Context(), clinicID.(uuid.UUID), professionalID, rangeStart, rangeEnd)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": events})
}

// Materialize converts a virtual RRULE occurrence into a real appointment
func (h *sessionPlanHandler) Materialize(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	var req model.MaterializeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	appointment, err := h.materializeSvc.Materialize(c.Request.Context(), clinicID.(uuid.UUID), planID, req.Date)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, appointment.ToResponse())
}

// CancelOccurrence cancels a single virtual occurrence of a session plan
func (h *sessionPlanHandler) CancelOccurrence(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	var req model.CancelOccurrenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	appointment, err := h.materializeSvc.CancelOccurrence(c.Request.Context(), clinicID.(uuid.UUID), planID, req.Date)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, appointment.ToResponse())
}

// RescheduleOccurrence reschedules a single virtual occurrence of a session plan
func (h *sessionPlanHandler) RescheduleOccurrence(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	var req model.RescheduleOccurrenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	newTime, err := time.Parse(time.RFC3339, req.NewStartTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid new_start_time format. Use RFC3339")))
		return
	}

	appointment, err := h.materializeSvc.RescheduleOccurrence(c.Request.Context(), clinicID.(uuid.UUID), planID, req.OriginalDate, newTime)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, appointment.ToResponse())
}

// BulkCancel handles cancelling multiple session plans
func (h *sessionPlanHandler) BulkCancel(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.service.BulkCancel(c.Request.Context(), clinicID.(uuid.UUID), req.IDs); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Planos de sessão cancelados com sucesso"})
}
