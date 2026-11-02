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
	GetRecordHistory(c *gin.Context)
	// SessionNote endpoints
	CreateNote(c *gin.Context)
	GetNote(c *gin.Context)
	ListNotes(c *gin.Context)
	UpdateNote(c *gin.Context)
	DeleteNote(c *gin.Context)

	// AnamneseTemplate endpoints
	ListTemplates(c *gin.Context)
	CreateTemplate(c *gin.Context)
	GetTemplate(c *gin.Context)
	UpdateTemplate(c *gin.Context)
	DeleteTemplate(c *gin.Context)
	CreateField(c *gin.Context)
	UpdateField(c *gin.Context)
	DeleteField(c *gin.Context)
	ReorderFields(c *gin.Context)
	SetTemplateSpecialties(c *gin.Context)
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
		patients.GET("/:id/record/history", h.GetRecordHistory)

		// Session notes
		patients.POST("/:id/notes", h.CreateNote)
		patients.GET("/:id/notes", h.ListNotes)
		patients.GET("/:id/notes/:note_id", h.GetNote)
		patients.PUT("/:id/notes/:note_id", h.UpdateNote)
		patients.DELETE("/:id/notes/:note_id", h.DeleteNote)
	}

	// Anamnese template routes
	templates := router.Group("/anamnese-templates")
	{
		templates.GET("", h.ListTemplates)
		templates.POST("", h.CreateTemplate)
		templates.GET("/:tid", h.GetTemplate)
		templates.PUT("/:tid", h.UpdateTemplate)
		templates.DELETE("/:tid", h.DeleteTemplate)

		// Template fields
		templates.POST("/:tid/fields", h.CreateField)
		templates.PUT("/:tid/fields/:fid", h.UpdateField)
		templates.DELETE("/:tid/fields/:fid", h.DeleteField)
		templates.PATCH("/:tid/reorder", h.ReorderFields)

		// Specialties mapping
		templates.POST("/:tid/specialties", h.SetTemplateSpecialties)
	}
}

// GetRecord handles getting a patient's clinical record (latest version)
func (h *patientRecordHandler) GetRecord(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	patientIDParam := c.Param("id")
	patientID, err := uuid.Parse(patientIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID do paciente inválido")))
		return
	}

	record, err := h.service.GetRecordByPatientID(c.Request.Context(), patientID, clinicID.(uuid.UUID))
	if err != nil {
		if errors.IsNotFound(err) {
			// Resolve template for a new record to show an empty form
			userID, _ := c.Get("user_id")
			var professionalID *uuid.UUID
			if uid, ok := userID.(uuid.UUID); ok {
				professionalID = &uid
			}
			template, _ := h.service.ResolveTemplate(c.Request.Context(), clinicID.(uuid.UUID), professionalID, nil)
			c.JSON(http.StatusOK, gin.H{
				"patient_id": patientID,
				"clinic_id":  clinicID,
				"template":   template,
			})
			return
		}
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, record)
}

// UpdateRecord handles updating a patient's clinical record (creates new version)
func (h *patientRecordHandler) UpdateRecord(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	patientIDParam := c.Param("id")
	patientID, err := uuid.Parse(patientIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID do paciente inválido")))
		return
	}

	var req model.CreatePatientRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	record, err := h.service.CreateRecord(c.Request.Context(), clinicID.(uuid.UUID), patientID, userID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, record)
}

// GetRecordHistory handles getting all versions of a patient record
func (h *patientRecordHandler) GetRecordHistory(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	patientIDParam := c.Param("id")
	patientID, err := uuid.Parse(patientIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID do paciente inválido")))
		return
	}

	history, err := h.service.GetRecordHistory(c.Request.Context(), patientID, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, history)
}

// GetFullRecord handles getting the full patient record with session notes
func (h *patientRecordHandler) GetFullRecord(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	patientIDParam := c.Param("id")
	patientID, err := uuid.Parse(patientIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID do paciente inválido")))
		return
	}

	// Get user's professional ID
	var professionalID *uuid.UUID
	if uid, ok := userID.(uuid.UUID); ok {
		professionalID = &uid
	}

	record, err := h.service.GetFullRecord(c.Request.Context(), patientID, clinicID.(uuid.UUID), professionalID, nil)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, record)
}

// Template Endpoints

func (h *patientRecordHandler) ListTemplates(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	templates, err := h.service.ListTemplates(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, templates)
}

func (h *patientRecordHandler) CreateTemplate(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	var template model.AnamneseTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.CreateTemplate(c.Request.Context(), clinicID.(uuid.UUID), &template)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *patientRecordHandler) GetTemplate(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	tid, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de template inválido"})
		return
	}

	template, err := h.service.GetTemplate(c.Request.Context(), tid, clinicID.(uuid.UUID))
	if err != nil {
		c.JSON(http.StatusNotFound, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, template)
}

func (h *patientRecordHandler) UpdateTemplate(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	tid, _ := uuid.Parse(c.Param("tid"))
	var template model.AnamneseTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.UpdateTemplate(c.Request.Context(), tid, clinicID.(uuid.UUID), &template)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *patientRecordHandler) DeleteTemplate(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	tid, _ := uuid.Parse(c.Param("tid"))
	if err := h.service.DeleteTemplate(c.Request.Context(), tid, clinicID.(uuid.UUID)); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Template excluído com sucesso"})
}

func (h *patientRecordHandler) CreateField(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	tid, _ := uuid.Parse(c.Param("tid"))
	var field model.AnamneseTemplateField
	if err := c.ShouldBindJSON(&field); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.CreateField(c.Request.Context(), tid, clinicID.(uuid.UUID), &field)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *patientRecordHandler) UpdateField(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	tid, _ := uuid.Parse(c.Param("tid"))
	fid, _ := uuid.Parse(c.Param("fid"))
	var field model.AnamneseTemplateField
	if err := c.ShouldBindJSON(&field); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.UpdateField(c.Request.Context(), fid, tid, clinicID.(uuid.UUID), &field)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *patientRecordHandler) DeleteField(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	tid, _ := uuid.Parse(c.Param("tid"))
	fid, _ := uuid.Parse(c.Param("fid"))
	if err := h.service.DeleteField(c.Request.Context(), fid, tid, clinicID.(uuid.UUID)); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Campo excluído com sucesso"})
}

func (h *patientRecordHandler) ReorderFields(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	tid, _ := uuid.Parse(c.Param("tid"))
	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.service.ReorderFields(c.Request.Context(), tid, clinicID.(uuid.UUID), req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Campos reordenados com sucesso"})
}

func (h *patientRecordHandler) SetTemplateSpecialties(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	tid, _ := uuid.Parse(c.Param("tid"))
	var req struct {
		SpecialtyIDs []uuid.UUID `json:"specialty_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.service.SetTemplateSpecialties(c.Request.Context(), tid, clinicID.(uuid.UUID), req.SpecialtyIDs); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Especialidades do template atualizadas com sucesso"})
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
