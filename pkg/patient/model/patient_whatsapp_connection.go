package model

import (
	"time"

	"github.com/google/uuid"
)

// PatientWhatsAppConnection links a patient to a specific WhatsApp connection
// with per-patient notification and AI scheduling preferences.
// Absence of a record means default behavior (everything enabled).
type PatientWhatsAppConnection struct {
	ID                   uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID             uuid.UUID `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID            uuid.UUID `json:"patient_id" gorm:"type:uuid;not null;index"`
	WhatsAppConnectionID uuid.UUID `json:"whatsapp_connection_id" gorm:"type:uuid;not null;index"`
	AISchedulingEnabled  bool      `json:"ai_scheduling_enabled" gorm:"default:true"`
	RemindersEnabled     bool      `json:"reminders_enabled" gorm:"default:true"`
	// HumanTakeoverAt is set when a clinic user replied to this patient
	// directly through the WhatsApp app. Stored here (not on
	// conversation_contexts) because takeover is a persistent per-patient
	// state that must survive conversation rotation/expiry.
	HumanTakeoverAt *time.Time `json:"human_takeover_at,omitempty" gorm:"index"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// IsHumanTakeoverActive reports whether a human takeover is still within the
// given pause duration relative to now. pauseDuration == 0 disables the
// feature and always returns false.
func (p *PatientWhatsAppConnection) IsHumanTakeoverActive(pauseDuration time.Duration, now time.Time) bool {
	if p == nil || p.HumanTakeoverAt == nil || pauseDuration <= 0 {
		return false
	}
	return now.Before(p.HumanTakeoverAt.Add(pauseDuration))
}

// TableName returns the table name for GORM
func (PatientWhatsAppConnection) TableName() string {
	return "patient_whatsapp_connections"
}

// PatientWhatsAppConnectionResponse represents the API response
type PatientWhatsAppConnectionResponse struct {
	ID                   uuid.UUID  `json:"id"`
	ClinicID             uuid.UUID  `json:"clinic_id"`
	PatientID            uuid.UUID  `json:"patient_id"`
	WhatsAppConnectionID uuid.UUID  `json:"whatsapp_connection_id"`
	AISchedulingEnabled  bool       `json:"ai_scheduling_enabled"`
	RemindersEnabled     bool       `json:"reminders_enabled"`
	HumanTakeoverAt      *time.Time `json:"human_takeover_at,omitempty"`
	HumanTakeoverUntil   *time.Time `json:"human_takeover_until,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// ToResponse converts PatientWhatsAppConnection to PatientWhatsAppConnectionResponse.
// pauseDuration is the clinic-configured AI pause window; when >0 and a takeover
// is set, HumanTakeoverUntil is populated so the frontend can render "IA pausada
// até HH:MM" without fetching clinic settings separately.
func (p *PatientWhatsAppConnection) ToResponse(pauseDuration time.Duration) *PatientWhatsAppConnectionResponse {
	resp := &PatientWhatsAppConnectionResponse{
		ID:                   p.ID,
		ClinicID:             p.ClinicID,
		PatientID:            p.PatientID,
		WhatsAppConnectionID: p.WhatsAppConnectionID,
		AISchedulingEnabled:  p.AISchedulingEnabled,
		RemindersEnabled:     p.RemindersEnabled,
		HumanTakeoverAt:      p.HumanTakeoverAt,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
	}
	if p.HumanTakeoverAt != nil && pauseDuration > 0 {
		until := p.HumanTakeoverAt.Add(pauseDuration)
		resp.HumanTakeoverUntil = &until
	}
	return resp
}

// UpdatePatientWhatsAppConnectionRequest represents the request to update preferences
type UpdatePatientWhatsAppConnectionRequest struct {
	AISchedulingEnabled *bool `json:"ai_scheduling_enabled,omitempty"`
	RemindersEnabled    *bool `json:"reminders_enabled,omitempty"`
}
