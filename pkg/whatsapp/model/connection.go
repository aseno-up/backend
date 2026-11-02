package model

import (
	"time"

	"github.com/google/uuid"
)

// WhatsAppConnection stores the WhatsApp connection state for a clinic.
// A clinic may have multiple connections in the future; only one is active at a time (MVP).
type WhatsAppConnection struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID      uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Provider      string     `json:"provider" gorm:"default:'EVOLUTION'"` // "EVOLUTION" or "GUPSHUP"
	Instance      *string    `json:"instance" gorm:"uniqueIndex"`         // Evolution API instance name
	WabaID        *string    `json:"waba_id"`                             // Official API Waba ID
	PhoneNumberID *string    `json:"phone_number_id"`                     // Official API Phone Number ID
	Token         *string    `json:"token"`                               // Official API Token
	Connected     bool       `json:"connected" gorm:"default:false"`
	IsActive      bool       `json:"is_active" gorm:"default:true"` // active = used by the system
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (WhatsAppConnection) TableName() string {
	return "whatsapp_connections"
}
