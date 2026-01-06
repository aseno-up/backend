package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/patientrecord/model"
	"github.com/aseno/backend/pkg/patientrecord/service"
)

// PatientRecordHandler defines the interface for patient record handlers
type PatientRecordHandler interface {
	RegisterRoutes(router gin.IRouter)
	// PatientRecord endpoints
	GetRecord(c *gin.Context)
	UpdateRecord(c *gin.Context)
	GetFullRecord(c *gin.Context)
	// SessionNote endpoints
	CreateNote(c *gin.Context)
	GetNote(c *gin.Context)
	ListNotes(c *gin.Context)
	UpdateNote(c *gin.Context)
	DeleteNote(c *gin.Context)
}

// patientRecordHandler implements PatientRecordHandler
type patientRecordHandler struct {
	service service.PatientRecordService
}

// NewPatientRecordHandler creates a new patient record handler
func NewPatientRecordHandler(service service.PatientRecordService) PatientRecordHandler {
	return &patientRecordHandler{service: service}
}

// RegisterRoutes registers patient record routes
func (h *patientRecordHandler) RegisterRoutes(router gin.IRouter) {
	// Patient record routes - nested under patients
	patients := router.Group("/patients")
	{
		// Patient clinical record
		patients.GET("/:id/record", h.GetRecord)
		patients.PUT("/:id/record", h.UpdateRecord)
		patients.GET("/:id/record/full", h.GetFullRecord)

		// Session notes
		patients.POST("/:id/notes", h.CreateNote)
		patients.GET("/:id/notes", h.ListNotes)
		patients.GET("/:id/notes/:note_id", h.GetNote)
		patients.PUT("/:id/notes/:note_id", h.UpdateNote)
		patients.DELETE("/:id/notes/:note_id", h.DeleteNote)
	}
}

// GetRecord handles getting a patient's clinical record
func (h *patientRecordHandler) GetRecord(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	patientIDParam := c.Param("id")
	patientID, err := uuid.Parse(patientIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	record, err := h.service.GetOrCreateRecord(c.Request.Context(), clinicID.(uuid.UUID), patientID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, record)
}

// UpdateRecord handles updating a patient's clinical record
func (h *patientRecordHandler) UpdateRecord(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	patientIDParam := c.Param("id")
	patientID, err := uuid.Parse(patientIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	var req model.UpdatePatientRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	// First get or create the record
	existingRecord, err := h.service.GetOrCreateRecord(c.Request.Context(), clinicID.(uuid.UUID), patientID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Now update it
	record, err := h.service.UpdateRecord(c.Request.Context(), existingRecord.ID, clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, record)
}

// GetFullRecord handles getting the full patient record with session notes
func (h *patientRecordHandler) GetFullRecord(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	patientIDParam := c.Param("id")
	patientID, err := uuid.Parse(patientIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	// Get user's professional ID (might be nil if not a professional)
	var professionalID *uuid.UUID
	if uid, ok := userID.(uuid.UUID); ok {
		professionalID = &uid
	}

	record, err := h.service.GetFullRecord(c.Request.Context(), patientID, clinicID.(uuid.UUID), professionalID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, record)
}

// CreateNote handles creating a new session note
func (h *patientRecordHandler) CreateNote(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	patientIDParam := c.Param("id")
	patientID, err := uuid.Parse(patientIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	var req model.CreateSessionNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	note, err := h.service.CreateNote(c.Request.Context(), clinicID.(uuid.UUID), patientID, userID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, note)
}

// GetNote handles getting a session note by ID
func (h *patientRecordHandler) GetNote(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	noteIDParam := c.Param("note_id")
	noteID, err := uuid.Parse(noteIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid note ID")))
		return
	}

	note, err := h.service.GetNoteByID(c.Request.Context(), noteID, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, note)
}

// ListNotes handles listing session notes for a patient
func (h *patientRecordHandler) ListNotes(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	patientIDParam := c.Param("id")
	patientID, err := uuid.Parse(patientIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid patient ID")))
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	result, err := h.service.ListNotesByPatient(c.Request.Context(), patientID, clinicID.(uuid.UUID), page, perPage)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// UpdateNote handles updating a session note
func (h *patientRecordHandler) UpdateNote(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	noteIDParam := c.Param("note_id")
	noteID, err := uuid.Parse(noteIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid note ID")))
		return
	}

	var req model.UpdateSessionNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	note, err := h.service.UpdateNote(c.Request.Context(), noteID, clinicID.(uuid.UUID), userID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, note)
}

// DeleteNote handles deleting a session note
func (h *patientRecordHandler) DeleteNote(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	noteIDParam := c.Param("note_id")
	noteID, err := uuid.Parse(noteIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid note ID")))
		return
	}

	if err := h.service.DeleteNote(c.Request.Context(), noteID, clinicID.(uuid.UUID), userID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session note deleted successfully"})
}
