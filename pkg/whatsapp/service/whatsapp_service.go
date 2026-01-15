package service

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	aptModel "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/whatsapp/classifier"
	"github.com/aseno/backend/pkg/whatsapp/model"
)

// WhatsAppClient defines the interface for WhatsApp API operations
// This interface allows swapping between different providers (Evolution, Z-API, etc.)
// All methods receive an instance parameter to support per-clinic WhatsApp connections
type WhatsAppClient interface {
	// CreateInstance creates a new WhatsApp instance for a clinic
	CreateInstance(ctx context.Context, instance string) error

	// SendTextMessage sends a simple text message
	SendTextMessage(ctx context.Context, instance, phone, message string) (*model.SendMessageResponse, error)

	// SendButtonMessage sends a message with buttons
	SendButtonMessage(ctx context.Context, instance, phone, message string, buttons []model.MessageButton) (*model.SendMessageResponse, error)

	// GetConnectionStatus gets the current connection status
	GetConnectionStatus(ctx context.Context, instance string) (*model.ConnectionStatus, error)

	// GetQRCode gets the QR code for connecting WhatsApp
	GetQRCode(ctx context.Context, instance string) (*model.QRCodeData, error)

	// Logout disconnects the WhatsApp session
	Logout(ctx context.Context, instance string) error

	// DeleteInstance removes a WhatsApp instance
	DeleteInstance(ctx context.Context, instance string) error
}

// MessageRepository defines the interface for message persistence
type MessageRepository interface {
	Create(ctx context.Context, message *model.Message) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.MessageStatus) error
	GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) ([]*model.Message, error)
	GetPendingReminders(ctx context.Context, before time.Time) ([]*model.Message, error)
}

// MessageLogRepository defines the interface for message log persistence
type MessageLogRepository interface {
	Create(ctx context.Context, log *model.MessageLog) error
	GetByMessageID(ctx context.Context, messageID uuid.UUID) ([]*model.MessageLog, error)
	GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) ([]*model.MessageLog, error)
	GetByPatientID(ctx context.Context, patientID uuid.UUID, limit int) ([]*model.MessageLog, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit int) ([]*model.MessageLog, error)
}

// WhatsAppService defines the interface for WhatsApp business logic
type WhatsAppService interface {
	// CreateInstance creates a new WhatsApp instance for a clinic
	CreateInstance(ctx context.Context, instance string) error

	// SendTextMessage sends a simple text message
	SendTextMessage(ctx context.Context, instance, phone, message string) (*model.SendMessageResponse, error)

	// SendConfirmationMessage sends appointment confirmation
	SendConfirmationMessage(ctx context.Context, instance string, msg *model.ConfirmationMessage) error

	// SendReminderMessage sends appointment reminder
	SendReminderMessage(ctx context.Context, instance string, msg *model.ReminderMessage) error

	// HandleIncomingMessage processes an incoming message from webhook
	HandleIncomingMessage(ctx context.Context, clinicID uuid.UUID, msg *model.ParsedIncomingMessage) error

	// ClassifyMessage classifies the intent of an incoming message
	ClassifyMessage(content string) model.MessageIntent

	// GetConnectionStatus gets the current connection status
	GetConnectionStatus(ctx context.Context, instance string) (*model.ConnectionStatus, error)

	// GetQRCode gets the QR code for WhatsApp connection
	GetQRCode(ctx context.Context, instance string) (*model.QRCodeData, error)

	// Logout disconnects a WhatsApp instance
	Logout(ctx context.Context, instance string) error

	// DeleteInstance removes a WhatsApp instance
	DeleteInstance(ctx context.Context, instance string) error

	// SetAppointmentFinder sets the appointment finder for WhatsApp response handling
	SetAppointmentFinder(finder AppointmentFinder)

	// SetWaitlistHandler sets the waitlist handler for encaixe responses
	SetWaitlistHandler(handler WaitlistResponseHandler)

	// SetMessageLogService sets the message log service for tracking message lifecycle
	SetMessageLogService(service MessageLogService)

	// SetAppointmentUpdater sets the appointment status updater for WhatsApp responses
	SetAppointmentUpdater(updater AppointmentStatusUpdater)

	// SetClassifier sets the AI classifier for message classification
	SetClassifier(c classifier.MessageClassifier, confidenceThreshold float64)

	// SetSlotSuggester sets the slot suggester for reschedule options
	SetSlotSuggester(suggester SlotSuggester)

	// SetAppointmentCreator sets the appointment creator for reschedule
	SetAppointmentCreator(creator AppointmentCreator)
}

// whatsAppService implements WhatsAppService
type whatsAppService struct {
	client              WhatsAppClient
	messageRepo         MessageRepository
	messageLogService   MessageLogService
	appointmentUpdater  AppointmentStatusUpdater
	appointmentFinder   AppointmentFinder
	waitlistHandler     WaitlistResponseHandler
	slotSuggester       SlotSuggester
	appointmentCreator  AppointmentCreator
	rescheduleCache     *rescheduleCache
	classifier          classifier.MessageClassifier
	confidenceThreshold float64
}

// AppointmentStatusUpdater interface for updating appointment status
type AppointmentStatusUpdater interface {
	ConfirmByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, patientNotes string) error
	CancelByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, reason string) error
}

// AppointmentInfo represents appointment info for WhatsApp messages
type AppointmentInfo struct {
	ID             uuid.UUID
	PatientID      uuid.UUID
	ProfessionalID uuid.UUID
	PatientName    string
	PatientPhone   string
	Professional   string
	StartTime      time.Time
	EndTime        time.Time
}

// SlotSuggester interface for suggesting reschedule slots
type SlotSuggester interface {
	SuggestSmartSlots(ctx context.Context, clinicID, professionalID uuid.UUID, date time.Time, days int, durationMinutes int) ([]*aptModel.SmartSlotSuggestion, error)
}

// AppointmentCreator interface for creating appointments from reschedule
type AppointmentCreator interface {
	CreateFromReschedule(ctx context.Context, clinicID uuid.UUID, patientPhone string, slot *aptModel.SmartSlotSuggestion, originalApt *AppointmentInfo) (*AppointmentInfo, error)
}

// rescheduleCache stores pending reschedule options with expiration
type rescheduleCache struct {
	mu      sync.RWMutex
	entries map[string]*rescheduleCacheEntry
}

type rescheduleCacheEntry struct {
	slots       []*aptModel.SmartSlotSuggestion
	originalApt *AppointmentInfo
	clinicID    uuid.UUID
	expiresAt   time.Time
}

func newRescheduleCache() *rescheduleCache {
	return &rescheduleCache{
		entries: make(map[string]*rescheduleCacheEntry),
	}
}

func (c *rescheduleCache) set(phone string, slots []*aptModel.SmartSlotSuggestion, apt *AppointmentInfo, clinicID uuid.UUID, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[phone] = &rescheduleCacheEntry{
		slots:       slots,
		originalApt: apt,
		clinicID:    clinicID,
		expiresAt:   time.Now().Add(ttl),
	}
}

func (c *rescheduleCache) get(phone string) (*rescheduleCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[phone]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry, true
}

func (c *rescheduleCache) delete(phone string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, phone)
}

// AppointmentFinder interface for finding appointments by phone
type AppointmentFinder interface {
	GetPendingByPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*AppointmentInfo, error)
}

// WaitlistResponseHandler interface for handling waitlist responses
type WaitlistResponseHandler interface {
	HandlePatientResponse(ctx context.Context, clinicID uuid.UUID, phone string, accepted bool) error
}

// NewWhatsAppService creates a new WhatsApp service
func NewWhatsAppService(
	client WhatsAppClient,
	messageRepo MessageRepository,
	appointmentUpdater AppointmentStatusUpdater,
) WhatsAppService {
	return &whatsAppService{
		client:             client,
		messageRepo:        messageRepo,
		appointmentUpdater: appointmentUpdater,
		rescheduleCache:    newRescheduleCache(),
	}
}

// SetAppointmentFinder sets the appointment finder for WhatsApp response handling
func (s *whatsAppService) SetAppointmentFinder(finder AppointmentFinder) {
	s.appointmentFinder = finder
}

// SetWaitlistHandler sets the waitlist handler for encaixe responses
func (s *whatsAppService) SetWaitlistHandler(handler WaitlistResponseHandler) {
	s.waitlistHandler = handler
}

// SetMessageLogService sets the message log service for tracking message lifecycle
func (s *whatsAppService) SetMessageLogService(service MessageLogService) {
	s.messageLogService = service
}

// SetAppointmentUpdater sets the appointment status updater for WhatsApp responses
func (s *whatsAppService) SetAppointmentUpdater(updater AppointmentStatusUpdater) {
	s.appointmentUpdater = updater
}

// SetClassifier sets the AI classifier for message classification
func (s *whatsAppService) SetClassifier(c classifier.MessageClassifier, confidenceThreshold float64) {
	s.classifier = c
	s.confidenceThreshold = confidenceThreshold
}

// SetSlotSuggester sets the slot suggester for reschedule options
func (s *whatsAppService) SetSlotSuggester(suggester SlotSuggester) {
	s.slotSuggester = suggester
}

// SetAppointmentCreator sets the appointment creator for reschedule
func (s *whatsAppService) SetAppointmentCreator(creator AppointmentCreator) {
	s.appointmentCreator = creator
}

// CreateInstance creates a new WhatsApp instance for a clinic
func (s *whatsAppService) CreateInstance(ctx context.Context, instance string) error {
	return s.client.CreateInstance(ctx, instance)
}

// SendTextMessage sends a simple text message
func (s *whatsAppService) SendTextMessage(ctx context.Context, instance, phone, message string) (*model.SendMessageResponse, error) {
	formattedPhone := formatPhoneNumber(phone)
	return s.client.SendTextMessage(ctx, instance, formattedPhone, message)
}

// SendConfirmationFeedback sends a message confirming the appointment was confirmed
func (s *whatsAppService) SendConfirmationFeedback(ctx context.Context, instance, phone string, apt *AppointmentInfo) error {
	dateStr := apt.StartTime.Format("02/01/2006")
	timeStr := apt.StartTime.Format("15:04")
	
	message := fmt.Sprintf(
		"✅ Confirmado! Te aguardamos no dia %s as %s com %s.\n\nAte la! 😊",
		dateStr,
		timeStr,
		apt.Professional,
	)

	_, err := s.SendTextMessage(ctx, instance, phone, message)
	return err
}

// SendCancellationFeedback sends a message confirming the appointment was cancelled
func (s *whatsAppService) SendCancellationFeedback(ctx context.Context, instance, phone string) error {
	message := "❌ Entendido, seu agendamento foi cancelado.\n\nSe precisar reagendar, entre em contato conosco."
	_, err := s.SendTextMessage(ctx, instance, phone, message)
	return err
}

// SendRescheduleOptions sends reschedule options after cancellation
func (s *whatsAppService) SendRescheduleOptions(ctx context.Context, instance, phone string, apt *AppointmentInfo, slots []*aptModel.SmartSlotSuggestion, clinicID uuid.UUID) error {
	// Store slots in cache for later lookup
	s.rescheduleCache.set(phone, slots, apt, clinicID, 30*time.Minute)

	// Build message with options
	message := fmt.Sprintf(
		"❌ Seu agendamento com %s foi cancelado.\n\n"+
			"Deseja reagendar? Escolha um novo horario:\n\n",
		apt.Professional,
	)

	buttons := make([]model.MessageButton, 0, len(slots))
	for i, slot := range slots {
		dateStr := slot.StartTime.Format("02/01 15:04")
		message += fmt.Sprintf("%d. %s (%s)\n", i+1, dateStr, slot.Label)
		buttons = append(buttons, model.MessageButton{
			ID:    fmt.Sprintf("rsch_%d", i),
			Title: dateStr,
		})
	}

	message += "\nOu responda NAO se nao quiser reagendar."

	_, err := s.client.SendButtonMessage(ctx, instance, phone, message, buttons)
	return err
}

// handleRescheduleResponse handles a reschedule button response
func (s *whatsAppService) handleRescheduleResponse(ctx context.Context, msg *model.ParsedIncomingMessage) error {
	// Extract slot index from button ID (rsch_0, rsch_1, etc.)
	idxStr := strings.TrimPrefix(msg.ButtonID, "rsch_")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		s.SendTextMessage(ctx, msg.Instance, msg.Phone, "Opcao invalida. Por favor, entre em contato conosco.")
		return err
	}

	// Get cached slots
	entry, ok := s.rescheduleCache.get(msg.Phone)
	if !ok {
		s.SendTextMessage(ctx, msg.Instance, msg.Phone, "Desculpe, as opcoes expiraram. Por favor, entre em contato conosco para reagendar.")
		return nil // Not an error, just expired options
	}

	if idx < 0 || idx >= len(entry.slots) {
		s.SendTextMessage(ctx, msg.Instance, msg.Phone, "Opcao invalida. Por favor, entre em contato conosco.")
		return nil
	}

	selectedSlot := entry.slots[idx]

	// Create new appointment if creator is available
	if s.appointmentCreator == nil {
		s.SendTextMessage(ctx, msg.Instance, msg.Phone, "Reagendamento nao disponivel no momento. Por favor, entre em contato conosco.")
		return nil
	}

	newApt, err := s.appointmentCreator.CreateFromReschedule(ctx, entry.clinicID, msg.Phone, selectedSlot, entry.originalApt)
	if err != nil {
		log.Printf("[WhatsApp] Failed to create reschedule appointment: %v", err)
		s.SendTextMessage(ctx, msg.Instance, msg.Phone, "Nao foi possivel reagendar. Por favor, entre em contato conosco.")
		return err
	}

	// Clear cache
	s.rescheduleCache.delete(msg.Phone)

	// Send confirmation
	return s.SendConfirmationFeedback(ctx, msg.Instance, msg.Phone, newApt)
}

// SendConfirmationMessage sends an appointment confirmation message
func (s *whatsAppService) SendConfirmationMessage(ctx context.Context, instance string, msg *model.ConfirmationMessage) error {
	// Format phone number
	phone := formatPhoneNumber(msg.PatientPhone)

	// Format date/time in Brazilian format
	dateStr := msg.DateTime.Format("02/01/2006")
	timeStr := msg.DateTime.Format("15:04")

	// Build confirmation message with buttons
	message := fmt.Sprintf(
		"Ola %s!\n\n"+
			"Sua consulta foi agendada:\n\n"+
			"📅 Data: %s\n"+
			"🕐 Horario: %s\n"+
			"👨‍⚕️ Profissional: %s\n"+
			"🏥 Local: %s\n\n"+
			"Por favor, confirme sua presenca:",
		msg.PatientName,
		dateStr,
		timeStr,
		msg.Professional,
		msg.ClinicName,
	)

	buttons := []model.MessageButton{
		{ID: "confirm", Title: "✅ Confirmar"},
		{ID: "cancel", Title: "❌ Cancelar"},
	}

	// Send message
	resp, err := s.client.SendButtonMessage(ctx, instance, phone, message, buttons)
	if err != nil {
		// Log failed message
		s.logMessage(ctx, msg.ClinicID, &msg.AppointmentID, phone, model.MessageDirectionOutbound, message, model.MessageStatusFailed, err.Error())
		return err
	}

	// Log successful message
	s.logMessage(ctx, msg.ClinicID, &msg.AppointmentID, phone, model.MessageDirectionOutbound, message, model.MessageStatusSent, resp.ExternalID)

	return nil
}

// SendReminderMessage sends an appointment reminder
func (s *whatsAppService) SendReminderMessage(ctx context.Context, instance string, msg *model.ReminderMessage) error {
	phone := formatPhoneNumber(msg.PatientPhone)

	dateStr := msg.DateTime.Format("02/01/2006")
	timeStr := msg.DateTime.Format("15:04")

	var reminderText string
	switch msg.ReminderType {
	case model.ReminderType48Hours:
		reminderText = "Lembrete: sua consulta e em 2 dias!"
	case model.ReminderType24Hours:
		reminderText = "Lembrete: sua consulta e amanha!"
	case model.ReminderType2Hours:
		reminderText = "Lembrete: sua consulta e em 2 horas!"
	case model.ReminderType1Hour:
		reminderText = "Lembrete: sua consulta e em 1 hora!"
	default:
		reminderText = "Lembrete de consulta"
	}

	message := fmt.Sprintf(
		"🔔 %s\n\n"+
			"Ola %s!\n\n"+
			"📅 Data: %s\n"+
			"🕐 Horario: %s\n"+
			"👨‍⚕️ Profissional: %s\n"+
			"🏥 Local: %s\n\n"+
			"Caso precise remarcar ou cancelar, responda esta mensagem.",
		reminderText,
		msg.PatientName,
		dateStr,
		timeStr,
		msg.Professional,
		msg.ClinicName,
	)

	resp, err := s.client.SendTextMessage(ctx, instance, phone, message)
	if err != nil {
		s.logMessage(ctx, msg.ClinicID, &msg.AppointmentID, phone, model.MessageDirectionOutbound, message, model.MessageStatusFailed, err.Error())
		return err
	}

	s.logMessage(ctx, msg.ClinicID, &msg.AppointmentID, phone, model.MessageDirectionOutbound, message, model.MessageStatusSent, resp.ExternalID)

	return nil
}

// HandleIncomingMessage processes an incoming message
func (s *whatsAppService) HandleIncomingMessage(ctx context.Context, clinicID uuid.UUID, msg *model.ParsedIncomingMessage) error {
	// Log incoming message
	s.logMessage(ctx, clinicID, nil, msg.Phone, model.MessageDirectionInbound, msg.Content, model.MessageStatusDelivered, msg.MessageID)

	// Check for reschedule button response first (rsch_0, rsch_1, etc.)
	if msg.ButtonID != "" && strings.HasPrefix(msg.ButtonID, "rsch_") {
		return s.handleRescheduleResponse(ctx, msg)
	}

	// Classify intent
	intent := s.ClassifyMessage(msg.Content)

	// If it's a button response, use the button ID directly
	if msg.ButtonID != "" {
		switch msg.ButtonID {
		case "confirm":
			intent = model.MessageIntentConfirm
		case "cancel":
			intent = model.MessageIntentCancel
		case "reschedule":
			intent = model.MessageIntentReschedule
		}
	}

	// Handle the response based on intent
	switch intent {
	case model.MessageIntentConfirm:
		// First check if this is a waitlist response
		if s.waitlistHandler != nil {
			if err := s.waitlistHandler.HandlePatientResponse(ctx, clinicID, msg.Phone, true); err == nil {
				// Waitlist response handled successfully
				return nil
			}
		}

		// If not a waitlist response, try to confirm an appointment
		if s.appointmentFinder != nil && s.appointmentUpdater != nil {
			apt, err := s.appointmentFinder.GetPendingByPhone(ctx, clinicID, msg.Phone)
			if err == nil && apt != nil {
				if err := s.appointmentUpdater.ConfirmByPatient(ctx, apt.ID, clinicID, ""); err == nil {
					// Send feedback message
					s.SendConfirmationFeedback(ctx, msg.Instance, msg.Phone, apt)
					return nil
				}
			}
		}

	case model.MessageIntentCancel:
		// First check if this is a waitlist response (declining a slot offer)
		if s.waitlistHandler != nil {
			if err := s.waitlistHandler.HandlePatientResponse(ctx, clinicID, msg.Phone, false); err == nil {
				// Waitlist response handled successfully
				return nil
			}
		}

		// If not a waitlist response, try to cancel an appointment
		if s.appointmentFinder != nil && s.appointmentUpdater != nil {
			apt, err := s.appointmentFinder.GetPendingByPhone(ctx, clinicID, msg.Phone)
			if err == nil && apt != nil {
				if err := s.appointmentUpdater.CancelByPatient(ctx, apt.ID, clinicID, "Cancelado via WhatsApp"); err == nil {
					// Try to offer reschedule options
					if s.slotSuggester != nil {
						durationMinutes := int(apt.EndTime.Sub(apt.StartTime).Minutes())
						if durationMinutes <= 0 {
							durationMinutes = 50 // Default duration
						}

						slots, err := s.slotSuggester.SuggestSmartSlots(
							ctx, clinicID, apt.ProfessionalID,
							time.Now(), 14, // Next 14 days
							durationMinutes,
						)

						if err == nil && len(slots) >= 3 {
							// Limit to 3 options (WhatsApp button limit)
							if len(slots) > 3 {
								slots = slots[:3]
							}
							s.SendRescheduleOptions(ctx, msg.Instance, msg.Phone, apt, slots, clinicID)
							return nil
						}
					}

					// Fallback: simple cancellation message
					s.SendCancellationFeedback(ctx, msg.Instance, msg.Phone)
					return nil
				}
			}
		}

	case model.MessageIntentReschedule:
		// Reschedule requests need manual handling for now
		// Could send a message asking to contact the clinic
		break

	case model.MessageIntentQuestion, model.MessageIntentUnknown:
		// Unknown or question intents - no automatic action
		break
	}

	return nil
}

// ClassifyMessage classifies the intent of a message (uses AI with regex fallback)
func (s *whatsAppService) ClassifyMessage(content string) model.MessageIntent {
	// Use AI classification if available
	if s.classifier != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := s.classifier.Classify(ctx, content)
		if err == nil && result.Confidence >= s.confidenceThreshold {
			log.Printf("[Classifier] AI (%s/%s): intent=%s confidence=%.2f latency=%v reason=%q",
				result.Provider, result.Model, result.Intent, result.Confidence, result.Latency, result.Reasoning)
			return result.Intent
		}
		if err != nil {
			log.Printf("[Classifier] AI error, falling back to regex: %v", err)
		} else {
			log.Printf("[Classifier] AI confidence %.2f below threshold %.2f, falling back to regex",
				result.Confidence, s.confidenceThreshold)
		}
	}

	// Fallback to regex classification
	intent := s.classifyWithRegex(content)
	log.Printf("[Classifier] Regex fallback: intent=%s content=%q", intent, content)
	return intent
}

// classifyWithRegex classifies the intent using regex patterns (fallback)
func (s *whatsAppService) classifyWithRegex(content string) model.MessageIntent {
	content = strings.ToLower(strings.TrimSpace(content))

	// Confirm patterns
	confirmPatterns := []string{
		"^1$", "confirmo", "confirmado", "confirmar", "sim", "ok", "vou", "estarei", "irei",
		"pode confirmar", "confirma", "✅", "👍",
	}

	// Cancel patterns
	cancelPatterns := []string{
		"^2$", "cancela", "cancelar", "cancelado", "nao vou", "não vou", "nao posso", "não posso",
		"desmarcar", "desmarca", "❌",
	}

	// Reschedule patterns
	reschedulePatterns := []string{
		"remarcar", "reagendar", "mudar", "trocar", "outro dia", "outra data", "outro horario",
	}

	for _, pattern := range confirmPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return model.MessageIntentConfirm
		}
	}

	for _, pattern := range cancelPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return model.MessageIntentCancel
		}
	}

	for _, pattern := range reschedulePatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return model.MessageIntentReschedule
		}
	}

	// Check if it's a question
	if strings.Contains(content, "?") {
		return model.MessageIntentQuestion
	}

	return model.MessageIntentUnknown
}

// GetConnectionStatus returns the WhatsApp connection status
func (s *whatsAppService) GetConnectionStatus(ctx context.Context, instance string) (*model.ConnectionStatus, error) {
	return s.client.GetConnectionStatus(ctx, instance)
}

// GetQRCode returns the QR code for connecting WhatsApp
func (s *whatsAppService) GetQRCode(ctx context.Context, instance string) (*model.QRCodeData, error) {
	return s.client.GetQRCode(ctx, instance)
}

// Logout disconnects a WhatsApp instance
func (s *whatsAppService) Logout(ctx context.Context, instance string) error {
	return s.client.Logout(ctx, instance)
}

// DeleteInstance removes a WhatsApp instance
func (s *whatsAppService) DeleteInstance(ctx context.Context, instance string) error {
	return s.client.DeleteInstance(ctx, instance)
}

// logMessage logs a message to the database and creates lifecycle logs
func (s *whatsAppService) logMessage(
	ctx context.Context,
	clinicID uuid.UUID,
	appointmentID *uuid.UUID,
	phone string,
	direction model.MessageDirection,
	content string,
	status model.MessageStatus,
	externalIDOrError string,
) {
	msg := &model.Message{
		ClinicID:      clinicID,
		AppointmentID: appointmentID,
		Phone:         phone,
		Direction:     direction,
		Type:          model.MessageTypeText,
		Content:       content,
		Status:        status,
	}

	if status == model.MessageStatusFailed {
		msg.ErrorMessage = externalIDOrError
	} else {
		msg.ExternalID = externalIDOrError
		now := time.Now()
		msg.SentAt = &now
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		log.Printf("Failed to create message: %v", err)
		return
	}

	// Create lifecycle logs if service is available
	if s.messageLogService != nil {
		payload := model.MessageLogPayload{
			"content":     content,
			"external_id": externalIDOrError,
		}

		// Log message created
		if err := s.messageLogService.LogCreated(ctx, msg, payload); err != nil {
			log.Printf("Failed to log message created: %v", err)
		}

		// Log sent or failed based on status
		if status == model.MessageStatusFailed {
			if err := s.messageLogService.LogFailed(ctx, msg, externalIDOrError, payload); err != nil {
				log.Printf("Failed to log message failed: %v", err)
			}
		} else if status == model.MessageStatusSent {
			if err := s.messageLogService.LogSent(ctx, msg, payload); err != nil {
				log.Printf("Failed to log message sent: %v", err)
			}
		}
	}
}

// formatPhoneNumber formats a phone number for WhatsApp
func formatPhoneNumber(phone string) string {
	// Remove all non-numeric characters
	re := regexp.MustCompile(`[^0-9]`)
	phone = re.ReplaceAllString(phone, "")

	// Add Brazil country code if not present
	if len(phone) == 10 || len(phone) == 11 {
		phone = "55" + phone
	}

	return phone + "@s.whatsapp.net"
}
