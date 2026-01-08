package model

import (
	"time"

	"github.com/google/uuid"
)

// WaitlistStatus represents the status of a waitlist entry
type WaitlistStatus string

const (
	WaitlistStatusWaiting   WaitlistStatus = "waiting"   // Aguardando vaga
	WaitlistStatusNotified  WaitlistStatus = "notified"  // Notificado, aguardando resposta
	WaitlistStatusScheduled WaitlistStatus = "scheduled" // Agendado com sucesso
	WaitlistStatusDeclined  WaitlistStatus = "declined"  // Recusou a vaga oferecida
	WaitlistStatusExpired   WaitlistStatus = "expired"   // Expirou sem resposta
	WaitlistStatusCancelled WaitlistStatus = "cancelled" // Removido da lista
)

// PreferredDays is a custom type for storing preferred days as JSON
type PreferredDays []string

// WaitlistEntry represents a patient waiting for an appointment slot
type WaitlistEntry struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID       uuid.UUID      `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID      uuid.UUID      `json:"patient_id" gorm:"type:uuid;not null;index"`
	ProfessionalID *uuid.UUID     `json:"professional_id" gorm:"type:uuid;index"` // nil = qualquer profissional

	// Preferências de horário
	PreferredDays      PreferredDays `json:"preferred_days" gorm:"serializer:json"`       // ["monday", "tuesday", ...]
	PreferredTimeStart string        `json:"preferred_time_start" gorm:"default:'08:00'"` // "08:00"
	PreferredTimeEnd   string        `json:"preferred_time_end" gorm:"default:'18:00'"`   // "18:00"

	Status WaitlistStatus `json:"status" gorm:"type:varchar(20);default:'waiting';index"`
	Notes  string         `json:"notes"`

	// Tracking de notificação atual
	PendingSlotTime         *time.Time `json:"pending_slot_time"`         // Horário oferecido aguardando resposta
	PendingSlotProfessional *uuid.UUID `json:"pending_slot_professional"` // Profissional do slot oferecido
	NotifiedAt              *time.Time `json:"notified_at"`               // Quando foi notificado

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiresAt *time.Time `json:"expires_at"` // Opcional: expira automaticamente
}

// TableName returns the table name for GORM
func (WaitlistEntry) TableName() string {
	return "waitlist_entries"
}

// WaitlistEntryWithPatient includes patient info for responses
type WaitlistEntryWithPatient struct {
	WaitlistEntry
	PatientName     string `json:"patient_name"`
	PatientPhone    string `json:"patient_phone"`
	PatientWhatsApp string `json:"patient_whatsapp"`
}

// WaitlistEntryResponse represents the API response for a waitlist entry
type WaitlistEntryResponse struct {
	ID                 uuid.UUID      `json:"id"`
	ClinicID           uuid.UUID      `json:"clinic_id"`
	PatientID          uuid.UUID      `json:"patient_id"`
	PatientName        string         `json:"patient_name,omitempty"`
	PatientPhone       string         `json:"patient_phone,omitempty"`
	ProfessionalID     *uuid.UUID     `json:"professional_id,omitempty"`
	ProfessionalName   string         `json:"professional_name,omitempty"`
	PreferredDays      []string       `json:"preferred_days"`
	PreferredTimeStart string         `json:"preferred_time_start"`
	PreferredTimeEnd   string         `json:"preferred_time_end"`
	Status             WaitlistStatus `json:"status"`
	Notes              string         `json:"notes,omitempty"`
	PendingSlotTime    *time.Time     `json:"pending_slot_time,omitempty"`
	NotifiedAt         *time.Time     `json:"notified_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	ExpiresAt          *time.Time     `json:"expires_at,omitempty"`
}

// ToResponse converts WaitlistEntry to WaitlistEntryResponse
func (w *WaitlistEntry) ToResponse() *WaitlistEntryResponse {
	return &WaitlistEntryResponse{
		ID:                 w.ID,
		ClinicID:           w.ClinicID,
		PatientID:          w.PatientID,
		ProfessionalID:     w.ProfessionalID,
		PreferredDays:      w.PreferredDays,
		PreferredTimeStart: w.PreferredTimeStart,
		PreferredTimeEnd:   w.PreferredTimeEnd,
		Status:             w.Status,
		Notes:              w.Notes,
		PendingSlotTime:    w.PendingSlotTime,
		NotifiedAt:         w.NotifiedAt,
		CreatedAt:          w.CreatedAt,
		ExpiresAt:          w.ExpiresAt,
	}
}

// CreateWaitlistRequest represents the request to add a patient to the waitlist
type CreateWaitlistRequest struct {
	PatientID          uuid.UUID  `json:"patient_id" binding:"required"`
	ProfessionalID     *uuid.UUID `json:"professional_id"`                              // nil = qualquer profissional
	PreferredDays      []string   `json:"preferred_days" binding:"required,min=1"`      // ["monday", "tuesday", ...]
	PreferredTimeStart string     `json:"preferred_time_start" binding:"omitempty"`     // "08:00"
	PreferredTimeEnd   string     `json:"preferred_time_end" binding:"omitempty"`       // "18:00"
	Notes              string     `json:"notes"`
	ExpiresAt          *time.Time `json:"expires_at"`
}

// UpdateWaitlistRequest represents the request to update a waitlist entry
type UpdateWaitlistRequest struct {
	ProfessionalID     *uuid.UUID `json:"professional_id"`
	PreferredDays      []string   `json:"preferred_days"`
	PreferredTimeStart string     `json:"preferred_time_start"`
	PreferredTimeEnd   string     `json:"preferred_time_end"`
	Notes              string     `json:"notes"`
	ExpiresAt          *time.Time `json:"expires_at"`
}

// WaitlistListResponse represents a paginated list of waitlist entries
type WaitlistListResponse struct {
	Data       []*WaitlistEntryResponse `json:"data"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PerPage    int                      `json:"per_page"`
	TotalPages int                      `json:"total_pages"`
}

// SlotOffer represents a slot being offered to a waitlist patient
type SlotOffer struct {
	WaitlistEntryID uuid.UUID  `json:"waitlist_entry_id"`
	ClinicID        uuid.UUID  `json:"clinic_id"`
	PatientID       uuid.UUID  `json:"patient_id"`
	PatientName     string     `json:"patient_name"`
	PatientPhone    string     `json:"patient_phone"`
	ProfessionalID  uuid.UUID  `json:"professional_id"`
	ProfessionalName string    `json:"professional_name"`
	SlotTime        time.Time  `json:"slot_time"`
	ClinicName      string     `json:"clinic_name"`
	Instance        string     `json:"instance"` // WhatsApp instance
}
