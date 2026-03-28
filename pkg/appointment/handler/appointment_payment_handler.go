package handler

import (
	"errors"
	"net/http"

	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/appointment/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AppointmentPaymentHandler handles HTTP requests for appointment payments
type AppointmentPaymentHandler interface {
	RegisterRoutes(router gin.IRouter)
}

type appointmentPaymentHandler struct {
	service  service.AppointmentPaymentService
	auditSvc audit_service.AuditService
}

func NewAppointmentPaymentHandler(s service.AppointmentPaymentService, auditSvc audit_service.AuditService) AppointmentPaymentHandler {
	return &appointmentPaymentHandler{service: s, auditSvc: auditSvc}
}

// RegisterRoutes must be called on the /appointments group to avoid wildcard conflicts
func (h *appointmentPaymentHandler) RegisterRoutes(appointmentsGroup gin.IRouter) {
	appointmentsGroup.GET("/:id/payments", h.List)
	appointmentsGroup.POST("/:id/payments", h.Create)
	appointmentsGroup.PUT("/:id/payments/:paymentId", h.Update)
	appointmentsGroup.DELETE("/:id/payments/:paymentId", h.Delete)
}

func (h *appointmentPaymentHandler) getClinicID(c *gin.Context) (uuid.UUID, error) {
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

func (h *appointmentPaymentHandler) List(c *gin.Context) {
	clinicID, err := h.getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	appointmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do agendamento inválido"})
		return
	}

	payments, err := h.service.ListByAppointment(c.Request.Context(), clinicID, appointmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": payments})
}

func (h *appointmentPaymentHandler) Create(c *gin.Context) {
	clinicID, err := h.getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	appointmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do agendamento inválido"})
		return
	}

	var req model.CreateAppointmentPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	payment, err := h.service.Create(c.Request.Context(), clinicID, appointmentID, &req)
	if err != nil {
		if errors.Is(err, service.ErrAppointmentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Agendamento não encontrado"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntityAppointmentPayment, payment.ID.String(), nil, payment)
	}

	c.JSON(http.StatusCreated, payment)
}

func (h *appointmentPaymentHandler) Update(c *gin.Context) {
	clinicID, err := h.getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	paymentID, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do pagamento invalido"})
		return
	}

	var req model.CreateAppointmentPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados invalidos: " + err.Error()})
		return
	}

	// Get old data for audit
	oldPayments, _ := h.service.ListByAppointment(c.Request.Context(), clinicID, uuid.Nil)
	var oldData interface{}
	for _, p := range oldPayments {
		if p.ID == paymentID {
			oldData = p
			break
		}
	}

	payment, err := h.service.Update(c.Request.Context(), clinicID, paymentID, &req)
	if err != nil {
		if errors.Is(err, service.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Pagamento nao encontrado"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityAppointmentPayment, paymentID.String(), oldData, payment)
	}

	c.JSON(http.StatusOK, payment)
}

func (h *appointmentPaymentHandler) Delete(c *gin.Context) {
	clinicID, err := h.getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	paymentID, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do pagamento inválido"})
		return
	}

	// Get old data for audit before deleting
	appointmentID, _ := uuid.Parse(c.Param("id"))
	oldPayments, _ := h.service.ListByAppointment(c.Request.Context(), clinicID, appointmentID)
	var oldData interface{}
	for _, p := range oldPayments {
		if p.ID == paymentID {
			oldData = p
			break
		}
	}

	if err := h.service.Delete(c.Request.Context(), clinicID, paymentID); err != nil {
		if errors.Is(err, service.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Pagamento não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityAppointmentPayment, paymentID.String(), oldData, nil)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pagamento removido"})
}
