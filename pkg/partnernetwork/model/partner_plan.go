package model

import (
	"time"

	"github.com/google/uuid"
)

// PartnerPlan is a specific plan offered by a PartnerNetwork
// (e.g., "Silver" within Gympass, "Tp2" within Total Pass).
// MaxSessionsPerWeek caps how many group-session attendances a patient
// enrolled under this plan may have in a given week.
type PartnerPlan struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PartnerNetworkID   uuid.UUID  `json:"partner_network_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_partner_plan_network_name"`
	Name               string     `json:"name" gorm:"size:100;not null;uniqueIndex:idx_partner_plan_network_name"`
	MaxSessionsPerWeek *int       `json:"max_sessions_per_week"` // null = no limit
	IsActive           bool       `json:"is_active" gorm:"default:true"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

func (PartnerPlan) TableName() string {
	return "partner_plans"
}

type PartnerPlanResponse struct {
	ID                 uuid.UUID `json:"id"`
	PartnerNetworkID   uuid.UUID `json:"partner_network_id"`
	Name               string    `json:"name"`
	MaxSessionsPerWeek *int      `json:"max_sessions_per_week"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
}

func (p *PartnerPlan) ToResponse() *PartnerPlanResponse {
	return &PartnerPlanResponse{
		ID:                 p.ID,
		PartnerNetworkID:   p.PartnerNetworkID,
		Name:               p.Name,
		MaxSessionsPerWeek: p.MaxSessionsPerWeek,
		IsActive:           p.IsActive,
		CreatedAt:          p.CreatedAt,
	}
}

type CreatePartnerPlanRequest struct {
	PartnerNetworkID   uuid.UUID `json:"partner_network_id" binding:"required"`
	Name               string    `json:"name" binding:"required,min=1,max=100"`
	MaxSessionsPerWeek *int      `json:"max_sessions_per_week" binding:"omitempty,min=1,max=14"`
}

type UpdatePartnerPlanRequest struct {
	Name               *string `json:"name" binding:"omitempty,min=1,max=100"`
	MaxSessionsPerWeek *int    `json:"max_sessions_per_week" binding:"omitempty,min=1,max=14"`
	IsActive           *bool   `json:"is_active"`
}
