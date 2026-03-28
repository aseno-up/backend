package handler

import (
	"errors"
	"net/http"

	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/paymentmethod/model"
	"github.com/aseno/backend/pkg/paymentmethod/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PaymentMethodHandler struct {
	service  service.PaymentMethodService
	auditSvc audit_service.AuditService
}

func NewPaymentMethodHandler(s service.PaymentMethodService, auditSvc audit_service.AuditService) *PaymentMethodHandler {
	return &PaymentMethodHandler{service: s, auditSvc: auditSvc}
}

func (h *PaymentMethodHandler) RegisterRoutes(r *gin.RouterGroup) {
	methods := r.Group("/payment-methods")
	{
		methods.POST("", h.Create)
		methods.GET("", h.List)
		methods.GET("/active", h.ListActive)
		methods.GET("/:id", h.GetByID)
		methods.PUT("/:id", h.Update)
		methods.DELETE("/:id", h.Delete)
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

func (h *PaymentMethodHandler) Create(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req model.CreatePaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	method, err := h.service.Create(clinicID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntityPaymentMethod, method.ID.String(), nil, method)
	}

	c.JSON(http.StatusCreated, method)
}

func (h *PaymentMethodHandler) List(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	_ = h.service.EnsureDefaults(clinicID)

	methods, err := h.service.List(clinicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": methods})
}

func (h *PaymentMethodHandler) ListActive(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	_ = h.service.EnsureDefaults(clinicID)

	methods, err := h.service.ListActive(clinicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": methods})
}

func (h *PaymentMethodHandler) GetByID(c *gin.Context) {
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

	method, err := h.service.GetByID(clinicID, id)
	if err != nil {
		if errors.Is(err, service.ErrPaymentMethodNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Método de pagamento não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, method)
}

func (h *PaymentMethodHandler) Update(c *gin.Context) {
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

	// Get old data for audit
	oldData, _ := h.service.GetByID(clinicID, id)

	var req model.UpdatePaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	method, err := h.service.Update(clinicID, id, &req)
	if err != nil {
		if errors.Is(err, service.ErrPaymentMethodNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Método de pagamento não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityPaymentMethod, id.String(), oldData, method)
	}

	c.JSON(http.StatusOK, method)
}

func (h *PaymentMethodHandler) Delete(c *gin.Context) {
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

	// Get old data for audit
	oldData, _ := h.service.GetByID(clinicID, id)

	if err := h.service.Delete(clinicID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityPaymentMethod, id.String(), oldData, nil)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Método de pagamento removido"})
}
