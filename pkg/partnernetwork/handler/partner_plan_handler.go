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

type PartnerPlanHandler struct {
	service  service.PartnerPlanService
	auditSvc audit_service.AuditService
}

func NewPartnerPlanHandler(s service.PartnerPlanService, auditSvc audit_service.AuditService) *PartnerPlanHandler {
	return &PartnerPlanHandler{service: s, auditSvc: auditSvc}
}

func (h *PartnerPlanHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/partner-plans")
	{
		g.POST("", h.Create)
		g.GET("/by-network/:network_id", h.ListByNetwork)
		g.GET("/:id", h.GetByID)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
	}
}

func (h *PartnerPlanHandler) Create(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req model.CreatePartnerPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}
	resp, err := h.service.Create(clinicID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPartnerNetworkNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Rede de convênio não encontrada"})
		case errors.Is(err, service.ErrPartnerPlanNameExists):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntityPartnerPlan, resp.ID.String(), nil, resp)
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *PartnerPlanHandler) ListByNetwork(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	networkID, err := uuid.Parse(c.Param("network_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}
	items, err := h.service.ListByNetwork(clinicID, networkID)
	if err != nil {
		if errors.Is(err, service.ErrPartnerNetworkNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rede de convênio não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *PartnerPlanHandler) GetByID(c *gin.Context) {
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
		if errors.Is(err, service.ErrPartnerPlanNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plano de convênio não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *PartnerPlanHandler) Update(c *gin.Context) {
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

	var req model.UpdatePartnerPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}
	resp, err := h.service.Update(clinicID, id, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPartnerPlanNotFound), errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Plano de convênio não encontrado"})
		case errors.Is(err, service.ErrPartnerPlanNameExists):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityPartnerPlan, id.String(), oldData, resp)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *PartnerPlanHandler) Delete(c *gin.Context) {
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
		if errors.Is(err, service.ErrPartnerPlanNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plano de convênio não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityPartnerPlan, id.String(), oldData, nil)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plano de convênio removido"})
}
