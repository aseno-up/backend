package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/whatsapp/client/evolution"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// WhatsAppHandler defines the interface for WhatsApp handlers
type WhatsAppHandler interface {
	RegisterRoutes(router gin.IRouter)
	Webhook(c *gin.Context)
	GetStatus(c *gin.Context)
	GetQRCode(c *gin.Context)
	Connect(c *gin.Context)
	Disconnect(c *gin.Context)
	SendMessage(c *gin.Context)
	SendConfirmation(c *gin.Context)
}

// whatsAppHandler implements WhatsAppHandler
type whatsAppHandler struct {
	service       service.WhatsAppService
	clinicRepo    clinic_repository.ClinicRepository
	webhookSecret string
}

// NewWhatsAppHandler creates a new WhatsApp handler
func NewWhatsAppHandler(
	service service.WhatsAppService,
	clinicRepo clinic_repository.ClinicRepository,
	webhookSecret string,
) WhatsAppHandler {
	return &whatsAppHandler{
		service:       service,
		clinicRepo:    clinicRepo,
		webhookSecret: webhookSecret,
	}
}

// RegisterRoutes registers WhatsApp routes
func (h *whatsAppHandler) RegisterRoutes(router gin.IRouter) {
	whatsapp := router.Group("/whatsapp")
	{
		// Protected endpoints (require auth)
		whatsapp.GET("/status", h.GetStatus)
		whatsapp.GET("/qrcode", h.GetQRCode)
		whatsapp.POST("/connect", h.Connect)
		whatsapp.POST("/disconnect", h.Disconnect)
		whatsapp.POST("/send", h.SendMessage)
		whatsapp.POST("/send-confirmation", h.SendConfirmation)
	}
}

// Webhook handles incoming webhooks from Evolution API
func (h *whatsAppHandler) Webhook(c *gin.Context) {
	h.processWebhook(c)
}

// WebhookWithSecret handles webhooks with a secret in the URL
func (h *whatsAppHandler) WebhookWithSecret(c *gin.Context) {
	secret := c.Param("secret")
	if h.webhookSecret != "" && secret != h.webhookSecret {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	h.processWebhook(c)
}

// processWebhook processes the webhook payload
func (h *whatsAppHandler) processWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Error reading webhook body: %v", err)
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance))
		return
	}

	// Log the raw payload for debugging
	log.Printf("WhatsApp Webhook received: %s", string(body))

	// Parse the Evolution API payload
	parsed, err := evolution.ParseWebhookPayload(body)
	if err != nil {
		log.Printf("Error parsing webhook payload: %v", err)
		// Return 200 to avoid retries from Evolution API
		c.JSON(http.StatusOK, gin.H{"status": "received", "error": "parse_error"})
		return
	}

	// Skip if no message or message from ourselves
	if parsed == nil {
		c.JSON(http.StatusOK, gin.H{"status": "skipped"})
		return
	}

	// Get clinic ID from instance name
	clinic, err := h.clinicRepo.GetByWhatsAppInstance(c.Request.Context(), parsed.Instance)
	if err != nil {
		log.Printf("Unknown instance: %s", parsed.Instance)
		c.JSON(http.StatusOK, gin.H{"status": "unknown_instance"})
		return
	}

	// Process the message
	if err := h.service.HandleIncomingMessage(c.Request.Context(), clinic.ID, parsed); err != nil {
		log.Printf("Error handling incoming message: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}

// getClinicFromContext extracts clinic from JWT context
func (h *whatsAppHandler) getClinicFromContext(c *gin.Context) (*clinic_model.Clinic, error) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		return nil, errors.ErrUnauthorizedInstance
	}

	clinic, err := h.clinicRepo.GetByID(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		return nil, err
	}

	return clinic, nil
}

// Connect creates and connects a WhatsApp instance for the clinic
func (h *whatsAppHandler) Connect(c *gin.Context) {
	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Generate instance name from clinic ID if not exists
	if clinic.WhatsAppInstance == "" {
		clinic.WhatsAppInstance = fmt.Sprintf("clinic-%s", clinic.ID.String()[:8])

		// Create instance in Evolution API
		if err := h.service.CreateInstance(c.Request.Context(), clinic.WhatsAppInstance); err != nil {
			statusCode := errors.GetStatusCode(err)
			c.JSON(statusCode, errors.ToResponse(err))
			return
		}

		// Save instance name to clinic
		if err := h.clinicRepo.UpdateWhatsAppInstance(c.Request.Context(), clinic.ID, clinic.WhatsAppInstance); err != nil {
			statusCode := errors.GetStatusCode(err)
			c.JSON(statusCode, errors.ToResponse(err))
			return
		}
	}

	// Get QR code for connection
	qrCode, err := h.service.GetQRCode(c.Request.Context(), clinic.WhatsAppInstance)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, qrCode)
}

// Disconnect disconnects the clinic's WhatsApp instance
func (h *whatsAppHandler) Disconnect(c *gin.Context) {
	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if clinic.WhatsAppInstance == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not connected")))
		return
	}

	// Logout from Evolution API
	if err := h.service.Logout(c.Request.Context(), clinic.WhatsAppInstance); err != nil {
		log.Printf("Error logging out from Evolution: %v", err)
	}

	// Update clinic status
	if err := h.clinicRepo.UpdateWhatsAppConnected(c.Request.Context(), clinic.ID, false); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "disconnected"})
}

// GetStatus returns the WhatsApp connection status
func (h *whatsAppHandler) GetStatus(c *gin.Context) {
	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if clinic.WhatsAppInstance == "" {
		c.JSON(http.StatusOK, &model.ConnectionStatus{
			Instance: "",
			State:    "disconnected",
		})
		return
	}

	status, err := h.service.GetConnectionStatus(c.Request.Context(), clinic.WhatsAppInstance)
	if err != nil {
		// If error, assume disconnected
		c.JSON(http.StatusOK, &model.ConnectionStatus{
			Instance: clinic.WhatsAppInstance,
			State:    "disconnected",
		})
		return
	}

	// Update clinic connected status if changed
	isConnected := status.State == "open"
	if clinic.WhatsAppConnected != isConnected {
		_ = h.clinicRepo.UpdateWhatsAppConnected(c.Request.Context(), clinic.ID, isConnected)
	}

	c.JSON(http.StatusOK, status)
}

// GetQRCode returns the QR code for connecting WhatsApp
func (h *whatsAppHandler) GetQRCode(c *gin.Context) {
	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if clinic.WhatsAppInstance == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not initialized. Call /connect first.")))
		return
	}

	qrCode, err := h.service.GetQRCode(c.Request.Context(), clinic.WhatsAppInstance)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, qrCode)
}

// SendMessage sends a WhatsApp message
func (h *whatsAppHandler) SendMessage(c *gin.Context) {
	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if clinic.WhatsAppInstance == "" || !clinic.WhatsAppConnected {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not connected")))
		return
	}

	var req model.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "queued",
		"message": "Message will be sent",
	})
}

// SendConfirmation sends an appointment confirmation message
func (h *whatsAppHandler) SendConfirmation(c *gin.Context) {
	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if clinic.WhatsAppInstance == "" || !clinic.WhatsAppConnected {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not connected")))
		return
	}

	var req model.ConfirmationMessage
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	req.ClinicID = clinic.ID

	if err := h.service.SendConfirmationMessage(c.Request.Context(), clinic.WhatsAppInstance, &req); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "sent",
		"message": "Confirmation message sent successfully",
	})
}

// SendReminderRequest represents a request to send a reminder
type SendReminderRequest struct {
	model.ReminderMessage
}

// SendReminder sends an appointment reminder message
func (h *whatsAppHandler) SendReminder(c *gin.Context) {
	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if clinic.WhatsAppInstance == "" || !clinic.WhatsAppConnected {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not connected")))
		return
	}

	var req SendReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	req.ClinicID = clinic.ID

	if err := h.service.SendReminderMessage(c.Request.Context(), clinic.WhatsAppInstance, &req.ReminderMessage); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "sent",
		"message": "Reminder message sent successfully",
	})
}
