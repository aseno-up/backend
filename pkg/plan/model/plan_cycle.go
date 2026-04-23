package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CycleType represents the billing cycle duration
type CycleType string

const (
	CycleTypeMonthly CycleType = "monthly"
	CycleTypeYearly  CycleType = "yearly"
)

// PlanCycle defines the pricing for a given plan per billing cycle.
type PlanCycle struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PlanID    uuid.UUID `json:"plan_id" gorm:"type:uuid;not null;index"`
	CycleType CycleType `json:"cycle_type" gorm:"not null"`
	PriceBRL  float64   `json:"price_brl" gorm:"not null"`
	// SoloPriceBRL is the discounted price when the customer subscribes as a solo
	// practitioner (single professional). Nil means this cycle does not offer solo pricing.
	SoloPriceBRL *float64       `json:"solo_price_brl,omitempty" gorm:"type:numeric(10,2)"`
	CreatedAt    time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName returns the table name for GORM
func (PlanCycle) TableName() string {
	return "plan_cycles"
}
