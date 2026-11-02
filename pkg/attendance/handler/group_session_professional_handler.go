package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/attendance/model"
	"github.com/aseno/backend/pkg/attendance/service"
)

// GroupSessionProfessionalHandler defines the interface for group session professional handlers
type GroupSessionProfessionalHandler interface {
	RegisterRoutes(router gin.IRouter)
	SetProfessional(c *gin.Context)
	GetProfessional(c *gin.Context)
}

type groupSessionProfessionalHandler struct {
	service service.GroupSessionProfessionalService
}

// NewGroupSessionProfessionalHandler creates a new handler
func NewGroupSessionProfessionalHandler(svc service.GroupSessionProfessionalService) GroupSessionProfessionalHandler {
	return &groupSessionProfessionalHandler{service: svc}
}

// RegisterRoutes registers routes under /session-plans/:id/sessions/:date/professional
func (h *groupSessionProfessionalHandler) RegisterRoutes(router gin.IRouter) {
	plans := router.Group("/session-plans")
	{
		plans.PUT("/:id/sessions/:date/professional", h.SetProfessional)
		plans.GET("/:id/sessions/:date/professional", h.GetProfessional)
	}
}

// SetProfessional assigns a professional to a group session date
func (h *groupSessionProfessionalHandler) SetProfessional(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	sessionPlanID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID do plano inválido")))
		return
	}

	date := c.Param("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Data é obrigatória")))
		return
	}

	var req model.SetSessionProfessionalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.SetProfessional(c.Request.Context(), clinicID.(uuid.UUID), sessionPlanID, date, req.ProfessionalID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetProfessional retrieves the professional assigned to a group session date
func (h *groupSessionProfessionalHandler) GetProfessional(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	sessionPlanID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID do plano inválido")))
		return
	}

	date := c.Param("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Data é obrigatória")))
		return
	}

	record, err := h.service.GetProfessional(c.Request.Context(), clinicID.(uuid.UUID), sessionPlanID, date)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if record == nil {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": &model.SessionProfessionalResponse{
		SessionPlanID:    record.SessionPlanID,
		SessionDate:      record.SessionDate,
		ProfessionalID:   record.ProfessionalID,
		ProfessionalName: record.ProfessionalName,
	}})
}
