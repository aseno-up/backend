package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/attendance/model"
	"github.com/aseno/backend/pkg/attendance/service"
)

// AttendanceHandler defines the interface for attendance handlers
type AttendanceHandler interface {
	RegisterRoutes(router gin.IRouter)
	RecordAttendance(c *gin.Context)
	GetByAppointment(c *gin.Context)
}

type attendanceHandler struct {
	service service.AttendanceService
}

// NewAttendanceHandler creates a new attendance handler
func NewAttendanceHandler(svc service.AttendanceService) AttendanceHandler {
	return &attendanceHandler{service: svc}
}

// RegisterRoutes registers attendance routes under /appointments/:id/attendance
func (h *attendanceHandler) RegisterRoutes(router gin.IRouter) {
	appointments := router.Group("/appointments")
	{
		appointments.POST("/:id/attendance", h.RecordAttendance)
		appointments.GET("/:id/attendance", h.GetByAppointment)
	}
}

// RecordAttendance records attendance for a patient at an appointment
func (h *attendanceHandler) RecordAttendance(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	appointmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	var req model.RecordAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	attendance, err := h.service.RecordAttendance(c.Request.Context(), clinicID.(uuid.UUID), appointmentID, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, attendance.ToResponse())
}

// GetByAppointment retrieves all attendance records for an appointment
func (h *attendanceHandler) GetByAppointment(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	appointmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid appointment ID")))
		return
	}

	records, err := h.service.GetByAppointment(c.Request.Context(), clinicID.(uuid.UUID), appointmentID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	responses := make([]*model.AttendanceResponse, len(records))
	for i, r := range records {
		responses[i] = r.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}
