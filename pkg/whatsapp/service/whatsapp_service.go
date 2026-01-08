package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

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
}

// whatsAppService implements WhatsAppService
type whatsAppService struct {
	client     WhatsAppClient
	messageRepo MessageRepository
	appointmentUpdater AppointmentStatusUpdater
}

// AppointmentStatusUpdater interface for updating appointment status
type AppointmentStatusUpdater interface {
	ConfirmByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, patientNotes string) error
	CancelByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, reason string) error
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
	}
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

	// TODO: Find the pending appointment for this phone number
	// For now, just log the intent
	_ = intent

	return nil
}

// ClassifyMessage classifies the intent of a message
func (s *whatsAppService) ClassifyMessage(content string) model.MessageIntent {
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

// logMessage logs a message to the database
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

	_ = s.messageRepo.Create(ctx, msg)
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
