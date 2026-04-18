package tools

import (
	"context"
	"fmt"
	"strings"
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
	ProcedureLister          ProcedureLister
	SpecialtyLister          SpecialtyLister
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
	PatientNameUpdater       PatientNameUpdater
}

// Service interfaces that tools will call

type ProfessionalLister interface {
	ListAvailable(ctx context.Context, clinicID uuid.UUID, specialty string) ([]*ProfessionalInfo, error)
}

type ProfessionalInfo struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	Specialty    string         `json:"specialty,omitempty"` // legacy single-value fallback, populated with the first specialty name when available
	Specialties  []SpecialtyRef `json:"specialties,omitempty"`
	WorkingHours *WorkingHours  `json:"working_hours,omitempty"`
	IsAvailable  bool           `json:"is_available"`
}

type SpecialtyRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// WorkingHours mirrors the professional model's weekly recurring schedule so
// the AI can answer questions like "when does Dr. X usually work?" without an
// extra tool call.
type WorkingHours struct {
	Monday    []TimeSlot `json:"monday,omitempty"`
	Tuesday   []TimeSlot `json:"tuesday,omitempty"`
	Wednesday []TimeSlot `json:"wednesday,omitempty"`
	Thursday  []TimeSlot `json:"thursday,omitempty"`
	Friday    []TimeSlot `json:"friday,omitempty"`
	Saturday  []TimeSlot `json:"saturday,omitempty"`
	Sunday    []TimeSlot `json:"sunday,omitempty"`
}

type TimeSlot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type ProcedureLister interface {
	ListProcedures(ctx context.Context, clinicID uuid.UUID, specialty string) ([]*ProcedureInfo, error)
}

type SpecialtyLister interface {
	ListSpecialties(ctx context.Context, clinicID uuid.UUID) ([]*SpecialtyRef, error)
}

type ProcedureInfo struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Price           float64   `json:"price"`
	DurationMinutes int       `json:"duration_minutes"`
	SpecialtyName   string    `json:"specialty_name,omitempty"`
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

type PatientNameUpdater interface {
	UpdateName(ctx context.Context, clinicID, patientID uuid.UUID, newName string) error
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
	case "list_procedures":
		return e.listProcedures(execCtx, params)
	case "list_specialties":
		return e.listSpecialties(execCtx, params)
	case "get_available_slots":
		return e.getAvailableSlots(execCtx, params)
	case "create_appointment":
		return e.createAppointment(execCtx, params)
	case "quick_schedule":
		return e.quickSchedule(execCtx, params)
	case "quick_reschedule":
		return e.quickReschedule(execCtx, params)
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
	case "update_patient_name":
		return e.updatePatientName(execCtx, params)
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

func (e *Executor) listSpecialties(execCtx *ExecutionContext, _ map[string]interface{}) (interface{}, error) {
	if execCtx.SpecialtyLister == nil {
		return nil, fmt.Errorf("SpecialtyLister not configured")
	}
	specs, err := execCtx.SpecialtyLister.ListSpecialties(execCtx.Context, execCtx.ClinicID)
	if err != nil {
		return nil, fmt.Errorf("failed to list specialties: %w", err)
	}
	return specs, nil
}

func (e *Executor) listProcedures(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	specialty := ""
	if s, ok := params["specialty"].(string); ok {
		specialty = s
	}

	if execCtx.ProcedureLister == nil {
		return nil, fmt.Errorf("ProcedureLister not configured")
	}

	procedures, err := execCtx.ProcedureLister.ListProcedures(execCtx.Context, execCtx.ClinicID, specialty)
	if err != nil {
		return nil, fmt.Errorf("failed to list procedures: %w", err)
	}

	return procedures, nil
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

	// Accept either professional_id (UUID) OR professional_name (fragment).
	// Cheaper LLMs frequently hallucinate UUIDs from examples/memory, so the
	// name fallback saves the flow — if the LLM passed a bad UUID, we can
	// still resolve it when the patient's original message said a name.
	var profIDStr, profNameFrag string
	if v, ok := params["professional_id"].(string); ok {
		profIDStr = v
	}
	if v, ok := params["professional_name"].(string); ok {
		profNameFrag = v
	}

	if profIDStr != "" || profNameFrag != "" {
		var profID uuid.UUID
		var resolvedByName bool

		if profIDStr != "" {
			if parsed, err := uuid.Parse(profIDStr); err == nil {
				profID = parsed
			} else if profNameFrag == "" {
				return nil, fmt.Errorf("invalid professional_id %q: must be a UUID from get_available_professionals, or pass professional_name instead", profIDStr)
			}
		}

		// Validate/resolve against the real list of professionals.
		if execCtx.ProfessionalLister != nil {
			profs, listErr := execCtx.ProfessionalLister.ListAvailable(execCtx.Context, execCtx.ClinicID, "")
			if listErr == nil {
				valid := make([]string, 0, len(profs))
				var foundByID *ProfessionalInfo
				var nameMatches []*ProfessionalInfo
				frag := strings.ToLower(strings.TrimSpace(profNameFrag))
				for _, p := range profs {
					valid = append(valid, fmt.Sprintf("%s (%s)", p.ID, p.Name))
					if profID != uuid.Nil && p.ID == profID {
						foundByID = p
					}
					if frag != "" && strings.Contains(strings.ToLower(p.Name), frag) {
						nameMatches = append(nameMatches, p)
					}
				}

				switch {
				case foundByID != nil:
					profID = foundByID.ID
				case len(nameMatches) == 1:
					profID = nameMatches[0].ID
					resolvedByName = true
				case len(nameMatches) > 1:
					cands := make([]string, len(nameMatches))
					for i, m := range nameMatches {
						cands[i] = fmt.Sprintf("%s (%s)", m.ID, m.Name)
					}
					return nil, fmt.Errorf("multiple professionals match %q: [%s] — ask the patient which one and pass the exact UUID", profNameFrag, strings.Join(cands, ", "))
				default:
					return nil, fmt.Errorf(
						"professional not found — neither id %q nor name %q resolved. Use one of: [%s]",
						profIDStr, profNameFrag, strings.Join(valid, ", "),
					)
				}
				_ = resolvedByName // currently informational; could be surfaced later
			}
		}

		// Require a resolved UUID at this point; if the lister wasn't wired
		// and the caller didn't provide a valid UUID, bail out cleanly.
		if profID == uuid.Nil {
			return nil, fmt.Errorf("could not resolve professional from id=%q or name=%q", profIDStr, profNameFrag)
		}

		// Collect slots from the next `daysAhead` days. A prior version
		// stopped at 5 slots total, which meant a professional with many
		// morning openings never surfaced afternoon/evening availability —
		// e.g., a 9–18h schedule would only return 09:00–13:00 and the LLM
		// never saw 14:00+. We now walk full days and sample across the day
		// so the patient gets a realistic view of the professional's agenda.
		maxSlots := 15
		var allSlots []*AvailableSlot
		for day := 0; day < daysAhead; day++ {
			searchDate := date.AddDate(0, 0, day)

			daySlots, err := execCtx.SlotFinder.GetAvailableSlots(execCtx.Context, execCtx.ClinicID, profID, searchDate, int(durationMinutes))
			if err != nil {
				return nil, fmt.Errorf("failed to get slots for day %d: %w", day, err)
			}

			// Sample slots spread across the day (morning + afternoon)
			// instead of taking only the first N — otherwise we'd bias
			// heavily toward early-morning picks.
			allSlots = append(allSlots, sampleSlotsAcrossDay(daySlots, 5)...)

			if len(allSlots) >= maxSlots {
				break
			}
		}

		if len(allSlots) > maxSlots {
			allSlots = allSlots[:maxSlots]
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
		return nil, fmt.Errorf("invalid professional_id %q: must be a UUID from get_available_professionals", profIDStr)
	}

	// Validate professional exists in this clinic. LLMs frequently hallucinate
	// UUIDs; without this check, we'd surface a generic foreign-key error and
	// the model has no chance of self-correcting on retry.
	if execCtx.ProfessionalLister != nil {
		profs, listErr := execCtx.ProfessionalLister.ListAvailable(execCtx.Context, execCtx.ClinicID, "")
		if listErr == nil {
			found := false
			validIDs := make([]string, 0, len(profs))
			for _, p := range profs {
				validIDs = append(validIDs, fmt.Sprintf("%s (%s)", p.ID, p.Name))
				if p.ID == profID {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf(
					"professional_id %s does not exist in this clinic — you must use one of the exact UUIDs returned by get_available_professionals. Valid IDs: [%s]. Call get_available_professionals again and use the correct id",
					profID, strings.Join(validIDs, ", "),
				)
			}
		}
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
			loc, lerr := time.LoadLocation("America/Sao_Paulo")
			if lerr != nil {
				loc = time.Local
			}
			options := make([]map[string]string, len(appointments))
			for i, apt := range appointments {
				prof := apt.Professional
				if prof == "" {
					prof = "profissional"
				}
				local := apt.StartTime.In(loc)
				options[i] = map[string]string{
					"id":         apt.ID.String(),
					"display":    local.Format("02/01/2006 15:04") + " com " + prof,
					"start_time": local.Format(time.RFC3339),
				}
			}
			return map[string]interface{}{
				"success":      false,
				"status":       "multiple_appointments",
				"message":      "Paciente tem múltiplos agendamentos pendentes. Liste as opções abaixo ao paciente usando o campo 'display' e pergunte qual ele quer confirmar. NUNCA peça o ID/UUID diretamente. Após o paciente escolher, chame confirm_appointment de novo passando o 'id' correto no parâmetro appointment_id.",
				"appointments": options,
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

		// Filter to future, still-actionable appointments. Only "pending" and
		// "confirmed" count — canceled/no_show/no_response/completed/rescheduled
		// are terminal states and must be excluded. NOTE: the status enum is
		// "canceled" (1 L) not "cancelled" (2 Ls) — easy typo.
		var futureApts []*AppointmentInfo
		now := time.Now()
		for _, apt := range appointments {
			if apt.StartTime.Before(now) {
				continue
			}
			if apt.Status != "pending" && apt.Status != "confirmed" {
				continue
			}
			futureApts = append(futureApts, apt)
		}

		if len(futureApts) == 0 {
			return map[string]interface{}{
				"success": false,
				"message": "Nenhum agendamento futuro encontrado",
			}, nil
		}

		// Optional time filter (HH:MM in clinic-local time) so the LLM can
		// disambiguate when the patient says "cancela o das 17h". The patient
		// speaks in local time (BRT), but StartTime may be in UTC — we
		// convert to the clinic TZ (falling back to Local) before comparing.
		if tfRaw, ok := params["time_filter"].(string); ok && tfRaw != "" {
			h, m, ok := parseHHMM(tfRaw)
			if ok {
				loc, lerr := time.LoadLocation("America/Sao_Paulo")
				if lerr != nil {
					loc = time.Local
				}
				var matched []*AppointmentInfo
				for _, apt := range futureApts {
					local := apt.StartTime.In(loc)
					if local.Hour() == h && local.Minute() == m {
						matched = append(matched, apt)
					}
				}
				if len(matched) >= 1 {
					futureApts = matched
				}
				// zero matches → keep futureApts unchanged so the LLM sees everything and responds honestly
			}
		}

		if len(futureApts) > 1 {
			// Return human-friendly displays (in clinic-local TZ) so the LLM
			// can LIST them to the patient and ask which to cancel — NOT ask
			// for a raw UUID. The old response caused the LLM to say "me diga
			// o ID do agendamento" which is terrible UX.
			loc, lerr := time.LoadLocation("America/Sao_Paulo")
			if lerr != nil {
				loc = time.Local
			}
			options := make([]map[string]string, len(futureApts))
			for i, apt := range futureApts {
				prof := apt.Professional
				if prof == "" {
					prof = "profissional"
				}
				local := apt.StartTime.In(loc)
				options[i] = map[string]string{
					"id":         apt.ID.String(),
					"display":    local.Format("02/01/2006 15:04") + " com " + prof,
					"start_time": local.Format(time.RFC3339),
				}
			}
			return map[string]interface{}{
				"success":      false,
				"status":       "multiple_appointments",
				"message":      "Paciente tem múltiplos agendamentos futuros. Liste as opções abaixo ao paciente usando o campo 'display' e pergunte qual ele quer cancelar. NUNCA peça o ID/UUID diretamente. Após o paciente escolher, chame cancel_appointment de novo passando o 'id' correto no parâmetro appointment_id.",
				"appointments": options,
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

func (e *Executor) updatePatientName(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	newName, ok := params["new_name"].(string)
	if !ok || newName == "" {
		return nil, fmt.Errorf("new_name is required")
	}

	if execCtx.PatientID == nil {
		return nil, fmt.Errorf("patient not identified yet")
	}

	if execCtx.PatientNameUpdater == nil {
		return nil, fmt.Errorf("PatientNameUpdater not configured")
	}

	oldName := execCtx.PatientName

	if err := execCtx.PatientNameUpdater.UpdateName(execCtx.Context, execCtx.ClinicID, *execCtx.PatientID, newName); err != nil {
		return nil, fmt.Errorf("failed to update patient name: %w", err)
	}

	// Update the execution context so subsequent tool calls see the new name
	execCtx.PatientName = newName

	return map[string]interface{}{
		"success":  true,
		"old_name": oldName,
		"new_name": newName,
		"message":  fmt.Sprintf("Nome atualizado de %s para %s", oldName, newName),
	}, nil
}

// quickReschedule is the rescheduling equivalent of quick_schedule: resolves
// the CURRENT appointment (by time_filter / auto-detect) AND validates the
// NEW slot in one call, then reschedules. Handles the common case where the
// patient says "remarca o das 16h pra quarta 10h" without any UUIDs.
func (e *Executor) quickReschedule(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	newDateStr, _ := params["new_date"].(string)
	newTimeStr, _ := params["new_time"].(string)
	hasNewSlot := newDateStr != "" && newTimeStr != ""

	durationMinutes := 60
	if d, ok := params["duration_minutes"].(float64); ok && d > 0 {
		durationMinutes = int(d)
	}

	// 1. Resolve the CURRENT appointment.
	var appointmentID uuid.UUID
	if aptIDStr, ok := params["appointment_id"].(string); ok && aptIDStr != "" {
		parsed, err := uuid.Parse(aptIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid appointment_id %q: %w", aptIDStr, err)
		}
		appointmentID = parsed
	} else {
		if execCtx.AppointmentFinder == nil {
			return nil, fmt.Errorf("AppointmentFinder not configured")
		}
		apts, err := execCtx.AppointmentFinder.GetByPatientPhone(execCtx.Context, execCtx.ClinicID, execCtx.PatientPhone, "", 10)
		if err != nil {
			return nil, fmt.Errorf("failed to find appointments: %w", err)
		}

		// Filter to future, still-actionable: pending/confirmed only.
		// (canceled/no_show/completed/rescheduled are terminal.)
		now := time.Now()
		var future []*AppointmentInfo
		for _, apt := range apts {
			if apt.StartTime.Before(now) {
				continue
			}
			if apt.Status != "pending" && apt.Status != "confirmed" {
				continue
			}
			future = append(future, apt)
		}
		if len(future) == 0 {
			return map[string]interface{}{
				"status":  "no_current_appointment",
				"message": "Paciente não tem agendamento futuro para remarcar.",
			}, nil
		}

		// Optional filter by current time.
		loc, lerr := time.LoadLocation("America/Sao_Paulo")
		if lerr != nil {
			loc = time.Local
		}
		if tfRaw, ok := params["current_time_filter"].(string); ok && tfRaw != "" {
			if h, m, ok := parseHHMM(tfRaw); ok {
				var matched []*AppointmentInfo
				for _, apt := range future {
					local := apt.StartTime.In(loc)
					if local.Hour() == h && local.Minute() == m {
						matched = append(matched, apt)
					}
				}
				if len(matched) >= 1 {
					future = matched
				}
			}
		}

		// Build options regardless — useful for the missing-new-slot branch.
		optionsList := func(list []*AppointmentInfo) []map[string]string {
			opts := make([]map[string]string, len(list))
			for i, apt := range list {
				prof := apt.Professional
				if prof == "" {
					prof = "profissional"
				}
				local := apt.StartTime.In(loc)
				opts[i] = map[string]string{
					"id":      apt.ID.String(),
					"display": local.Format("02/01/2006 15:04") + " com " + prof,
				}
			}
			return opts
		}

		if len(future) > 1 {
			return map[string]interface{}{
				"status":       "ambiguous_appointment",
				"message":      "Paciente tem múltiplos agendamentos futuros. Liste usando 'display' e pergunte qual remarcar E para quando — NUNCA mostre IDs.",
				"appointments": optionsList(future),
			}, nil
		}
		appointmentID = future[0].ID

		// Single appointment but patient didn't say the new slot yet.
		if !hasNewSlot {
			return map[string]interface{}{
				"status":       "needs_new_slot",
				"message":      "Paciente tem 1 agendamento futuro (mostre usando 'display'). Pergunte pra qual dia e hora ele quer remarcar, depois chame quick_reschedule de novo com new_date e new_time.",
				"appointments": optionsList(future),
			}, nil
		}
	}

	// Reached only when new_date+new_time ARE set. Enforce late so the listing
	// path above can fire without the patient having given the new slot yet.
	if !hasNewSlot {
		return map[string]interface{}{
			"status":  "needs_new_slot",
			"message": "Falta saber o novo dia e horário. Pergunte ao paciente.",
		}, nil
	}

	// 2. Validate the NEW slot is free for the same professional.
	current, err := execCtx.AppointmentFinder.GetByID(execCtx.Context, appointmentID, execCtx.ClinicID)
	if err != nil || current == nil {
		return nil, fmt.Errorf("failed to load current appointment: %w", err)
	}

	newDate, err := time.Parse("2006-01-02", newDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid new_date %q, use YYYY-MM-DD: %w", newDateStr, err)
	}
	hour, minute, ok := parseHHMM(newTimeStr)
	if !ok {
		return nil, fmt.Errorf("invalid new_time %q, use HH:MM", newTimeStr)
	}

	if execCtx.SlotFinder == nil {
		return nil, fmt.Errorf("SlotFinder not configured")
	}
	daySlots, err := execCtx.SlotFinder.GetAvailableSlots(execCtx.Context, execCtx.ClinicID, current.ProfessionalID, newDate, durationMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get slots: %w", err)
	}

	loc, lerr := time.LoadLocation("America/Sao_Paulo")
	if lerr != nil {
		loc = time.Local
	}
	var newStart time.Time
	var foundMatch bool
	sameDay := make([]*AvailableSlot, 0, len(daySlots))
	for _, s := range daySlots {
		local := s.StartTime.In(loc)
		if sameYMD(local, newDate) {
			sameDay = append(sameDay, s)
			if local.Hour() == hour && local.Minute() == minute {
				newStart = s.StartTime
				foundMatch = true
			}
		}
	}
	if !foundMatch {
		picked := sampleSlotsAcrossDay(sameDay, 3)
		alt := make([]map[string]string, 0, len(picked))
		for _, s := range picked {
			alt = append(alt, map[string]string{"display": s.StartTime.In(loc).Format("02/01 15:04")})
		}
		return map[string]interface{}{
			"status":             "not_available",
			"message":            fmt.Sprintf("Novo horário %s em %s não disponível. Alternativas do mesmo dia listadas (até 3).", newTimeStr, newDateStr),
			"available_same_day": alt,
		}, nil
	}

	// 3. Reschedule.
	if execCtx.AppointmentRescheduler == nil {
		return nil, fmt.Errorf("AppointmentRescheduler not configured")
	}
	if err := execCtx.AppointmentRescheduler.Reschedule(execCtx.Context, appointmentID, execCtx.ClinicID, newStart, nil); err != nil {
		return nil, fmt.Errorf("failed to reschedule: %w", err)
	}

	return map[string]interface{}{
		"status":         "rescheduled",
		"appointment_id": appointmentID,
		"new_start_time": newStart.Format(time.RFC3339),
		"display":        newStart.In(loc).Format("02/01/2006 15:04"),
	}, nil
}

// quickSchedule executes the "patient gave everything in one message" path:
// name fragment + date + time → exactly one appointment, no listing. Returns
// structured responses so the LLM can reply correctly in each branch:
//   - success → {"status": "scheduled", ...appointment}
//   - many matches → {"status": "ambiguous_professional", "candidates": [...]}
//   - no match → {"status": "professional_not_found", "name_fragment": "..."}
//   - slot taken / outside hours → {"status": "not_available", "requested": ..., "available_today": [...]}
func (e *Executor) quickSchedule(execCtx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
	nameFragment, _ := params["professional_name"].(string)
	dateStr, _ := params["date"].(string)
	timeStr, _ := params["start_time"].(string)
	if nameFragment == "" || dateStr == "" || timeStr == "" {
		return nil, fmt.Errorf("professional_name, date and start_time are all required")
	}

	durationMinutes := 60
	if d, ok := params["duration_minutes"].(float64); ok && d > 0 {
		durationMinutes = int(d)
	}

	// 1. Resolve professional by substring match (case-insensitive).
	if execCtx.ProfessionalLister == nil {
		return nil, fmt.Errorf("ProfessionalLister not configured")
	}
	profs, err := execCtx.ProfessionalLister.ListAvailable(execCtx.Context, execCtx.ClinicID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list professionals: %w", err)
	}
	frag := strings.ToLower(strings.TrimSpace(nameFragment))
	var matches []*ProfessionalInfo
	for _, p := range profs {
		if strings.Contains(strings.ToLower(p.Name), frag) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 0 {
		available := make([]map[string]string, 0, len(profs))
		for _, p := range profs {
			available = append(available, map[string]string{"id": p.ID.String(), "name": p.Name})
		}
		return map[string]interface{}{
			"status":                  "professional_not_found",
			"name_fragment":           nameFragment,
			"message":                 fmt.Sprintf("Nenhum profissional encontrado contendo %q no nome.", nameFragment),
			"available_professionals": available,
		}, nil
	}
	if len(matches) > 1 {
		cands := make([]map[string]string, len(matches))
		for i, m := range matches {
			cands[i] = map[string]string{"id": m.ID.String(), "name": m.Name, "specialty": m.Specialty}
		}
		return map[string]interface{}{
			"status":     "ambiguous_professional",
			"candidates": cands,
			"message":    fmt.Sprintf("Mais de um profissional corresponde a %q — pergunte ao paciente qual deles.", nameFragment),
		}, nil
	}
	prof := matches[0]

	// 2. Parse date + time in clinic-local timezone. Without ParseInLocation
	// the slot lands in UTC and the validation against working_hours fails.
	targetDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q, use YYYY-MM-DD: %w", dateStr, err)
	}
	hour, minute, ok := parseHHMM(timeStr)
	if !ok {
		return nil, fmt.Errorf("invalid start_time %q, use HH:MM (e.g. 16:00)", timeStr)
	}

	// 3. Check availability: call SlotFinder for the target day and look for
	// an exact match. If the SlotFinder adapter resolves clinic timezone
	// internally (it does), the returned StartTimes are directly comparable.
	if execCtx.SlotFinder == nil {
		return nil, fmt.Errorf("SlotFinder not configured")
	}
	daySlots, err := execCtx.SlotFinder.GetAvailableSlots(execCtx.Context, execCtx.ClinicID, prof.ID, targetDate, durationMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get slots for %s: %w", prof.Name, err)
	}

	var chosen *AvailableSlot
	var sameDay []*AvailableSlot
	for _, s := range daySlots {
		local := s.StartTime
		if local.Hour() == hour && local.Minute() == minute && sameYMD(local, targetDate) {
			chosen = s
			break
		}
		if sameYMD(s.StartTime, targetDate) {
			sameDay = append(sameDay, s)
		}
	}
	if chosen == nil {
		// Cap to 3 alternatives distributed across the day (morning + afternoon)
		// instead of dumping the full agenda — the LLM was showing 7+ options
		// and overwhelming the patient.
		picked := sampleSlotsAcrossDay(sameDay, 3)
		alternatives := make([]map[string]string, 0, len(picked))
		for _, s := range picked {
			alternatives = append(alternatives, map[string]string{
				"start_time": s.StartTime.Format(time.RFC3339),
				"display":    s.StartTime.Format("02/01 15:04"),
			})
		}
		return map[string]interface{}{
			"status":             "not_available",
			"professional_name":  prof.Name,
			"requested_date":     dateStr,
			"requested_time":     timeStr,
			"available_same_day": alternatives,
			"message":            fmt.Sprintf("%s não tem %s disponível em %s. Alternativas no mesmo dia listadas (até 3).", prof.Name, timeStr, dateStr),
		}, nil
	}

	// 4. Ensure patient exists (reuse the same guard as createAppointment).
	if execCtx.PatientID == nil {
		if execCtx.PatientFinderCreator == nil {
			return nil, fmt.Errorf("PatientFinderCreator not configured")
		}
		patient, err := execCtx.PatientFinderCreator.FindOrCreateByPhone(
			execCtx.Context, execCtx.ClinicID, execCtx.PatientPhone, execCtx.PatientName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to find/create patient: %w", err)
		}
		execCtx.PatientID = &patient.ID
	}

	if execCtx.AppointmentCreator == nil {
		return nil, fmt.Errorf("AppointmentCreator not configured")
	}
	appointment, err := execCtx.AppointmentCreator.CreateFromSchedulingIntent(
		execCtx.Context, execCtx.ClinicID, *execCtx.PatientID, prof.ID, chosen.StartTime, durationMinutes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create appointment: %w", err)
	}

	return map[string]interface{}{
		"status":            "scheduled",
		"appointment_id":    appointment.ID,
		"professional_id":   prof.ID,
		"professional_name": prof.Name,
		"start_time":        chosen.StartTime.Format(time.RFC3339),
		"end_time":          chosen.EndTime.Format(time.RFC3339),
		"display":           chosen.StartTime.Format("02/01/2006 15:04"),
	}, nil
}

// parseHHMM parses "HH:MM" or "HH:MM:SS" into hour/minute.
func parseHHMM(s string) (hour, minute int, ok bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) < 2 {
		return 0, 0, false
	}
	h, err1 := strconvAtoi(parts[0])
	m, err2 := strconvAtoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

func strconvAtoi(s string) (int, error) {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func sameYMD(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// sampleSlotsAcrossDay picks up to `maxPerDay` slots distributed evenly
// across the day. Critical: the sample MUST span the full range, otherwise
// a full-day agenda (9–17) gets cut at early afternoon and patients asking
// for late times like 16:00 are told "not available" even when they are.
//
// Strategy: pick indices at equal float steps across the slot array so the
// first and last are always included. For 9 slots with max=5 this returns
// indices [0, 2, 4, 6, 8] → 09:00, 11:00, 13:00, 15:00, 17:00.
func sampleSlotsAcrossDay(slots []*AvailableSlot, maxPerDay int) []*AvailableSlot {
	if maxPerDay <= 0 || len(slots) == 0 {
		return nil
	}
	if len(slots) <= maxPerDay {
		return slots
	}
	step := float64(len(slots)-1) / float64(maxPerDay-1)
	out := make([]*AvailableSlot, 0, maxPerDay)
	seen := make(map[int]bool, maxPerDay)
	for i := 0; i < maxPerDay; i++ {
		idx := int(float64(i)*step + 0.5) // round half up
		if idx >= len(slots) {
			idx = len(slots) - 1
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true
		out = append(out, slots[idx])
	}
	return out
}
