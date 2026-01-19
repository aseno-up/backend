package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"github.com/aseno/backend/internal/logger"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/whatsapp/client/evolution"
	"github.com/aseno/backend/pkg/whatsapp/client/waha"
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
	provider      string // "evolution" or "waha"
}

// NewWhatsAppHandler creates a new WhatsApp handler
func NewWhatsAppHandler(
	service service.WhatsAppService,
	clinicRepo clinic_repository.ClinicRepository,
	webhookSecret string,
	provider string,
) WhatsAppHandler {
	return &whatsAppHandler{
		service:       service,
		clinicRepo:    clinicRepo,
		webhookSecret: webhookSecret,
		provider:      provider,
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
		logger.Printf("Error reading webhook body: %v", err)
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance))
		return
	}

	// Log the raw payload for debugging (pretty-printed)
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
		formatted := prettyJSON.String()
		if len(formatted) > 2000 {
			formatted = formatted[:2000] + "\n  ... (truncated)"
		}
		logger.Printf("[Webhook] WhatsApp message received:\n%s", formatted)
	} else {
		logger.Printf("[Webhook] WhatsApp message received (not JSON): %s", string(body))
	}

	// Parse the webhook payload based on provider
	var parsed *model.ParsedIncomingMessage
	switch h.provider {
	case "waha":
		parsed, err = waha.ParseWebhookPayload(body)
	case "evolution":
		parsed, err = evolution.ParseWebhookPayload(body)
	default:
		// Default to Evolution for backwards compatibility
		parsed, err = evolution.ParseWebhookPayload(body)
	}

	if err != nil {
		logger.Printf("Error parsing webhook payload: %v", err)
		// Return 200 to avoid retries from WhatsApp API
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
		logger.Printf("Unknown instance: %s", parsed.Instance)
		c.JSON(http.StatusOK, gin.H{"status": "unknown_instance"})
		return
	}

	// Process the message
	if err := h.service.HandleIncomingMessage(c.Request.Context(), clinic.ID, parsed); err != nil {
		logger.Printf("Error handling incoming message: %v", err)
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

	// Generate a descriptive instance name if it doesn't exist
	if clinic.WhatsAppInstance == "" {
		// Sanitize clinic name for the instance name
		reg := regexp.MustCompile(`[^a-z0-9]+`)
		safeClinicName := strings.ToLower(clinic.Name)
		safeClinicName = reg.ReplaceAllString(safeClinicName, "-")
		safeClinicName = strings.Trim(safeClinicName, "-")
		if len(safeClinicName) > 30 {
			safeClinicName = safeClinicName[:30]
		}
		
		clinic.WhatsAppInstance = fmt.Sprintf("%s-%s", safeClinicName, clinic.ID.String()[:8])

		if err := h.clinicRepo.UpdateWhatsAppInstance(c.Request.Context(), clinic.ID, clinic.WhatsAppInstance); err != nil {
			logger.Printf("[Connect Handler] Failed to save new instance name to DB: %v", err)
			statusCode := errors.GetStatusCode(err)
			c.JSON(statusCode, errors.ToResponse(err))
			return
		}
	}

	// Attempt to create the instance. The Evolution API may return the QR code directly
	// if the instance is new, or may do nothing if it already exists.
	logger.Printf("[Connect Handler] Attempting to create or fetch QR for instance %s", clinic.WhatsAppInstance)
	qrCode, err := h.service.CreateInstance(c.Request.Context(), clinic.WhatsAppInstance)
	if err != nil {
		// This can happen if the instance exists but is in a weird state.
		// We'll log it and try to get the QR code separately as a fallback.
		logger.Printf("[Connect Handler] Info: Error during CreateInstance, will attempt GetQRCode as fallback. Error: %v", err)
	}

	// If CreateInstance didn't return a QR code (e.g., instance already existed), get it now.
	if qrCode == nil || qrCode.Base64 == "" {
		logger.Printf("[Connect Handler] CreateInstance did not return a QR code. Fetching separately for instance %s.", clinic.WhatsAppInstance)

		// Try to get QR code with retries (instance may take a few seconds to generate it)
		maxRetries := 5
		retryDelay := 2 * time.Second

		for i := 0; i < maxRetries; i++ {
			time.Sleep(retryDelay)

			qrCode, err = h.service.GetQRCode(c.Request.Context(), clinic.WhatsAppInstance)
			if err != nil {
				logger.Printf("[Connect Handler] GetQRCode attempt %d/%d failed: %v", i+1, maxRetries, err)
				if i == maxRetries-1 {
					statusCode := errors.GetStatusCode(err)
					c.JSON(statusCode, errors.ToResponse(err))
					return
				}
				continue
			}

			// If we got a QR code, break out of the retry loop
			if qrCode != nil && qrCode.Base64 != "" {
				logger.Printf("[Connect Handler] Successfully obtained QR code on attempt %d/%d", i+1, maxRetries)
				break
			}

			logger.Printf("[Connect Handler] QR code not ready yet (attempt %d/%d), will retry...", i+1, maxRetries)
		}

		// If still no QR code (Base64 is empty), the instance might be in a stuck state
		// Check the connection status before deciding to delete
		if qrCode == nil || qrCode.Base64 == "" {
			logger.Printf("[Connect Handler] No QR code available for instance %s. Checking connection status.", clinic.WhatsAppInstance)

			// Check if the instance is already connected
			status, statusErr := h.service.GetConnectionStatus(c.Request.Context(), clinic.WhatsAppInstance)
			if statusErr == nil && status != nil && status.State == "open" {
				// Instance is already connected, no need for QR code
				logger.Printf("[Connect Handler] Instance %s is already connected (state: open). No QR code needed.", clinic.WhatsAppInstance)
				c.JSON(http.StatusOK, gin.H{
					"message": "WhatsApp já está conectado",
					"state":   "open",
				})
				return
			}

			// Instance is not connected and has no QR code - restart it
			logger.Printf("[Connect Handler] Instance %s is not connected (state: %s). Restarting instance.",
				clinic.WhatsAppInstance,
				func() string {
					if status != nil {
						return status.State
					}
					return "unknown"
				}())

			// Delete the instance completely to clear any stuck state
			if err := h.service.DeleteInstance(c.Request.Context(), clinic.WhatsAppInstance); err != nil {
				logger.Printf("[Connect Handler] Warning: Delete instance failed during restart: %v", err)
			}

			// Wait a moment for deletion to complete
			time.Sleep(2 * time.Second)

			// Try creating instance again
			qrCode, err = h.service.CreateInstance(c.Request.Context(), clinic.WhatsAppInstance)
			if err != nil {
				logger.Printf("[Connect Handler] Failed to restart instance: %v", err)
				statusCode := errors.GetStatusCode(err)
				c.JSON(statusCode, errors.ToResponse(err))
				return
			}

			// If still no QR code after restart, try GetQRCode one more time
			if qrCode == nil || qrCode.Base64 == "" {
				time.Sleep(2 * time.Second)
				qrCode, err = h.service.GetQRCode(c.Request.Context(), clinic.WhatsAppInstance)
				if err != nil {
					logger.Printf("[Connect Handler] Failed to get QR code after restart: %v", err)
					statusCode := errors.GetStatusCode(err)
					c.JSON(statusCode, errors.ToResponse(err))
					return
				}
			}
		}
	}

	// Final check: if we still don't have a QR code, return an error
	if qrCode == nil || qrCode.Base64 == "" {
		logger.Printf("[Connect Handler] Failed to obtain QR code for instance %s after all attempts", clinic.WhatsAppInstance)
		c.JSON(http.StatusInternalServerError, errors.ToResponse(
			errors.ErrInternalInstance.WithMessage("Não foi possível gerar o QR Code. Por favor, tente novamente."),
		))
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
		logger.Printf("Error logging out from Evolution: %v", err)
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
	logger.Printf("[SendMessage] Request received")

	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		logger.Printf("[SendMessage] Error getting clinic from context: %v", err)
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	logger.Printf("[SendMessage] Clinic: %s, Instance: %s, Connected: %v", clinic.ID, clinic.WhatsAppInstance, clinic.WhatsAppConnected)

	if clinic.WhatsAppInstance == "" || !clinic.WhatsAppConnected {
		logger.Printf("[SendMessage] WhatsApp not connected")
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not connected")))
		return
	}

	var req model.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Printf("[SendMessage] Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	// Log message details in pretty JSON format
	reqJSON, _ := json.MarshalIndent(map[string]interface{}{
		"phone":   req.Phone,
		"message": req.Message,
		"buttons": req.Buttons,
	}, "", "  ")
	logger.Printf("[SendMessage] Sending message:\n%s", string(reqJSON))

	// Send the message
	resp, err := h.service.SendTextMessage(c.Request.Context(), clinic.WhatsAppInstance, req.Phone, req.Message)
	if err != nil {
		logger.Printf("[SendMessage] Error sending message: %v", err)
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	logger.Printf("[SendMessage] Message sent successfully: %s", resp.MessageID)

	c.JSON(http.StatusOK, gin.H{
		"status":    "sent",
		"messageId": resp.MessageID,
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