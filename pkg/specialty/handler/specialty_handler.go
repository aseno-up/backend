package handler

import (
	"errors"
	"net/http"

	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/specialty/model"
	"github.com/aseno/backend/pkg/specialty/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SpecialtyHandler struct {
	service  service.SpecialtyService
	auditSvc audit_service.AuditService
}

func NewSpecialtyHandler(s service.SpecialtyService, auditSvc audit_service.AuditService) *SpecialtyHandler {
	return &SpecialtyHandler{service: s, auditSvc: auditSvc}
}

func (h *SpecialtyHandler) RegisterRoutes(r *gin.RouterGroup) {
	specialties := r.Group("/specialties")
	{
		specialties.POST("", h.Create)
		specialties.GET("", h.List)
		specialties.GET("/active", h.ListActive)
		specialties.GET("/:id", h.GetByID)
		specialties.PUT("/:id", h.Update)
		specialties.DELETE("/:id", h.Delete)
	}
}

func getClinicID(c *gin.Context) (uuid.UUID, error) {
	val, exists := c.Get("clinic_id")
	if !exists {
		return uuid.Nil, errors.New("clinic_id não encontrado no contexto")
	}
	clinicID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("clinic_id no formato incorreto")
	}
	return clinicID, nil
}

func (h *SpecialtyHandler) Create(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req model.CreateSpecialtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	specialty, err := h.service.Create(clinicID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntitySpecialty, specialty.ID.String(), nil, specialty)
	}

	c.JSON(http.StatusCreated, specialty)
}

func (h *SpecialtyHandler) List(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	specialties, err := h.service.List(clinicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": specialties})
}

func (h *SpecialtyHandler) ListActive(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	specialties, err := h.service.ListActive(clinicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": specialties})
}

func (h *SpecialtyHandler) GetByID(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	specialty, err := h.service.GetByID(clinicID, id)
	if err != nil {
		if errors.Is(err, service.ErrSpecialtyNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Especialidade não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, specialty)
}

func (h *SpecialtyHandler) Update(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	oldData, _ := h.service.GetByID(clinicID, id)

	var req model.UpdateSpecialtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	specialty, err := h.service.Update(clinicID, id, &req)
	if err != nil {
		if errors.Is(err, service.ErrSpecialtyNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Especialidade não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntitySpecialty, id.String(), oldData, specialty)
	}

	c.JSON(http.StatusOK, specialty)
}

func (h *SpecialtyHandler) Delete(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	oldData, _ := h.service.GetByID(clinicID, id)

	if err := h.service.Delete(clinicID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntitySpecialty, id.String(), oldData, nil)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Especialidade removida"})
}
