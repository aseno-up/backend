package model

import (
	"time"

	"github.com/google/uuid"
)

// AppointmentStatus represents the status of an appointment
type AppointmentStatus string

const (
	AppointmentStatusAwaitingPayment AppointmentStatus = "awaiting_payment" // Awaiting online payment before confirming
	AppointmentStatusPending         AppointmentStatus = "pending"          // Awaiting confirmation
	AppointmentStatusConfirmed       AppointmentStatus = "confirmed"        // Confirmed by patient
	AppointmentStatusCanceled        AppointmentStatus = "canceled"         // Canceled
	AppointmentStatusNoShow          AppointmentStatus = "no_show"          // Patient didn't show up
	AppointmentStatusNoResponse      AppointmentStatus = "no_response"      // Patient didn't respond to confirmation
	AppointmentStatusCompleted       AppointmentStatus = "completed"        // Appointment finished
	AppointmentStatusRescheduled     AppointmentStatus = "rescheduled"      // Rescheduled to another time
)

// AppointmentType represents the type of appointment
type AppointmentType string

const (
	AppointmentTypeRegular   AppointmentType = "regular"   // Single appointment
	AppointmentTypeRecurring AppointmentType = "recurring" // Part of a session plan
	AppointmentTypeReturn    AppointmentType = "return"    // Return appointment
)

// Appointment represents an appointment/consultation
type Appointment struct {
	ID             uuid.UUID              `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID       uuid.UUID              `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID      uuid.UUID              `json:"patient_id" gorm:"type:uuid;not null;index"`
	ProfessionalID uuid.UUID              `json:"professional_id" gorm:"type:uuid;not null;index"`
	SessionPlanID  *uuid.UUID             `json:"session_plan_id,omitempty" gorm:"type:uuid;index"` // If part of a session plan
	IsException    bool                   `json:"is_exception" gorm:"not null;default:false"`       // True if rescheduled/edited individually from recurrence
	OriginalDate   *time.Time             `json:"original_date,omitempty"`                          // Original scheduled date before rescheduling
	StartTime      time.Time              `json:"start_time" gorm:"not null;index"`
	EndTime        time.Time              `json:"end_time" gorm:"not null"`
	Status         AppointmentStatus      `json:"status" gorm:"not null;default:'pending';index"`
	Type           AppointmentType        `json:"type" gorm:"not null;default:'regular'"`
	Title          string                 `json:"title"`                                                               // Optional custom title
	Notes          string                 `json:"notes"`                                                               // Internal notes
	PatientNotes   string                 `json:"patient_notes"`                                                       // Notes from patient
	Procedures     []AppointmentProcedure `json:"procedures,omitempty" gorm:"foreignKey:AppointmentID;references:ID"`  // N:N relationship with procedures
	Specialties    []AppointmentSpecialty `json:"specialties,omitempty" gorm:"foreignKey:AppointmentID;references:ID"` // N:N relationship with specialties
	Price          float64                `json:"price" gorm:"type:decimal(10,2)"`                                     // Total price for this appointment
	PaymentStatus  PaymentStatus          `json:"payment_status" gorm:"not null;default:'pending';index"`              // Denormalized: pending, partial, paid
	ConfirmedAt    *time.Time             `json:"confirmed_at,omitempty"`
	CanceledAt     *time.Time             `json:"canceled_at,omitempty"`
	CancelReason   string                 `json:"cancel_reason"`
	// Reminder tracking fields
	Reminder48hSentAt *time.Time `json:"reminder_48h_sent_at,omitempty"`
	Reminder24hSentAt *time.Time `json:"reminder_24h_sent_at,omitempty"`
	Reminder2hSentAt  *time.Time `json:"reminder_2h_sent_at,omitempty"`
	// Confirmation timeout - when this expires without response, status changes to no_response
	ConfirmationExpiresAt *time.Time `json:"confirmation_expires_at,omitempty" gorm:"index"`

	// Legacy field (deprecated, use specific reminder fields)
	ReminderSentAt   *time.Time `json:"reminder_sent_at,omitempty"`
	GoogleEventID    string     `json:"-"` // Google Calendar event ID
	PaymentToken     string     `json:"payment_token,omitempty" gorm:"index"`
	PaymentExpiresAt *time.Time `json:"payment_expires_at,omitempty" gorm:"index"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty" gorm:"index"`

	// Relationships (for preloading)
	Patient      *PatientInfo      `json:"patient,omitempty" gorm:"foreignKey:PatientID;references:ID"`
	Professional *ProfessionalInfo `json:"professional,omitempty" gorm:"foreignKey:ProfessionalID;references:ID"`
}

// AppointmentSpecialty represents the many-to-many relationship between appointments and specialties
type AppointmentSpecialty struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID      uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	AppointmentID uuid.UUID  `json:"appointment_id" gorm:"type:uuid;not null;index"`
	SpecialtyID   uuid.UUID  `json:"specialty_id" gorm:"type:uuid;not null;index"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

func (AppointmentSpecialty) TableName() string {
	return "appointment_specialties"
}

// AppointmentSpecialtyResponse represents the API response for an appointment specialty
type AppointmentSpecialtyResponse struct {
	SpecialtyID uuid.UUID `json:"specialty_id"`
	Name        string    `json:"name,omitempty"`
}

// PatientInfo is a slim version of Patient for embedding
type PatientInfo struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Phone    string    `json:"phone"`
	WhatsApp string    `json:"whatsapp" gorm:"column:whatsapp"`
}

func (PatientInfo) TableName() string {
	return "patients"
}

// ProfessionalInfo is a slim version of Professional for embedding
type ProfessionalInfo struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	Name      string    `json:"name"`
	Specialty string    `json:"specialty"`
	Color     string    `json:"color"`
}

func (ProfessionalInfo) TableName() string {
	return "professionals"
}

// TableName returns the table name for GORM
func (Appointment) TableName() string {
	return "appointments"
}

// AppointmentResponse represents the API response for an appointment
type AppointmentResponse struct {
	ID             uuid.UUID                       `json:"id"`
	ClinicID       uuid.UUID                       `json:"clinic_id"`
	PatientID      uuid.UUID                       `json:"patient_id"`
	ProfessionalID uuid.UUID                       `json:"professional_id"`
	SessionPlanID  *uuid.UUID                      `json:"session_plan_id,omitempty"`
	StartTime      time.Time                       `json:"start_time"`
	EndTime        time.Time                       `json:"end_time"`
	Status         AppointmentStatus               `json:"status"`
	Type           AppointmentType                 `json:"type"`
	Title          string                          `json:"title"`
	Notes          string                          `json:"notes"`
	PatientNotes   string                          `json:"patient_notes"`
	Procedures     []*AppointmentProcedureResponse `json:"procedures,omitempty"`
	Specialties    []*AppointmentSpecialtyResponse `json:"specialties,omitempty"`
	Price          float64                         `json:"price"`
	PaymentStatus  PaymentStatus                   `json:"payment_status"`
	ConfirmedAt    *time.Time                      `json:"confirmed_at,omitempty"`
	CanceledAt     *time.Time                      `json:"canceled_at,omitempty"`
	CancelReason   string                          `json:"cancel_reason,omitempty"`
	Patient        *PatientInfo                    `json:"patient,omitempty"`
	Professional   *ProfessionalInfo               `json:"professional,omitempty"`
	CreatedAt      time.Time                       `json:"created_at"`
}

// ToResponse converts Appointment to AppointmentResponse
func (a *Appointment) ToResponse() *AppointmentResponse {
	resp := &AppointmentResponse{
		ID:             a.ID,
		ClinicID:       a.ClinicID,
		PatientID:      a.PatientID,
		ProfessionalID: a.ProfessionalID,
		SessionPlanID:  a.SessionPlanID,
		StartTime:      a.StartTime,
		EndTime:        a.EndTime,
		Status:         a.Status,
		Type:           a.Type,
		Title:          a.Title,
		Notes:          a.Notes,
		PatientNotes:   a.PatientNotes,
		Price:          a.Price,
		PaymentStatus:  a.PaymentStatus,
		ConfirmedAt:    a.ConfirmedAt,
		CanceledAt:     a.CanceledAt,
		CancelReason:   a.CancelReason,
		Patient:        a.Patient,
		Professional:   a.Professional,
		CreatedAt:      a.CreatedAt,
	}

	if len(a.Procedures) > 0 {
		resp.Procedures = make([]*AppointmentProcedureResponse, len(a.Procedures))
		for i, p := range a.Procedures {
			resp.Procedures[i] = p.ToResponse()
		}
	}

	if len(a.Specialties) > 0 {
		resp.Specialties = make([]*AppointmentSpecialtyResponse, len(a.Specialties))
		for i, s := range a.Specialties {
			resp.Specialties[i] = &AppointmentSpecialtyResponse{
				SpecialtyID: s.SpecialtyID,
			}
		}
	}

	return resp
}

// DurationMinutes returns the duration in minutes
func (a *Appointment) DurationMinutes() int {
	return int(a.EndTime.Sub(a.StartTime).Minutes())
}

// CreateAppointmentRequest represents the request to create an appointment
type CreateAppointmentRequest struct {
	PatientID         uuid.UUID                   `json:"patient_id" binding:"required"`
	ProfessionalID    uuid.UUID                   `json:"professional_id" binding:"required"`
	SpecialtyIDs      []uuid.UUID                 `json:"specialty_ids"`
	StartTime         time.Time                   `json:"start_time" binding:"required"`
	DurationMinutes   int                         `json:"duration_minutes" binding:"required,min=5,max=480"`
	Title             string                      `json:"title"`
	Notes             string                      `json:"notes"`
	Procedures        []AppointmentProcedureInput `json:"procedures"`
	Price             float64                     `json:"price"`
	Type              AppointmentType             `json:"type"`
	WantsAnticipation bool                        `json:"wants_anticipation"` // If true, adds to waitlist for earlier slot
}

// UpdateAppointmentRequest represents the request to update an appointment
type UpdateAppointmentRequest struct {
	PatientID         *uuid.UUID                   `json:"patient_id"`
	ProfessionalID    *uuid.UUID                   `json:"professional_id"`
	SpecialtyIDs      *[]uuid.UUID                 `json:"specialty_ids"`
	StartTime         *time.Time                   `json:"start_time"`
	DurationMinutes   *int                         `json:"duration_minutes" binding:"omitempty,min=5,max=480"`
	Status            AppointmentStatus            `json:"status"`
	Title             string                       `json:"title"`
	Notes             string                       `json:"notes"`
	PatientNotes      string                       `json:"patient_notes"`
	Procedures        *[]AppointmentProcedureInput `json:"procedures"`
	Price             *float64                     `json:"price"`
	CancelReason      string                       `json:"cancel_reason"`
	WantsAnticipation *bool                        `json:"wants_anticipation"` // Update anticipation preference
}

// AppointmentListResponse represents a list of appointments
type AppointmentListResponse struct {
	Data       []*AppointmentResponse `json:"data"`
	Total      int64                  `json:"total"`
	Page       int                    `json:"page"`
	PerPage    int                    `json:"per_page"`
	TotalPages int                    `json:"total_pages"`
}

// AppointmentFilter represents filter options for listing appointments
type AppointmentFilter struct {
	ProfessionalID *uuid.UUID        `form:"professional_id"`
	PatientID      *uuid.UUID        `form:"patient_id"`
	Status         AppointmentStatus `form:"status"`
	StartDate      *time.Time        `form:"start_date"`
	EndDate        *time.Time        `form:"end_date"`
	Page           int               `form:"page"`
	PerPage        int               `form:"per_page"`
}

// ConfirmAppointmentRequest represents a confirmation request
type ConfirmAppointmentRequest struct {
	PatientNotes string `json:"patient_notes"`
}

// CancelAppointmentRequest represents a cancellation request
type CancelAppointmentRequest struct {
	Reason string `json:"reason"`
}

// AppointmentWithClinic represents an appointment with clinic info for reminders
type AppointmentWithClinic struct {
	Appointment
	ClinicName        string `json:"clinic_name" gorm:"column:clinic_name"`
	WhatsAppInstance  string `json:"whatsapp_instance" gorm:"column:whatsapp_instance"`
	WhatsAppConnected bool   `json:"whatsapp_connected" gorm:"column:whatsapp_connected"`
}

// SlotPerformanceStats represents historical performance of a time slot
type SlotPerformanceStats struct {
	DayOfWeek int     `json:"day_of_week"` // 0 = Sunday, 1 = Monday, ...
	HourOfDay int     `json:"hour_of_day"` // 0-23
	Total     int     `json:"total"`
	Completed int     `json:"completed"`
	Rate      float64 `json:"rate"` // Completed / Total (0.0 to 1.0)
}

// SmartSlotSuggestion represents a suggested time slot with a score
type SmartSlotSuggestion struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Score     float64   `json:"score"`  // 0.0 to 10.0 (higher is better)
	Label     string    `json:"label"`  // e.g. "Alta chance de comparecimento"
	Reason    string    `json:"reason"` // e.g. "Histórico de 90% de presença neste horário"
}
