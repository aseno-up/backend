package model

import (
	"time"

	"github.com/google/uuid"

	user_model "github.com/aseno/backend/pkg/user/model"
)

// ClinicMember represents a user's membership in a clinic with a specific role
type ClinicMember struct {
	ID        uuid.UUID           `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID           `json:"user_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_user_clinic"`
	ClinicID  uuid.UUID           `json:"clinic_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_user_clinic"`
	Role      user_model.UserRole `json:"role" gorm:"not null;default:'receptionist'"`
	IsActive  bool                `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
	DeletedAt *time.Time          `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (ClinicMember) TableName() string {
	return "clinic_members"
}

// ClinicMemberResponse represents the API response for a clinic membership
type ClinicMemberResponse struct {
	ID       uuid.UUID           `json:"id"`
	UserID   uuid.UUID           `json:"user_id"`
	ClinicID uuid.UUID           `json:"clinic_id"`
	Role     user_model.UserRole `json:"role"`
	IsActive bool                `json:"is_active"`
}

// ToResponse converts ClinicMember to ClinicMemberResponse
func (m *ClinicMember) ToResponse() *ClinicMemberResponse {
	return &ClinicMemberResponse{
		ID:       m.ID,
		UserID:   m.UserID,
		ClinicID: m.ClinicID,
		Role:     m.Role,
		IsActive: m.IsActive,
	}
}
