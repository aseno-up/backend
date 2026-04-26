package model

import (
	"time"

	"github.com/google/uuid"
)

// PartnerNetwork represents a third-party partner network the clinic accepts
// (e.g., Gympass, Total Pass, Namu Pass, Gogood). Each network has one or more
// PartnerPlan rows with its own session-frequency limits.
type PartnerNetwork struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID  uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_partner_network_clinic_name"`
	Name      string     `json:"name" gorm:"size:100;not null;uniqueIndex:idx_partner_network_clinic_name"`
	IsActive  bool       `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`

	Plans []PartnerPlan `json:"plans,omitempty" gorm:"foreignKey:PartnerNetworkID"`
}

func (PartnerNetwork) TableName() string {
	return "partner_networks"
}

type PartnerNetworkResponse struct {
	ID        uuid.UUID              `json:"id"`
	ClinicID  uuid.UUID              `json:"clinic_id"`
	Name      string                 `json:"name"`
	IsActive  bool                   `json:"is_active"`
	Plans     []*PartnerPlanResponse `json:"plans,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

func (n *PartnerNetwork) ToResponse() *PartnerNetworkResponse {
	resp := &PartnerNetworkResponse{
		ID:        n.ID,
		ClinicID:  n.ClinicID,
		Name:      n.Name,
		IsActive:  n.IsActive,
		CreatedAt: n.CreatedAt,
	}
	if len(n.Plans) > 0 {
		resp.Plans = make([]*PartnerPlanResponse, len(n.Plans))
		for i := range n.Plans {
			plan := n.Plans[i]
			resp.Plans[i] = plan.ToResponse()
		}
	}
	return resp
}

type CreatePartnerNetworkRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}

type UpdatePartnerNetworkRequest struct {
	Name     *string `json:"name" binding:"omitempty,min=2,max=100"`
	IsActive *bool   `json:"is_active"`
}
