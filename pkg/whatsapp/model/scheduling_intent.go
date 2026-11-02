package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// SchedulingIntentStatus represents the status of a scheduling intent
type SchedulingIntentStatus string

const (
	SchedulingIntentStatusPending   SchedulingIntentStatus = "pending_selection" // Waiting for patient to select a slot
	SchedulingIntentStatusScheduled SchedulingIntentStatus = "scheduled"         // Appointment was created
	SchedulingIntentStatusExpired   SchedulingIntentStatus = "expired"           // Intent expired without selection
	SchedulingIntentStatusCancelled SchedulingIntentStatus = "cancelled"         // Intent was cancelled
)

// SlotOption represents a suggested time slot for scheduling
type SlotOption struct {
	ProfessionalID   uuid.UUID `json:"professional_id"`
	ProfessionalName string    `json:"professional_name"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	ButtonID         string    `json:"button_id"` // ID for WhatsApp button (e.g., "slot_1")
}

// SlotOptions is a custom type for JSONB columns containing slot options
type SlotOptions []SlotOption

// Value implements driver.Valuer for SlotOptions
func (s SlotOptions) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for SlotOptions
func (s *SlotOptions) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for SlotOptions")
	}

	return json.Unmarshal(data, s)
}

// SelectedSlot is a custom type for JSONB columns containing the selected slot
type SelectedSlot struct {
	SlotOption
	SelectedAt time.Time `json:"selected_at"`
}

// Value implements driver.Valuer for SelectedSlot
func (s *SelectedSlot) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for SelectedSlot
func (s *SelectedSlot) Scan(value interface{}) error {
	if value == nil {
		s = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for SelectedSlot")
	}

	return json.Unmarshal(data, s)
}

// SchedulingIntent represents a pending scheduling intent from a patient
type SchedulingIntent struct {
	ID             uuid.UUID              `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID       uuid.UUID              `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID      *uuid.UUID             `json:"patient_id,omitempty" gorm:"type:uuid;index"` // Set after patient is created/found
	PatientPhone   string                 `json:"patient_phone" gorm:"type:varchar(50);not null;index"`
	PatientName    string                 `json:"patient_name" gorm:"type:varchar(255)"` // Extracted from WhatsApp PushName
	IntentType     string                 `json:"intent_type" gorm:"type:varchar(50)"`   // "avaliacao", "retorno", "consulta", "exame"
	Specialty      string                 `json:"specialty" gorm:"type:varchar(100)"`    // "fisioterapia", "psicologia", etc
	SuggestedSlots SlotOptions            `json:"suggested_slots" gorm:"type:jsonb"`     // Slots offered to patient
	SelectedSlot   *SelectedSlot          `json:"selected_slot,omitempty" gorm:"type:jsonb"`
	AppointmentID  *uuid.UUID             `json:"appointment_id,omitempty" gorm:"type:uuid;index"` // Set after appointment is created
	Status         SchedulingIntentStatus `json:"status" gorm:"type:varchar(50);not null;default:'pending_selection';index"`
	Confidence     float64                `json:"confidence" gorm:"type:decimal(3,2)"`
	ExpiresAt      time.Time              `json:"expires_at" gorm:"index"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// TableName returns the table name for GORM
func (SchedulingIntent) TableName() string {
	return "scheduling_intents"
}

// IsExpired checks if the intent has expired
func (s *SchedulingIntent) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsPending checks if the intent is still pending selection
func (s *SchedulingIntent) IsPending() bool {
	return s.Status == SchedulingIntentStatusPending && !s.IsExpired()
}
