package service

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aseno/backend/internal/logger"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/queue"
	aptModel "github.com/aseno/backend/pkg/appointment/model"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	"github.com/aseno/backend/pkg/whatsapp/agent"
	"github.com/aseno/backend/pkg/whatsapp/classifier"
	"github.com/aseno/backend/pkg/whatsapp/model"
)

// WhatsAppClient defines the interface for WhatsApp API operations
// This interface allows swapping between different providers (Evolution, Z-API, etc.)
// All methods receive an instance parameter to support per-clinic WhatsApp connections
type WhatsAppClient interface {
	// CreateInstance creates a new WhatsApp instance for a clinic
	CreateInstance(ctx context.Context, instance string) (*model.QRCodeData, error)

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

// SchedulingIntentRepository defines the interface for scheduling intent persistence
type SchedulingIntentRepository interface {
	Create(ctx context.Context, intent *model.SchedulingIntent) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.SchedulingIntent, error)
	GetPendingByPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*model.SchedulingIntent, error)
	Update(ctx context.Context, intent *model.SchedulingIntent) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.SchedulingIntentStatus) error
	GetExpired(ctx context.Context) ([]*model.SchedulingIntent, error)
	ExpireOld(ctx context.Context) error
}

// MessageQueue defines the interface for enqueueing WhatsApp messages
type MessageQueue interface {
	PublishWithDelay(ctx context.Context, job *queue.MessageJob, delay time.Duration) error
}

// ClinicRepository defines the interface for clinic operations
type ClinicRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*clinic_model.Clinic, error)
}

// ScheduledMessageRepository defines the interface for scheduled message operations
type ScheduledMessageRepository interface {
	Create(ctx context.Context, msg *model.ScheduledMessage) error
	GetLastScheduledForClinic(ctx context.Context, clinicID uuid.UUID) (*model.ScheduledMessage, error)
}

// WhatsAppService defines the interface for WhatsApp business logic
type WhatsAppService interface {
	// CreateInstance creates a new WhatsApp instance for a clinic
	CreateInstance(ctx context.Context, instance string) (*model.QRCodeData, error)

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

	// SetSchedulingIntentRepo sets the scheduling intent repository for new scheduling flow
	SetSchedulingIntentRepo(repo SchedulingIntentRepository)

	// SetPatientFinderCreator sets the patient finder/creator for scheduling
	SetPatientFinderCreator(finder PatientFinderCreator)

	// SetProfessionalLister sets the professional lister for scheduling
	SetProfessionalLister(lister ProfessionalLister)

	// SetSlotFinder sets the slot finder for scheduling
	SetSlotFinder(finder SlotFinder)

	// SetNewAppointmentCreator sets the new appointment creator for scheduling
	SetNewAppointmentCreator(creator NewAppointmentCreator)

	// SetMessageQueue sets the message queue for segmented message delivery
	SetMessageQueue(queue MessageQueue)

	// SetAIAgentService wires up the AI agent for dynamic conversations
	SetAIAgentService(agentService agent.Service)

	// SetClinicRepo sets the clinic repository for fetching clinic settings
	SetClinicRepo(repo ClinicRepository)

	// SetScheduledMessageRepo sets the scheduled message repository
	SetScheduledMessageRepo(repo ScheduledMessageRepository)

	// SetDefaultDelayBetweenPatients sets the default delay between patients from config
	SetDefaultDelayBetweenPatients(delay int)

	// SetDebounceDelay sets the idle window before the AI processes a patient's message.
	// Any new message received within the window resets the timer.
	SetDebounceDelay(d time.Duration)
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
	// New scheduling dependencies
	schedulingIntentRepo  SchedulingIntentRepository
	patientFinderCreator  PatientFinderCreator
	professionalLister    ProfessionalLister
	slotFinder            SlotFinder
	newAppointmentCreator NewAppointmentCreator
	// Message queue for segmented delivery
	messageQueue MessageQueue
	// AI Agent service for dynamic conversations
	aiAgentService agent.Service
	// Clinic repository for fetching clinic settings
	clinicRepo ClinicRepository
	// Scheduled message repository for outside business hours
	scheduledMsgRepo ScheduledMessageRepository
	// Default delay between patients from config
	defaultDelayBetweenPatients int
	// Debouncer: delays AI processing until patient stops typing
	debouncer     *messageDebouncer
	debounceDelay time.Duration
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

// PatientInfo represents patient info for scheduling
type PatientInfo struct {
	ID    uuid.UUID
	Name  string
	Phone string
}

// PatientFinderCreator interface for finding or creating patients
type PatientFinderCreator interface {
	FindOrCreateByPhone(ctx context.Context, clinicID uuid.UUID, phone, name string) (*PatientInfo, error)
}

// ProfessionalInfo represents professional info for scheduling
type ProfessionalInfo struct {
	ID          uuid.UUID
	Name        string
	Specialty   string
	IsAvailable bool
}

// ProfessionalLister interface for listing professionals
type ProfessionalLister interface {
	ListAvailable(ctx context.Context, clinicID uuid.UUID, specialty string) ([]*ProfessionalInfo, error)
}

// AvailableSlot represents an available time slot for scheduling
type AvailableSlot struct {
	ProfessionalID   uuid.UUID
	ProfessionalName string
	StartTime        time.Time
	EndTime          time.Time
}

// SlotFinder interface for finding available slots
type SlotFinder interface {
	GetAvailableSlots(ctx context.Context, clinicID uuid.UUID, professionalID uuid.UUID, date time.Time, durationMinutes int) ([]*AvailableSlot, error)
	GetAvailableSlotsMultipleProfessionals(ctx context.Context, clinicID uuid.UUID, professionalIDs []uuid.UUID, date time.Time, days int, durationMinutes int, limit int) ([]*AvailableSlot, error)
}

// NewAppointmentCreator interface for creating new appointments (not reschedule)
type NewAppointmentCreator interface {
	CreateFromSchedulingIntent(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) (*AppointmentInfo, error)
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

// messageDebouncer debounces incoming messages per (clinicID, phone) pair.
// When a new message arrives, any pending timer for that key is reset.
// Processing is triggered only after the configured idle window elapses.
type messageDebouncer struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	msgs   map[string]*pendingMessage
}

type pendingMessage struct {
	clinicID uuid.UUID
	msg      *model.ParsedIncomingMessage
}

func newMessageDebouncer() *messageDebouncer {
	return &messageDebouncer{
		timers: make(map[string]*time.Timer),
		msgs:   make(map[string]*pendingMessage),
	}
}

// debounceKey builds the map key for a given (clinicID, phone) pair.
func debounceKey(clinicID uuid.UUID, phone string) string {
	return clinicID.String() + ":" + phone
}

// schedule (re)sets o timer de debounce para a chave (clinicID, phone).
// fn(lastMsg) é chamado uma vez após o idle window expirar, recebendo
// a última mensagem registrada para a chave.
func (d *messageDebouncer) schedule(clinicID uuid.UUID, msg *model.ParsedIncomingMessage, delay time.Duration, fn func(*model.ParsedIncomingMessage)) {
	key := debounceKey(clinicID, msg.Phone)

	d.mu.Lock()
	defer d.mu.Unlock()

	// Sempre sobrescreve com a mensagem mais recente.
	d.msgs[key] = &pendingMessage{clinicID: clinicID, msg: msg}

	if t, ok := d.timers[key]; ok {
		t.Reset(delay)
		return
	}

	d.timers[key] = time.AfterFunc(delay, func() {
		d.mu.Lock()
		pending := d.msgs[key] // lê antes de deletar
		delete(d.timers, key)
		delete(d.msgs, key)
		d.mu.Unlock()

		if pending != nil {
			fn(pending.msg)
		}
	})
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
		debouncer:          newMessageDebouncer(),
		debounceDelay:      25 * time.Second,
	}
}

// SetDebounceDelay sets the idle window before message processing.
func (s *whatsAppService) SetDebounceDelay(d time.Duration) {
	s.debounceDelay = d
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

// SetSchedulingIntentRepo sets the scheduling intent repository for new scheduling flow
func (s *whatsAppService) SetSchedulingIntentRepo(repo SchedulingIntentRepository) {
	s.schedulingIntentRepo = repo
}

// SetPatientFinderCreator sets the patient finder/creator for scheduling
func (s *whatsAppService) SetPatientFinderCreator(finder PatientFinderCreator) {
	s.patientFinderCreator = finder
}

// SetProfessionalLister sets the professional lister for scheduling
func (s *whatsAppService) SetProfessionalLister(lister ProfessionalLister) {
	s.professionalLister = lister
}

// SetSlotFinder sets the slot finder for scheduling
func (s *whatsAppService) SetSlotFinder(finder SlotFinder) {
	s.slotFinder = finder
}

// SetNewAppointmentCreator sets the new appointment creator for scheduling
func (s *whatsAppService) SetNewAppointmentCreator(creator NewAppointmentCreator) {
	s.newAppointmentCreator = creator
}

// SetMessageQueue sets the message queue for segmented message delivery
func (s *whatsAppService) SetMessageQueue(queue MessageQueue) {
	s.messageQueue = queue
}

// SetAIAgentService sets the AI agent service for dynamic conversations
func (s *whatsAppService) SetAIAgentService(agentService agent.Service) {
	s.aiAgentService = agentService
}

// SetClinicRepo sets the clinic repository for fetching clinic settings
func (s *whatsAppService) SetClinicRepo(repo ClinicRepository) {
	s.clinicRepo = repo
}

// SetScheduledMessageRepo sets the scheduled message repository
func (s *whatsAppService) SetScheduledMessageRepo(repo ScheduledMessageRepository) {
	s.scheduledMsgRepo = repo
}

// SetDefaultDelayBetweenPatients sets the default delay between patients from config
func (s *whatsAppService) SetDefaultDelayBetweenPatients(delay int) {
	s.defaultDelayBetweenPatients = delay
}

// CreateInstance creates a new WhatsApp instance for a clinic
func (s *whatsAppService) CreateInstance(ctx context.Context, instance string) (*model.QRCodeData, error) {
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
		logger.Printf("[WhatsApp] Failed to create reschedule appointment: %v", err)
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

	// Build greeting with time of day
	greeting := s.buildGreeting(msg.PatientName)

	// Build confirmation message with buttons
	message := fmt.Sprintf(
		"%s\n\n"+
			"Sua consulta foi agendada:\n\n"+
			"📅 Data: %s\n"+
			"🕐 Horario: %s\n"+
			"👨‍⚕️ Profissional: %s\n"+
			"🏥 Local: %s\n\n"+
			"Por favor, confirme sua presenca:",
		greeting,
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

	// Build greeting with time of day
	greeting := s.buildGreeting(msg.PatientName)

	message := fmt.Sprintf(
		"🔔 %s\n\n"+
			"%s\n\n"+
			"📅 Data: %s\n"+
			"🕐 Horario: %s\n"+
			"👨‍⚕️ Profissional: %s\n"+
			"🏥 Local: %s\n\n"+
			"Caso precise remarcar ou cancelar, responda esta mensagem.",
		reminderText,
		greeting,
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

	// Se o AI Agent está habilitado, debounce antes de processar.
	// Evita respostas instantâneas que podem acionar bloqueio pelo WhatsApp.
	if s.aiAgentService != nil {
		logger.Printf("[Debounce] Message from %s received — waiting %v idle before processing",
			msg.Phone, s.debounceDelay)

		bgCtx := context.Background()
		capClinicID := clinicID

		s.debouncer.schedule(clinicID, msg, s.debounceDelay, func(lastMsg *model.ParsedIncomingMessage) {
			logger.Printf("[Debounce] Idle window elapsed for %s — processing with AI Agent: %q",
				lastMsg.Phone, lastMsg.Content)
			if err := s.handleWithAIAgent(bgCtx, capClinicID, lastMsg); err != nil {
				logger.Printf("[Debounce] Error handling AI Agent message from %s: %v", lastMsg.Phone, err)
			}
		})
		return nil
	}

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

	case model.MessageIntentSchedule:
		// Handle new scheduling request
		return s.handleScheduleIntent(ctx, clinicID, msg)

	case model.MessageIntentQuestion, model.MessageIntentUnknown:
		// Check if this is a slot selection for a pending scheduling intent
		if s.schedulingIntentRepo != nil {
			pendingIntent, err := s.schedulingIntentRepo.GetPendingByPhone(ctx, clinicID, msg.Phone)
			if err == nil && pendingIntent != nil {
				return s.handleSlotSelection(ctx, clinicID, msg, pendingIntent)
			}
		}
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
			logger.Printf("[Classifier] AI (%s/%s): intent=%s confidence=%.2f latency=%v reason=%q",
				result.Provider, result.Model, result.Intent, result.Confidence, result.Latency, result.Reasoning)
			return result.Intent
		}
		if err != nil {
			logger.Printf("[Classifier] AI error, falling back to regex: %v", err)
		} else {
			logger.Printf("[Classifier] AI confidence %.2f below threshold %.2f, falling back to regex",
				result.Confidence, s.confidenceThreshold)
		}
	}

	// Fallback to regex classification
	intent := s.classifyWithRegex(content)
	logger.Printf("[Classifier] Regex fallback: intent=%s content=%q", intent, content)
	return intent
}

// classifyWithRegex classifies the intent using regex patterns (fallback)
func (s *whatsAppService) classifyWithRegex(content string) model.MessageIntent {
	content = strings.ToLower(strings.TrimSpace(content))

	// Schedule patterns (check first - patient wants to book NEW appointment)
	schedulePatterns := []string{
		"quero marcar", "quero agendar", "gostaria de marcar", "gostaria de agendar",
		"fazer uma consulta", "fazer consulta", "agendar consulta", "marcar consulta",
		"agendar avaliacao", "marcar avaliacao", "fazer avaliacao",
		"primeira consulta", "primeira vez", "novo paciente",
		"preciso de uma consulta", "preciso marcar", "preciso agendar",
	}

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

	for _, pattern := range schedulePatterns {
		if strings.Contains(content, pattern) {
			return model.MessageIntentSchedule
		}
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
		logger.Printf("Failed to create message: %v", err)
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
			logger.Printf("Failed to log message created: %v", err)
		}

		// Log sent or failed based on status
		if status == model.MessageStatusFailed {
			if err := s.messageLogService.LogFailed(ctx, msg, externalIDOrError, payload); err != nil {
				logger.Printf("Failed to log message failed: %v", err)
			}
		} else if status == model.MessageStatusSent {
			if err := s.messageLogService.LogSent(ctx, msg, payload); err != nil {
				logger.Printf("Failed to log message sent: %v", err)
			}
		}
	}
}

// handleWithAIAgent delegates message handling to the AI Agent
func (s *whatsAppService) handleWithAIAgent(ctx context.Context, clinicID uuid.UUID, msg *model.ParsedIncomingMessage) error {
	if s.aiAgentService == nil {
		logger.Printf("[AI Agent] Service is nil, falling back to traditional flow")
		return nil
	}

	// Check if AI agent is enabled for this clinic
	if s.clinicRepo != nil {
		clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
		if err != nil {
			logger.Printf("[AI Agent] Warning: failed to fetch clinic settings: %v", err)
		} else if !clinic.Settings.AIAgentEnabled {
			logger.Printf("[AI Agent] Clinic %s has AIAgentEnabled=false, AI agent will not respond", clinicID)
			return nil
		}
	}

	// Removed business hours check - messages are processed and scheduled if outside hours

	// Classify message intent
	intent := s.ClassifyMessage(msg.Content)

	// Create message request
	req := &agent.MessageRequest{
		ClinicID:     clinicID,
		ClinicName:   "", // TODO: Fetch clinic name if needed
		PatientPhone: msg.Phone,
		PatientName:  msg.SenderName,
		MessageText:  msg.Content,
		Intent:       intent,
	}

	logger.Printf("[AI Agent] Processing message from %s (intent: %s): %s", msg.Phone, intent, msg.Content)

	// Call AI Agent
	resp, err := s.aiAgentService.ProcessMessage(ctx, req)
	if err != nil {
		logger.Printf("[AI Agent] Error processing message: %v, falling back to traditional flow", err)
		return nil // Fall back gracefully
	}

	if resp == nil || len(resp.Segments) == 0 {
		logger.Printf("[AI Agent] No response segments from agent")
		return nil
	}

	// Use message queue for segmented delivery with delays (if available)
	if s.messageQueue != nil {
		return s.sendAIResponseSegmented(ctx, clinicID, msg.Instance, msg.Phone, resp)
	}

	// Fallback: send immediately without queue
	for i, segment := range resp.Segments {
		if i > 0 && segment.DelayAfter > 0 {
			time.Sleep(segment.DelayAfter)
		}

		_, err := s.SendTextMessage(ctx, msg.Instance, msg.Phone, segment.Content)
		if err != nil {
			logger.Printf("[AI Agent] Failed to send segment %d: %v", i, err)
			return err
		}
	}

	logger.Printf("[AI Agent] Successfully sent %d segments (tokens: %d, latency: %v)",
		len(resp.Segments), resp.TokensUsed, resp.Latency)
	return nil
}

// sendAIResponseSegmented sends AI response segments through the message queue with delays or schedules for later
func (s *whatsAppService) sendAIResponseSegmented(ctx context.Context, clinicID uuid.UUID, instance, phone string, resp *agent.MessageResponse) error {
	// Get clinic settings
	var clinic *clinic_model.Clinic
	if s.clinicRepo != nil {
		var err error
		clinic, err = s.clinicRepo.GetByID(ctx, clinicID)
		if err != nil {
			logger.Printf("[WhatsApp] Warning: failed to fetch clinic settings: %v", err)
		}
	}

	// Check if we're outside business hours
	outsideHours := clinic != nil && !clinic.IsWithinBusinessHours(time.Now())

	if outsideHours && s.scheduledMsgRepo != nil {
		// Schedule messages for next business hour
		return s.scheduleMessagesForNextBusinessHour(ctx, clinicID, clinic, instance, phone, resp)
	}

	// Normal flow: send through queue
	var cumulativeDelay time.Duration

	for i, segment := range resp.Segments {
		job := &queue.MessageJob{
			ID:          uuid.New().String(),
			ClinicID:    clinicID.String(),
			Instance:    instance,
			Phone:       phone,
			Message:     segment.Content,
			MessageType: fmt.Sprintf("ai_agent_%s", segment.Type),
			MaxAttempts: 3,
			CreatedAt:   time.Now(),
			ScheduledAt: time.Now().Add(cumulativeDelay),
		}

		logger.Printf("[AI Agent] Segment %d/%d: type=%s, delay=%v, content_preview=\"%s...\"",
			i+1, len(resp.Segments), segment.Type, cumulativeDelay,
			truncate(segment.Content, 50))

		if err := s.messageQueue.PublishWithDelay(ctx, job, cumulativeDelay); err != nil {
			logger.Printf("[AI Agent] Failed to enqueue segment %d: %v", i, err)
			// Continue with next segment - don't fail entirely
		}

		// Accumulate delay for next segment
		cumulativeDelay += segment.DelayAfter
	}

	logger.Printf("[AI Agent] Enqueued %d message segments for %s (tokens: %d, latency: %v)",
		len(resp.Segments), phone, resp.TokensUsed, resp.Latency)
	return nil
}

// scheduleMessagesForNextBusinessHour schedules AI response messages for the next business hour
func (s *whatsAppService) scheduleMessagesForNextBusinessHour(
	ctx context.Context,
	clinicID uuid.UUID,
	clinic *clinic_model.Clinic,
	instance, phone string,
	resp *agent.MessageResponse,
) error {
	// Get last scheduled message for this clinic
	lastScheduled, err := s.scheduledMsgRepo.GetLastScheduledForClinic(ctx, clinicID)
	if err != nil {
		logger.Printf("[WhatsApp] Warning: failed to get last scheduled message: %v", err)
	}

	// Calculate next business hour
	nextBusinessHour := clinic.CalculateNextBusinessHour(time.Now())

	// If there was a previous scheduled message, calculate next slot with delay
	var nextSlot time.Time
	if lastScheduled != nil && lastScheduled.ScheduledAt.After(nextBusinessHour) {
		// Use clinic-specific delay or default
		delay := clinic.GetMinDelayBetweenPatients(s.defaultDelayBetweenPatients)
		nextSlot = lastScheduled.ScheduledAt.Add(time.Duration(delay) * time.Second)
	} else {
		nextSlot = nextBusinessHour
	}

	logger.Printf("[WhatsApp] Message outside business hours - scheduling %d segments starting at %s",
		len(resp.Segments), nextSlot.Format(time.RFC3339))

	// Schedule all segments
	for _, segment := range resp.Segments {
		scheduledMsg := &model.ScheduledMessage{
			ID:          uuid.New(),
			ClinicID:    clinicID,
			Instance:    instance,
			Phone:       phone,
			Message:     segment.Content,
			ScheduledAt: nextSlot,
			Sent:        false,
			CreatedAt:   time.Now(),
		}

		if err := s.scheduledMsgRepo.Create(ctx, scheduledMsg); err != nil {
			logger.Printf("[WhatsApp] Failed to create scheduled message: %v", err)
			// Continue with next segment
		} else {
			logger.Printf("[WhatsApp] Scheduled message %s for %s at %s",
				scheduledMsg.ID, phone, nextSlot.Format(time.RFC3339))
		}

		// Next segment gets scheduled after message-level delay
		if segment.DelayAfter > 0 {
			delay := clinic.GetMinDelayBetweenMessages(3) // default 3 seconds
			nextSlot = nextSlot.Add(time.Duration(delay) * time.Second)
		}
	}

	return nil
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
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

// handleScheduleIntent handles a new scheduling request from a patient
func (s *whatsAppService) handleScheduleIntent(ctx context.Context, clinicID uuid.UUID, msg *model.ParsedIncomingMessage) error {
	logger.Printf("[Schedule] Handling schedule intent for phone=%s clinic=%s", msg.Phone, clinicID)

	// Check if we have all required dependencies
	if s.schedulingIntentRepo == nil || s.patientFinderCreator == nil || s.slotFinder == nil {
		logger.Printf("[Schedule] Missing dependencies for scheduling, skipping")
		return nil
	}

	// Check if there's already a pending intent for this phone
	existingIntent, _ := s.schedulingIntentRepo.GetPendingByPhone(ctx, clinicID, msg.Phone)
	if existingIntent != nil {
		logger.Printf("[Schedule] Found existing pending intent, resending options")
		return s.SendSchedulingOptions(ctx, msg.Instance, msg.Phone, existingIntent)
	}

	// Find or create patient
	patientName := msg.SenderName
	if patientName == "" {
		patientName = "Paciente"
	}
	patient, err := s.patientFinderCreator.FindOrCreateByPhone(ctx, clinicID, msg.Phone, patientName)
	if err != nil {
		logger.Printf("[Schedule] Failed to find/create patient: %v", err)
		return err
	}

	// Get available professionals
	var professionalIDs []uuid.UUID
	if s.professionalLister != nil {
		professionals, err := s.professionalLister.ListAvailable(ctx, clinicID, "")
		if err == nil && len(professionals) > 0 {
			for _, p := range professionals {
				professionalIDs = append(professionalIDs, p.ID)
			}
		}
	}

	if len(professionalIDs) == 0 {
		logger.Printf("[Schedule] No professionals available")
		_, _ = s.SendTextMessage(ctx, msg.Instance, msg.Phone,
			"Desculpe, no momento nao temos horarios disponiveis. Por favor, entre em contato com a clinica.")
		return nil
	}

	// Get available slots (next 14 days, max 5 slots)
	slots, err := s.slotFinder.GetAvailableSlotsMultipleProfessionals(
		ctx, clinicID, professionalIDs,
		time.Now(), 14, 50, 5, // 50 minutes default, 5 slots max
	)
	if err != nil || len(slots) == 0 {
		logger.Printf("[Schedule] No slots available: %v", err)
		_, _ = s.SendTextMessage(ctx, msg.Instance, msg.Phone,
			"Desculpe, no momento nao temos horarios disponiveis. Por favor, entre em contato com a clinica.")
		return nil
	}

	// Convert to SlotOptions
	var slotOptions model.SlotOptions
	for i, slot := range slots {
		slotOptions = append(slotOptions, model.SlotOption{
			ProfessionalID:   slot.ProfessionalID,
			ProfessionalName: slot.ProfessionalName,
			StartTime:        slot.StartTime,
			EndTime:          slot.EndTime,
			ButtonID:         fmt.Sprintf("slot_%d", i),
		})
	}

	// Create scheduling intent
	intent := &model.SchedulingIntent{
		ClinicID:       clinicID,
		PatientID:      &patient.ID,
		PatientPhone:   msg.Phone,
		PatientName:    patientName,
		IntentType:     "consulta", // Default, could be extracted from AI classification
		SuggestedSlots: slotOptions,
		Status:         model.SchedulingIntentStatusPending,
		Confidence:     0.9, // Could use actual confidence from classifier
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}

	if err := s.schedulingIntentRepo.Create(ctx, intent); err != nil {
		logger.Printf("[Schedule] Failed to create intent: %v", err)
		return err
	}

	// Send scheduling options
	return s.SendSchedulingOptions(ctx, msg.Instance, msg.Phone, intent)
}

// SendSchedulingOptions sends available time slots as WhatsApp messages
func (s *whatsAppService) SendSchedulingOptions(ctx context.Context, instance, phone string, intent *model.SchedulingIntent) error {
	if len(intent.SuggestedSlots) == 0 {
		return nil
	}

	patientName := intent.PatientName
	if patientName == "" {
		patientName = "Paciente"
	}

	// Use segmented messages if queue is available
	if s.messageQueue != nil {
		return s.sendSchedulingOptionsSegmented(ctx, instance, phone, intent, patientName)
	}

	// Fallback: send as single message
	return s.sendSchedulingOptionsSimple(ctx, instance, phone, intent)
}

// sendSchedulingOptionsSegmented sends scheduling options as segmented messages
func (s *whatsAppService) sendSchedulingOptionsSegmented(ctx context.Context, instance, phone string, intent *model.SchedulingIntent, patientName string) error {
	segments := s.buildSchedulingMessageSegments(intent, patientName)

	var cumulativeDelay time.Duration
	for i, segment := range segments {
		job := &queue.MessageJob{
			ID:          uuid.New().String(),
			Instance:    instance,
			Phone:       phone,
			Message:     segment.Content,
			MessageType: fmt.Sprintf("scheduling_%s", segment.Type),
			MaxAttempts: 3,
			CreatedAt:   time.Now(),
			ScheduledAt: time.Now().Add(cumulativeDelay),
		}

		if err := s.messageQueue.PublishWithDelay(ctx, job, cumulativeDelay); err != nil {
			logger.Printf("[Schedule] Failed to enqueue segment %d: %v", i, err)
			// Continue with next segment - don't fail entirely
		}

		// Accumulate delay for next segment
		cumulativeDelay += segment.DelayAfter
	}

	logger.Printf("[Schedule] Enqueued %d message segments for %s", len(segments), phone)
	return nil
}

// sendSchedulingOptionsSimple sends scheduling options as a single message (fallback)
func (s *whatsAppService) sendSchedulingOptionsSimple(ctx context.Context, instance, phone string, intent *model.SchedulingIntent) error {
	// Build message
	var sb strings.Builder
	sb.WriteString("Perfeito! Encontrei alguns horarios disponiveis:\n\n")

	for i, slot := range intent.SuggestedSlots {
		weekday := translateWeekday(slot.StartTime.Weekday())
		dateStr := slot.StartTime.Format("02/01")
		timeStr := slot.StartTime.Format("15:04")
		sb.WriteString(fmt.Sprintf("%d. %s %s as %s - %s\n",
			i+1, weekday, dateStr, timeStr, slot.ProfessionalName))
	}

	sb.WriteString("\nQual horario voce prefere?")

	// Send as text message (buttons have a limit of 3, we might have more slots)
	formattedPhone := formatPhoneNumber(phone)
	_, err := s.client.SendTextMessage(ctx, instance, formattedPhone, sb.String())
	if err != nil {
		logger.Printf("[Schedule] Failed to send scheduling options: %v", err)
		return err
	}

	logger.Printf("[Schedule] Sent %d slot options to %s", len(intent.SuggestedSlots), phone)
	return nil
}

// handleSlotSelection handles when a patient selects a time slot
func (s *whatsAppService) handleSlotSelection(ctx context.Context, clinicID uuid.UUID, msg *model.ParsedIncomingMessage, intent *model.SchedulingIntent) error {
	content := strings.TrimSpace(msg.Content)
	slotIndex := -1

	// Step 1: Try AI matching first (if classifier is available)
	if s.classifier != nil {
		result, err := s.classifier.MatchSlot(ctx, content, intent.SuggestedSlots)
		if err == nil && result.Confidence >= s.confidenceThreshold {
			slotIndex = result.SlotIndex
			logger.Printf("[Schedule] AI matched slot %d with confidence %.2f: %s",
				slotIndex, result.Confidence, result.Reasoning)
		} else if err != nil {
			logger.Printf("[Schedule] AI matching failed: %v", err)
		} else {
			logger.Printf("[Schedule] AI confidence too low: %.2f < %.2f",
				result.Confidence, s.confidenceThreshold)
		}
	}

	// Step 2: Fallback to numeric parsing (still support numbers)
	if slotIndex == -1 {
		if num, err := strconv.Atoi(content); err == nil {
			if num >= 1 && num <= len(intent.SuggestedSlots) {
				slotIndex = num - 1
				logger.Printf("[Schedule] Numeric selection: %d", num)
			}
		}
	}

	// Step 3: If still no match, send helpful message
	if slotIndex == -1 {
		logger.Printf("[Schedule] Could not parse slot selection from: %q", content)
		_, _ = s.SendTextMessage(ctx, msg.Instance, msg.Phone,
			"Desculpe, nao consegui entender sua escolha. Voce pode me dizer qual dia/horario prefere, ou responder com o numero da opcao?")
		return nil
	}

	selectedSlot := intent.SuggestedSlots[slotIndex]
	logger.Printf("[Schedule] Patient selected slot %d: %s at %s",
		slotIndex+1, selectedSlot.ProfessionalName, selectedSlot.StartTime)

	// Check if we have all required dependencies
	if s.newAppointmentCreator == nil {
		logger.Printf("[Schedule] Missing appointment creator, cannot create appointment")
		return nil
	}

	// Create appointment
	var patientID uuid.UUID
	if intent.PatientID != nil {
		patientID = *intent.PatientID
	} else {
		logger.Printf("[Schedule] No patient ID in intent")
		return nil
	}

	appointment, err := s.newAppointmentCreator.CreateFromSchedulingIntent(
		ctx, clinicID, patientID, selectedSlot.ProfessionalID,
		selectedSlot.StartTime, 50, // 50 minutes default
	)
	if err != nil {
		logger.Printf("[Schedule] Failed to create appointment: %v", err)
		_, _ = s.SendTextMessage(ctx, msg.Instance, msg.Phone,
			"Desculpe, ocorreu um erro ao agendar. Por favor, tente novamente.")
		return err
	}

	// Update intent status
	intent.Status = model.SchedulingIntentStatusScheduled
	now := time.Now()
	intent.SelectedSlot = &model.SelectedSlot{
		SlotOption: selectedSlot,
		SelectedAt: now,
	}
	intent.AppointmentID = &appointment.ID
	if err := s.schedulingIntentRepo.Update(ctx, intent); err != nil {
		logger.Printf("[Schedule] Failed to update intent status: %v", err)
	}

	// Send confirmation
	weekday := translateWeekday(selectedSlot.StartTime.Weekday())
	dateStr := selectedSlot.StartTime.Format("02/01/2006")
	timeStr := selectedSlot.StartTime.Format("15:04")

	message := fmt.Sprintf(
		"Consulta agendada com sucesso!\n\n"+
			"%s Data: %s %s\n"+
			"%s Horario: %s\n"+
			"%s Profissional: %s\n\n"+
			"Te aguardamos!",
		"📅", weekday, dateStr,
		"🕐", timeStr,
		"👨‍⚕️", selectedSlot.ProfessionalName,
	)

	formattedPhone := formatPhoneNumber(msg.Phone)
	_, err = s.client.SendTextMessage(ctx, msg.Instance, formattedPhone, message)
	if err != nil {
		logger.Printf("[Schedule] Failed to send confirmation: %v", err)
	}

	logger.Printf("[Schedule] Successfully scheduled appointment %s", appointment.ID)
	return nil
}

// translateWeekday translates Go weekday to Portuguese
func translateWeekday(weekday time.Weekday) string {
	days := map[time.Weekday]string{
		time.Sunday:    "Domingo",
		time.Monday:    "Segunda",
		time.Tuesday:   "Terca",
		time.Wednesday: "Quarta",
		time.Thursday:  "Quinta",
		time.Friday:    "Sexta",
		time.Saturday:  "Sabado",
	}
	return days[weekday]
}

// buildGreeting creates a personalized greeting based on time of day
func (s *whatsAppService) buildGreeting(patientName string) string {
	timeOfDay := getTimeOfDay()

	greetings := map[string]string{
		"morning":   "Ola %s, bom dia! 😊",
		"afternoon": "Ola %s, boa tarde! 😊",
		"evening":   "Ola %s, boa noite! 😊",
	}

	greeting := greetings[timeOfDay]
	return fmt.Sprintf(greeting, patientName)
}

// getTimeOfDay returns the current time of day period
func getTimeOfDay() string {
	hour := time.Now().Hour()

	if hour >= 6 && hour < 12 {
		return "morning"
	} else if hour >= 12 && hour < 18 {
		return "afternoon"
	}
	return "evening"
}

// buildSlotsText builds the formatted text for time slots
func (s *whatsAppService) buildSlotsText(slots model.SlotOptions) string {
	var sb strings.Builder
	sb.WriteString("Encontrei alguns horarios disponiveis:\n\n")

	for i, slot := range slots {
		weekday := translateWeekday(slot.StartTime.Weekday())
		dateStr := slot.StartTime.Format("02/01")
		timeStr := slot.StartTime.Format("15:04")
		sb.WriteString(fmt.Sprintf("%d. %s %s as %s - %s\n",
			i+1, weekday, dateStr, timeStr, slot.ProfessionalName))
	}

	return sb.String()
}

// calculateDelayByLength calculates delay based on message length (3-10 seconds)
func (s *whatsAppService) calculateDelayByLength(messageLength int) time.Duration {
	// Formula: 3s base + 1s per 100 characters
	// Min: 3s, Max: 10s
	const baseDelay = 3 * time.Second
	const maxDelay = 10 * time.Second
	const charsPerSecond = 100

	additionalDelay := time.Duration(messageLength/charsPerSecond) * time.Second
	totalDelay := baseDelay + additionalDelay

	if totalDelay > maxDelay {
		return maxDelay
	}
	return totalDelay
}

// calculateSegmentDelay calculates the delay for a message segment
func (s *whatsAppService) calculateSegmentDelay(segment model.MessageSegment) time.Duration {
	if segment.DelayAfter > 0 {
		return segment.DelayAfter
	}
	return s.calculateDelayByLength(len(segment.Content))
}

// buildSchedulingMessageSegments builds message segments for scheduling options
func (s *whatsAppService) buildSchedulingMessageSegments(
	intent *model.SchedulingIntent,
	patientName string,
) []model.MessageSegment {
	segments := []model.MessageSegment{}

	// 1. Greeting
	greeting := s.buildGreeting(patientName)
	segments = append(segments, model.MessageSegment{
		Content:    greeting,
		Type:       model.SegmentTypeGreeting,
		DelayAfter: 3 * time.Second, // Minimum 3s
	})

	// 2. Slots list
	slotsText := s.buildSlotsText(intent.SuggestedSlots)
	segments = append(segments, model.MessageSegment{
		Content:    slotsText,
		Type:       model.SegmentTypeContent,
		DelayAfter: s.calculateDelayByLength(len(slotsText)),
	})

	// 3. Question
	segments = append(segments, model.MessageSegment{
		Content:    "Qual horario voce prefere?",
		Type:       model.SegmentTypeQuestion,
		DelayAfter: 0, // Last message
	})

	return segments
}
