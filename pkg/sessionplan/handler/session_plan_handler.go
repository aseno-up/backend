package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/sessionplan/model"
	"github.com/saas/backend/pkg/sessionplan/service"
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
	GenerateAppointments(c *gin.Context)
}

// sessionPlanHandler implements SessionPlanHandler
type sessionPlanHandler struct {
	service service.SessionPlanService
}

// NewSessionPlanHandler creates a new session plan handler
func NewSessionPlanHandler(service service.SessionPlanService) SessionPlanHandler {
	return &sessionPlanHandler{
		service: service,
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
		plans.POST("/:id/generate", h.GenerateAppointments)
		plans.GET("/patient/:patient_id", h.GetByPatient)
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

// ListRequest represents the query parameters for listing session plans
type ListRequest struct {
	Page    int    `form:"page"`
	PerPage int    `form:"per_page"`
	Status  string `form:"status"`
}

// List retrieves all session plans for a clinic
func (h *sessionPlanHandler) List(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PerPage < 1 || req.PerPage > 100 {
		req.PerPage = 20
	}

	var status *model.PlanStatus
	if req.Status != "" {
		s := model.PlanStatus(req.Status)
		status = &s
	}

	plans, total, err := h.service.List(c.Request.Context(), clinicID.(uuid.UUID), req.Page, req.PerPage, status)
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

	totalPages := int(total) / req.PerPage
	if int(total)%req.PerPage > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        responses,
		"total":       total,
		"page":        req.Page,
		"per_page":    req.PerPage,
		"total_pages": totalPages,
	})
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

// GenerateAppointmentsRequest represents the request to generate appointments
type GenerateAppointmentsRequest struct {
	WeeksAhead int `json:"weeks_ahead" binding:"min=1,max=12"`
}

// GenerateAppointments manually triggers appointment generation for a plan
func (h *sessionPlanHandler) GenerateAppointments(c *gin.Context) {
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

	var req GenerateAppointmentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.WeeksAhead = 2 // Default to 2 weeks
	}
	if req.WeeksAhead < 1 {
		req.WeeksAhead = 2
	}

	plan, err := h.service.GetByID(c.Request.Context(), clinicID.(uuid.UUID), id)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	count, err := h.service.GenerateAppointments(c.Request.Context(), plan, req.WeeksAhead)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Appointments generated successfully",
		"appointments_created": count,
	})
}
