package model

import (
	"time"

	"github.com/google/uuid"
)

// PatientWhatsAppConnection links a patient to a specific WhatsApp connection
// with per-patient notification and AI scheduling preferences.
// Absence of a record means default behavior (everything enabled).
type PatientWhatsAppConnection struct {
	ID                    uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID              uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID             uuid.UUID  `json:"patient_id" gorm:"type:uuid;not null;index"`
	WhatsAppConnectionID  uuid.UUID  `json:"whatsapp_connection_id" gorm:"type:uuid;not null;index"`
	AISchedulingEnabled   bool       `json:"ai_scheduling_enabled" gorm:"default:true"`
	RemindersEnabled      bool       `json:"reminders_enabled" gorm:"default:true"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	DeletedAt             *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (PatientWhatsAppConnection) TableName() string {
	return "patient_whatsapp_connections"
}

// PatientWhatsAppConnectionResponse represents the API response
type PatientWhatsAppConnectionResponse struct {
	ID                   uuid.UUID `json:"id"`
	ClinicID             uuid.UUID `json:"clinic_id"`
	PatientID            uuid.UUID `json:"patient_id"`
	WhatsAppConnectionID uuid.UUID `json:"whatsapp_connection_id"`
	AISchedulingEnabled  bool      `json:"ai_scheduling_enabled"`
	RemindersEnabled     bool      `json:"reminders_enabled"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// ToResponse converts PatientWhatsAppConnection to PatientWhatsAppConnectionResponse
func (p *PatientWhatsAppConnection) ToResponse() *PatientWhatsAppConnectionResponse {
	return &PatientWhatsAppConnectionResponse{
		ID:                   p.ID,
		ClinicID:             p.ClinicID,
		PatientID:            p.PatientID,
		WhatsAppConnectionID: p.WhatsAppConnectionID,
		AISchedulingEnabled:  p.AISchedulingEnabled,
		RemindersEnabled:     p.RemindersEnabled,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
	}
}

// UpdatePatientWhatsAppConnectionRequest represents the request to update preferences
type UpdatePatientWhatsAppConnectionRequest struct {
	AISchedulingEnabled *bool `json:"ai_scheduling_enabled,omitempty"`
	RemindersEnabled    *bool `json:"reminders_enabled,omitempty"`
}
