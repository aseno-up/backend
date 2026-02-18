package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ExecutionContext holds the context needed to execute tools
type ExecutionContext struct {
	Context        context.Context
	ClinicID       uuid.UUID
	PatientPhone   string
	PatientID      *uuid.UUID
	PatientName    string
	ConversationID uuid.UUID

	// Service dependencies (injected from WhatsApp service)
	ProfessionalLister       ProfessionalLister
	SlotFinder               SlotFinder
	AppointmentCreator       AppointmentCreator
	AppointmentFinder        AppointmentFinder
	AppointmentConfirmer     AppointmentConfirmer
	AppointmentCanceller     AppointmentCanceller
	AppointmentRescheduler   AppointmentRescheduler
	ClinicInfoProvider       ClinicInfoProvider
	ProfessionalInfoProvider ProfessionalInfoProvider
	PatientInfoProvider      PatientInfoProvider
	PatientFinderCreator     PatientFinderCreator
}

// Service interfaces that tools will call

type ProfessionalLister interface {
	ListAvailable(ctx context.Context, clinicID uuid.UUID, specialty string) ([]*ProfessionalInfo, error)
}

type ProfessionalInfo struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Specialty   string    `json:"specialty"`
	IsAvailable bool      `json:"is_available"`
}

type SlotFinder interface {
	GetAvailableSlots(ctx context.Context, clinicID uuid.UUID, professionalID uuid.UUID, date time.Time, durationMinutes int) ([]*AvailableSlot, error)
	GetAvailableSlotsMultipleProfessionals(ctx context.Context, clinicID uuid.UUID, professionalIDs []uuid.UUID, date time.Time, days int, durationMinutes int, limit int) ([]*AvailableSlot, error)
}

type AvailableSlot struct {
	ProfessionalID   uuid.UUID `json:"professional_id"`
	ProfessionalName string    `json:"professional_name"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
}

type AppointmentCreator interface {
	CreateFromSchedulingIntent(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) (*AppointmentInfo, error)
}

type AppointmentInfo struct {
	ID             uuid.UUID `json:"id"`
	PatientID      uuid.UUID `json:"patient_id"`
	ProfessionalID uuid.UUID `json:"professional_id"`
	PatientName    string    `json:"patient_name"`
	PatientPhone   string    `json:"patient_phone"`
	Professional   string    `json:"professional"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	Status         string    `json:"status"`
}

type AppointmentFinder interface {
	GetByPatientPhone(ctx context.Context, clinicID uuid.UUID, phone string, status string, limit int) ([]*AppointmentInfo, error)
	GetByID(ctx context.Context, appointmentID, clinicID uuid.UUID) (*AppointmentInfo, error)
}

type AppointmentConfirmer interface {
	ConfirmByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, patientNotes string) error
}

type AppointmentCanceller interface {
	CancelByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, reason string) error
}

type AppointmentRescheduler interface {
	Reschedule(ctx context.Context, appointmentID, clinicID uuid.UUID, newStartTime time.Time, newProfessionalID *uuid.UUID) error
}

type ClinicInfoProvider interface {
	GetClinicInfo(ctx context.Context, clinicID uuid.UUID) (*ClinicInfo, error)
}

type ClinicInfo struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	Phone         string `json:"phone"`
	BusinessHours string `json:"business_hours"`
}

type ProfessionalInfoProvider interface {
	GetProfessionalInfo(ctx context.Context, professionalID uuid.UUID) (*ProfessionalDetailedInfo, error)
}

type ProfessionalDetailedInfo struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Specialty   string    `json:"specialty"`
	Description string    `json:"description"`
	IsAvailable bool      `json:"is_available"`
}

type PatientInfoProvider interface {
	GetPatientInfo(ctx context.Context, patientID uuid.UUID) (*PatientInfo, error)
}

type PatientInfo struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Phone string    `json:"phone"`
	Email string    `json:"email,omitempty"`
}

type PatientFinderCreator interface {
	FindOrCreateByPhone(ctx context.Context, clinicID uuid.UUID, phone, name string) (*PatientInfo, error)
}

// Executor executes tool calls using the provided context
type Executor struct {
	registry *Registry
}

// NewExecutor creates a new tool executor
func NewExecutor(registry *Registry) *Executor {
	return &Executor{
		registry: registry,
	}
}

// Execute executes a tool by name with the given parameters
func (e *Executor) Execute(execCtx *ExecutionContext, toolName string, params map[string]interface{}) (interface{}, error) {
	tool, exists := e.registry.GetTool(toolName)
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// Route to appropriate handler
	switch toolName {
	case "get_available_professionals":
		return e.getAvailableProfessionals(execCtx, params)
	case "get_available_slots":
		return e.getAvailableSlots(execCtx, params)
	case "create_appointment":
		return e.createAppointment(execCtx, params)
	case "confirm_appointment":
		return e.confirmAppointment(execCtx, params)
	case "cancel_appointment":
		return e.cancelAppointment(execCtx, params)
	case "reschedule_appointment":
		return e.rescheduleAppointment(execCtx, params)
	case "get_patient_appointments":
		return e.getPatientAppointments(execCtx, params)
	case "search_appointment_history":
		return e.searchAppointmentHistory(execCtx, params)
	case "get_clinic_info":
		return e.getClinicInfo(execCtx, params)
	case "get_professional_info":
		return e.getProfessionalInfo(execCtx, params)
	case "get_patient_info":
		return e.getPatientInfo(execCtx, params)
	default:
		// Fallback to generic handler if exists
		if tool.Handler != nil {
			return tool.Handler(params)
		}
		return nil, fmt.Errorf("no handler for tool: %s", toolName)
	}
}

// Tool implementation methods

func (e *Executor) getAvailableProfessionals(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	specialty := ""
	if s, ok := params["specialty"].(string); ok {
		specialty = s
	}

	if execCtx.ProfessionalLister == nil {
		return nil, fmt.Errorf("ProfessionalLister not configured")
	}

	professionals, err := execCtx.ProfessionalLister.ListAvailable(execCtx.Context, execCtx.ClinicID, specialty)
	if err != nil {
		return nil, fmt.Errorf("failed to list professionals: %w", err)
	}

	return professionals, nil
}

func (e *Executor) getAvailableSlots(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	// Default to 60 minutes if not specified (standard consultation duration)
	durationMinutes := 60.0
	if d, ok := params["duration_minutes"].(float64); ok {
		durationMinutes = d
	}

	// Parse optional date (default to today)
	date := time.Now()
	if dateStr, ok := params["date"].(string); ok && dateStr != "" && dateStr != "null" && dateStr != "undefined" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid date format, use YYYY-MM-DD: %w", err)
		}
		date = parsedDate
	}

	// Parse optional days_ahead (default 7)
	daysAhead := 7
	if d, ok := params["days_ahead"].(float64); ok {
		daysAhead = int(d)
	}

	if execCtx.SlotFinder == nil {
		return nil, fmt.Errorf("SlotFinder not configured")
	}

	// If professional_id is provided, search for that professional only
	if profIDStr, ok := params["professional_id"].(string); ok && profIDStr != "" {
		profID, err := uuid.Parse(profIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid professional_id: %w", err)
		}

		// Search across multiple days to find available slots
		// This ensures we always return slots even if today has none (e.g., weekends)
		var allSlots []*AvailableSlot
		for day := 0; day < daysAhead; day++ {
			searchDate := date.AddDate(0, 0, day)

			daySlots, err := execCtx.SlotFinder.GetAvailableSlots(execCtx.Context, execCtx.ClinicID, profID, searchDate, int(durationMinutes))
			if err != nil {
				return nil, fmt.Errorf("failed to get slots for day %d: %w", day, err)
			}

			allSlots = append(allSlots, daySlots...)

			// Stop searching if we have enough slots
			if len(allSlots) >= 5 {
				break
			}
		}

		// Limit to 5 slots
		if len(allSlots) > 5 {
			allSlots = allSlots[:5]
		}

		return allSlots, nil
	}

	// Otherwise, search across all professionals
	slots, err := execCtx.SlotFinder.GetAvailableSlotsMultipleProfessionals(
		execCtx.Context,
		execCtx.ClinicID,
		nil, // nil means all professionals
		date,
		daysAhead,
		int(durationMinutes),
		5, // Limit to 5 slots
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get slots: %w", err)
	}

	return slots, nil
}

func (e *Executor) createAppointment(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	profIDStr, ok := params["professional_id"].(string)
	if !ok {
		return nil, fmt.Errorf("professional_id is required")
	}

	profID, err := uuid.Parse(profIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid professional_id: %w", err)
	}

	startTimeStr, ok := params["start_time"].(string)
	if !ok {
		return nil, fmt.Errorf("start_time is required")
	}

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start_time format, use ISO 8601: %w", err)
	}

	// Default to 60 minutes if not specified (standard consultation duration)
	durationMinutes := 60.0
	if d, ok := params["duration_minutes"].(float64); ok {
		durationMinutes = d
	}

	// Ensure patient exists
	if execCtx.PatientID == nil {
		if execCtx.PatientFinderCreator == nil {
			return nil, fmt.Errorf("PatientFinderCreator not configured")
		}

		patient, err := execCtx.PatientFinderCreator.FindOrCreateByPhone(
			execCtx.Context,
			execCtx.ClinicID,
			execCtx.PatientPhone,
			execCtx.PatientName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to find/create patient: %w", err)
		}

		execCtx.PatientID = &patient.ID
	}

	if execCtx.AppointmentCreator == nil {
		return nil, fmt.Errorf("AppointmentCreator not configured")
	}

	// Create appointment
	appointment, err := execCtx.AppointmentCreator.CreateFromSchedulingIntent(
		execCtx.Context,
		execCtx.ClinicID,
		*execCtx.PatientID,
		profID,
		startTime,
		int(durationMinutes),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create appointment: %w", err)
	}

	return appointment, nil
}

func (e *Executor) confirmAppointment(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	var appointmentID uuid.UUID

	// If appointment_id is provided, use it
	if aptIDStr, ok := params["appointment_id"].(string); ok && aptIDStr != "" {
		aptID, err := uuid.Parse(aptIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid appointment_id: %w", err)
		}
		appointmentID = aptID
	} else {
		// Auto-detect: find pending appointment
		if execCtx.AppointmentFinder == nil {
			return nil, fmt.Errorf("AppointmentFinder not configured")
		}

		appointments, err := execCtx.AppointmentFinder.GetByPatientPhone(
			execCtx.Context,
			execCtx.ClinicID,
			execCtx.PatientPhone,
			"pending",
			1,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to find pending appointments: %w", err)
		}

		if len(appointments) == 0 {
			return map[string]interface{}{
				"success": false,
				"message": "Nenhum agendamento pendente encontrado",
			}, nil
		}

		if len(appointments) > 1 {
			return map[string]interface{}{
				"success":      false,
				"message":      "Múltiplos agendamentos pendentes. Por favor, especifique qual deseja confirmar.",
				"appointments": appointments,
			}, nil
		}

		appointmentID = appointments[0].ID
	}

	if execCtx.AppointmentConfirmer == nil {
		return nil, fmt.Errorf("AppointmentConfirmer not configured")
	}

	// Confirm appointment
	if err := execCtx.AppointmentConfirmer.ConfirmByPatient(execCtx.Context, appointmentID, execCtx.ClinicID, ""); err != nil {
		return nil, fmt.Errorf("failed to confirm appointment: %w", err)
	}

	return map[string]interface{}{
		"success":        true,
		"appointment_id": appointmentID.String(),
		"message":        "Agendamento confirmado com sucesso",
	}, nil
}

func (e *Executor) cancelAppointment(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	var appointmentID uuid.UUID

	// If appointment_id is provided, use it
	if aptIDStr, ok := params["appointment_id"].(string); ok && aptIDStr != "" {
		aptID, err := uuid.Parse(aptIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid appointment_id: %w", err)
		}
		appointmentID = aptID
	} else {
		// Auto-detect: find future appointment
		if execCtx.AppointmentFinder == nil {
			return nil, fmt.Errorf("AppointmentFinder not configured")
		}

		appointments, err := execCtx.AppointmentFinder.GetByPatientPhone(
			execCtx.Context,
			execCtx.ClinicID,
			execCtx.PatientPhone,
			"", // All statuses
			10,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to find appointments: %w", err)
		}

		// Filter to future appointments only
		var futureApts []*AppointmentInfo
		now := time.Now()
		for _, apt := range appointments {
			if apt.StartTime.After(now) && apt.Status != "cancelled" {
				futureApts = append(futureApts, apt)
			}
		}

		if len(futureApts) == 0 {
			return map[string]interface{}{
				"success": false,
				"message": "Nenhum agendamento futuro encontrado",
			}, nil
		}

		if len(futureApts) > 1 {
			return map[string]interface{}{
				"success":      false,
				"message":      "Múltiplos agendamentos futuros. Por favor, especifique qual deseja cancelar.",
				"appointments": futureApts,
			}, nil
		}

		appointmentID = futureApts[0].ID
	}

	reason := ""
	if r, ok := params["reason"].(string); ok {
		reason = r
	}

	if execCtx.AppointmentCanceller == nil {
		return nil, fmt.Errorf("AppointmentCanceller not configured")
	}

	// Cancel appointment
	if err := execCtx.AppointmentCanceller.CancelByPatient(execCtx.Context, appointmentID, execCtx.ClinicID, reason); err != nil {
		return nil, fmt.Errorf("failed to cancel appointment: %w", err)
	}

	return map[string]interface{}{
		"success":        true,
		"appointment_id": appointmentID.String(),
		"message":        "Agendamento cancelado com sucesso",
	}, nil
}

func (e *Executor) rescheduleAppointment(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	aptIDStr, ok := params["appointment_id"].(string)
	if !ok {
		return nil, fmt.Errorf("appointment_id is required")
	}

	aptID, err := uuid.Parse(aptIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid appointment_id: %w", err)
	}

	newStartTimeStr, ok := params["new_start_time"].(string)
	if !ok {
		return nil, fmt.Errorf("new_start_time is required")
	}

	newStartTime, err := time.Parse(time.RFC3339, newStartTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid new_start_time format, use ISO 8601: %w", err)
	}

	var newProfessionalID *uuid.UUID
	if profIDStr, ok := params["new_professional_id"].(string); ok && profIDStr != "" {
		profID, err := uuid.Parse(profIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid new_professional_id: %w", err)
		}
		newProfessionalID = &profID
	}

	if execCtx.AppointmentRescheduler == nil {
		return nil, fmt.Errorf("AppointmentRescheduler not configured")
	}

	// Reschedule appointment
	if err := execCtx.AppointmentRescheduler.Reschedule(execCtx.Context, aptID, execCtx.ClinicID, newStartTime, newProfessionalID); err != nil {
		return nil, fmt.Errorf("failed to reschedule appointment: %w", err)
	}

	return map[string]interface{}{
		"success":        true,
		"appointment_id": aptID.String(),
		"message":        "Agendamento remarcado com sucesso",
	}, nil
}

func (e *Executor) getPatientAppointments(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	status := ""
	if s, ok := params["status"].(string); ok {
		status = s
	}

	limit := 10
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	if execCtx.AppointmentFinder == nil {
		return nil, fmt.Errorf("AppointmentFinder not configured")
	}

	appointments, err := execCtx.AppointmentFinder.GetByPatientPhone(
		execCtx.Context,
		execCtx.ClinicID,
		execCtx.PatientPhone,
		status,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get patient appointments: %w", err)
	}

	return appointments, nil
}

func (e *Executor) searchAppointmentHistory(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	// Parse period parameter
	periodStr, ok := params["period"].(string)
	if !ok {
		return nil, fmt.Errorf("period is required")
	}

	// Parse the period string and convert to date range
	startDate, endDate, err := parsePeriod(periodStr)
	if err != nil {
		return nil, fmt.Errorf("invalid period format: %w", err)
	}

	// Parse optional filters
	status := ""
	if s, ok := params["status"].(string); ok {
		status = s
	}

	var professionalID *uuid.UUID
	if profIDStr, ok := params["professional_id"].(string); ok && profIDStr != "" {
		profID, err := uuid.Parse(profIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid professional_id: %w", err)
		}
		professionalID = &profID
	}

	limit := 20
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	if execCtx.AppointmentFinder == nil {
		return nil, fmt.Errorf("AppointmentFinder not configured")
	}

	// Get all appointments for the patient
	appointments, err := execCtx.AppointmentFinder.GetByPatientPhone(
		execCtx.Context,
		execCtx.ClinicID,
		execCtx.PatientPhone,
		status,
		limit*2, // Get more to account for filtering
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search appointment history: %w", err)
	}

	// Filter by date range and optional professional
	var filtered []*AppointmentInfo
	for _, apt := range appointments {
		// Check if appointment is within date range
		if apt.StartTime.Before(startDate) || apt.StartTime.After(endDate) {
			continue
		}

		// Check professional filter if specified
		if professionalID != nil && apt.ProfessionalID != *professionalID {
			continue
		}

		filtered = append(filtered, apt)

		// Apply limit
		if len(filtered) >= limit {
			break
		}
	}

	return map[string]interface{}{
		"appointments": filtered,
		"period":       periodStr,
		"start_date":   startDate.Format("2006-01-02"),
		"end_date":     endDate.Format("2006-01-02"),
		"count":        len(filtered),
	}, nil
}

// parsePeriod converts a period string to start and end dates
func parsePeriod(period string) (startDate time.Time, endDate time.Time, err error) {
	now := time.Now()
	endDate = now

	switch period {
	case "last_week":
		startDate = now.AddDate(0, 0, -7)
	case "last_month":
		startDate = now.AddDate(0, -1, 0)
	case "last_3_months":
		startDate = now.AddDate(0, -3, 0)
	case "last_6_months":
		startDate = now.AddDate(0, -6, 0)
	case "last_year":
		startDate = now.AddDate(-1, 0, 0)
	case "year_2024":
		startDate = time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local)
		endDate = time.Date(2024, 12, 31, 23, 59, 59, 0, time.Local)
	case "year_2023":
		startDate = time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local)
		endDate = time.Date(2023, 12, 31, 23, 59, 59, 0, time.Local)
	case "year_2022":
		startDate = time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)
		endDate = time.Date(2022, 12, 31, 23, 59, 59, 0, time.Local)
	default:
		// Try to parse as specific date (YYYY-MM-DD)
		if specificDate, err := time.Parse("2006-01-02", period); err == nil {
			startDate = specificDate
			endDate = specificDate.AddDate(0, 0, 1) // Next day
			return startDate, endDate, nil
		}
		return time.Time{}, time.Time{}, fmt.Errorf("unknown period format: %s (use: last_week, last_month, year_2024, or YYYY-MM-DD)", period)
	}

	return startDate, endDate, nil
}

func (e *Executor) getClinicInfo(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	if execCtx.ClinicInfoProvider == nil {
		return nil, fmt.Errorf("ClinicInfoProvider not configured")
	}

	info, err := execCtx.ClinicInfoProvider.GetClinicInfo(execCtx.Context, execCtx.ClinicID)
	if err != nil {
		return nil, fmt.Errorf("failed to get clinic info: %w", err)
	}

	return info, nil
}

func (e *Executor) getProfessionalInfo(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	profIDStr, ok := params["professional_id"].(string)
	if !ok {
		return nil, fmt.Errorf("professional_id is required")
	}

	profID, err := uuid.Parse(profIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid professional_id: %w", err)
	}

	if execCtx.ProfessionalInfoProvider == nil {
		return nil, fmt.Errorf("ProfessionalInfoProvider not configured")
	}

	info, err := execCtx.ProfessionalInfoProvider.GetProfessionalInfo(execCtx.Context, profID)
	if err != nil {
		return nil, fmt.Errorf("failed to get professional info: %w", err)
	}

	return info, nil
}

func (e *Executor) getPatientInfo(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	if execCtx.PatientID == nil {
		// Try to find/create patient first
		if execCtx.PatientFinderCreator == nil {
			return nil, fmt.Errorf("PatientFinderCreator not configured")
		}

		patient, err := execCtx.PatientFinderCreator.FindOrCreateByPhone(
			execCtx.Context,
			execCtx.ClinicID,
			execCtx.PatientPhone,
			execCtx.PatientName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to find/create patient: %w", err)
		}

		return patient, nil
	}

	if execCtx.PatientInfoProvider == nil {
		return nil, fmt.Errorf("PatientInfoProvider not configured")
	}

	info, err := execCtx.PatientInfoProvider.GetPatientInfo(execCtx.Context, *execCtx.PatientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get patient info: %w", err)
	}

	return info, nil
}
