package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/marketplace/model"
	"github.com/aseno/backend/pkg/marketplace/service"
	payment_model "github.com/aseno/backend/pkg/payment/model"
)

type MarketplaceHandler interface {
	RegisterRoutes(router gin.IRouter)
	RegisterPublicRoutes(router gin.IRouter)
	RegisterWebhookRoutes(router gin.IRouter)
}

type marketplaceHandler struct {
	service       service.MarketplaceService
	webhookSecret string
}

func NewMarketplaceHandler(service service.MarketplaceService, webhookSecret string) MarketplaceHandler {
	return &marketplaceHandler{
		service:       service,
		webhookSecret: webhookSecret,
	}
}

// RegisterRoutes registers protected routes (auth + financial access required)
func (h *marketplaceHandler) RegisterRoutes(router gin.IRouter) {
	marketplace := router.Group("/marketplace")
	{
		// Subaccount
		marketplace.POST("/subaccount", h.CreateSubaccount)
		marketplace.GET("/subaccount", h.GetSubaccount)

		// Split config
		marketplace.GET("/split-config", h.GetSplitConfig)
		marketplace.PUT("/split-config", h.UpdateSplitConfig)

		// Financial dashboard
		marketplace.GET("/balance", h.GetBalance)
		marketplace.GET("/payments", h.GetPaymentHistory)
		marketplace.GET("/summary", h.GetFinancialSummary)
	}
}

// RegisterPublicRoutes registers patient-facing routes (no auth or portal auth)
func (h *marketplaceHandler) RegisterPublicRoutes(router gin.IRouter) {
	marketplace := router.Group("/marketplace")
	{
		marketplace.GET("/check/:clinicId", h.CheckClinicPaymentConfig)
		marketplace.POST("/charge/:clinicId/:appointmentId", h.CreateCharge)
		marketplace.GET("/charge/:paymentId/pix", h.GetChargePixQRCode)
		marketplace.GET("/charge/:paymentId/status", h.GetChargeStatus)

		// Payment page (token-based, public)
		marketplace.GET("/pagar/:token", h.GetPaymentPage)
		marketplace.POST("/pagar/:token/charge", h.CreateChargeByToken)
	}
}

// RegisterWebhookRoutes registers webhook endpoint (public, validated by secret)
func (h *marketplaceHandler) RegisterWebhookRoutes(router gin.IRouter) {
	router.POST("/marketplace/webhook", h.HandleWebhook)
}

// CreateSubaccount creates an Asaas subaccount for the authenticated clinic
func (h *marketplaceHandler) CreateSubaccount(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req service.CreateSubaccountInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	resp, err := h.service.CreateSubaccount(c.Request.Context(), clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// GetSubaccount returns the subaccount for the authenticated clinic
func (h *marketplaceHandler) GetSubaccount(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	resp, err := h.service.GetSubaccount(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetSplitConfig returns the split config for the authenticated clinic
func (h *marketplaceHandler) GetSplitConfig(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	resp, err := h.service.GetSplitConfig(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateSplitConfig updates the split config for the authenticated clinic
func (h *marketplaceHandler) UpdateSplitConfig(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req service.UpdateSplitConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	resp, err := h.service.UpdateSplitConfig(c.Request.Context(), clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetBalance returns the financial balance for the authenticated clinic's subaccount
func (h *marketplaceHandler) GetBalance(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	resp, err := h.service.GetBalance(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetPaymentHistory returns paginated clinic payments with filters
func (h *marketplaceHandler) GetPaymentHistory(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var filters model.PaymentFilters
	if err := c.ShouldBindQuery(&filters); err != nil {
		filters = model.PaymentFilters{}
	}
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PerPage <= 0 || filters.PerPage > 100 {
		filters.PerPage = 20
	}

	payments, total, err := h.service.GetPaymentHistory(c.Request.Context(), clinicID.(uuid.UUID), filters)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	totalPages := (int(total) + filters.PerPage - 1) / filters.PerPage

	c.JSON(http.StatusOK, gin.H{
		"data":        payments,
		"total":       total,
		"page":        filters.Page,
		"per_page":    filters.PerPage,
		"total_pages": totalPages,
	})
}

// GetFinancialSummary returns aggregated financial data for the authenticated clinic
func (h *marketplaceHandler) GetFinancialSummary(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	dateFrom := c.DefaultQuery("date_from", "")
	dateTo := c.DefaultQuery("date_to", "")

	resp, err := h.service.GetFinancialSummary(c.Request.Context(), clinicID.(uuid.UUID), dateFrom, dateTo)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateCharge creates a payment charge for an appointment (patient-facing)
func (h *marketplaceHandler) CreateCharge(c *gin.Context) {
	clinicIDStr := c.Param("clinicId")
	clinicID, err := uuid.Parse(clinicIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Invalid clinic ID")))
		return
	}

	appointmentIDStr := c.Param("appointmentId")
	appointmentID, err := uuid.Parse(appointmentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Invalid appointment ID")))
		return
	}

	var req service.CreateChargeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	resp, err := h.service.CreateAppointmentCharge(c.Request.Context(), clinicID, appointmentID, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// GetChargePixQRCode returns the PIX QR code for a clinic payment
func (h *marketplaceHandler) GetChargePixQRCode(c *gin.Context) {
	paymentID := c.Param("paymentId")
	if paymentID == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Invalid payment ID")))
		return
	}

	// We need the clinicID to find the subaccount.
	// Look up the payment first to find the clinic.
	clinicIDStr := c.Query("clinic_id")
	clinicID, err := uuid.Parse(clinicIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("clinic_id query parameter required")))
		return
	}

	resp, err := h.service.GetPaymentPixQRCode(c.Request.Context(), clinicID, paymentID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CheckClinicPaymentConfig checks if a clinic has an active subaccount and payment settings (public endpoint)
func (h *marketplaceHandler) CheckClinicPaymentConfig(c *gin.Context) {
	clinicIDStr := c.Param("clinicId")
	clinicID, err := uuid.Parse(clinicIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Invalid clinic ID")))
		return
	}

	config, err := h.service.GetClinicPaymentConfig(c.Request.Context(), clinicID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"has_subaccount": false, "require_payment": false})
		return
	}

	c.JSON(http.StatusOK, config)
}

// GetChargeStatus returns the current status of a clinic payment (public endpoint for polling)
func (h *marketplaceHandler) GetChargeStatus(c *gin.Context) {
	paymentID := c.Param("paymentId")
	if paymentID == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Invalid payment ID")))
		return
	}

	paymentUUID, err := uuid.Parse(paymentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Invalid payment ID")))
		return
	}

	payment, err := h.service.GetClinicPaymentByID(c.Request.Context(), paymentUUID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":     payment.ID,
		"status": payment.Status,
	})
}

// GetPaymentPage returns data for the public payment page (token-based)
func (h *marketplaceHandler) GetPaymentPage(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Token invalido")))
		return
	}

	data, err := h.service.GetPaymentPageData(c.Request.Context(), token)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, data)
}

// CreateChargeByToken creates a charge using a payment token (public, token-validated)
func (h *marketplaceHandler) CreateChargeByToken(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrInvalidInstance.WithMessage("Token invalido")))
		return
	}

	var req service.CreateChargeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	resp, err := h.service.CreateChargeByToken(c.Request.Context(), token, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// HandleWebhook processes webhook events from Asaas for subaccount payments
func (h *marketplaceHandler) HandleWebhook(c *gin.Context) {
	// Validate webhook secret
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
		logger.Printf("Marketplace webhook: error reading body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	logger.Printf("Marketplace webhook received: %s", string(body))

	var payload payment_model.AsaasWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Printf("Marketplace webhook: error parsing payload: %v", err)
		c.JSON(http.StatusOK, gin.H{"status": "received", "error": "parse_error"})
		return
	}

	if err := h.service.HandleSubaccountWebhook(c.Request.Context(), &payload); err != nil {
		logger.Printf("Marketplace webhook: error handling: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
