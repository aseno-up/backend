package model

import (
	"time"

	"github.com/google/uuid"
)

// CalendarEvent represents either a virtual (RRULE-calculated) or materialized (real) appointment
// on the calendar. Virtual events have no database ID and are rendered differently.
type CalendarEvent struct {
	// Discriminator
	IsVirtual bool `json:"is_virtual"`

	// Common fields (present in both virtual and real)
	SessionPlanID  *uuid.UUID `json:"session_plan_id,omitempty"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        time.Time  `json:"end_time"`
	Title          string     `json:"title"`
	PatientID      *uuid.UUID `json:"patient_id,omitempty"` // null for group plans
	ProfessionalID *uuid.UUID `json:"professional_id,omitempty"`
	PlanType       PlanType   `json:"plan_type,omitempty"` // "individual" or "group"
	DurationMins   int        `json:"duration_mins,omitempty"`

	// Virtual-only fields
	VirtualDate string `json:"virtual_date,omitempty"` // YYYY-MM-DD key for materialization

	// Group-only fields
	EnrolledCount int        `json:"enrolled_count,omitempty"`
	Capacity      *int       `json:"capacity,omitempty"`
	SlotID        *uuid.UUID `json:"slot_id,omitempty"` // populated for slot-based group events

	// Real appointment fields (only when IsVirtual=false)
	AppointmentID *uuid.UUID `json:"id,omitempty"`
	Status        string     `json:"status,omitempty"`
	Type          string     `json:"type,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	PatientNotes  string     `json:"patient_notes,omitempty"`
	Price         float64    `json:"price,omitempty"`
	PaymentStatus string     `json:"payment_status,omitempty"`
	IsException   bool       `json:"is_exception,omitempty"`

	// Relations (for display)
	PatientName       string `json:"patient_name,omitempty"`
	ProfessionalName  string `json:"professional_name,omitempty"`
	ProfessionalColor string `json:"professional_color,omitempty"`
	ProcedureName     string `json:"procedure_name,omitempty"`
}

// CalendarEventsRequest represents the query params for calendar events
type CalendarEventsRequest struct {
	ProfessionalID string `form:"professional_id"`
	StartDate      string `form:"start_date" binding:"required"` // YYYY-MM-DD
	EndDate        string `form:"end_date" binding:"required"`   // YYYY-MM-DD
}

// MaterializeRequest represents the request to materialize a virtual appointment
type MaterializeRequest struct {
	Date string `json:"date" binding:"required"` // YYYY-MM-DD
}

// CancelOccurrenceRequest represents the request to cancel a single virtual occurrence
type CancelOccurrenceRequest struct {
	Date string `json:"date" binding:"required"` // YYYY-MM-DD of the occurrence to cancel
}

// RescheduleOccurrenceRequest represents the request to reschedule a single virtual occurrence
type RescheduleOccurrenceRequest struct {
	OriginalDate string `json:"original_date" binding:"required"`  // YYYY-MM-DD of the original occurrence
	NewStartTime string `json:"new_start_time" binding:"required"` // RFC3339 datetime for the new time
}
