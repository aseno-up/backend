package service

import (
	"context"
	"fmt"
	"github.com/aseno/backend/internal/logger"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/internal/queue"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/waitlist/model"
	"github.com/aseno/backend/pkg/waitlist/repository"
)

// ClinicInfoProvider provides clinic information for waitlist operations
type ClinicInfoProvider interface {
	GetClinicInfo(ctx context.Context, clinicID uuid.UUID) (*ClinicInfo, error)
}

// ClinicInfo contains clinic information needed for waitlist operations
type ClinicInfo struct {
	ID                uuid.UUID
	Name              string
	WhatsAppInstance  string
	WhatsAppConnected bool
}

// ProfessionalInfoProvider provides professional information
type ProfessionalInfoProvider interface {
	GetProfessionalInfo(ctx context.Context, professionalID, clinicID uuid.UUID) (*ProfessionalInfo, error)
}

// ProfessionalInfo contains professional information
type ProfessionalInfo struct {
	ID   uuid.UUID
	Name string
}

// AppointmentCreator creates appointments for waitlist patients
type AppointmentCreator interface {
	CreateFromWaitlist(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) (uuid.UUID, error)
	Reschedule(ctx context.Context, appointmentID, clinicID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) error
}

// WaitlistService defines the interface for waitlist business logic
type WaitlistService interface {
	// CRUD operations
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateWaitlistRequest) (*model.WaitlistEntryResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.WaitlistEntryResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, status string, page, perPage int) (*model.WaitlistListResponse, error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateWaitlistRequest) (*model.WaitlistEntryResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error

	// Encaixe operations
	ProcessCancelledAppointment(ctx context.Context, clinicID, professionalID uuid.UUID, slotTime time.Time, durationMinutes int) error
	HandlePatientResponse(ctx context.Context, clinicID uuid.UUID, phone string, accepted bool) error
}

// waitlistService implements WaitlistService
type waitlistService struct {
	repo                 repository.WaitlistRepository
	clinicProvider       ClinicInfoProvider
	professionalProvider ProfessionalInfoProvider
	appointmentCreator   AppointmentCreator
	messageQueue         queue.Queue
	defaultDuration      int // Default appointment duration in minutes
	clinicRepo           clinic_repository.ClinicRepository
}

// NewWaitlistService creates a new waitlist service
func NewWaitlistService(
	repo repository.WaitlistRepository,
	clinicProvider ClinicInfoProvider,
	professionalProvider ProfessionalInfoProvider,
	appointmentCreator AppointmentCreator,
	messageQueue queue.Queue,
	clinicRepo clinic_repository.ClinicRepository,
) WaitlistService {
	return &waitlistService{
		repo:                 repo,
		clinicProvider:       clinicProvider,
		professionalProvider: professionalProvider,
		appointmentCreator:   appointmentCreator,
		messageQueue:         messageQueue,
		defaultDuration:      50, // Default 50 minutes
		clinicRepo:           clinicRepo,
	}
}

// checkPlanRestriction checks if the clinic plan allows access to advanced features
func (s *waitlistService) checkPlanRestriction(ctx context.Context, clinicID uuid.UUID) error {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return err
	}

	if clinic.Plan == clinic_model.PlanTypeBasic {
		return errors.ErrUpgradeRequiredInstance.WithMessage("Lista de encaixe disponível apenas nos planos Profissional e Avançado")
	}

	return nil
}

// Create adds a patient to the waitlist
func (s *waitlistService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateWaitlistRequest) (*model.WaitlistEntryResponse, error) {
	if err := s.checkPlanRestriction(ctx, clinicID); err != nil {
		return nil, err
	}

	// Set default time range if not provided
	timeStart := req.PreferredTimeStart
	if timeStart == "" {
		timeStart = "08:00"
	}
	timeEnd := req.PreferredTimeEnd
	if timeEnd == "" {
		timeEnd = "18:00"
	}

	entry := &model.WaitlistEntry{
		ClinicID:           clinicID,
		PatientID:          req.PatientID,
		ProfessionalID:     req.ProfessionalID,
		PreferredDays:      req.PreferredDays,
		PreferredTimeStart: timeStart,
		PreferredTimeEnd:   timeEnd,
		Status:             model.WaitlistStatusWaiting,
		Notes:              req.Notes,
		ExpiresAt:          req.ExpiresAt,
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, err
	}

	return entry.ToResponse(), nil
}

// GetByID retrieves a waitlist entry by ID
func (s *waitlistService) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.WaitlistEntryResponse, error) {
	entry, err := s.repo.GetWithPatientInfo(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	resp := entry.ToResponse()
	resp.PatientName = entry.PatientName
	resp.PatientPhone = entry.PatientPhone

	// Get professional name if set
	if entry.ProfessionalID != nil && s.professionalProvider != nil {
		prof, err := s.professionalProvider.GetProfessionalInfo(ctx, *entry.ProfessionalID, clinicID)
		if err == nil && prof != nil {
			resp.ProfessionalName = prof.Name
		}
	}

	return resp, nil
}

// List retrieves a paginated list of waitlist entries
func (s *waitlistService) List(ctx context.Context, clinicID uuid.UUID, status string, page, perPage int) (*model.WaitlistListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	entries, total, err := s.repo.GetByClinicID(ctx, clinicID, status, perPage, offset)
	if err != nil {
		return nil, err
	}

	data := make([]*model.WaitlistEntryResponse, len(entries))
	for i, e := range entries {
		data[i] = e.ToResponse()
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	return &model.WaitlistListResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

// Update updates a waitlist entry
func (s *waitlistService) Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateWaitlistRequest) (*model.WaitlistEntryResponse, error) {
	entry, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	// Only allow updates if status is waiting
	if entry.Status != model.WaitlistStatusWaiting {
		return nil, fmt.Errorf("cannot update entry with status %s", entry.Status)
	}

	// Update fields if provided
	if req.ProfessionalID != nil {
		entry.ProfessionalID = req.ProfessionalID
	}
	if len(req.PreferredDays) > 0 {
		entry.PreferredDays = req.PreferredDays
	}
	if req.PreferredTimeStart != "" {
		entry.PreferredTimeStart = req.PreferredTimeStart
	}
	if req.PreferredTimeEnd != "" {
		entry.PreferredTimeEnd = req.PreferredTimeEnd
	}
	if req.Notes != "" {
		entry.Notes = req.Notes
	}
	if req.ExpiresAt != nil {
		entry.ExpiresAt = req.ExpiresAt
	}

	if err := s.repo.Update(ctx, entry); err != nil {
		return nil, err
	}

	return entry.ToResponse(), nil
}

// Delete removes a patient from the waitlist
func (s *waitlistService) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.Delete(ctx, id, clinicID)
}

// ProcessCancelledAppointment handles the encaixe logic when an appointment is cancelled
func (s *waitlistService) ProcessCancelledAppointment(ctx context.Context, clinicID, professionalID uuid.UUID, slotTime time.Time, durationMinutes int) error {
	// Check plan restriction silently (log instead of error return, since this is async)
	if err := s.checkPlanRestriction(ctx, clinicID); err != nil {
		logger.Printf("[Waitlist] Skipping encaixe for clinic %s: %v", clinicID, err)
		return nil
	}

	// Get clinic info for WhatsApp
	clinic, err := s.clinicProvider.GetClinicInfo(ctx, clinicID)
	if err != nil {
		logger.Printf("[Waitlist] Error getting clinic info: %v", err)
		return err
	}

	if !clinic.WhatsAppConnected || clinic.WhatsAppInstance == "" {
		logger.Printf("[Waitlist] Clinic %s does not have WhatsApp connected, skipping encaixe", clinicID)
		return nil
	}

	// Get professional info
	var professionalName string
	if s.professionalProvider != nil {
		prof, err := s.professionalProvider.GetProfessionalInfo(ctx, professionalID, clinicID)
		if err == nil && prof != nil {
			professionalName = prof.Name
		}
	}

	// Get day of week from slot time
	dayOfWeek := strings.ToLower(slotTime.Weekday().String())

	// Find the next matching patient in the waitlist
	entry, err := s.repo.FindNextMatchingPatient(ctx, clinicID, &professionalID, dayOfWeek, slotTime)
	if err != nil {
		logger.Printf("[Waitlist] Error finding matching patient: %v", err)
		return err
	}

	if entry == nil {
		logger.Printf("[Waitlist] No matching patient found for slot %s", slotTime)
		return nil
	}

	// Update entry status to notified
	now := time.Now()
	err = s.repo.UpdateStatus(ctx, entry.ID, model.WaitlistStatusNotified, &now, &slotTime, &professionalID)
	if err != nil {
		logger.Printf("[Waitlist] Error updating entry status: %v", err)
		return err
	}

	// Build and send WhatsApp message
	message := s.buildSlotOfferMessage(entry.PatientName, professionalName, clinic.Name, slotTime, entry.OriginalAppointmentID != nil)

	if s.messageQueue != nil && s.messageQueue.IsConnected() {
		job := &queue.MessageJob{
			ID:            uuid.New().String(),
			ClinicID:      clinicID.String(),
			AppointmentID: "", // No appointment yet
			Instance:      clinic.WhatsAppInstance,
			Phone:         entry.PatientWhatsApp,
			PatientName:   entry.PatientName,
			Message:       message,
			MessageType:   "slot_offer",
			Attempts:      0,
			MaxAttempts:   3,
			CreatedAt:     time.Now(),
			ScheduledAt:   time.Now(),
		}

		if err := s.messageQueue.Publish(ctx, job); err != nil {
			logger.Printf("[Waitlist] Error publishing message to queue: %v", err)
			return err
		}

		logger.Printf("[Waitlist] Sent slot offer to %s for slot %s", entry.PatientName, slotTime)
	}

	return nil
}

// HandlePatientResponse processes the patient's response to a slot offer
func (s *waitlistService) HandlePatientResponse(ctx context.Context, clinicID uuid.UUID, phone string, accepted bool) error {
	// Find the pending waitlist entry for this patient
	entry, err := s.repo.GetPendingByPatientPhone(ctx, clinicID, phone)
	if err != nil {
		return err
	}

	if entry == nil {
		logger.Printf("[Waitlist] No pending entry found for phone %s", phone)
		return nil
	}

	if accepted {
		// Patient accepted - create the appointment or reschedule existing one
		if s.appointmentCreator != nil && entry.PendingSlotTime != nil && entry.PendingSlotProfessional != nil {
			var err error
			var action string

			if entry.OriginalAppointmentID != nil {
				// Anticipation: Reschedule existing appointment
				err = s.appointmentCreator.Reschedule(
					ctx,
					*entry.OriginalAppointmentID,
					clinicID,
					*entry.PendingSlotProfessional,
					*entry.PendingSlotTime,
					s.defaultDuration,
				)
				action = "rescheduled"
			} else {
				// Regular waitlist: Create new appointment
				var newID uuid.UUID
				newID, err = s.appointmentCreator.CreateFromWaitlist(
					ctx,
					clinicID,
					entry.PatientID,
					*entry.PendingSlotProfessional,
					*entry.PendingSlotTime,
					s.defaultDuration,
				)
				if err == nil {
					logger.Printf("[Waitlist] Created appointment %s from waitlist entry %s", newID, entry.ID)
				}
				action = "created"
			}

			if err != nil {
				logger.Printf("[Waitlist] Error processing acceptance (%s): %v", action, err)
				// Still mark as scheduled? No, maybe kept as notified or failed?
				// For now, let's keep it simple and mark as scheduled but log error,
				// or maybe we should send a message saying "sorry, slot taken"?
				// Ideally we should handle this failure gracefully.
			} else {
				logger.Printf("[Waitlist] Successfully %s appointment for entry %s", action, entry.ID)

				// Mark entry as scheduled only if successful
				err = s.repo.UpdateStatus(ctx, entry.ID, model.WaitlistStatusScheduled, nil, nil, nil)
				if err != nil {
					logger.Printf("[Waitlist] Error updating status to scheduled: %v", err)
				}

				// Send confirmation message
				if err := s.sendConfirmationMessage(ctx, clinicID, entry); err != nil {
					logger.Printf("[Waitlist] Error sending confirmation message: %v", err)
				}
			}
		}
	} else {
		// Patient declined - mark as declined and offer to next patient
		err = s.repo.UpdateStatus(ctx, entry.ID, model.WaitlistStatusDeclined, nil, nil, nil)
		if err != nil {
			return err
		}

		logger.Printf("[Waitlist] Patient %s declined slot, looking for next patient", entry.PatientID)

		// Try to offer to the next patient
		if entry.PendingSlotTime != nil && entry.PendingSlotProfessional != nil {
			go func() {
				// Use background context since this is async
				bgCtx := context.Background()
				if err := s.ProcessCancelledAppointment(bgCtx, clinicID, *entry.PendingSlotProfessional, *entry.PendingSlotTime, s.defaultDuration); err != nil {
					logger.Printf("[Waitlist] Error processing next patient: %v", err)
				}
			}()
		}
	}

	return nil
}

// buildSlotOfferMessage creates the WhatsApp message for a slot offer
func (s *waitlistService) buildSlotOfferMessage(patientName, professionalName, clinicName string, slotTime time.Time, isAnticipation bool) string {
	// Get weekday name in Portuguese
	weekdays := map[string]string{
		"Sunday":    "Domingo",
		"Monday":    "Segunda-feira",
		"Tuesday":   "Terca-feira",
		"Wednesday": "Quarta-feira",
		"Thursday":  "Quinta-feira",
		"Friday":    "Sexta-feira",
		"Saturday":  "Sabado",
	}
	weekday := weekdays[slotTime.Weekday().String()]

	dateStr := slotTime.Format("02/01/2006")
	timeStr := slotTime.Format("15:04")

	if isAnticipation {
		return fmt.Sprintf(
			"Ola %s!\n\n"+
				"Surgiu uma vaga para *ANTECIPAR* sua consulta!\n\n"+
				"📅 %s, %s as %s\n"+
				"👨‍⚕️ %s\n"+
				"🏥 %s\n\n"+
				"Deseja trocar seu horario atual para este novo horario?\n"+
				"Responda SIM para confirmar ou NAO para manter o agendamento atual.",
			patientName,
			weekday,
			dateStr,
			timeStr,
			professionalName,
			clinicName,
		)
	}

	return fmt.Sprintf(
		"Ola %s!\n\n"+
			"Surgiu uma vaga na agenda!\n\n"+
			"📅 %s, %s as %s\n"+
			"👨‍⚕️ %s\n"+
			"🏥 %s\n\n"+
			"Deseja agendar?\n"+
			"Responda SIM para confirmar ou NAO para recusar.",
		patientName,
		weekday,
		dateStr,
		timeStr,
		professionalName,
		clinicName,
	)
}

// sendConfirmationMessage sends a confirmation message after patient accepts
func (s *waitlistService) sendConfirmationMessage(ctx context.Context, clinicID uuid.UUID, entry *model.WaitlistEntry) error {
	if s.messageQueue == nil || !s.messageQueue.IsConnected() {
		return nil
	}

	clinic, err := s.clinicProvider.GetClinicInfo(ctx, clinicID)
	if err != nil {
		return err
	}

	// Get patient info
	entryWithPatient, err := s.repo.GetWithPatientInfo(ctx, entry.ID, clinicID)
	if err != nil {
		return err
	}

	var professionalName string
	if entry.PendingSlotProfessional != nil && s.professionalProvider != nil {
		prof, err := s.professionalProvider.GetProfessionalInfo(ctx, *entry.PendingSlotProfessional, clinicID)
		if err == nil && prof != nil {
			professionalName = prof.Name
		}
	}

	weekdays := map[string]string{
		"Sunday":    "Domingo",
		"Monday":    "Segunda-feira",
		"Tuesday":   "Terca-feira",
		"Wednesday": "Quarta-feira",
		"Thursday":  "Quinta-feira",
		"Friday":    "Sexta-feira",
		"Saturday":  "Sabado",
	}

	var weekday, dateStr, timeStr string
	if entry.PendingSlotTime != nil {
		weekday = weekdays[entry.PendingSlotTime.Weekday().String()]
		dateStr = entry.PendingSlotTime.Format("02/01/2006")
		timeStr = entry.PendingSlotTime.Format("15:04")
	}

	message := fmt.Sprintf(
		"✅ Agendamento confirmado!\n\n"+
			"📅 %s, %s as %s\n"+
			"👨‍⚕️ %s\n\n"+
			"Ate la! 😊",
		weekday,
		dateStr,
		timeStr,
		professionalName,
	)

	job := &queue.MessageJob{
		ID:            uuid.New().String(),
		ClinicID:      clinicID.String(),
		AppointmentID: "",
		Instance:      clinic.WhatsAppInstance,
		Phone:         entryWithPatient.PatientWhatsApp,
		PatientName:   entryWithPatient.PatientName,
		Message:       message,
		MessageType:   "slot_confirmation",
		Attempts:      0,
		MaxAttempts:   3,
		CreatedAt:     time.Now(),
		ScheduledAt:   time.Now(),
	}

	return s.messageQueue.Publish(ctx, job)
}
