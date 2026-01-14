package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	appointmentModel "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/booking/model"
	clinicModel "github.com/aseno/backend/pkg/clinic/model"
	notificationModel "github.com/aseno/backend/pkg/notification/model"
	patientModel "github.com/aseno/backend/pkg/patient/model"
	professionalModel "github.com/aseno/backend/pkg/professional/model"
)

// ClinicProvider provides clinic information
type ClinicProvider interface {
	GetBySlug(ctx context.Context, slug string) (*clinicModel.Clinic, error)
}

// ProfessionalProvider provides professional information
type ProfessionalProvider interface {
	GetByClinicID(ctx context.Context, clinicID uuid.UUID, activeOnly bool, limit, offset int) ([]*professionalModel.Professional, int64, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*professionalModel.Professional, error)
}

// AppointmentProvider provides appointment operations
type AppointmentProvider interface {
	GetByDateRange(ctx context.Context, clinicID uuid.UUID, start, end time.Time, professionalID *uuid.UUID) ([]*appointmentModel.Appointment, error)
	CheckConflict(ctx context.Context, clinicID, professionalID uuid.UUID, start, end time.Time, excludeID *uuid.UUID) (bool, error)
	Create(ctx context.Context, appointment *appointmentModel.Appointment) error
}

// PatientProvider provides patient operations
type PatientProvider interface {
	GetByPhone(ctx context.Context, phone string, clinicID uuid.UUID) (*patientModel.Patient, error)
	Create(ctx context.Context, patient *patientModel.Patient) error
}

// NotificationCreator creates notifications
type NotificationCreator interface {
	Create(ctx context.Context, notification *notificationModel.Notification) error
}

// BookingService defines the interface for public booking operations
type BookingService interface {
	GetClinicInfo(ctx context.Context, slug string) (*model.ClinicBookingInfo, error)
	GetProfessionals(ctx context.Context, slug string) ([]*model.ProfessionalBookingInfo, error)
	GetAvailableSlots(ctx context.Context, slug string, professionalID uuid.UUID, date time.Time) (*model.AvailableSlotsResponse, error)
	CreateBooking(ctx context.Context, slug string, req *model.CreateBookingRequest) (*model.BookingResponse, error)
}

// bookingService implements BookingService
type bookingService struct {
	clinicRepo       ClinicProvider
	professionalRepo ProfessionalProvider
	appointmentRepo  AppointmentProvider
	patientRepo      PatientProvider
	notificationSvc  NotificationCreator
}

// NewBookingService creates a new booking service
func NewBookingService(
	clinicRepo ClinicProvider,
	professionalRepo ProfessionalProvider,
	appointmentRepo AppointmentProvider,
	patientRepo PatientProvider,
	notificationSvc NotificationCreator,
) BookingService {
	return &bookingService{
		clinicRepo:       clinicRepo,
		professionalRepo: professionalRepo,
		appointmentRepo:  appointmentRepo,
		patientRepo:      patientRepo,
		notificationSvc:  notificationSvc,
	}
}

// GetClinicInfo returns public clinic information for booking page
func (s *bookingService) GetClinicInfo(ctx context.Context, slug string) (*model.ClinicBookingInfo, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Check if online booking is enabled
	if !clinic.Settings.AllowOnlineBooking {
		return nil, errors.ErrValidationInstance.WithMessage("Agendamento online nao habilitado para esta clinica")
	}

	// Check if subscription is active
	if !clinic.IsSubscriptionActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Clinica com assinatura inativa")
	}

	defaultMinutes := clinic.Settings.DefaultAppointmentMinutes
	if defaultMinutes == 0 {
		defaultMinutes = 50 // Default to 50 minutes
	}

	return &model.ClinicBookingInfo{
		Name:                      clinic.Name,
		Slug:                      clinic.Slug,
		LogoURL:                   clinic.LogoURL,
		PrimaryColor:              clinic.PrimaryColor,
		Phone:                     clinic.Phone,
		Address:                   clinic.Address,
		City:                      clinic.City,
		State:                     clinic.State,
		DefaultAppointmentMinutes: defaultMinutes,
	}, nil
}

// GetProfessionals returns list of active professionals for booking
func (s *bookingService) GetProfessionals(ctx context.Context, slug string) ([]*model.ProfessionalBookingInfo, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Check if online booking is enabled
	if !clinic.Settings.AllowOnlineBooking {
		return nil, errors.ErrValidationInstance.WithMessage("Agendamento online nao habilitado para esta clinica")
	}

	professionals, _, err := s.professionalRepo.GetByClinicID(ctx, clinic.ID, true, 100, 0)
	if err != nil {
		return nil, err
	}

	result := make([]*model.ProfessionalBookingInfo, 0, len(professionals))
	for _, p := range professionals {
		// Only include professionals with working hours defined
		if !hasWorkingHours(p.WorkingHours) {
			continue
		}

		result = append(result, &model.ProfessionalBookingInfo{
			ID:           p.ID,
			Name:         p.Name,
			Specialty:    p.Specialty,
			AvatarURL:    p.AvatarURL,
			Bio:          p.Bio,
			Color:        p.Color,
			WorkingHours: p.WorkingHours,
		})
	}

	return result, nil
}

// GetAvailableSlots returns available time slots for a professional on a given date
func (s *bookingService) GetAvailableSlots(ctx context.Context, slug string, professionalID uuid.UUID, date time.Time) (*model.AvailableSlotsResponse, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Check if online booking is enabled
	if !clinic.Settings.AllowOnlineBooking {
		return nil, errors.ErrValidationInstance.WithMessage("Agendamento online nao habilitado para esta clinica")
	}

	// Get professional
	professional, err := s.professionalRepo.GetByID(ctx, professionalID, clinic.ID)
	if err != nil {
		return nil, err
	}

	// Duration in minutes
	durationMinutes := clinic.Settings.DefaultAppointmentMinutes
	if durationMinutes == 0 {
		durationMinutes = 50
	}

	// Get working hours for the day
	weekday := date.Weekday()
	var daySlots []professionalModel.TimeSlot
	switch weekday {
	case time.Monday:
		daySlots = professional.WorkingHours.Monday
	case time.Tuesday:
		daySlots = professional.WorkingHours.Tuesday
	case time.Wednesday:
		daySlots = professional.WorkingHours.Wednesday
	case time.Thursday:
		daySlots = professional.WorkingHours.Thursday
	case time.Friday:
		daySlots = professional.WorkingHours.Friday
	case time.Saturday:
		daySlots = professional.WorkingHours.Saturday
	case time.Sunday:
		daySlots = professional.WorkingHours.Sunday
	}

	if len(daySlots) == 0 {
		return &model.AvailableSlotsResponse{
			Date:            date.Format("2006-01-02"),
			ProfessionalID:  professionalID,
			DurationMinutes: durationMinutes,
			Slots:           []*model.AvailableSlot{},
		}, nil
	}

	// Get existing appointments for the day
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	existingAppointments, err := s.appointmentRepo.GetByDateRange(ctx, clinic.ID, startOfDay, endOfDay, &professionalID)
	if err != nil {
		return nil, err
	}

	// Generate available slots
	now := time.Now()
	var availableSlots []*model.AvailableSlot

	for _, slot := range daySlots {
		h1, m1 := parseTime(slot.Start)
		h2, m2 := parseTime(slot.End)

		slotStart := time.Date(date.Year(), date.Month(), date.Day(), h1, m1, 0, 0, date.Location())
		slotEnd := time.Date(date.Year(), date.Month(), date.Day(), h2, m2, 0, 0, date.Location())

		// Generate time blocks
		for curr := slotStart; curr.Add(time.Duration(durationMinutes)*time.Minute).Before(slotEnd) || curr.Add(time.Duration(durationMinutes)*time.Minute).Equal(slotEnd); curr = curr.Add(time.Duration(durationMinutes) * time.Minute) {
			candStart := curr
			candEnd := curr.Add(time.Duration(durationMinutes) * time.Minute)

			// Skip past times
			if candStart.Before(now) {
				continue
			}

			// Check for conflicts with existing appointments
			isConflict := false
			for _, apt := range existingAppointments {
				if apt.Status == appointmentModel.AppointmentStatusCanceled {
					continue
				}
				// Check overlap
				if apt.StartTime.Before(candEnd) && apt.EndTime.After(candStart) {
					isConflict = true
					break
				}
			}

			if !isConflict {
				availableSlots = append(availableSlots, &model.AvailableSlot{
					StartTime: candStart,
					EndTime:   candEnd,
				})
			}
		}
	}

	return &model.AvailableSlotsResponse{
		Date:            date.Format("2006-01-02"),
		ProfessionalID:  professionalID,
		DurationMinutes: durationMinutes,
		Slots:           availableSlots,
	}, nil
}

// CreateBooking creates a new booking from public form
func (s *bookingService) CreateBooking(ctx context.Context, slug string, req *model.CreateBookingRequest) (*model.BookingResponse, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Check if online booking is enabled
	if !clinic.Settings.AllowOnlineBooking {
		return nil, errors.ErrValidationInstance.WithMessage("Agendamento online nao habilitado para esta clinica")
	}

	// Check if subscription is active
	if !clinic.IsSubscriptionActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Clinica com assinatura inativa")
	}

	// Validate start time is in the future
	if req.StartTime.Before(time.Now()) {
		return nil, errors.ErrValidationInstance.WithMessage("Horario deve ser no futuro")
	}

	// Get professional
	professional, err := s.professionalRepo.GetByID(ctx, req.ProfessionalID, clinic.ID)
	if err != nil {
		return nil, err
	}

	// Calculate end time
	durationMinutes := clinic.Settings.DefaultAppointmentMinutes
	if durationMinutes == 0 {
		durationMinutes = 50
	}
	endTime := req.StartTime.Add(time.Duration(durationMinutes) * time.Minute)

	// Check for conflicts
	hasConflict, err := s.appointmentRepo.CheckConflict(ctx, clinic.ID, req.ProfessionalID, req.StartTime, endTime, nil)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, errors.ErrSlotUnavailableInstance
	}

	// Find or create patient
	patient, err := s.findOrCreatePatient(ctx, clinic.ID, req.PatientName, req.PatientPhone)
	if err != nil {
		return nil, err
	}

	// Create appointment
	appointment := &appointmentModel.Appointment{
		ClinicID:       clinic.ID,
		PatientID:      patient.ID,
		ProfessionalID: req.ProfessionalID,
		StartTime:      req.StartTime,
		EndTime:        endTime,
		Status:         appointmentModel.AppointmentStatusPending,
		Type:           appointmentModel.AppointmentTypeRegular,
		Title:          "Agendamento Online",
		PatientNotes:   req.PatientNotes,
	}

	if err := s.appointmentRepo.Create(ctx, appointment); err != nil {
		return nil, err
	}

	// Create notification for clinic
	s.createBookingNotification(ctx, clinic.ID, req.PatientName, professional.Name, req.StartTime, appointment.ID)

	return &model.BookingResponse{
		ID:           appointment.ID,
		Professional: professional.Name,
		StartTime:    appointment.StartTime,
		EndTime:      appointment.EndTime,
		Status:       string(appointment.Status),
		Message:      "Agendamento realizado com sucesso! Aguarde a confirmacao da clinica.",
	}, nil
}

// findOrCreatePatient finds an existing patient by phone or creates a new one
func (s *bookingService) findOrCreatePatient(ctx context.Context, clinicID uuid.UUID, name, phone string) (*patientModel.Patient, error) {
	// Try to find existing patient
	patient, err := s.patientRepo.GetByPhone(ctx, phone, clinicID)
	if err == nil {
		return patient, nil
	}

	// Check if it's a "not found" error
	if !errors.IsNotFound(err) {
		return nil, err
	}

	// Create new patient
	newPatient := &patientModel.Patient{
		ClinicID: clinicID,
		Name:     name,
		Phone:    phone,
		WhatsApp: phone,
		IsActive: true,
	}

	if err := s.patientRepo.Create(ctx, newPatient); err != nil {
		return nil, err
	}

	return newPatient, nil
}

// createBookingNotification creates a notification for the clinic about a new booking
func (s *bookingService) createBookingNotification(ctx context.Context, clinicID uuid.UUID, patientName, professionalName string, startTime time.Time, appointmentID uuid.UUID) {
	notification := &notificationModel.Notification{
		ClinicID: clinicID,
		Type:     notificationModel.NotificationType("new_booking"),
		Title:    "Novo Agendamento Online",
		Message:  fmt.Sprintf("%s agendou online com %s para %s", patientName, professionalName, startTime.Format("02/01 as 15:04")),
		Data: notificationModel.JSON{
			"appointment_id":    appointmentID.String(),
			"patient_name":      patientName,
			"professional_name": professionalName,
			"start_time":        startTime.Format(time.RFC3339),
		},
	}

	// Create notification in background - don't fail booking if notification fails
	_ = s.notificationSvc.Create(ctx, notification)
}

// hasWorkingHours checks if a professional has any working hours defined
func hasWorkingHours(wh professionalModel.WorkingHours) bool {
	return len(wh.Monday) > 0 ||
		len(wh.Tuesday) > 0 ||
		len(wh.Wednesday) > 0 ||
		len(wh.Thursday) > 0 ||
		len(wh.Friday) > 0 ||
		len(wh.Saturday) > 0 ||
		len(wh.Sunday) > 0
}

// parseTime parses time string "HH:MM" into hours and minutes
func parseTime(t string) (int, int) {
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m)
	return h, m
}
