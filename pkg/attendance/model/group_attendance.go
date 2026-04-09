package model

import (
	"time"

	"github.com/google/uuid"
)

// GroupSessionAttendance tracks attendance for group plan sessions.
// Unlike AppointmentAttendance, this is linked to a session plan + date
// instead of an individual appointment — group plans do not materialize.
type GroupSessionAttendance struct {
	ID            uuid.UUID        `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClinicID      uuid.UUID        `json:"clinic_id" gorm:"type:uuid;not null;index"`
	SessionPlanID uuid.UUID        `json:"session_plan_id" gorm:"type:uuid;not null;index"`
	SessionDate   string           `json:"session_date" gorm:"size:10;not null;index"` // YYYY-MM-DD
	PatientID     uuid.UUID        `json:"patient_id" gorm:"type:uuid;not null;index"`
	EnrollmentID  *uuid.UUID       `json:"enrollment_id,omitempty" gorm:"type:uuid;index"`
	Status        AttendanceStatus `json:"status" gorm:"size:20;not null"`
	IsGuest       bool             `json:"is_guest" gorm:"not null;default:false"`
	CreatedAt     time.Time        `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time        `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations (for response, not persisted)
	PatientName string `json:"patient_name,omitempty" gorm:"-"`
}

// TableName returns the table name
func (GroupSessionAttendance) TableName() string {
	return "group_session_attendance"
}

// GroupAttendanceEntry represents a single entry in a bulk group attendance request
type GroupAttendanceEntry struct {
	PatientID       uuid.UUID        `json:"patient_id" binding:"required"`
	Status          AttendanceStatus `json:"status" binding:"required"`
	IsGuest         bool             `json:"is_guest"`
	PaymentMethodID *uuid.UUID       `json:"payment_method_id,omitempty"` // For guest avulso payment
	PaymentAmount   *float64         `json:"payment_amount,omitempty"`    // For guest avulso payment
}

// RecordGroupAttendanceRequest represents the request to record attendance for a group session
type RecordGroupAttendanceRequest struct {
	Date    string                 `json:"date" binding:"required"` // YYYY-MM-DD
	Entries []GroupAttendanceEntry `json:"entries" binding:"required,min=1"`
}

// GroupAttendanceResponse represents the API response
type GroupAttendanceResponse struct {
	ID            uuid.UUID        `json:"id"`
	SessionPlanID uuid.UUID        `json:"session_plan_id"`
	SessionDate   string           `json:"session_date"`
	PatientID     uuid.UUID        `json:"patient_id"`
	EnrollmentID  *uuid.UUID       `json:"enrollment_id,omitempty"`
	Status        AttendanceStatus `json:"status"`
	IsGuest       bool             `json:"is_guest"`
	PatientName   string           `json:"patient_name,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

// ToResponse converts to API response
func (g *GroupSessionAttendance) ToResponse() *GroupAttendanceResponse {
	return &GroupAttendanceResponse{
		ID:            g.ID,
		SessionPlanID: g.SessionPlanID,
		SessionDate:   g.SessionDate,
		PatientID:     g.PatientID,
		EnrollmentID:  g.EnrollmentID,
		Status:        g.Status,
		IsGuest:       g.IsGuest,
		PatientName:   g.PatientName,
		CreatedAt:     g.CreatedAt,
	}
}
