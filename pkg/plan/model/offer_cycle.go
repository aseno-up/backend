package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OfferCycle defines the discounted or custom pricing for an offer per billing cycle.
type OfferCycle struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OfferID        uuid.UUID      `json:"offer_id" gorm:"type:uuid;not null;index"`
	CycleType      CycleType      `json:"cycle_type" gorm:"not null"` // "monthly", "yearly"
	CustomPriceBRL float64        `json:"custom_price_brl" gorm:"not null"`
	CreatedAt      time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName returns the table name for GORM
func (OfferCycle) TableName() string {
	return "offer_cycles"
}
