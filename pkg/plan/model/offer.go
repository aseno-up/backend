package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Offer represents a promotional coupon or special offer tied to a specific plan.
type Offer struct {
	ID                     uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PlanID                 uuid.UUID      `json:"plan_id" gorm:"type:uuid;not null;index"`
	Plan                   *Plan          `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
	Token                  string         `json:"token" gorm:"uniqueIndex;not null"`
	IsActive               bool           `json:"is_active" gorm:"not null;default:true"`
	ValidFrom              *time.Time     `json:"valid_from"`
	ValidUntil             *time.Time     `json:"valid_until"`
	MaxUses                *int           `json:"max_uses"` // nil means infinite
	CurrentUses            int            `json:"current_uses" gorm:"not null;default:0"`
	CustomMaxProfessionals *int           `json:"custom_max_professionals"`
	CustomMaxClinics       *int           `json:"custom_max_clinics"`
	CreatedAt              time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt              gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	Cycles []OfferCycle `json:"cycles,omitempty" gorm:"foreignKey:OfferID"`
}

// TableName returns the table name for GORM
func (Offer) TableName() string {
	return "offers"
}

// IsValid checks the invariants INV-003 for Offer validity.
func (o *Offer) IsValid() bool {
	if !o.IsActive {
		return false
	}
	now := time.Now()
	if o.ValidFrom != nil && now.Before(*o.ValidFrom) {
		return false
	}
	if o.ValidUntil != nil && now.After(*o.ValidUntil) {
		return false
	}
	if o.MaxUses != nil && *o.MaxUses > 0 && o.CurrentUses >= *o.MaxUses {
		return false
	}
	return true
}
