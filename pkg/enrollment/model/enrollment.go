package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// EnrollmentStatus represents the status of an enrollment
type EnrollmentStatus string

const (
	EnrollmentStatusActive    EnrollmentStatus = "active"
	EnrollmentStatusSuspended EnrollmentStatus = "suspended"
	EnrollmentStatusCancelled EnrollmentStatus = "cancelled"
)

// PaymentMode represents how the enrollment is paid
type PaymentMode string

const (
	PaymentModePerSession PaymentMode = "per_session"
	PaymentModeUpfront    PaymentMode = "upfront"
	PaymentModeMonthly    PaymentMode = "monthly"
)

// SessionPlanEnrollment represents a patient enrolled in a group session plan
type SessionPlanEnrollment struct {
	ID            uuid.UUID        `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClinicID      uuid.UUID        `json:"clinic_id" gorm:"type:uuid;not null;index"`
	SessionPlanID uuid.UUID        `json:"session_plan_id" gorm:"type:uuid;not null;index"`
	PatientID     uuid.UUID        `json:"patient_id" gorm:"type:uuid;not null;index"`
	Status        EnrollmentStatus `json:"status" gorm:"size:20;not null;default:'active'"`

	// Which weekdays this patient attends (0=Sunday ... 6=Saturday).
	// Must be a subset of the plan's available days.
	WeekDays pq.Int64Array `json:"week_days" gorm:"type:integer[]"`

	// Individual package within the group
	TotalSessions *int `json:"total_sessions"` // null = indeterminate (follows plan)
	SessionsDone  int  `json:"sessions_done" gorm:"not null;default:0"`

	// Payment
	PaymentMode   PaymentMode `json:"payment_mode" gorm:"size:20;not null;default:'per_session'"`
	MonthlyAmount *float64    `json:"monthly_amount" gorm:"type:decimal(10,2)"`
	BillingDay    *int        `json:"billing_day"` // Day of month for monthly billing

	// Dates
	JoinedAt time.Time  `json:"joined_at" gorm:"not null"`
	LeftAt   *time.Time `json:"left_at"`

	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations (for response)
	PatientName string `json:"patient_name,omitempty" gorm:"-"`
}

// TableName returns the table name
func (SessionPlanEnrollment) TableName() string {
	return "session_plan_enrollments"
}

// IsActive returns true if the enrollment is active
func (e *SessionPlanEnrollment) IsActive() bool {
	return e.Status == EnrollmentStatusActive
}

// Progress returns the progress percentage
func (e *SessionPlanEnrollment) Progress() float64 {
	if e.TotalSessions == nil || *e.TotalSessions <= 0 {
		return 0
	}
	return float64(e.SessionsDone) / float64(*e.TotalSessions) * 100
}

// EnrollRequest represents the request to enroll a patient
type EnrollRequest struct {
	PatientID     uuid.UUID   `json:"patient_id" binding:"required"`
	WeekDays      []int       `json:"week_days"`
	TotalSessions *int        `json:"total_sessions"`
	PaymentMode   PaymentMode `json:"payment_mode"`
	MonthlyAmount *float64    `json:"monthly_amount"`
	BillingDay    *int        `json:"billing_day"`
}

// EnrollmentResponse represents the API response
type EnrollmentResponse struct {
	ID            uuid.UUID        `json:"id"`
	SessionPlanID uuid.UUID        `json:"session_plan_id"`
	PatientID     uuid.UUID        `json:"patient_id"`
	Status        EnrollmentStatus `json:"status"`
	WeekDays      []int            `json:"week_days"`
	TotalSessions *int             `json:"total_sessions"`
	SessionsDone  int              `json:"sessions_done"`
	Progress      float64          `json:"progress"`
	PaymentMode   PaymentMode      `json:"payment_mode"`
	MonthlyAmount *float64         `json:"monthly_amount,omitempty"`
	BillingDay    *int             `json:"billing_day,omitempty"`
	JoinedAt      time.Time        `json:"joined_at"`
	LeftAt        *time.Time       `json:"left_at,omitempty"`
	PatientName   string           `json:"patient_name,omitempty"`
}

// ToResponse converts to API response
func (e *SessionPlanEnrollment) ToResponse() *EnrollmentResponse {
	weekDays := make([]int, len(e.WeekDays))
	for i, d := range e.WeekDays {
		weekDays[i] = int(d)
	}
	return &EnrollmentResponse{
		ID:            e.ID,
		SessionPlanID: e.SessionPlanID,
		PatientID:     e.PatientID,
		Status:        e.Status,
		WeekDays:      weekDays,
		TotalSessions: e.TotalSessions,
		SessionsDone:  e.SessionsDone,
		Progress:      e.Progress(),
		PaymentMode:   e.PaymentMode,
		MonthlyAmount: e.MonthlyAmount,
		BillingDay:    e.BillingDay,
		JoinedAt:      e.JoinedAt,
		LeftAt:        e.LeftAt,
		PatientName:   e.PatientName,
	}
}

// HasWeekDay returns true if this enrollment attends the given weekday (0=Sun..6=Sat).
func (e *SessionPlanEnrollment) HasWeekDay(day int) bool {
	for _, d := range e.WeekDays {
		if int(d) == day {
			return true
		}
	}
	return false
}
