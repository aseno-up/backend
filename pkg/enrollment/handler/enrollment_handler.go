package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/enrollment/model"
	"github.com/aseno/backend/pkg/enrollment/service"
)

// EnrollmentHandler defines the interface for enrollment handlers
type EnrollmentHandler interface {
	RegisterRoutes(router gin.IRouter)
	Enroll(c *gin.Context)
	ListByPlan(c *gin.Context)
	Suspend(c *gin.Context)
	Reactivate(c *gin.Context)
	Cancel(c *gin.Context)
}

type enrollmentHandler struct {
	service service.EnrollmentService
}

// NewEnrollmentHandler creates a new enrollment handler
func NewEnrollmentHandler(svc service.EnrollmentService) EnrollmentHandler {
	return &enrollmentHandler{service: svc}
}

// RegisterRoutes registers enrollment routes under /session-plans/:id/enrollments
func (h *enrollmentHandler) RegisterRoutes(router gin.IRouter) {
	plans := router.Group("/session-plans")
	{
		plans.POST("/:id/enrollments", h.Enroll)
		plans.GET("/:id/enrollments", h.ListByPlan)
		plans.PUT("/:id/enrollments/:enrollment_id/suspend", h.Suspend)
		plans.PUT("/:id/enrollments/:enrollment_id/reactivate", h.Reactivate)
		plans.PUT("/:id/enrollments/:enrollment_id/cancel", h.Cancel)
	}
}

// Enroll enrolls a patient in a group session plan
func (h *enrollmentHandler) Enroll(c *gin.Context) {
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

	var req model.EnrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	enrollment, err := h.service.Enroll(c.Request.Context(), clinicID.(uuid.UUID), planID, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, enrollment.ToResponse())
}

// ListByPlan lists all enrollments for a session plan
func (h *enrollmentHandler) ListByPlan(c *gin.Context) {
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

	enrollments, err := h.service.ListByPlan(c.Request.Context(), clinicID.(uuid.UUID), planID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	responses := make([]*model.EnrollmentResponse, len(enrollments))
	for i, e := range enrollments {
		responses[i] = e.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// Suspend suspends an enrollment
func (h *enrollmentHandler) Suspend(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	enrollmentID, err := uuid.Parse(c.Param("enrollment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid enrollment ID")))
		return
	}

	if err := h.service.Suspend(c.Request.Context(), clinicID.(uuid.UUID), enrollmentID); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "suspended", "message": "Enrollment suspended"})
}

// Reactivate reactivates a suspended enrollment
func (h *enrollmentHandler) Reactivate(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	enrollmentID, err := uuid.Parse(c.Param("enrollment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid enrollment ID")))
		return
	}

	if err := h.service.Reactivate(c.Request.Context(), clinicID.(uuid.UUID), enrollmentID); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "active", "message": "Enrollment reactivated"})
}

// Cancel cancels an enrollment
func (h *enrollmentHandler) Cancel(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	enrollmentID, err := uuid.Parse(c.Param("enrollment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid enrollment ID")))
		return
	}

	if err := h.service.Cancel(c.Request.Context(), clinicID.(uuid.UUID), enrollmentID); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled", "message": "Enrollment cancelled"})
}
