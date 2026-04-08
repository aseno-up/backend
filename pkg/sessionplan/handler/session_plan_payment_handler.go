package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/service"
)

// PlanPaymentHandler defines the interface for plan payment handlers
type PlanPaymentHandler interface {
	RegisterRoutes(router gin.IRouter)
	CreatePlanPayment(c *gin.Context)
	GetPlanPayments(c *gin.Context)
	CreateEnrollmentPayment(c *gin.Context)
	GetEnrollmentPayments(c *gin.Context)
	IsPlanFullyPaid(c *gin.Context)
	DeletePayment(c *gin.Context)
}

type planPaymentHandler struct {
	service service.PlanPaymentService
}

// NewPlanPaymentHandler creates a new plan payment handler
func NewPlanPaymentHandler(svc service.PlanPaymentService) PlanPaymentHandler {
	return &planPaymentHandler{service: svc}
}

// RegisterRoutes registers plan payment routes
func (h *planPaymentHandler) RegisterRoutes(router gin.IRouter) {
	plans := router.Group("/session-plans")
	{
		plans.POST("/:id/payments", h.CreatePlanPayment)
		plans.GET("/:id/payments", h.GetPlanPayments)
		plans.GET("/:id/payment-status", h.IsPlanFullyPaid)
		plans.DELETE("/:id/payments/:payment_id", h.DeletePayment)
		plans.POST("/:id/enrollments/:enrollment_id/payments", h.CreateEnrollmentPayment)
		plans.GET("/:id/enrollments/:enrollment_id/payments", h.GetEnrollmentPayments)
	}
}

// CreatePlanPayment creates a payment for an individual plan (upfront)
func (h *planPaymentHandler) CreatePlanPayment(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	var req model.CreatePlanPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	payment, err := h.service.CreatePlanPayment(c.Request.Context(), clinicID.(uuid.UUID), planID, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, payment.ToResponse())
}

// GetPlanPayments retrieves all payments for a plan
func (h *planPaymentHandler) GetPlanPayments(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	payments, err := h.service.GetPlanPayments(c.Request.Context(), clinicID.(uuid.UUID), planID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	responses := make([]*model.PlanPaymentResponse, len(payments))
	for i, p := range payments {
		responses[i] = p.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// CreateEnrollmentPayment creates a payment for a group enrollment
func (h *planPaymentHandler) CreateEnrollmentPayment(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	enrollmentID, err := uuid.Parse(c.Param("enrollment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid enrollment ID")))
		return
	}

	var req model.CreatePlanPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	payment, err := h.service.CreateEnrollmentPayment(c.Request.Context(), clinicID.(uuid.UUID), planID, enrollmentID, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, payment.ToResponse())
}

// GetEnrollmentPayments retrieves all payments for an enrollment
func (h *planPaymentHandler) GetEnrollmentPayments(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	enrollmentID, err := uuid.Parse(c.Param("enrollment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid enrollment ID")))
		return
	}

	payments, err := h.service.GetEnrollmentPayments(c.Request.Context(), clinicID.(uuid.UUID), enrollmentID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	responses := make([]*model.PlanPaymentResponse, len(payments))
	for i, p := range payments {
		responses[i] = p.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// IsPlanFullyPaid checks if a plan has been fully paid
func (h *planPaymentHandler) IsPlanFullyPaid(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan ID")))
		return
	}

	fullyPaid, err := h.service.IsPlanFullyPaid(c.Request.Context(), clinicID.(uuid.UUID), planID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"fully_paid": fullyPaid})
}

// DeletePayment deletes a plan payment
func (h *planPaymentHandler) DeletePayment(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	paymentID, err := uuid.Parse(c.Param("payment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid payment ID")))
		return
	}

	if err := h.service.DeletePayment(c.Request.Context(), clinicID.(uuid.UUID), paymentID); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Payment deleted"})
}
