package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/patient/model"
	"github.com/saas/backend/pkg/patient/service"
)

// PatientHandler defines the interface for patient handlers
type PatientHandler interface {
	RegisterRoutes(router gin.IRouter)
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
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
		patients.GET("/:id", h.GetByID)
		patients.PUT("/:id", h.Update)
		patients.DELETE("/:id", h.Delete)
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

// List handles listing patients (with optional search)
func (h *patientHandler) List(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	query := c.Query("q")

	var result *model.PatientListResponse
	var err error

	if query != "" {
		result, err = h.service.Search(c.Request.Context(), clinicID.(uuid.UUID), query, page, perPage)
	} else {
		result, err = h.service.List(c.Request.Context(), clinicID.(uuid.UUID), page, perPage)
	}

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
