package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/attendance/model"
	"github.com/aseno/backend/pkg/attendance/service"
)

// GroupAttendanceHandler defines the interface for group attendance handlers
type GroupAttendanceHandler interface {
	RegisterRoutes(router gin.IRouter)
	RecordGroupAttendance(c *gin.Context)
	GetGroupAttendance(c *gin.Context)
}

type groupAttendanceHandler struct {
	service service.GroupAttendanceService
}

// NewGroupAttendanceHandler creates a new group attendance handler
func NewGroupAttendanceHandler(svc service.GroupAttendanceService) GroupAttendanceHandler {
	return &groupAttendanceHandler{service: svc}
}

// RegisterRoutes registers group attendance routes under /session-plans/:id/attendance
func (h *groupAttendanceHandler) RegisterRoutes(router gin.IRouter) {
	plans := router.Group("/session-plans")
	{
		plans.POST("/:id/attendance", h.RecordGroupAttendance)
		plans.GET("/:id/attendance", h.GetGroupAttendance)
	}
}

// RecordGroupAttendance records attendance for a group session
func (h *groupAttendanceHandler) RecordGroupAttendance(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	sessionPlanID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid session plan ID")))
		return
	}

	var req model.RecordGroupAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	results, err := h.service.RecordGroupAttendance(c.Request.Context(), clinicID.(uuid.UUID), sessionPlanID, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	responses := make([]*model.GroupAttendanceResponse, len(results))
	for i, r := range results {
		responses[i] = r.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// GetGroupAttendance retrieves attendance records for a group session on a specific date
func (h *groupAttendanceHandler) GetGroupAttendance(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	sessionPlanID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid session plan ID")))
		return
	}

	date := c.Query("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("date query parameter is required")))
		return
	}

	records, err := h.service.GetGroupAttendance(c.Request.Context(), clinicID.(uuid.UUID), sessionPlanID, date)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	responses := make([]*model.GroupAttendanceResponse, len(records))
	for i, r := range records {
		responses[i] = r.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}
