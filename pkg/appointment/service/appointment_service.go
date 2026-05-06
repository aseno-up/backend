package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/aseno/backend/internal/logger"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/appointment/repository"
	calendar_service "github.com/aseno/backend/pkg/calendar/service"
	profModel "github.com/aseno/backend/pkg/professional/model"
	waitlist_model "github.com/aseno/backend/pkg/waitlist/model"
)

// WaitlistService defines the interface for waitlist operations needed by appointment service
type WaitlistService interface {
	ProcessCancelledAppointment(ctx context.Context, clinicID, professionalID uuid.UUID, slotTime time.Time, durationMinutes int) error
	Create(ctx context.Context, clinicID uuid.UUID, req *waitlist_model.CreateWaitlistRequest) (*waitlist_model.WaitlistEntryResponse, error)
}

// PatientRiskCalculator calculates patient no-show risk
type PatientRiskCalculator interface {
	CalculateRiskScore(ctx context.Context, patientID, clinicID uuid.UUID) error
}

// AppointmentService defines the interface for appointment business logic
type AppointmentService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateAppointmentRequest) (*model.AppointmentResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, filter *model.AppointmentFilter) (*model.AppointmentListResponse, error)
	GetByDay(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error)
	GetByWeek(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateAppointmentRequest) (*model.AppointmentResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID, deletePayments bool) error
	GetDeletePreview(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentDeletePreview, error)
	Confirm(ctx context.Context, id, clinicID uuid.UUID, req *model.ConfirmAppointmentRequest) (*model.AppointmentResponse, error)
	Cancel(ctx context.Context, id, clinicID uuid.UUID, req *model.CancelAppointmentRequest) (*model.AppointmentResponse, error)
	Complete(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error)
	MarkNoShow(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error)

	// CreateFromWaitlist creates an appointment from a waitlist entry
	CreateFromWaitlist(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) (uuid.UUID, error)

	// Reschedule updates an appointment's time and professional (used by waitlist anticipation)
	Reschedule(ctx context.Context, appointmentID, clinicID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) error

	// SetWaitlistProcessor sets the waitlist processor for encaixe
	SetWaitlistProcessor(service WaitlistService)

	// SetPatientRiskCalculator sets the patient risk calculator
	SetPatientRiskCalculator(calculator PatientRiskCalculator)

	// SetCommissionCalculator sets the commission calculator
	SetCommissionCalculator(calculator CommissionCalculator)

	// SetCalendarService sets the calendar service for Google Calendar sync
	SetCalendarService(service calendar_service.CalendarService)

	// SetSessionPlanHandler sets the handler for session plan updates on appointment completion
	SetSessionPlanHandler(handler SessionPlanCompletionHandler)

	// SetPaymentLister wires the source used to load the payments under an
	// appointment before it is deleted, so the linked finance rows can be
	// cleaned up.
	SetPaymentLister(lister PaymentLister)

	// SetFinanceUpserter wires the finance sink used to drop the
	// `financial_transactions` rows associated with payments removed in
	// cascade with the appointment.
	SetFinanceUpserter(u FinanceUpserter)

	// SetSalesPackagePicker wires the salespackage FIFO picker used during
	// appointment creation: when the patient has package balance for a
	// procedure, the join row is bound to the oldest available item.
	SetSalesPackagePicker(picker SalesPackagePicker)

	// SuggestSmartSlots suggests best time slots based on historical data
	SuggestSmartSlots(ctx context.Context, clinicID, professionalID uuid.UUID, date time.Time, days int, durationMinutes int) ([]*model.SmartSlotSuggestion, error)
}

// SalesPackagePicker resolves the oldest active sales_package_item for a
// (patient, procedure) pair so the appointment can charge against it.
// Implemented by the salespackage service; nil when not wired (ad-hoc only).
type SalesPackagePicker interface {
	PickFIFOItemID(ctx context.Context, clinicID, patientID, procedureID uuid.UUID) (*uuid.UUID, error)
}

// PaymentLister exposes just the "list payments for an appointment" capability
// so the appointment service can enumerate the payments that are about to be
// dropped and clean up their finance counterparts. Narrow interface to keep
// tests mock-friendly and avoid a wider dependency on the full payment repo.
type PaymentLister interface {
	ListByAppointment(appointmentID, clinicID uuid.UUID) ([]*model.AppointmentPayment, error)
}

// ProfessionalInfoProvider provides professional information
type ProfessionalInfoProvider interface {
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*profModel.Professional, error)
}

// CommissionCalculator calculates and stores commissions for completed appointments
type CommissionCalculator interface {
	CalculateAndStore(ctx context.Context, appointment *model.Appointment, clinicID uuid.UUID) error
	VoidByAppointment(ctx context.Context, appointmentID, clinicID uuid.UUID) error
}

// SessionPlanCompletionHandler handles session plan updates when an appointment is completed
type SessionPlanCompletionHandler interface {
	OnAppointmentCompleted(ctx context.Context, clinicID uuid.UUID, appointmentID uuid.UUID) error
}

// appointmentService implements AppointmentService
type appointmentService struct {
	repo                 repository.AppointmentRepository
	profRepo             ProfessionalInfoProvider
	waitlistService      WaitlistService
	riskCalculator       PatientRiskCalculator
	commissionCalculator CommissionCalculator
	calendarService      calendar_service.CalendarService
	sessionPlanHandler   SessionPlanCompletionHandler
	paymentLister        PaymentLister
	financeUpserter      FinanceUpserter
	salesPackagePicker   SalesPackagePicker
}

// NewAppointmentService creates a new appointment service
func NewAppointmentService(repo repository.AppointmentRepository, profRepo ProfessionalInfoProvider) AppointmentService {
	return &appointmentService{
		repo:     repo,
		profRepo: profRepo,
	}
}

// Reschedule updates an appointment's time and professional
func (s *appointmentService) Reschedule(ctx context.Context, appointmentID, clinicID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) error {
	appointment, err := s.repo.GetByID(ctx, appointmentID, clinicID)
	if err != nil {
		return err
	}

	endTime := startTime.Add(time.Duration(durationMinutes) * time.Minute)

	// Check for conflicts, excluding the current appointment
	hasConflict, err := s.repo.CheckConflict(ctx, clinicID, professionalID, startTime, endTime, &appointmentID)
	if err != nil {
		return err
	}
	if hasConflict {
		return errors.ErrSlotUnavailableInstance
	}

	// Update fields
	appointment.ProfessionalID = professionalID
	appointment.StartTime = startTime
	appointment.EndTime = endTime
	appointment.Status = model.AppointmentStatusConfirmed // Auto-confirm for anticipation

	// Add note about rescheduling
	now := time.Now()
	appointment.UpdatedAt = now
	appointment.Notes = appointment.Notes + "\n[Sistema] Antecipado em " + now.Format("02/01/2006 15:04")

	return s.repo.Update(ctx, appointment)
}

// SetWaitlistProcessor sets the waitlist service
func (s *appointmentService) SetWaitlistProcessor(service WaitlistService) {
	s.waitlistService = service
}

// SetPatientRiskCalculator sets the patient risk calculator
func (s *appointmentService) SetPatientRiskCalculator(calculator PatientRiskCalculator) {
	s.riskCalculator = calculator
}

// SetCommissionCalculator sets the commission calculator for automatic commission on completion
func (s *appointmentService) SetCommissionCalculator(calculator CommissionCalculator) {
	s.commissionCalculator = calculator
}

// SetCalendarService sets the calendar service for Google Calendar sync
func (s *appointmentService) SetCalendarService(service calendar_service.CalendarService) {
	s.calendarService = service
}

// SetSessionPlanHandler sets the handler for session plan updates on appointment completion
func (s *appointmentService) SetSessionPlanHandler(handler SessionPlanCompletionHandler) {
	s.sessionPlanHandler = handler
}

// SetPaymentLister wires the payment list source used during delete cascade.
func (s *appointmentService) SetPaymentLister(lister PaymentLister) {
	s.paymentLister = lister
}

// SetSalesPackagePicker wires the FIFO sales-package picker.
func (s *appointmentService) SetSalesPackagePicker(picker SalesPackagePicker) {
	s.salesPackagePicker = picker
}

// SetFinanceUpserter wires the finance sink used during delete cascade.
func (s *appointmentService) SetFinanceUpserter(u FinanceUpserter) {
	s.financeUpserter = u
}

// toCalendarData converts an appointment to calendar sync data
func (s *appointmentService) toCalendarData(apt *model.Appointment) *calendar_service.AppointmentData {
	data := &calendar_service.AppointmentData{
		ID:        apt.ID,
		Title:     apt.Title,
		StartTime: apt.StartTime,
		EndTime:   apt.EndTime,
		Notes:     apt.Notes,
	}

	if data.Title == "" {
		if apt.Patient != nil {
			data.Title = fmt.Sprintf("Consulta - %s", apt.Patient.Name)
		} else {
			data.Title = "Consulta"
		}
	}

	if apt.Patient != nil {
		data.PatientName = apt.Patient.Name
		data.PatientEmail = apt.Patient.Email
	}

	if apt.Professional != nil {
		data.ProfessionalName = apt.Professional.Name
	}

	return data
}

// syncToGoogleCalendar syncs an appointment to Google Calendar in background
func (s *appointmentService) syncToGoogleCalendar(clinicID uuid.UUID, apt *model.Appointment) {
	if s.calendarService == nil {
		return
	}

	go func() {
		bgCtx := context.Background()
		aptData := s.toCalendarData(apt)
		if err := s.calendarService.SyncAppointmentToGoogle(bgCtx, clinicID, aptData); err != nil {
			logger.Printf("[Calendar Sync] Error syncing appointment %s: %v", apt.ID, err)
		}
	}()
}

// updateGoogleCalendar updates an appointment in Google Calendar in background
func (s *appointmentService) updateGoogleCalendar(clinicID uuid.UUID, apt *model.Appointment) {
	if s.calendarService == nil {
		return
	}

	go func() {
		bgCtx := context.Background()
		aptData := s.toCalendarData(apt)
		if err := s.calendarService.UpdateAppointmentInGoogle(bgCtx, clinicID, aptData); err != nil {
			logger.Printf("[Calendar Sync] Error updating appointment %s in Google: %v", apt.ID, err)
		}
	}()
}

// deleteFromGoogleCalendar removes an appointment from Google Calendar in background
func (s *appointmentService) deleteFromGoogleCalendar(clinicID uuid.UUID, appointmentID uuid.UUID) {
	if s.calendarService == nil {
		return
	}

	go func() {
		bgCtx := context.Background()
		if err := s.calendarService.DeleteAppointmentFromGoogle(bgCtx, clinicID, appointmentID); err != nil {
			logger.Printf("[Calendar Sync] Error removing appointment %s from Google: %v", appointmentID, err)
		}
	}()
}

// SuggestSmartSlots suggests best time slots based on historical data
func (s *appointmentService) SuggestSmartSlots(ctx context.Context, clinicID, professionalID uuid.UUID, date time.Time, days int, durationMinutes int) ([]*model.SmartSlotSuggestion, error) {
	// 1. Get Professional Working Hours
	prof, err := s.profRepo.GetByID(ctx, professionalID, clinicID)
	if err != nil {
		return nil, err
	}

	// 2. Get Performance Stats
	stats, err := s.repo.GetSlotPerformanceStats(ctx, clinicID, professionalID)
	if err != nil {
		// Log error but continue without stats
		logger.Printf("Error getting slot stats: %v", err)
	}

	// Map stats for easy lookup: [DayOfWeek][Hour] -> Rate
	statMap := make(map[int]map[int]float64)
	for _, stat := range stats {
		if _, ok := statMap[stat.DayOfWeek]; !ok {
			statMap[stat.DayOfWeek] = make(map[int]float64)
		}
		statMap[stat.DayOfWeek][stat.HourOfDay] = stat.Rate
	}

	// 3. Generate Slots
	var suggestions []*model.SmartSlotSuggestion

	// Normalize start date to beginning of day
	currentDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endDate := currentDate.AddDate(0, 0, days)

	// Fetch existing appointments for the whole range for conflict checking
	existingAppointments, err := s.repo.GetByDateRange(ctx, clinicID, currentDate, endDate, &professionalID)
	if err != nil {
		return nil, err
	}

	for d := 0; d < days; d++ {
		dayDate := currentDate.AddDate(0, 0, d)
		weekday := dayDate.Weekday()

		var slots []profModel.TimeSlot
		switch weekday {
		case time.Monday:
			slots = prof.WorkingHours.Monday
		case time.Tuesday:
			slots = prof.WorkingHours.Tuesday
		case time.Wednesday:
			slots = prof.WorkingHours.Wednesday
		case time.Thursday:
			slots = prof.WorkingHours.Thursday
		case time.Friday:
			slots = prof.WorkingHours.Friday
		case time.Saturday:
			slots = prof.WorkingHours.Saturday
		case time.Sunday:
			slots = prof.WorkingHours.Sunday
		}

		for _, slot := range slots {
			// Parse start/end times (HH:MM)
			h1, m1 := parseTime(slot.Start)
			h2, m2 := parseTime(slot.End)

			slotStart := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), h1, m1, 0, 0, dayDate.Location())
			slotEnd := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), h2, m2, 0, 0, dayDate.Location())

			// Generate chunks of 'durationMinutes'
			for curr := slotStart; curr.Add(time.Duration(durationMinutes)*time.Minute).Before(slotEnd) || curr.Add(time.Duration(durationMinutes)*time.Minute).Equal(slotEnd); curr = curr.Add(time.Duration(durationMinutes) * time.Minute) {

				candStart := curr
				candEnd := curr.Add(time.Duration(durationMinutes) * time.Minute)

				// Check if in past
				if candStart.Before(time.Now()) {
					continue
				}

				// Check conflicts
				isConflict := false
				for _, apt := range existingAppointments {
					// Check overlap
					if apt.StartTime.Before(candEnd) && apt.EndTime.After(candStart) {
						isConflict = true
						break
					}
				}
				if isConflict {
					continue
				}

				// Calculate Score
				score := 5.0 // Base score
				reason := "Horário disponível"

				dow := int(weekday)
				hour := candStart.Hour()

				if dayStats, ok := statMap[dow]; ok {
					if rate, ok := dayStats[hour]; ok {
						if rate >= 0.9 {
							score += 3.0
							reason = "Alta taxa de comparecimento histórico"
						} else if rate >= 0.7 {
							score += 1.0
							reason = "Boa taxa de comparecimento"
						} else if rate < 0.5 {
							score -= 2.0
							reason = "Histórico de faltas neste horário"
						}
					}
				}

				// Prefer working hours core (9-17) slightly
				if hour >= 9 && hour <= 17 {
					score += 0.5
				}

				// Create suggestion
				suggestions = append(suggestions, &model.SmartSlotSuggestion{
					StartTime: candStart,
					EndTime:   candEnd,
					Score:     score,
					Label:     getScoreLabel(score),
					Reason:    reason,
				})
			}
		}
	}

	// Sort by score desc
	// (Simple bubble sort or slice sort, doing simple for now)
	for i := 0; i < len(suggestions); i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[i].Score < suggestions[j].Score {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	// Limit to top 20
	if len(suggestions) > 20 {
		suggestions = suggestions[:20]
	}

	return suggestions, nil
}

func parseTime(t string) (int, int) {
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m)
	return h, m
}

func getScoreLabel(score float64) string {
	if score >= 8.0 {
		return "Excelente"
	}
	if score >= 6.0 {
		return "Bom"
	}
	if score >= 4.0 {
		return "Regular"
	}
	return "Arriscado"
}

// Create creates a new appointment
func (s *appointmentService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateAppointmentRequest) (*model.AppointmentResponse, error) {
	endTime := req.StartTime.Add(time.Duration(req.DurationMinutes) * time.Minute)

	// Check for conflicts
	hasConflict, err := s.repo.CheckConflict(ctx, clinicID, req.ProfessionalID, req.StartTime, endTime, nil)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, errors.ErrSlotUnavailableInstance
	}

	appointmentType := req.Type
	if appointmentType == "" {
		appointmentType = model.AppointmentTypeRegular
	}

	// Map procedures. When a sales_package picker is wired and the patient has
	// remaining package balance for the procedure, the join row gets bound to
	// the FIFO item — which lets the booking consume one session of that
	// package without the user choosing manually.
	var appointmentProcedures []model.AppointmentProcedure
	var calculatedProceduresPriceCents int64
	for _, p := range req.Procedures {
		ap := model.AppointmentProcedure{
			ProcedureID: p.ProcedureID,
			Price:       *p.Price, // assuming input defaults are handled by handler or we take pointer value
		}
		if s.salesPackagePicker != nil {
			itemID, pickErr := s.salesPackagePicker.PickFIFOItemID(ctx, clinicID, req.PatientID, p.ProcedureID)
			if pickErr != nil {
				logger.Printf("[Appointment] sales package FIFO lookup failed (procedure=%s): %v", p.ProcedureID, pickErr)
			} else if itemID != nil {
				ap.SalesPackageItemID = itemID
			}
		}
		appointmentProcedures = append(appointmentProcedures, ap)
		calculatedProceduresPriceCents += int64(math.Round(*p.Price * 100))
	}

	// Map and validate specialties
	var appointmentSpecialties []model.AppointmentSpecialty
	if len(req.SpecialtyIDs) > 0 {
		prof, err := s.profRepo.GetByID(ctx, req.ProfessionalID, clinicID)
		if err != nil {
			return nil, err
		}

		// Create a map of active professional specialties for fast lookup
		profSpecs := make(map[uuid.UUID]bool)
		for _, ps := range prof.Specialties {
			if ps.IsActive {
				profSpecs[ps.SpecialtyID] = true
			}
		}

		for _, sid := range req.SpecialtyIDs {
			if !profSpecs[sid] {
				return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Esse profissional não possui a especialidade %s", sid))
			}
			appointmentSpecialties = append(appointmentSpecialties, model.AppointmentSpecialty{
				ClinicID:    clinicID,
				SpecialtyID: sid,
			})
		}
	}

	priceToUse := req.Price
	if req.Price == 0 && len(appointmentProcedures) > 0 {
		priceToUse = float64(calculatedProceduresPriceCents) / 100.0
	}

	appointment := &model.Appointment{
		ClinicID:       clinicID,
		PatientID:      req.PatientID,
		ProfessionalID: req.ProfessionalID,
		StartTime:      req.StartTime,
		EndTime:        req.StartTime.Add(time.Duration(req.DurationMinutes) * time.Minute),
		Status:         model.AppointmentStatusPending,
		Type:           appointmentType,
		Title:          req.Title,
		Notes:          req.Notes,
		Procedures:     appointmentProcedures,
		Specialties:    appointmentSpecialties,
		Price:          priceToUse,
	}

	if err := s.repo.Create(ctx, appointment); err != nil {
		return nil, err
	}

	// Handle anticipation request (add to waitlist)
	if req.WantsAnticipation && s.waitlistService != nil {
		// Use a background context or the current one? Current is safer for now, but async might be better for latency.
		// Let's do it async to not block the response, but log errors.
		go func() {
			bgCtx := context.Background()

			// Default preferences: same professional, any day, business hours
			days := []string{"monday", "tuesday", "wednesday", "thursday", "friday"}

			wlReq := &waitlist_model.CreateWaitlistRequest{
				PatientID:             req.PatientID,
				ProfessionalID:        &req.ProfessionalID,
				OriginalAppointmentID: &appointment.ID,
				PreferredDays:         days,
				PreferredTimeStart:    "08:00",
				PreferredTimeEnd:      "19:00",
				Notes:                 "Solicitação de antecipação criada automaticamente",
			}

			if _, err := s.waitlistService.Create(bgCtx, clinicID, wlReq); err != nil {
				logger.Printf("[Appointment] Error creating anticipation waitlist entry: %v", err)
			}
		}()
	}

	// Reload with relations for response and calendar sync
	fullAppointment, err := s.repo.GetByID(ctx, appointment.ID, clinicID)
	if err != nil {
		return nil, err
	}

	// Sync to Google Calendar (async)
	s.syncToGoogleCalendar(clinicID, fullAppointment)

	return fullAppointment.ToResponse(), nil
}

// GetByID retrieves an appointment by ID
func (s *appointmentService) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error) {
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}
	return appointment.ToResponse(), nil
}

// List retrieves a paginated list of appointments
func (s *appointmentService) List(ctx context.Context, clinicID uuid.UUID, filter *model.AppointmentFilter) (*model.AppointmentListResponse, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 || filter.PerPage > 100 {
		filter.PerPage = 20
	}

	appointments, total, err := s.repo.List(ctx, clinicID, filter)
	if err != nil {
		return nil, err
	}

	data := make([]*model.AppointmentResponse, len(appointments))
	for i, a := range appointments {
		data[i] = a.ToResponse()
	}

	totalPages := int(math.Ceil(float64(total) / float64(filter.PerPage)))

	return &model.AppointmentListResponse{
		Data:       data,
		Total:      total,
		Page:       filter.Page,
		PerPage:    filter.PerPage,
		TotalPages: totalPages,
	}, nil
}

// GetByDay retrieves appointments for a specific day
func (s *appointmentService) GetByDay(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error) {
	appointments, err := s.repo.GetByDay(ctx, clinicID, date, professionalID)
	if err != nil {
		return nil, err
	}

	data := make([]*model.AppointmentResponse, len(appointments))
	for i, a := range appointments {
		data[i] = a.ToResponse()
	}

	return data, nil
}

// GetByWeek retrieves appointments for a week starting from the given date
func (s *appointmentService) GetByWeek(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error) {
	// Get start of week (Monday)
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday
	}
	start := date.AddDate(0, 0, -(weekday - 1))
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, date.Location())

	// End of week (Sunday 23:59:59)
	end := start.AddDate(0, 0, 7)

	appointments, err := s.repo.GetByDateRange(ctx, clinicID, start, end, professionalID)
	if err != nil {
		return nil, err
	}

	data := make([]*model.AppointmentResponse, len(appointments))
	for i, a := range appointments {
		data[i] = a.ToResponse()
	}

	return data, nil
}

// Update updates an appointment
func (s *appointmentService) Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateAppointmentRequest) (*model.AppointmentResponse, error) {
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	// Calculate new times if provided
	newStart := appointment.StartTime
	newEnd := appointment.EndTime

	if req.StartTime != nil {
		newStart = *req.StartTime
		if req.DurationMinutes != nil {
			newEnd = newStart.Add(time.Duration(*req.DurationMinutes) * time.Minute)
		} else {
			// Keep same duration
			duration := appointment.EndTime.Sub(appointment.StartTime)
			newEnd = newStart.Add(duration)
		}
	} else if req.DurationMinutes != nil {
		newEnd = newStart.Add(time.Duration(*req.DurationMinutes) * time.Minute)
	}

	professionalID := appointment.ProfessionalID
	if req.ProfessionalID != nil {
		professionalID = *req.ProfessionalID
	}

	// Check for conflicts if time or professional changed
	if newStart != appointment.StartTime || newEnd != appointment.EndTime || professionalID != appointment.ProfessionalID {
		hasConflict, err := s.repo.CheckConflict(ctx, clinicID, professionalID, newStart, newEnd, &id)
		if err != nil {
			return nil, err
		}
		if hasConflict {
			return nil, errors.ErrSlotUnavailableInstance
		}
	}

	// Apply updates
	appointment.StartTime = newStart
	appointment.EndTime = newEnd
	appointment.ProfessionalID = professionalID

	if req.PatientID != nil {
		appointment.PatientID = *req.PatientID
	}
	if req.Status != "" {
		appointment.Status = req.Status
	}
	if req.Title != "" {
		appointment.Title = req.Title
	}
	if req.Notes != "" {
		appointment.Notes = req.Notes
	}
	if req.PatientNotes != "" {
		appointment.PatientNotes = req.PatientNotes
	}
	if req.Procedures != nil {
		var appointmentProcedures []model.AppointmentProcedure
		var calculatedProceduresPriceCents int64
		for _, p := range *req.Procedures {
			appointmentProcedures = append(appointmentProcedures, model.AppointmentProcedure{
				AppointmentID: appointment.ID,
				ProcedureID:   p.ProcedureID,
				Price:         *p.Price,
			})
			calculatedProceduresPriceCents += int64(math.Round(*p.Price * 100))
		}

		// Optional: if repo.Update handles replacing associations, we can set it:
		appointment.Procedures = appointmentProcedures

		// Note: since GORM Save handles associations if configured properly, we check if price was overridden:
		if req.Price == nil {
			appointment.Price = float64(calculatedProceduresPriceCents) / 100.0
		}
	}

	if req.SpecialtyIDs != nil {
		var appointmentSpecialties []model.AppointmentSpecialty
		if len(*req.SpecialtyIDs) > 0 {
			prof, err := s.profRepo.GetByID(ctx, professionalID, clinicID)
			if err != nil {
				return nil, err
			}

			profSpecs := make(map[uuid.UUID]bool)
			for _, ps := range prof.Specialties {
				if ps.IsActive {
					profSpecs[ps.SpecialtyID] = true
				}
			}

			for _, sid := range *req.SpecialtyIDs {
				if !profSpecs[sid] {
					return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Esse profissional não possui a especialidade %s", sid))
				}
				appointmentSpecialties = append(appointmentSpecialties, model.AppointmentSpecialty{
					AppointmentID: appointment.ID,
					ClinicID:      clinicID,
					SpecialtyID:   sid,
				})
			}
		}
		appointment.Specialties = appointmentSpecialties
	}

	if req.Price != nil {
		appointment.Price = *req.Price
	}
	if req.CancelReason != "" {
		appointment.CancelReason = req.CancelReason
	}

	if err := s.repo.Update(ctx, appointment); err != nil {
		return nil, err
	}

	// Update in Google Calendar (async)
	s.updateGoogleCalendar(clinicID, appointment)

	return s.GetByID(ctx, id, clinicID)
}

// Delete deletes an appointment. Linked appointment_procedures are always
// removed in cascade because they are historical join rows tied to this
// specific appointment. appointment_payments are kept by default — they are
// part of the financial history — and are only removed when the caller
// explicitly opts in via deletePayments.
//
// Session-plan appointments are NOT hard-deleted: they are converted into a
// suppression tombstone (cancelled exception + deleted_at set) so the
// CalendarFacade keeps hiding the virtual occurrence at that slot. A hard
// delete would erase the record entirely and the RRULE would re-materialize
// the same date on the next calendar fetch — making the appointment appear to
// "come back" after the user asked to delete it.
func (s *appointmentService) Delete(ctx context.Context, id, clinicID uuid.UUID, deletePayments bool) error {
	apt, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return err
	}

	// Snapshot the payment IDs before the cascade runs so we can drop the
	// matching finance rows afterwards. When deletePayments=false the
	// repository keeps the payment rows intact as financial history and the
	// finance side must stay untouched — so we only collect IDs when the
	// caller opted to remove the payments too.
	var paymentIDs []uuid.UUID
	if deletePayments && s.paymentLister != nil {
		payments, listErr := s.paymentLister.ListByAppointment(id, clinicID)
		if listErr == nil {
			paymentIDs = make([]uuid.UUID, 0, len(payments))
			for _, p := range payments {
				paymentIDs = append(paymentIDs, p.ID)
			}
		} else {
			logger.Printf("[Appointment Delete] list payments failed for %s: %v", id, listErr)
		}
	}

	if apt.SessionPlanID != nil {
		if err := s.repo.SoftDeleteAsCanceledException(ctx, id, clinicID, deletePayments); err != nil {
			return err
		}
	} else {
		if err := s.repo.DeleteCascade(ctx, id, clinicID, deletePayments); err != nil {
			return err
		}
	}

	// Clean up the finance rows that mirror the deleted payments. Best-effort:
	// a failure here does not undo the appointment delete — the payment row
	// is already gone, so we only log and move on.
	if s.financeUpserter != nil {
		for _, pid := range paymentIDs {
			if err := s.financeUpserter.DeleteFromAppointmentPayment(ctx, pid, clinicID); err != nil {
				logger.Printf("[Appointment Delete] finance cleanup failed for payment %s: %v", pid, err)
			}
		}
	}
	return nil
}

// GetDeletePreview returns counts the UI uses to warn the user before
// deleting an appointment (how many procedures cascade, and how many
// payments exist and their total value).
func (s *appointmentService) GetDeletePreview(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentDeletePreview, error) {
	return s.repo.GetDeletePreview(ctx, id, clinicID)
}

// Confirm confirms an appointment
func (s *appointmentService) Confirm(ctx context.Context, id, clinicID uuid.UUID, req *model.ConfirmAppointmentRequest) (*model.AppointmentResponse, error) {
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	// Reactivation guard: if the appointment was previously cancelled, the slot
	// may have been taken by another patient. Check before allowing reactivation.
	if appointment.Status == model.AppointmentStatusCanceled {
		hasConflict, err := s.repo.CheckConflict(ctx, clinicID, appointment.ProfessionalID,
			appointment.StartTime, appointment.EndTime, &id)
		if err != nil {
			return nil, err
		}
		if hasConflict {
			return nil, errors.ErrSlotUnavailableInstance
		}
	}

	now := time.Now()
	appointment.Status = model.AppointmentStatusConfirmed
	appointment.ConfirmedAt = &now
	if req != nil && req.PatientNotes != "" {
		appointment.PatientNotes = req.PatientNotes
	}

	if err := s.repo.Update(ctx, appointment); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id, clinicID)
}

// Cancel cancels an appointment
func (s *appointmentService) Cancel(ctx context.Context, id, clinicID uuid.UUID, req *model.CancelAppointmentRequest) (*model.AppointmentResponse, error) {
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	// Store appointment info before cancellation for waitlist processing
	slotTime := appointment.StartTime
	professionalID := appointment.ProfessionalID
	durationMinutes := appointment.DurationMinutes()

	now := time.Now()
	appointment.Status = model.AppointmentStatusCanceled
	appointment.CanceledAt = &now
	if req != nil && req.Reason != "" {
		appointment.CancelReason = req.Reason
	}

	if err := s.repo.Update(ctx, appointment); err != nil {
		return nil, err
	}

	// Process waitlist (encaixe) in background if the slot is in the future
	if s.waitlistService != nil && slotTime.After(time.Now()) {
		go func() {
			bgCtx := context.Background()
			if err := s.waitlistService.ProcessCancelledAppointment(bgCtx, clinicID, professionalID, slotTime, durationMinutes); err != nil {
				logger.Printf("[Appointment] Error processing waitlist for cancelled appointment: %v", err)
			}
		}()
	}

	// Void any commission if appointment was previously completed
	if s.commissionCalculator != nil {
		_ = s.commissionCalculator.VoidByAppointment(ctx, id, clinicID)
	}

	// Delete from Google Calendar (async)
	s.deleteFromGoogleCalendar(clinicID, id)

	return s.GetByID(ctx, id, clinicID)
}

// Complete marks an appointment as completed
func (s *appointmentService) Complete(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error) {
	// Get appointment to retrieve patient ID for risk recalculation
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateStatus(ctx, id, clinicID, model.AppointmentStatusCompleted); err != nil {
		return nil, err
	}

	// NOTE: sessions_done increment is NOT done here — it's handled by the
	// attendance flow (AutoRecordPresent in the handler). Doing it here too
	// would cause double-counting.

	// Calculate and store commission for completed appointment
	if s.commissionCalculator != nil {
		if err := s.commissionCalculator.CalculateAndStore(ctx, appointment, clinicID); err != nil {
			logger.Printf("[Appointment] Error calculating commission for appointment %s: %v", id, err)
			// Don't fail the completion — commission can be recalculated later
		}
	}

	// Recalculate patient risk score in background
	if s.riskCalculator != nil {
		go func() {
			bgCtx := context.Background()
			if err := s.riskCalculator.CalculateRiskScore(bgCtx, appointment.PatientID, clinicID); err != nil {
				logger.Printf("[Appointment] Error recalculating patient risk after completion: %v", err)
			}
		}()
	}

	return s.GetByID(ctx, id, clinicID)
}

// MarkNoShow marks an appointment as no-show
func (s *appointmentService) MarkNoShow(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error) {
	// Get appointment to retrieve patient ID for risk recalculation
	appointment, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateStatus(ctx, id, clinicID, model.AppointmentStatusNoShow); err != nil {
		return nil, err
	}

	// Void any commission if appointment was previously completed
	if s.commissionCalculator != nil {
		_ = s.commissionCalculator.VoidByAppointment(ctx, id, clinicID)
	}

	// Recalculate patient risk score in background
	if s.riskCalculator != nil {
		go func() {
			bgCtx := context.Background()
			if err := s.riskCalculator.CalculateRiskScore(bgCtx, appointment.PatientID, clinicID); err != nil {
				logger.Printf("[Appointment] Error recalculating patient risk after no-show: %v", err)
			}
		}()
	}

	return s.GetByID(ctx, id, clinicID)
}

// CreateFromWaitlist creates an appointment from a waitlist entry
func (s *appointmentService) CreateFromWaitlist(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) (uuid.UUID, error) {
	endTime := startTime.Add(time.Duration(durationMinutes) * time.Minute)

	// Check for conflicts (slot might have been taken)
	hasConflict, err := s.repo.CheckConflict(ctx, clinicID, professionalID, startTime, endTime, nil)
	if err != nil {
		return uuid.Nil, err
	}
	if hasConflict {
		return uuid.Nil, errors.ErrSlotUnavailableInstance
	}

	appointment := &model.Appointment{
		ClinicID:       clinicID,
		PatientID:      patientID,
		ProfessionalID: professionalID,
		StartTime:      startTime,
		EndTime:        endTime,
		Status:         model.AppointmentStatusConfirmed, // Auto-confirm since patient said SIM
		Type:           model.AppointmentTypeRegular,
		Title:          "Encaixe",
		Notes:          "Agendamento criado via lista de espera",
	}

	// Set confirmed timestamp
	now := time.Now()
	appointment.ConfirmedAt = &now

	if err := s.repo.Create(ctx, appointment); err != nil {
		return uuid.Nil, err
	}

	return appointment.ID, nil
}
