package model

import (
	"time"

	"github.com/google/uuid"
)

// FeatureFlag represents a feature toggle
type FeatureFlag struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Key         string     `json:"key" gorm:"type:varchar(100);not null;uniqueIndex:idx_flag_key_clinic"`
	Description string     `json:"description" gorm:"type:varchar(500)"`
	Enabled     bool       `json:"enabled" gorm:"not null;default:false"`
	ClinicID    *uuid.UUID `json:"clinic_id,omitempty" gorm:"type:uuid;uniqueIndex:idx_flag_key_clinic"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName returns the table name
func (FeatureFlag) TableName() string {
	return "feature_flags"
}

// CreateFeatureFlagRequest represents the request to create a feature flag
type CreateFeatureFlagRequest struct {
	Key         string `json:"key" binding:"required"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	ClinicID    string `json:"clinic_id,omitempty"`
}

// UpdateFeatureFlagRequest represents the request to update a feature flag
type UpdateFeatureFlagRequest struct {
	Description *string `json:"description,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

// FeatureFlagResponse represents the response with all flags
type FeatureFlagResponse struct {
	Data []FeatureFlag `json:"data"`
}
