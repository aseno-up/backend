package handler

import (
	"errors"
	"net/http"

	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/partnernetwork/model"
	"github.com/aseno/backend/pkg/partnernetwork/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PartnerNetworkHandler struct {
	service  service.PartnerNetworkService
	auditSvc audit_service.AuditService
}

func NewPartnerNetworkHandler(s service.PartnerNetworkService, auditSvc audit_service.AuditService) *PartnerNetworkHandler {
	return &PartnerNetworkHandler{service: s, auditSvc: auditSvc}
}

func (h *PartnerNetworkHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/partner-networks")
	{
		g.POST("", h.Create)
		g.GET("", h.List)
		g.GET("/active", h.ListActive)
		g.GET("/:id", h.GetByID)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
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

func (h *PartnerNetworkHandler) Create(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req model.CreatePartnerNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	resp, err := h.service.Create(clinicID, &req)
	if err != nil {
		if errors.Is(err, service.ErrPartnerNetworkNameExists) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntityPartnerNetwork, resp.ID.String(), nil, resp)
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *PartnerNetworkHandler) List(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	items, err := h.service.List(clinicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *PartnerNetworkHandler) ListActive(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	items, err := h.service.ListActive(clinicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *PartnerNetworkHandler) GetByID(c *gin.Context) {
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
	resp, err := h.service.GetByID(clinicID, id)
	if err != nil {
		if errors.Is(err, service.ErrPartnerNetworkNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rede de convênio não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *PartnerNetworkHandler) Update(c *gin.Context) {
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

	var req model.UpdatePartnerNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}
	resp, err := h.service.Update(clinicID, id, &req)
	if err != nil {
		if errors.Is(err, service.ErrPartnerNetworkNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rede de convênio não encontrada"})
			return
		}
		if errors.Is(err, service.ErrPartnerNetworkNameExists) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityPartnerNetwork, id.String(), oldData, resp)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *PartnerNetworkHandler) Delete(c *gin.Context) {
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
		if errors.Is(err, service.ErrPartnerNetworkNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rede de convênio não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityPartnerNetwork, id.String(), oldData, nil)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Rede de convênio removida"})
}
