package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// DayTimesMap maps weekday abbreviation (MO, TU, WE, TH, FR, SA, SU) to time (HH:MM).
// Used to set different times for different days of the week in a session plan.
type DayTimesMap map[string]string

func (d DayTimesMap) Value() (driver.Value, error) {
	if d == nil || len(d) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (d *DayTimesMap) Scan(value interface{}) error {
	if value == nil {
		*d = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("DayTimesMap.Scan: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, d)
}

// PlanStatus represents the status of a session plan
type PlanStatus string

const (
	PlanStatusActive    PlanStatus = "active"
	PlanStatusPaused    PlanStatus = "paused"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusCancelled PlanStatus = "cancelled"
)

// PlanType represents the type of a session plan
type PlanType string

const (
	PlanTypeIndividual PlanType = "individual"
	PlanTypeGroup      PlanType = "group"
)

// PaymentMode represents how the plan is paid
type PaymentMode string

const (
	PaymentModePerSession PaymentMode = "per_session"
	PaymentModeUpfront    PaymentMode = "upfront"
	PaymentModeMonthly    PaymentMode = "monthly"
)

// Frequency represents how often sessions occur
type Frequency string

const (
	FrequencyDaily     Frequency = "daily"
	FrequencyWeekly    Frequency = "weekly"    // 1x per week
	FrequencyBiweekly  Frequency = "biweekly"  // 2x per week
	FrequencyTriweekly Frequency = "triweekly" // 3x per week
	FrequencyBimonthly Frequency = "bimonthly" // every 2 weeks
	FrequencyMonthly   Frequency = "monthly"   // 1x per month
	FrequencyCustom    Frequency = "custom"    // custom days
)

// SessionPlan represents a recurring session plan for a patient
type SessionPlan struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClinicID       uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID      uuid.UUID  `json:"patient_id" gorm:"type:uuid;not null;index"`
	ProfessionalID *uuid.UUID `json:"professional_id" gorm:"type:uuid;index"`

	// Plan details
	Title       string     `json:"title" gorm:"size:255;not null"`
	Description string     `json:"description" gorm:"type:text"`
	Status      PlanStatus `json:"status" gorm:"size:20;not null;default:'active'"`
	Type        PlanType   `json:"type" gorm:"size:20;not null;default:'individual'"`

	// RRULE-based recurrence (new)
	RRule   string     `json:"rrule" gorm:"type:text"`      // RFC 5545 RRULE string, e.g. "FREQ=WEEKLY;BYDAY=MO,WE"
	DtStart *time.Time `json:"dtstart" gorm:"type:timestamp"` // Start datetime (replaces start_date + preferred_time)

	// Per-day times (new) — different time per weekday, e.g. {"MO":"08:00","WE":"14:00"}
	DayTimes DayTimesMap `json:"day_times" gorm:"type:jsonb"`

	// Procedure link (new)
	ProcedureID   *uuid.UUID `json:"procedure_id" gorm:"type:uuid;index"`
	PriceOverride *float64   `json:"price_override" gorm:"type:decimal(10,2)"` // Optional override of procedure price

	// Group fields (new)
	Capacity *int `json:"capacity"` // Max patients for group plans

	// Payment mode (new)
	PaymentMode PaymentMode `json:"payment_mode" gorm:"size:20;not null;default:'per_session'"`

	// Frequency configuration (legacy — kept for backward compat, new plans use RRule)
	Frequency     Frequency     `json:"frequency" gorm:"size:20;not null"`
	WeekDays      pq.Int64Array `json:"week_days" gorm:"type:integer[]"` // 0=Sunday, 1=Monday, etc.
	PreferredTime string        `json:"preferred_time" gorm:"size:5"`    // HH:MM format
	DurationMins  int           `json:"duration_mins" gorm:"not null;default:50"`

	// Session limits
	TotalSessions int `json:"total_sessions" gorm:"not null"` // Total number of sessions in the plan
	SessionsDone  int `json:"sessions_done" gorm:"not null;default:0"`

	// Date range
	StartDate time.Time  `json:"start_date" gorm:"not null"`
	EndDate   *time.Time `json:"end_date"` // Optional, calculated from total_sessions if not set

	// Generation tracking (legacy — kept for backward compat)
	LastGeneratedAt *time.Time `json:"last_generated_at"`
	GenerateUntil   time.Time  `json:"generate_until"`

	// Pricing (legacy — new plans use procedure price or price_override)
	SessionPrice float64 `json:"session_price" gorm:"type:decimal(10,2);default:0"`

	// Metadata
	Notes                string     `json:"notes" gorm:"type:text"`
	WhatsappConfirmation bool       `json:"whatsapp_confirmation" gorm:"default:true"`
	PausedAt             *time.Time `json:"paused_at"`
	CancelledAt          *time.Time `json:"cancelled_at"`
	CompletedAt          *time.Time `json:"completed_at"`

	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations (for response only, not stored)
	PatientName      string `json:"patient_name,omitempty" gorm:"-"`
	ProfessionalName string `json:"professional_name,omitempty" gorm:"-"`
	ProcedureName    string `json:"procedure_name,omitempty" gorm:"-"`
}

// TableName returns the table name for SessionPlan
func (SessionPlan) TableName() string {
	return "session_plans"
}

// IsActive returns true if the plan is active
func (s *SessionPlan) IsActive() bool {
	return s.Status == PlanStatusActive
}

// IsPaused returns true if the plan is paused
func (s *SessionPlan) IsPaused() bool {
	return s.Status == PlanStatusPaused
}

// IsCompleted returns true if the plan is completed
func (s *SessionPlan) IsCompleted() bool {
	return s.Status == PlanStatusCompleted
}

// IsCancelled returns true if the plan is cancelled
func (s *SessionPlan) IsCancelled() bool {
	return s.Status == PlanStatusCancelled
}

// IsIndeterminate returns true if the plan has no session limit
func (s *SessionPlan) IsIndeterminate() bool {
	return s.TotalSessions <= 0
}

// RemainingSessions returns the number of remaining sessions (-1 for indeterminate)
func (s *SessionPlan) RemainingSessions() int {
	if s.IsIndeterminate() {
		return -1
	}
	return s.TotalSessions - s.SessionsDone
}

// Progress returns the progress percentage (0-100, 0 for indeterminate)
func (s *SessionPlan) Progress() float64 {
	if s.IsIndeterminate() {
		return 0
	}
	if s.TotalSessions == 0 {
		return 0
	}
	return float64(s.SessionsDone) / float64(s.TotalSessions) * 100
}

// GetSessionsPerWeek returns the number of sessions per week based on frequency
func (s *SessionPlan) GetSessionsPerWeek() int {
	switch s.Frequency {
	case FrequencyDaily:
		return 7
	case FrequencyWeekly:
		return 1
	case FrequencyBiweekly:
		return 2
	case FrequencyTriweekly:
		return 3
	case FrequencyBimonthly:
		return 0 // Special case, handled differently
	case FrequencyMonthly:
		return 0 // Special case, handled differently
	case FrequencyCustom:
		return len(s.WeekDays)
	default:
		return 1
	}
}

// CreateSessionPlanRequest represents the request to create a session plan
type CreateSessionPlanRequest struct {
	PatientID            uuid.UUID  `json:"patient_id"`        // Required for individual, null for group
	ProfessionalID       *uuid.UUID `json:"professional_id"`   // Optional
	Title                string     `json:"title" binding:"required,max=255"`
	Description          string     `json:"description"`
	Type                 PlanType   `json:"type"`              // "individual" (default) or "group"
	Frequency            Frequency  `json:"frequency" binding:"required"`
	WeekDays             []int      `json:"week_days"`                         // Required for custom frequency
	PreferredTime        string     `json:"preferred_time" binding:"required"` // HH:MM
	DurationMins         int        `json:"duration_mins" binding:"required,min=15,max=480"`
	TotalSessions        int        `json:"total_sessions" binding:"required,min=-1,max=200"` // -1 = indeterminate
	StartDate            string     `json:"start_date" binding:"required"`                    // YYYY-MM-DD
	SessionPrice         float64    `json:"session_price" binding:"min=0"`
	ProcedureID          *uuid.UUID `json:"procedure_id"`      // FK to procedures (new)
	PriceOverride        *float64   `json:"price_override"`    // Optional override of procedure price (new)
	Capacity             *int       `json:"capacity"`          // Max patients for group (new)
	PaymentMode          PaymentMode `json:"payment_mode"`     // per_session, upfront, monthly (new)
	RRule                string      `json:"rrule"`              // RFC 5545 RRULE string (new, optional — computed from frequency if empty)
	DayTimes             DayTimesMap `json:"day_times"`          // Per-day times (new, optional — e.g. {"MO":"08:00","WE":"14:00"})
	Notes                string      `json:"notes"`
	GenerateAll          bool       `json:"generate_all"` // If true, generate all sessions immediately
	WhatsappConfirmation bool       `json:"whatsapp_confirmation"`
}

// UpdateSessionPlanRequest represents the request to update a session plan
type UpdateSessionPlanRequest struct {
	Title          *string     `json:"title" binding:"omitempty,max=255"`
	Description    *string     `json:"description"`
	PreferredTime  *string     `json:"preferred_time"` // HH:MM
	DurationMins   *int        `json:"duration_mins" binding:"omitempty,min=15,max=480"`
	SessionPrice   *float64    `json:"session_price" binding:"omitempty,min=0"`
	Notes          *string     `json:"notes"`
	Frequency      *Frequency  `json:"frequency"`
	WeekDays       *[]int      `json:"week_days"`
	DayTimes       *DayTimesMap `json:"day_times"`
	ProfessionalID *uuid.UUID  `json:"professional_id"`
	Capacity       *int        `json:"capacity" binding:"omitempty,min=1,max=50"`
}

// SessionPlanResponse represents the response for a session plan
type SessionPlanResponse struct {
	*SessionPlan
	Progress          float64 `json:"progress"`
	RemainingSessions int     `json:"remaining_sessions"`
}

// GetEffectivePrice returns the effective session price considering price_override and procedure price
func (s *SessionPlan) GetEffectivePrice() float64 {
	if s.PriceOverride != nil {
		return *s.PriceOverride
	}
	// SessionPrice is the legacy field; also used as fallback when procedure price is loaded externally
	return s.SessionPrice
}

// ToResponse converts a SessionPlan to SessionPlanResponse
func (s *SessionPlan) ToResponse() *SessionPlanResponse {
	return &SessionPlanResponse{
		SessionPlan:       s,
		Progress:          s.Progress(),
		RemainingSessions: s.RemainingSessions(),
	}
}
