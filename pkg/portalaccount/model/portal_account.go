package model

import (
	"time"

	"github.com/google/uuid"
)

// PortalAccount represents a patient's login credentials for the public portal
type PortalAccount struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PatientID       uuid.UUID  `json:"patient_id" gorm:"type:uuid;not null;index"`
	Email           string     `json:"email" gorm:"type:varchar(255);not null;uniqueIndex"`
	PasswordHash    *string    `json:"-" gorm:"type:text"`        // null = hasn't created password yet
	EmailVerifiedAt *time.Time `json:"email_verified_at"`         // null = not verified
	CreatedAt       time.Time  `json:"created_at"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
}

func (PortalAccount) TableName() string {
	return "portal_accounts"
}
