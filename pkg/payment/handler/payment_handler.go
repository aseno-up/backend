package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/aseno/backend/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/payment/model"
	"github.com/aseno/backend/pkg/payment/service"
)

// PaymentHandler defines the interface for payment handlers
type PaymentHandler interface {
	RegisterRoutes(router gin.IRouter)
	CreateCustomer(c *gin.Context)
	GetCustomer(c *gin.Context)
	CreatePayment(c *gin.Context)
	GetPayments(c *gin.Context)
	GetPayment(c *gin.Context)
	GetPixQRCode(c *gin.Context)
	CancelPayment(c *gin.Context)
	GetSubscription(c *gin.Context)
	CreateSubscription(c *gin.Context)
	CancelSubscription(c *gin.Context)
	GetSubscriptionPayments(c *gin.Context)
	SwitchPlan(c *gin.Context)
	Webhook(c *gin.Context)
}

// paymentHandler implements PaymentHandler
type paymentHandler struct {
	service       service.PaymentService
	webhookSecret string
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(service service.PaymentService, webhookSecret string) PaymentHandler {
	return &paymentHandler{
		service:       service,
		webhookSecret: webhookSecret,
	}
}

// RegisterRoutes registers payment routes
func (h *paymentHandler) RegisterRoutes(router gin.IRouter) {
	billing := router.Group("/billing")
	{
		// Customer routes
		billing.POST("/customer", h.CreateCustomer)
		billing.GET("/customer", h.GetCustomer)

		// Payment routes
		billing.POST("/payments", h.CreatePayment)
		billing.GET("/payments", h.GetPayments)
		billing.GET("/payments/:id", h.GetPayment)
		billing.GET("/payments/:id/pix", h.GetPixQRCode)
		billing.DELETE("/payments/:id", h.CancelPayment)

		// Subscription routes
		billing.GET("/subscription", h.GetSubscription)
		billing.POST("/subscription", h.CreateSubscription)
		billing.DELETE("/subscription", h.CancelSubscription)
		billing.GET("/subscription/payments", h.GetSubscriptionPayments)
		billing.POST("/switch-plan", h.SwitchPlan)
	}
}

// CreateCustomerRequest represents the request to create a customer
type CreateCustomerRequest struct {
	Name          string `json:"name" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
	Phone         string `json:"phone"`
	Document      string `json:"document" binding:"required"`
	PostalCode    string `json:"postal_code"`
	Address       string `json:"address"`
	AddressNumber string `json:"address_number"`
	Province      string `json:"province"`
	City          string `json:"city"`
	State         string `json:"state"`
}

// CreateCustomer creates a billing customer
func (h *paymentHandler) CreateCustomer(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req CreateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	input := &service.CreateCustomerInput{
		Name:          req.Name,
		Email:         req.Email,
		Phone:         req.Phone,
		Document:      req.Document,
		PostalCode:    req.PostalCode,
		Address:       req.Address,
		AddressNumber: req.AddressNumber,
		Province:      req.Province,
		City:          req.City,
		State:         req.State,
	}

	customer, err := h.service.CreateOrGetCustomer(c.Request.Context(), clinicID.(uuid.UUID), input)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, customer)
}

// GetCustomer retrieves the billing customer for the clinic
func (h *paymentHandler) GetCustomer(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	customer, err := h.service.GetCustomerByClinic(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, customer)
}

// CreatePaymentRequest represents the request to create a payment
type CreatePaymentRequest struct {
	BillingType string  `json:"billing_type" binding:"required"`
	Value       float64 `json:"value" binding:"required,gt=0"`
	DueDate     string  `json:"due_date" binding:"required"`
	Description string  `json:"description"`
}

// CreatePayment creates a new payment
func (h *paymentHandler) CreatePayment(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	dueDate, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid due_date format. Use YYYY-MM-DD")))
		return
	}

	input := &service.CreatePaymentInput{
		BillingType: model.BillingType(req.BillingType),
		Value:       req.Value,
		DueDate:     dueDate,
		Description: req.Description,
	}

	payment, err := h.service.CreatePayment(c.Request.Context(), clinicID.(uuid.UUID), input)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, payment)
}

// GetPayments retrieves payments for the clinic
func (h *paymentHandler) GetPayments(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	payments, total, err := h.service.GetPayments(c.Request.Context(), clinicID.(uuid.UUID), page, perPage)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	totalPages := (int(total) + perPage - 1) / perPage

	c.JSON(http.StatusOK, gin.H{
		"data":        payments,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// GetPayment retrieves a specific payment
func (h *paymentHandler) GetPayment(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	paymentID := c.Param("id")
	if paymentID == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Invalid payment ID")))
		return
	}

	payment, err := h.service.GetPayment(c.Request.Context(), clinicID.(uuid.UUID), paymentID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, payment)
}

// GetPixQRCode retrieves the PIX QR code for a payment
func (h *paymentHandler) GetPixQRCode(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	paymentID := c.Param("id")
	if paymentID == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Invalid payment ID")))
		return
	}

	qrCode, err := h.service.GetPixQRCode(c.Request.Context(), clinicID.(uuid.UUID), paymentID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, qrCode)
}

// CancelPayment cancels a payment
func (h *paymentHandler) CancelPayment(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Invalid payment ID")))
		return
	}

	if err := h.service.CancelPayment(c.Request.Context(), clinicID.(uuid.UUID), paymentID); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "canceled"})
}

// GetSubscription retrieves the subscription for the clinic
func (h *paymentHandler) GetSubscription(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	subscription, err := h.service.GetSubscription(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, subscription)
}

// CreateSubscriptionRequest represents the request to create a subscription
type CreateSubscriptionRequest struct {
	BillingType string  `json:"billing_type" binding:"required"`
	PlanID      string  `json:"plan_id" binding:"required,uuid"`
	OfferToken  *string `json:"offer_token,omitempty"`
	Cycle       string  `json:"cycle" binding:"required"`
}

// CreateSubscription creates a new subscription
func (h *paymentHandler) CreateSubscription(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid plan_id format")))
		return
	}

	input := &service.CreateSubscriptionInput{
		BillingType: model.BillingType(req.BillingType),
		PlanID:      planID,
		OfferToken:  req.OfferToken,
		Cycle:       model.BillingCycle(req.Cycle),
	}

	subscription, err := h.service.CreateSubscription(c.Request.Context(), clinicID.(uuid.UUID), input)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, subscription)
}

// CancelSubscription cancels the subscription
func (h *paymentHandler) CancelSubscription(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	if err := h.service.CancelSubscription(c.Request.Context(), clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "canceled"})
}

// SwitchPlanRequest represents the request to switch plans
type SwitchPlanRequest struct {
	Plan                 string                      `json:"plan" binding:"required,oneof=basic professional advanced"`
	BillingType          string                      `json:"billing_type" binding:"required"`
	Cycle                string                      `json:"cycle" binding:"required"`
	OfferToken           *string                     `json:"offer_token,omitempty"`
	CreditCard           *model.CreditCardInfo       `json:"credit_card,omitempty"`
	CreditCardHolderInfo *model.CreditCardHolderInfo `json:"credit_card_holder_info,omitempty"`
}

// SwitchPlan handles plan switching (cancel current + create new)
func (h *paymentHandler) SwitchPlan(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req SwitchPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	subscription, err := h.service.SwitchPlan(c.Request.Context(), clinicID.(uuid.UUID), req.Plan, req.BillingType, req.Cycle, req.OfferToken, req.CreditCard, req.CreditCardHolderInfo)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, subscription)
}

// GetSubscriptionPayments returns payments for the current subscription
func (h *paymentHandler) GetSubscriptionPayments(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	payments, err := h.service.GetSubscriptionPayments(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, payments)
}

// Webhook handles incoming webhooks from Asaas
func (h *paymentHandler) Webhook(c *gin.Context) {
	// Optionally validate webhook secret from header or query
	secret := c.Query("secret")
	if h.webhookSecret != "" && secret != h.webhookSecret {
		headerSecret := c.GetHeader("asaas-access-token")
		if headerSecret != h.webhookSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook secret"})
			return
		}
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Printf("Error reading webhook body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	logger.Printf("Asaas Webhook received: %s", string(body))

	var payload model.AsaasWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Printf("Error parsing webhook payload: %v", err)
		c.JSON(http.StatusOK, gin.H{"status": "received", "error": "parse_error"})
		return
	}

	if err := h.service.HandleWebhook(c.Request.Context(), &payload); err != nil {
		logger.Printf("Error handling webhook: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
