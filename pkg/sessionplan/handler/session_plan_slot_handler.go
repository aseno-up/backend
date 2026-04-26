package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	internal_errors "github.com/aseno/backend/internal/errors"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/service"
)

// SessionPlanSlotHandler exposes slot CRUD for group session plans.
type SessionPlanSlotHandler struct {
	svc      service.SessionPlanSlotService
	auditSvc audit_service.AuditService
}

func NewSessionPlanSlotHandler(svc service.SessionPlanSlotService, auditSvc audit_service.AuditService) *SessionPlanSlotHandler {
	return &SessionPlanSlotHandler{svc: svc, auditSvc: auditSvc}
}

func (h *SessionPlanSlotHandler) RegisterRoutes(router gin.IRouter) {
	plans := router.Group("/session-plans")
	{
		plans.GET("/:id/slots", h.List)
		plans.POST("/:id/slots", h.Create)
		plans.PUT("/:id/slots/:slot_id", h.Update)
		plans.DELETE("/:id/slots/:slot_id", h.Delete)
	}
}

func (h *SessionPlanSlotHandler) List(c *gin.Context) {
	clinicID, ok := getCtxClinicID(c)
	if !ok {
		return
	}
	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}
	slots, err := h.svc.List(c.Request.Context(), clinicID, planID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]*model.SessionPlanSlotResponse, len(slots))
	for i, s := range slots {
		out[i] = s.ToResponse()
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *SessionPlanSlotHandler) Create(c *gin.Context) {
	clinicID, ok := getCtxClinicID(c)
	if !ok {
		return
	}
	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}
	var req model.CreateSessionPlanSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
		return
	}
	slot, err := h.svc.Create(c.Request.Context(), clinicID, planID, &req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntitySessionPlan, slot.ID.String(), nil, slot)
	}
	c.JSON(http.StatusCreated, slot.ToResponse())
}

func (h *SessionPlanSlotHandler) Update(c *gin.Context) {
	clinicID, ok := getCtxClinicID(c)
	if !ok {
		return
	}
	slotID, err := uuid.Parse(c.Param("slot_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slot id"})
		return
	}
	var req model.UpdateSessionPlanSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
		return
	}
	slot, err := h.svc.Update(c.Request.Context(), clinicID, slotID, &req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntitySessionPlan, slot.ID.String(), nil, slot)
	}
	c.JSON(http.StatusOK, slot.ToResponse())
}

func (h *SessionPlanSlotHandler) Delete(c *gin.Context) {
	clinicID, ok := getCtxClinicID(c)
	if !ok {
		return
	}
	slotID, err := uuid.Parse(c.Param("slot_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slot id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), clinicID, slotID); err != nil {
		writeServiceError(c, err)
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntitySessionPlan, slotID.String(), nil, nil)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Slot removido"})
}

// getCtxClinicID extracts the tenant from the gin context, writing 401 on failure.
func getCtxClinicID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "clinic_id missing from context"})
		return uuid.Nil, false
	}
	clinicID, ok := val.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "clinic_id has wrong type"})
		return uuid.Nil, false
	}
	return clinicID, true
}

func writeServiceError(c *gin.Context, err error) {
	if errors.Is(err, internal_errors.ErrNotFoundInstance) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, internal_errors.ErrValidationInstance) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
