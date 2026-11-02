package model

import (
	"time"

	"github.com/google/uuid"
)

// AttendanceStatus represents the presence status of a patient
type AttendanceStatus string

const (
	AttendanceStatusPresent     AttendanceStatus = "present"
	AttendanceStatusAbsent      AttendanceStatus = "absent"
	AttendanceStatusSubstituted AttendanceStatus = "substituted"
	// AttendanceStatusConfirmed is set BEFORE the class happens, when the
	// patient pre-confirms participation (e.g. via WhatsApp). It is not the
	// same as Present — sessions_done is not incremented for "confirmed";
	// the operator (or the AI's later check-in flow) flips the status to
	// "present" on class day, which is what triggers the increment.
	AttendanceStatusConfirmed AttendanceStatus = "confirmed"
)

// AppointmentAttendance tracks whether a patient was present at a session
type AppointmentAttendance struct {
	ID            uuid.UUID        `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClinicID      uuid.UUID        `json:"clinic_id" gorm:"type:uuid;not null;index"`
	AppointmentID uuid.UUID        `json:"appointment_id" gorm:"type:uuid;not null;index"`
	PatientID     uuid.UUID        `json:"patient_id" gorm:"type:uuid;not null;index"`
	EnrollmentID  *uuid.UUID       `json:"enrollment_id,omitempty" gorm:"type:uuid;index"` // For group plans (Phase 4)
	Status        AttendanceStatus `json:"status" gorm:"size:20;not null"`
	IsGuest       bool             `json:"is_guest" gorm:"not null;default:false"`
	PaymentID     *uuid.UUID       `json:"payment_id,omitempty" gorm:"type:uuid"` // For guest avulso payment
	CreatedAt     time.Time        `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time        `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations (for response)
	PatientName string `json:"patient_name,omitempty" gorm:"-"`
}

// TableName returns the table name
func (AppointmentAttendance) TableName() string {
	return "appointment_attendance"
}

// RecordAttendanceRequest represents the request to record attendance
type RecordAttendanceRequest struct {
	PatientID       uuid.UUID        `json:"patient_id" binding:"required"`
	Status          AttendanceStatus `json:"status" binding:"required"`
	IsGuest         bool             `json:"is_guest"`
	PaymentMethodID *uuid.UUID       `json:"payment_method_id,omitempty"` // For guest avulso payment
	PaymentAmount   *float64         `json:"payment_amount,omitempty"`    // For guest avulso payment
}

// AttendanceResponse represents the API response
type AttendanceResponse struct {
	ID            uuid.UUID        `json:"id"`
	AppointmentID uuid.UUID        `json:"appointment_id"`
	PatientID     uuid.UUID        `json:"patient_id"`
	EnrollmentID  *uuid.UUID       `json:"enrollment_id,omitempty"`
	Status        AttendanceStatus `json:"status"`
	IsGuest       bool             `json:"is_guest"`
	PatientName   string           `json:"patient_name,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

// ToResponse converts to API response
func (a *AppointmentAttendance) ToResponse() *AttendanceResponse {
	return &AttendanceResponse{
		ID:            a.ID,
		AppointmentID: a.AppointmentID,
		PatientID:     a.PatientID,
		EnrollmentID:  a.EnrollmentID,
		Status:        a.Status,
		IsGuest:       a.IsGuest,
		PatientName:   a.PatientName,
		CreatedAt:     a.CreatedAt,
	}
}
