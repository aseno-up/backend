package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// PlanStatus represents the status of a session plan
type PlanStatus string

const (
	PlanStatusActive    PlanStatus = "active"
	PlanStatusPaused    PlanStatus = "paused"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusCancelled PlanStatus = "cancelled"
)

// Frequency represents how often sessions occur
type Frequency string

const (
	FrequencyDaily       Frequency = "daily"
	FrequencyWeekly      Frequency = "weekly"      // 1x per week
	FrequencyBiweekly    Frequency = "biweekly"    // 2x per week
	FrequencyTriweekly   Frequency = "triweekly"   // 3x per week
	FrequencyBimonthly   Frequency = "bimonthly"   // every 2 weeks
	FrequencyMonthly     Frequency = "monthly"     // 1x per month
	FrequencyCustom      Frequency = "custom"      // custom days
)

// SessionPlan represents a recurring session plan for a patient
type SessionPlan struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClinicID       uuid.UUID      `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID      uuid.UUID      `json:"patient_id" gorm:"type:uuid;not null;index"`
	ProfessionalID *uuid.UUID     `json:"professional_id" gorm:"type:uuid;index"`

	// Plan details
	Title          string         `json:"title" gorm:"size:255;not null"`
	Description    string         `json:"description" gorm:"type:text"`
	Status         PlanStatus     `json:"status" gorm:"size:20;not null;default:'active'"`

	// Frequency configuration
	Frequency      Frequency      `json:"frequency" gorm:"size:20;not null"`
	WeekDays       pq.Int64Array  `json:"week_days" gorm:"type:integer[]"` // 0=Sunday, 1=Monday, etc.
	PreferredTime  string         `json:"preferred_time" gorm:"size:5"` // HH:MM format
	DurationMins   int            `json:"duration_mins" gorm:"not null;default:50"`

	// Session limits
	TotalSessions  int            `json:"total_sessions" gorm:"not null"` // Total number of sessions in the plan
	SessionsDone   int            `json:"sessions_done" gorm:"not null;default:0"`

	// Date range
	StartDate      time.Time      `json:"start_date" gorm:"not null"`
	EndDate        *time.Time     `json:"end_date"` // Optional, calculated from total_sessions if not set

	// Generation tracking
	LastGeneratedAt *time.Time    `json:"last_generated_at"`
	GenerateUntil   time.Time     `json:"generate_until"` // Generate appointments until this date

	// Pricing
	SessionPrice   float64        `json:"session_price" gorm:"type:decimal(10,2);default:0"`

	// Metadata
	Notes          string         `json:"notes" gorm:"type:text"`
	PausedAt       *time.Time     `json:"paused_at"`
	CancelledAt    *time.Time     `json:"cancelled_at"`
	CompletedAt    *time.Time     `json:"completed_at"`

	// Timestamps
	CreatedAt      time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations (for response only, not stored)
	PatientName      string `json:"patient_name,omitempty" gorm:"-"`
	ProfessionalName string `json:"professional_name,omitempty" gorm:"-"`
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

// RemainingSession returns the number of remaining sessions
func (s *SessionPlan) RemainingSessions() int {
	return s.TotalSessions - s.SessionsDone
}

// Progress returns the progress percentage (0-100)
func (s *SessionPlan) Progress() float64 {
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
	PatientID      uuid.UUID  `json:"patient_id" binding:"required"`
	ProfessionalID *uuid.UUID `json:"professional_id"` // Optional
	Title          string     `json:"title" binding:"required,max=255"`
	Description    string     `json:"description"`
	Frequency      Frequency  `json:"frequency" binding:"required"`
	WeekDays       []int      `json:"week_days"` // Required for custom frequency
	PreferredTime  string     `json:"preferred_time" binding:"required"` // HH:MM
	DurationMins   int        `json:"duration_mins" binding:"required,min=15,max=480"`
	TotalSessions  int        `json:"total_sessions" binding:"required,min=-1,max=200"` // -1 = indeterminate
	StartDate      string     `json:"start_date" binding:"required"` // YYYY-MM-DD
	SessionPrice   float64    `json:"session_price" binding:"min=0"`
	Notes          string     `json:"notes"`
	GenerateAll    bool       `json:"generate_all"` // If true, generate all sessions immediately
}

// UpdateSessionPlanRequest represents the request to update a session plan
type UpdateSessionPlanRequest struct {
	Title          *string    `json:"title" binding:"omitempty,max=255"`
	Description    *string    `json:"description"`
	PreferredTime  *string    `json:"preferred_time"` // HH:MM
	DurationMins   *int       `json:"duration_mins" binding:"omitempty,min=15,max=480"`
	SessionPrice   *float64   `json:"session_price" binding:"omitempty,min=0"`
	Notes          *string    `json:"notes"`
}

// SessionPlanResponse represents the response for a session plan
type SessionPlanResponse struct {
	*SessionPlan
	Progress          float64 `json:"progress"`
	RemainingSessions int     `json:"remaining_sessions"`
}

// ToResponse converts a SessionPlan to SessionPlanResponse
func (s *SessionPlan) ToResponse() *SessionPlanResponse {
	return &SessionPlanResponse{
		SessionPlan:       s,
		Progress:          s.Progress(),
		RemainingSessions: s.RemainingSessions(),
	}
}
