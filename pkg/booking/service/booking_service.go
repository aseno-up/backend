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
	emailService "github.com/aseno/backend/pkg/email/service"
	notificationModel "github.com/aseno/backend/pkg/notification/model"
	"github.com/aseno/backend/pkg/pagination"
	patientModel "github.com/aseno/backend/pkg/patient/model"
	procedureModel "github.com/aseno/backend/pkg/procedure/model"
	professionalModel "github.com/aseno/backend/pkg/professional/model"
	specialtyModel "github.com/aseno/backend/pkg/specialty/model"
)

// ClinicProvider provides clinic information
type ClinicProvider interface {
	GetBySlug(ctx context.Context, slug string) (*clinicModel.Clinic, error)
}

// ProfessionalProvider provides professional information
type ProfessionalProvider interface {
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*professionalModel.Professional, int64, error)
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

// SpecialtyProvider provides specialty operations
type SpecialtyProvider interface {
	ListActiveByClinic(clinicID uuid.UUID) ([]*specialtyModel.Specialty, error)
	CountByClinic(clinicID uuid.UUID) (int64, error)
	GetByProfessionalIDs(professionalIDs []uuid.UUID) (map[uuid.UUID][]*specialtyModel.Specialty, error)
}

// ProcedureProvider provides procedure operations
type ProcedureProvider interface {
	ListByClinicAndSpecialty(ctx context.Context, clinicID uuid.UUID, specialtyID *uuid.UUID) ([]*procedureModel.Procedure, error)
	GetByID(id, clinicID uuid.UUID) (*procedureModel.Procedure, error)
}

// BookingService defines the interface for public booking operations
type BookingService interface {
	GetClinicInfo(ctx context.Context, slug string) (*model.ClinicBookingInfo, error)
	GetProfessionals(ctx context.Context, slug string) ([]*model.ProfessionalBookingInfo, error)
	GetSpecialties(ctx context.Context, slug string) ([]*model.SpecialtyBookingInfo, error)
	GetProcedures(ctx context.Context, slug string, specialtyID *uuid.UUID) ([]*model.ProcedureBookingInfo, error)
	GetAvailableSlots(ctx context.Context, slug string, professionalID uuid.UUID, date time.Time, procedureID *uuid.UUID) (*model.AvailableSlotsResponse, error)
	CreateBooking(ctx context.Context, slug string, req *model.CreateBookingRequest) (*model.BookingResponse, error)
}

// bookingService implements BookingService
type bookingService struct {
	clinicRepo       ClinicProvider
	professionalRepo ProfessionalProvider
	appointmentRepo  AppointmentProvider
	patientRepo      PatientProvider
	notificationSvc  NotificationCreator
	specialtyRepo    SpecialtyProvider
	procedureRepo    ProcedureProvider
	emailSvc         emailService.EmailService
	frontendURL      string
}

// NewBookingService creates a new booking service
func NewBookingService(
	clinicRepo ClinicProvider,
	professionalRepo ProfessionalProvider,
	appointmentRepo AppointmentProvider,
	patientRepo PatientProvider,
	notificationSvc NotificationCreator,
	specialtyRepo SpecialtyProvider,
	procedureRepo ProcedureProvider,
	emailSvc emailService.EmailService,
	frontendURL string,
) BookingService {
	return &bookingService{
		clinicRepo:       clinicRepo,
		professionalRepo: professionalRepo,
		appointmentRepo:  appointmentRepo,
		patientRepo:      patientRepo,
		notificationSvc:  notificationSvc,
		specialtyRepo:    specialtyRepo,
		procedureRepo:    procedureRepo,
		emailSvc:         emailSvc,
		frontendURL:      frontendURL,
	}
}

// GetClinicInfo returns public clinic information for booking page
func (s *bookingService) GetClinicInfo(ctx context.Context, slug string) (*model.ClinicBookingInfo, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if !clinic.Settings.AllowOnlineBooking {
		return nil, errors.ErrValidationInstance.WithMessage("Não foi possível encontrar a clínica para agendamento online. Verifique o link ou tente novamente mais tarde.")
	}

	if !clinic.IsSubscriptionActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Não conseguimos buscar as informacoes da clinica, tente novamente mais tarde.")
	}

	defaultMinutes := clinic.Settings.DefaultAppointmentMinutes
	if defaultMinutes == 0 {
		defaultMinutes = 50
	}

	// Count professionals with active schedules
	professionals, _, err := s.professionalRepo.List(ctx, clinic.ID, pagination.Filter{Status: "active", Limit: 100})
	if err != nil {
		return nil, err
	}
	profCount := 0
	for _, p := range professionals {
		if hasWorkingHours(p.WorkingHours) {
			profCount++
		}
	}

	// Count active specialties
	specCount, _ := s.specialtyRepo.CountByClinic(clinic.ID)

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
		AboutUs:                   clinic.Settings.AboutUs,
		ProfessionalCount:         profCount,
		SpecialtyCount:            int(specCount),
	}, nil
}

// GetProfessionals returns list of active professionals for booking
func (s *bookingService) GetProfessionals(ctx context.Context, slug string) ([]*model.ProfessionalBookingInfo, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if !clinic.Settings.AllowOnlineBooking {
		return nil, errors.ErrValidationInstance.WithMessage("Não foi possível encontrar a clínica para agendamento online. Verifique o link ou tente novamente mais tarde.")
	}

	professionals, _, err := s.professionalRepo.List(ctx, clinic.ID, pagination.Filter{Status: "active", Limit: 100})
	if err != nil {
		return nil, err
	}

	// Batch-load specialties for all professionals
	profIDs := make([]uuid.UUID, len(professionals))
	for i, p := range professionals {
		profIDs[i] = p.ID
	}
	profSpecialties, _ := s.specialtyRepo.GetByProfessionalIDs(profIDs)

	result := make([]*model.ProfessionalBookingInfo, 0, len(professionals))
	for _, p := range professionals {
		hasSchedule := hasWorkingHours(p.WorkingHours)

		specs := profSpecialties[p.ID]
		specIDs := make([]uuid.UUID, len(specs))
		specNames := make([]string, len(specs))
		for i, sp := range specs {
			specIDs[i] = sp.ID
			specNames[i] = sp.Name
		}

		result = append(result, &model.ProfessionalBookingInfo{
			ID:                   p.ID,
			Name:                 p.Name,
			SpecialtyIDs:         specIDs,
			SpecialtyNames:       specNames,
			AvatarURL:            p.AvatarURL,
			Bio:                  p.Bio,
			Color:                p.Color,
			HasAvailableSchedule: hasSchedule,
			WorkingHours:         p.WorkingHours,
		})
	}

	return result, nil
}

// GetSpecialties returns active specialties for a clinic
func (s *bookingService) GetSpecialties(ctx context.Context, slug string) ([]*model.SpecialtyBookingInfo, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	specialties, err := s.specialtyRepo.ListActiveByClinic(clinic.ID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.SpecialtyBookingInfo, len(specialties))
	for i, sp := range specialties {
		result[i] = &model.SpecialtyBookingInfo{
			ID:   sp.ID,
			Name: sp.Name,
		}
	}
	return result, nil
}

// GetProcedures returns procedures for a clinic, optionally filtered by specialty
func (s *bookingService) GetProcedures(ctx context.Context, slug string, specialtyID *uuid.UUID) ([]*model.ProcedureBookingInfo, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	procedures, err := s.procedureRepo.ListByClinicAndSpecialty(ctx, clinic.ID, specialtyID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.ProcedureBookingInfo, len(procedures))
	for i, p := range procedures {
		result[i] = &model.ProcedureBookingInfo{
			ID:              p.ID,
			Name:            p.Name,
			SpecialtyID:     p.SpecialtyID,
			Price:           p.Price,
			DurationMinutes: p.DurationMinutes,
		}
	}
	return result, nil
}

// GetAvailableSlots returns available time slots for a professional on a given date
func (s *bookingService) GetAvailableSlots(ctx context.Context, slug string, professionalID uuid.UUID, date time.Time, procedureID *uuid.UUID) (*model.AvailableSlotsResponse, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if !clinic.Settings.AllowOnlineBooking {
		return nil, errors.ErrValidationInstance.WithMessage("Não foi possível encontrar a clínica para agendamento online. Verifique o link ou tente novamente mais tarde.")
	}

	professional, err := s.professionalRepo.GetByID(ctx, professionalID, clinic.ID)
	if err != nil {
		return nil, err
	}

	// Duration: use procedure duration if provided, otherwise clinic default
	durationMinutes := clinic.Settings.DefaultAppointmentMinutes
	if durationMinutes == 0 {
		durationMinutes = 50
	}
	if procedureID != nil {
		if proc, err := s.procedureRepo.GetByID(*procedureID, clinic.ID); err == nil && proc.DurationMinutes > 0 {
			durationMinutes = proc.DurationMinutes
		}
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

		for curr := slotStart; curr.Add(time.Duration(durationMinutes)*time.Minute).Before(slotEnd) || curr.Add(time.Duration(durationMinutes)*time.Minute).Equal(slotEnd); curr = curr.Add(time.Duration(durationMinutes) * time.Minute) {
			candStart := curr
			candEnd := curr.Add(time.Duration(durationMinutes) * time.Minute)

			if candStart.Before(now) {
				continue
			}

			isConflict := false
			for _, apt := range existingAppointments {
				if apt.Status == appointmentModel.AppointmentStatusCanceled {
					continue
				}
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
// LGPD: Always creates a new patient with source=SELF_REGISTERED, never matches by phone
func (s *bookingService) CreateBooking(ctx context.Context, slug string, req *model.CreateBookingRequest) (*model.BookingResponse, error) {
	clinic, err := s.clinicRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if !clinic.Settings.AllowOnlineBooking {
		return nil, errors.ErrValidationInstance.WithMessage("Não foi possível encontrar a clínica para agendamento online. Verifique o link ou tente novamente mais tarde.")
	}

	if !clinic.IsSubscriptionActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Não conseguimos buscar as informacoes da clinica, tente novamente mais tard")
	}

	if req.StartTime.Before(time.Now()) {
		return nil, errors.ErrValidationInstance.WithMessage("A data e hora do agendamento devem ser no futuro")
	}

	professional, err := s.professionalRepo.GetByID(ctx, req.ProfessionalID, clinic.ID)
	if err != nil {
		return nil, err
	}

	// Calculate end time: use procedure duration if provided
	durationMinutes := clinic.Settings.DefaultAppointmentMinutes
	if durationMinutes == 0 {
		durationMinutes = 50
	}
	if req.ProcedureID != nil {
		if proc, err := s.procedureRepo.GetByID(*req.ProcedureID, clinic.ID); err == nil && proc.DurationMinutes > 0 {
			durationMinutes = proc.DurationMinutes
		}
	}
	endTime := req.StartTime.Add(time.Duration(durationMinutes) * time.Minute)

	hasConflict, err := s.appointmentRepo.CheckConflict(ctx, clinic.ID, req.ProfessionalID, req.StartTime, endTime, nil)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, errors.ErrSlotUnavailableInstance
	}

	// LGPD: Always create new patient with source=SELF_REGISTERED
	// Never match by phone/CPF on the public flow to prevent data leakage
	newPatient := &patientModel.Patient{
		ClinicID: clinic.ID,
		Name:     req.PatientName,
		Phone:    req.PatientPhone,
		WhatsApp: req.PatientPhone,
		Email:    req.PatientEmail,
		IsActive: true,
		Source:   patientModel.PatientSourceSelfRegistered,
	}

	if err := s.patientRepo.Create(ctx, newPatient); err != nil {
		return nil, err
	}

	appointment := &appointmentModel.Appointment{
		ClinicID:       clinic.ID,
		PatientID:      newPatient.ID,
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

	s.createBookingNotification(ctx, clinic.ID, req.PatientName, professional.Name, req.StartTime, appointment.ID)

	// Send booking confirmation email to patient (if email provided)
	if req.PatientEmail != "" && s.emailSvc != nil {
		portalLink := ""
		if s.frontendURL != "" {
			portalLink = fmt.Sprintf("%s/agendar/%s", s.frontendURL, clinic.Slug)
		}

		// Convert times to clinic timezone for display in email
		emailStart := appointment.StartTime
		emailEnd := appointment.EndTime
		if clinic.Settings.Timezone != "" {
			if loc, err := time.LoadLocation(clinic.Settings.Timezone); err == nil {
				emailStart = emailStart.In(loc)
				emailEnd = emailEnd.In(loc)
			}
		}

		if err := s.emailSvc.SendBookingConfirmation(ctx, &emailService.BookingConfirmationEmailData{
			RecipientEmail:   req.PatientEmail,
			RecipientName:    req.PatientName,
			ClinicName:       clinic.Name,
			ProfessionalName: professional.Name,
			StartTime:        emailStart,
			EndTime:          emailEnd,
			PortalLink:       portalLink,
		}); err != nil {
			fmt.Printf("[BOOKING] Failed to send confirmation email to %s: %v\n", req.PatientEmail, err)
		}
	}

	return &model.BookingResponse{
		ID:           appointment.ID,
		Professional: professional.Name,
		StartTime:    appointment.StartTime,
		EndTime:      appointment.EndTime,
		Status:       string(appointment.Status),
		Message:      "Agendamento realizado com sucesso! Aguarde a confirmacao da clinica.",
	}, nil
}

// createBookingNotification creates a notification for the clinic about a new booking
func (s *bookingService) createBookingNotification(ctx context.Context, clinicID uuid.UUID, patientName, professionalName string, startTime time.Time, appointmentID uuid.UUID) {
	notification := &notificationModel.Notification{
		ClinicID: clinicID,
		Type:     notificationModel.NotificationTypeNewBooking,
		Title:    "Novo Agendamento Online",
		Message:  fmt.Sprintf("%s agendou online com %s para %s", patientName, professionalName, startTime.Format("02/01 às 15:04")),
		Data: notificationModel.JSON{
			"appointment_id":    appointmentID.String(),
			"patient_name":      patientName,
			"professional_name": professionalName,
			"start_time":        startTime.Format(time.RFC3339),
		},
	}

	if err := s.notificationSvc.Create(ctx, notification); err != nil {
		fmt.Printf("[BOOKING] Failed to create notification for clinic %s: %v\n", clinicID, err)
	}
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
