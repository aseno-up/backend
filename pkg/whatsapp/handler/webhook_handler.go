package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aseno/backend/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/internal/planlimit"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/whatsapp/client/evolution"
	"github.com/aseno/backend/pkg/whatsapp/client/gupshup"
	"github.com/aseno/backend/pkg/whatsapp/client/waha"
	"github.com/aseno/backend/pkg/whatsapp/model"
	whatsapp_repository "github.com/aseno/backend/pkg/whatsapp/repository"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// WhatsAppHandler defines the interface for WhatsApp handlers
type WhatsAppHandler interface {
	RegisterRoutes(router gin.IRouter)
	Webhook(c *gin.Context)
	WebhookWithSecret(c *gin.Context)
	WebhookEvolution(c *gin.Context)
	WebhookWaha(c *gin.Context)
	WebhookGupshup(c *gin.Context)
	GetStatus(c *gin.Context)
	GetQRCode(c *gin.Context)
	Connect(c *gin.Context)
	Configure(c *gin.Context)
	Disconnect(c *gin.Context)
	SendMessage(c *gin.Context)
	SendConfirmation(c *gin.Context)
}

// whatsAppHandler implements WhatsAppHandler
type whatsAppHandler struct {
	service       service.WhatsAppService
	clinicRepo    clinic_repository.ClinicRepository
	connRepo      whatsapp_repository.WhatsAppConnectionRepository
	webhookSecret string
	provider      string // "evolution" or "waha"
}

// NewWhatsAppHandler creates a new WhatsApp handler
func NewWhatsAppHandler(
	service service.WhatsAppService,
	clinicRepo clinic_repository.ClinicRepository,
	connRepo whatsapp_repository.WhatsAppConnectionRepository,
	webhookSecret string,
	provider string,
) WhatsAppHandler {
	return &whatsAppHandler{
		service:       service,
		clinicRepo:    clinicRepo,
		connRepo:      connRepo,
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
		whatsapp.POST("/config", h.Configure)
	}
}

// Webhook handles incoming webhooks (legacy generic endpoint)
func (h *whatsAppHandler) Webhook(c *gin.Context) {
	h.processWebhook(c, h.provider)
}

// WebhookWithSecret handles webhooks with a secret in the URL
func (h *whatsAppHandler) WebhookWithSecret(c *gin.Context) {
	secret := c.Param("secret")
	if h.webhookSecret != "" && secret != h.webhookSecret {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	h.processWebhook(c, h.provider)
}

// WebhookEvolution handles incoming webhooks specifically for Evolution
func (h *whatsAppHandler) WebhookEvolution(c *gin.Context) {
	h.processWebhook(c, "evolution")
}

// WebhookWaha handles incoming webhooks specifically for WAHA
func (h *whatsAppHandler) WebhookWaha(c *gin.Context) {
	h.processWebhook(c, "waha")
}

// WebhookGupshup handles incoming webhooks specifically for Gupshup/Official
func (h *whatsAppHandler) WebhookGupshup(c *gin.Context) {
	h.processWebhook(c, "gupshup")
}

// processWebhook processes the webhook payload
func (h *whatsAppHandler) processWebhook(c *gin.Context, provider string) {
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

	// Connection lifecycle events (Evolution: connection.update) update the
	// `connected` flag on whatsapp_connections. Without this, reminder_worker
	// filters the clinic out of its query and no reminders ever leave.
	if provider == "evolution" {
		var envelope struct {
			Event    string `json:"event"`
			Instance string `json:"instance"`
			Data     struct {
				State string `json:"state"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &envelope); err == nil && envelope.Event == "connection.update" {
			conn, err := h.connRepo.GetByInstance(c.Request.Context(), envelope.Instance)
			if err != nil {
				logger.Printf("[Webhook] connection.update for unknown instance: %s", envelope.Instance)
				c.JSON(http.StatusOK, gin.H{"status": "unknown_instance"})
				return
			}
			// Baileys emits "connecting" during transient reconnects; only
			// "open" and "close" are terminal states. Flipping on "connecting"
			// causes the reminder worker to drop the clinic mid-flight.
			switch envelope.Data.State {
			case "open":
				if err := h.connRepo.UpdateConnected(c.Request.Context(), conn.ClinicID, true); err != nil {
					logger.Printf("[Webhook] Failed to update connected for clinic %s: %v", conn.ClinicID, err)
				}
				c.JSON(http.StatusOK, gin.H{"status": "connection_updated", "connected": true})
				return
			case "close":
				if err := h.connRepo.UpdateConnected(c.Request.Context(), conn.ClinicID, false); err != nil {
					logger.Printf("[Webhook] Failed to update connected for clinic %s: %v", conn.ClinicID, err)
				}
				c.JSON(http.StatusOK, gin.H{"status": "connection_updated", "connected": false})
				return
			default:
				c.JSON(http.StatusOK, gin.H{"status": "connection_transient", "state": envelope.Data.State})
				return
			}
		}
	}

	// Parse the webhook payload based on provider
	var parsed *model.ParsedIncomingMessage
	switch provider {
	case "waha":
		parsed, err = waha.ParseWebhookPayload(body)
	case "evolution":
		parsed, err = evolution.ParseWebhookPayload(body)
	case "gupshup":
		parsed, err = gupshup.ParseWebhookPayload(body)
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

	// Get clinic ID from instance name (now querying whatsapp_connections table)
	conn, err := h.connRepo.GetByInstance(c.Request.Context(), parsed.Instance)
	if err != nil {
		logger.Printf("Unknown instance: %s", parsed.Instance)
		c.JSON(http.StatusOK, gin.H{"status": "unknown_instance"})
		return
	}

	// Process the message
	if err := h.service.HandleIncomingMessage(c.Request.Context(), conn.ClinicID, parsed); err != nil {
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

	// Check WhatsApp connection limit per clinic
	if clinic.Account != nil {
		limit := clinic.Account.MaxWhatsappConnections
		if !planlimit.IsUnlimited(limit) {
			count, err := h.connRepo.CountActiveByClinicID(c.Request.Context(), clinic.ID)
			if err != nil {
				statusCode := errors.GetStatusCode(err)
				c.JSON(statusCode, errors.ToResponse(err))
				return
			}
			// Allow if this clinic already had a connection (reconnecting after disconnect), block only truly new ones
			hasExisting := h.connRepo.HasAnyByClinicID(c.Request.Context(), clinic.ID)
			if !hasExisting && int(count) >= limit {
				c.JSON(http.StatusForbidden, errors.ToResponse(
					errors.ErrValidationInstance.WithMessage(
						fmt.Sprintf("Limite de %d conexões WhatsApp por unidade atingido. Faça upgrade para adicionar mais.", limit))))
				return
			}
		}
	}

	// Get or create the active WhatsApp connection for this clinic.
	// If the connection row exists but Instance is nil/empty (dados legados ou
	// efeito colateral de UpdateProvider), tratamos como "precisa gerar nome novo"
	// — caso contrario enviariamos string vazia para a Evolution API e ela
	// responderia 403 "This name \"\" is already in use.".
	conn, connErr := h.connRepo.GetActiveByClinicID(c.Request.Context(), clinic.ID)
	var instance string
	if connErr == nil && conn.Instance != nil && *conn.Instance != "" {
		instance = *conn.Instance
	} else {
		instance = generateInstanceName(clinic)
		if err := h.connRepo.UpsertInstance(c.Request.Context(), clinic.ID, instance); err != nil {
			logger.Printf("[Connect Handler] Failed to save new instance name to DB: %v", err)
			statusCode := errors.GetStatusCode(err)
			c.JSON(statusCode, errors.ToResponse(err))
			return
		}
	}

	// Attempt to create the instance. The Evolution API may return the QR code directly
	// if the instance is new, or may do nothing if it already exists.
	logger.Printf("[Connect Handler] Attempting to create or fetch QR for instance %s", instance)
	qrCode, err := h.service.CreateInstance(c.Request.Context(), instance)
	if err != nil {
		// This can happen if the instance exists but is in a weird state.
		// We'll log it and try to get the QR code separately as a fallback.
		logger.Printf("[Connect Handler] Info: Error during CreateInstance, will attempt GetQRCode as fallback. Error: %v", err)
	}

	// If CreateInstance didn't return a QR code (e.g., instance already existed), get it now.
	if qrCode == nil || qrCode.Base64 == "" {
		logger.Printf("[Connect Handler] CreateInstance did not return a QR code. Fetching separately for instance %s.", instance)

		// Try to get QR code with retries (instance may take a few seconds to generate it)
		maxRetries := 5
		retryDelay := 2 * time.Second

		for i := 0; i < maxRetries; i++ {
			time.Sleep(retryDelay)

			qrCode, err = h.service.GetQRCode(c.Request.Context(), instance)
			if err != nil {
				logger.Printf("[Connect Handler] GetQRCode attempt %d/%d failed: %v", i+1, maxRetries, err)

				// If instance doesn't exist on provider (404), recreate it
				if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "404") {
					logger.Printf("[Connect Handler] Instance %s not found on provider, recreating...", instance)
					if newQR, createErr := h.service.CreateInstance(c.Request.Context(), instance); createErr == nil && newQR != nil && newQR.Base64 != "" {
						qrCode = newQR
						break
					}
				}

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
			logger.Printf("[Connect Handler] No QR code available for instance %s. Checking connection status.", instance)

			// Check if the instance is already connected
			status, statusErr := h.service.GetConnectionStatus(c.Request.Context(), instance)
			if statusErr == nil && status != nil && status.State == "open" {
				// Instance is already connected, no need for QR code
				logger.Printf("[Connect Handler] Instance %s is already connected (state: open). No QR code needed.", instance)
				c.JSON(http.StatusOK, gin.H{
					"message": "WhatsApp já está conectado",
					"state":   "open",
				})
				return
			}

			// Instance is not connected and has no QR code - restart it
			logger.Printf("[Connect Handler] Instance %s is not connected (state: %s). Restarting instance.",
				instance,
				func() string {
					if status != nil {
						return status.State
					}
					return "unknown"
				}())

			// Delete the instance completely to clear any stuck state
			if err := h.service.DeleteInstance(c.Request.Context(), instance); err != nil {
				logger.Printf("[Connect Handler] Warning: Delete instance failed during restart: %v", err)
			}

			// Wait a moment for deletion to complete
			time.Sleep(2 * time.Second)

			// Try creating instance again
			qrCode, err = h.service.CreateInstance(c.Request.Context(), instance)
			if err != nil {
				logger.Printf("[Connect Handler] Failed to restart instance: %v", err)
				statusCode := errors.GetStatusCode(err)
				c.JSON(statusCode, errors.ToResponse(err))
				return
			}

			// If still no QR code after restart, try GetQRCode one more time
			if qrCode == nil || qrCode.Base64 == "" {
				time.Sleep(2 * time.Second)
				qrCode, err = h.service.GetQRCode(c.Request.Context(), instance)
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
		logger.Printf("[Connect Handler] Failed to obtain QR code for instance %s after all attempts", instance)
		c.JSON(http.StatusInternalServerError, errors.ToResponse(
			errors.ErrInternalInstance.WithMessage("Não foi possível gerar o QR Code. Por favor, tente novamente."),
		))
		return
	}

	c.JSON(http.StatusOK, qrCode)
}

// generateInstanceName builds a deterministic Evolution instance name from the
// clinic name + short UUID. Kept outside Connect so the same logic is used
// both on first connection and when recovering from a corrupted row with a
// nil/empty Instance.
func generateInstanceName(clinic *clinic_model.Clinic) string {
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	safe := strings.ToLower(clinic.Name)
	safe = reg.ReplaceAllString(safe, "-")
	safe = strings.Trim(safe, "-")
	if len(safe) > 30 {
		safe = safe[:30]
	}
	return fmt.Sprintf("%s-%s", safe, clinic.ID.String()[:8])
}

// ConfigRequest payload for saving WhatsApp config
type ConfigRequest struct {
	Provider      string  `json:"provider"`
	WabaID        *string `json:"waba_id,omitempty"`
	PhoneNumberID *string `json:"phone_number_id,omitempty"`
	Token         *string `json:"token,omitempty"`
	Instance      *string `json:"instance,omitempty"`
}

// Configure saves WhatsApp connection info like Provider, Token, etc.
func (h *whatsAppHandler) Configure(c *gin.Context) {
	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Payload inválido")))
		return
	}

	conn, connErr := h.connRepo.GetActiveByClinicID(c.Request.Context(), clinic.ID)

	if connErr != nil {
		if req.Provider == "EVOLUTION" {
			// Evolution connections are created by the Connect handler (which generates the instance name).
			// Just acknowledge the provider switch here.
			c.JSON(http.StatusOK, gin.H{"status": "success"})
			return
		}
		// Create new (official API — instance is provided)
		conn = &model.WhatsAppConnection{
			ClinicID:      clinic.ID,
			Provider:      req.Provider,
			WabaID:        req.WabaID,
			PhoneNumberID: req.PhoneNumberID,
			Token:         req.Token,
			Instance:      req.Instance,
			Connected:     true,
			IsActive:      true,
		}
		if err := h.connRepo.Create(c.Request.Context(), conn); err != nil {
			c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
			return
		}
	} else {
		// Update existing
		conn.Provider = req.Provider
		conn.WabaID = req.WabaID
		conn.PhoneNumberID = req.PhoneNumberID
		conn.Token = req.Token
		if req.Instance != nil {
			conn.Instance = req.Instance
		}

		if req.Provider == "EVOLUTION" {
			conn.Connected = false
		} else {
			conn.Connected = true
		}

		if err := h.connRepo.Update(c.Request.Context(), conn); err != nil {
			c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// Disconnect disconnects the clinic's WhatsApp instance
func (h *whatsAppHandler) Disconnect(c *gin.Context) {
	clinic, err := h.getClinicFromContext(c)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	conn, err := h.connRepo.GetActiveByClinicID(c.Request.Context(), clinic.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not connected")))
		return
	}

	// Logout from provider
	var instance string
	if conn.Instance != nil {
		instance = *conn.Instance
	}
	if err := h.service.Logout(c.Request.Context(), instance); err != nil {
		logger.Printf("Error logging out from provider: %v", err)
	}

	// Deactivate connection so it won't be found by GetStatus
	if err := h.connRepo.Deactivate(c.Request.Context(), clinic.ID); err != nil {
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

	conn, err := h.connRepo.GetActiveByClinicID(c.Request.Context(), clinic.ID)
	if err != nil {
		c.JSON(http.StatusOK, &model.ConnectionStatus{
			Instance: "",
			State:    "disconnected",
		})
		return
	}

	instance := ""
	if conn.Instance != nil {
		instance = *conn.Instance
	}

	status, err := h.service.GetConnectionStatus(c.Request.Context(), instance)
	if err != nil {
		// If error, assume disconnected
		c.JSON(http.StatusOK, &model.ConnectionStatus{
			Instance: instance,
			Provider: conn.Provider,
			State:    "disconnected",
		})
		return
	}

	status.Provider = conn.Provider

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

	conn, err := h.connRepo.GetActiveByClinicID(c.Request.Context(), clinic.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not initialized. Call /connect first.")))
		return
	}

	instance := ""
	if conn.Instance != nil {
		instance = *conn.Instance
	}
	qrCode, err := h.service.GetQRCode(c.Request.Context(), instance)
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

	conn, err := h.connRepo.GetActiveByClinicID(c.Request.Context(), clinic.ID)
	if err != nil || !conn.Connected {
		logger.Printf("[SendMessage] WhatsApp not connected for clinic %s", clinic.ID)
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not connected")))
		return
	}

	logger.Printf("[SendMessage] Clinic: %s, Instance: %s, Connected: %v", clinic.ID, conn.Instance, conn.Connected)

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

	instance := ""
	if conn.Instance != nil {
		instance = *conn.Instance
	}
	// Send the message
	resp, err := h.service.SendTextMessage(c.Request.Context(), instance, req.Phone, req.Message)
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

	conn, err := h.connRepo.GetActiveByClinicID(c.Request.Context(), clinic.ID)
	if err != nil || !conn.Connected {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not connected")))
		return
	}

	var req model.ConfirmationMessage
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	req.ClinicID = clinic.ID

	instance := ""
	if conn.Instance != nil {
		instance = *conn.Instance
	}
	if err := h.service.SendConfirmationMessage(c.Request.Context(), instance, &req); err != nil {
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

	conn, err := h.connRepo.GetActiveByClinicID(c.Request.Context(), clinic.ID)
	if err != nil || !conn.Connected {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("WhatsApp not connected")))
		return
	}

	var req SendReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	req.ClinicID = clinic.ID

	instance := ""
	if conn.Instance != nil {
		instance = *conn.Instance
	}
	if err := h.service.SendReminderMessage(c.Request.Context(), instance, &req.ReminderMessage); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "sent",
		"message": "Reminder message sent successfully",
	})
}
