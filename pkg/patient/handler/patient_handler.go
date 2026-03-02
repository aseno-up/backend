package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/patient/model"
	"github.com/aseno/backend/pkg/patient/service"
)

// PatientHandler defines the interface for patient handlers
type PatientHandler interface {
	RegisterRoutes(router gin.IRouter)
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	BulkDelete(c *gin.Context)
	// Risk assessment endpoints
	GetRisk(c *gin.Context)
	RecalculateRisk(c *gin.Context)
	RecalculateAllRisks(c *gin.Context)
}

// patientHandler implements PatientHandler
type patientHandler struct {
	service service.PatientService
}

// NewPatientHandler creates a new patient handler
func NewPatientHandler(service service.PatientService) PatientHandler {
	return &patientHandler{service: service}
}

// RegisterRoutes registers patient routes
func (h *patientHandler) RegisterRoutes(router gin.IRouter) {
	patients := router.Group("/patients")
	{
		patients.POST("", h.Create)
		patients.GET("", h.List)
		patients.POST("/risk/recalculate-all", h.RecalculateAllRisks)
		patients.GET("/:id", h.GetByID)
		patients.PUT("/:id", h.Update)
		patients.DELETE("/:id", h.Delete)
		patients.POST("/bulk-delete", h.BulkDelete)
		patients.GET("/:id/risk", h.GetRisk)
		patients.POST("/:id/risk/recalculate", h.RecalculateRisk)
	}
}

// Create handles creating a new patient
func (h *patientHandler) Create(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	var req model.CreatePatientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	patient, err := h.service.Create(c.Request.Context(), clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, patient)
}

// GetByID handles getting a patient by ID
func (h *patientHandler) GetByID(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	patient, err := h.service.GetByID(c.Request.Context(), id, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, patient)
}

// List handles listing patients (with optional search and filters)
func (h *patientHandler) List(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	var filter pagination.Filter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.List(c.Request.Context(), clinicID.(uuid.UUID), filter)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// Update handles updating a patient
func (h *patientHandler) Update(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	var req model.UpdatePatientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	patient, err := h.service.Update(c.Request.Context(), id, clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, patient)
}

// Delete handles deleting a patient
func (h *patientHandler) Delete(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	if err := h.service.Delete(c.Request.Context(), id, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Patient deleted successfully"})
}

// BulkDelete handles deleting multiple patients
func (h *patientHandler) BulkDelete(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.service.BulkDelete(c.Request.Context(), req.IDs, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Patients deleted successfully"})
}

// GetRisk handles getting a patient's risk assessment
func (h *patientHandler) GetRisk(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	risk, err := h.service.GetRisk(c.Request.Context(), id, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, risk)
}

// RecalculateRisk handles recalculating a patient's risk score
func (h *patientHandler) RecalculateRisk(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	if err := h.service.CalculateRiskScore(c.Request.Context(), id, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Return updated risk
	risk, err := h.service.GetRisk(c.Request.Context(), id, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, risk)
}

// RecalculateAllRisks handles recalculating risk scores for all patients
func (h *patientHandler) RecalculateAllRisks(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	updated, err := h.service.RecalculateAllRiskScores(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Risk scores recalculated successfully",
		"patients_updated": updated,
	})
}
