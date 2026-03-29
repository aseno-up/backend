package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FeeType string

const (
	FeeTypePercentage FeeType = "percentage"
	FeeTypeFixed      FeeType = "fixed"
)

// SplitConfig defines how payments are split between clinic and Aseno
type SplitConfig struct {
	ID            uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID      uuid.UUID      `json:"clinic_id" gorm:"type:uuid;uniqueIndex;not null"`
	AsenoWalletID string         `json:"aseno_wallet_id" gorm:"not null"`
	FeeType       FeeType        `json:"fee_type" gorm:"not null;default:'percentage'"`
	FeeValue      float64        `json:"fee_value" gorm:"not null;default:0"` // 0 = no Aseno fee
	Active        bool           `json:"active" gorm:"not null;default:true"`
	CreatedAt     time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}
