package model

import (
	"time"

	"github.com/google/uuid"
)

// GroupSessionProfessional tracks which professional is assigned to a specific
// group session date. For fixed-professional plans, this is auto-populated.
// For flexible-professional plans, the user selects the professional per session.
type GroupSessionProfessional struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClinicID        uuid.UUID `json:"clinic_id" gorm:"type:uuid;not null;index"`
	SessionPlanID   uuid.UUID `json:"session_plan_id" gorm:"type:uuid;not null;uniqueIndex:idx_gsp_plan_date"`
	SessionDate     string    `json:"session_date" gorm:"size:10;not null;uniqueIndex:idx_gsp_plan_date"` // YYYY-MM-DD
	ProfessionalID  uuid.UUID `json:"professional_id" gorm:"type:uuid;not null;index"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations (for response, not persisted)
	ProfessionalName string `json:"professional_name,omitempty" gorm:"-"`
}

// TableName returns the table name
func (GroupSessionProfessional) TableName() string {
	return "group_session_professionals"
}

// SetSessionProfessionalRequest represents the request to assign a professional to a group session
type SetSessionProfessionalRequest struct {
	ProfessionalID uuid.UUID `json:"professional_id" binding:"required"`
}

// SessionProfessionalResponse represents the API response
type SessionProfessionalResponse struct {
	SessionPlanID    uuid.UUID `json:"session_plan_id"`
	SessionDate      string    `json:"session_date"`
	ProfessionalID   uuid.UUID `json:"professional_id"`
	ProfessionalName string    `json:"professional_name,omitempty"`
}
