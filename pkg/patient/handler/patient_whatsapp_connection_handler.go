package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/patient/model"
	"github.com/aseno/backend/pkg/patient/service"
)

// PatientWhatsAppConnectionHandler defines the interface for patient-whatsapp connection handlers
type PatientWhatsAppConnectionHandler interface {
	RegisterRoutes(router gin.IRouter)
	Get(c *gin.Context)
	Update(c *gin.Context)
}

// patientWhatsAppConnectionHandler implements PatientWhatsAppConnectionHandler
type patientWhatsAppConnectionHandler struct {
	service service.PatientWhatsAppConnectionService
}

// NewPatientWhatsAppConnectionHandler creates a new handler
func NewPatientWhatsAppConnectionHandler(service service.PatientWhatsAppConnectionService) PatientWhatsAppConnectionHandler {
	return &patientWhatsAppConnectionHandler{service: service}
}

// RegisterRoutes registers patient whatsapp connection routes
func (h *patientWhatsAppConnectionHandler) RegisterRoutes(router gin.IRouter) {
	patients := router.Group("/patients")
	{
		patients.GET("/:id/whatsapp-connection", h.Get)
		patients.PUT("/:id/whatsapp-connection", h.Update)
	}
}

// Get returns the whatsapp connection config for a patient
func (h *patientWhatsAppConnectionHandler) Get(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	patientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("invalid patient ID")))
		return
	}

	result, err := h.service.GetByPatientID(c.Request.Context(), patientID, clinicID.(uuid.UUID))
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// Update updates the whatsapp connection config for a patient
func (h *patientWhatsAppConnectionHandler) Update(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	patientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("invalid patient ID")))
		return
	}

	var req model.UpdatePatientWhatsAppConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage(err.Error())))
		return
	}

	result, err := h.service.Update(c.Request.Context(), patientID, clinicID.(uuid.UUID), &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}
