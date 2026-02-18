package model

import (
	"time"

	user_model "github.com/aseno/backend/pkg/user/model"
	"github.com/google/uuid"
)

// InviteStatus represents the status of an invitation
type InviteStatus string

const (
	InviteStatusPending  InviteStatus = "pending"
	InviteStatusAccepted InviteStatus = "accepted"
	InviteStatusExpired  InviteStatus = "expired"
	InviteStatusCanceled InviteStatus = "canceled"
)

// Invite represents an invitation to join a clinic
type Invite struct {
	ID         uuid.UUID           `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID   uuid.UUID           `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Email      string              `json:"email" gorm:"not null;index"`
	Role       user_model.UserRole `json:"role" gorm:"not null"`
	Token      string              `json:"-" gorm:"uniqueIndex;not null"`
	Status     InviteStatus        `json:"status" gorm:"default:'pending';index"`
	InvitedBy  uuid.UUID           `json:"invited_by" gorm:"type:uuid;not null"`
	ExpiresAt  time.Time           `json:"expires_at" gorm:"not null"`
	AcceptedAt *time.Time          `json:"accepted_at,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// TableName returns the table name for GORM
func (Invite) TableName() string {
	return "invites"
}

// IsExpired checks if the invite has expired
func (i *Invite) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// IsValid checks if the invite can be accepted
func (i *Invite) IsValid() bool {
	return i.Status == InviteStatusPending && !i.IsExpired()
}

// InviteResponse represents the API response for an invite
type InviteResponse struct {
	ID        uuid.UUID           `json:"id"`
	ClinicID  uuid.UUID           `json:"clinic_id"`
	Email     string              `json:"email"`
	Role      user_model.UserRole `json:"role"`
	Status    InviteStatus        `json:"status"`
	ExpiresAt time.Time           `json:"expires_at"`
	CreatedAt time.Time           `json:"created_at"`
}

// ToResponse converts Invite to InviteResponse
func (i *Invite) ToResponse() *InviteResponse {
	return &InviteResponse{
		ID:        i.ID,
		ClinicID:  i.ClinicID,
		Email:     i.Email,
		Role:      i.Role,
		Status:    i.Status,
		ExpiresAt: i.ExpiresAt,
		CreatedAt: i.CreatedAt,
	}
}

// InviteWithClinic includes clinic name for display
type InviteWithClinic struct {
	Invite
	ClinicName  string `json:"clinic_name"`
	InviterName string `json:"inviter_name"`
}

// CreateInviteRequest represents the request to create an invite
type CreateInviteRequest struct {
	Email string              `json:"email" binding:"required,email"`
	Role  user_model.UserRole `json:"role" binding:"required,oneof=admin professional receptionist"`
}

// AcceptInviteRequest represents the request to accept an invite
type AcceptInviteRequest struct {
	Token    string `json:"token" binding:"required"`
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Password string `json:"password" binding:"required,min=8"`
}

// InviteDetailsResponse is returned when fetching invite details by token
type InviteDetailsResponse struct {
	ID          uuid.UUID           `json:"id"`
	Email       string              `json:"email"`
	Role        user_model.UserRole `json:"role"`
	ClinicName  string              `json:"clinic_name"`
	InviterName string              `json:"inviter_name"`
	ExpiresAt   time.Time           `json:"expires_at"`
	IsValid     bool                `json:"is_valid"`
	IsExpired   bool                `json:"is_expired"`
}

// InviteListResponse represents a paginated list of invites
type InviteListResponse struct {
	Data       []*InviteResponse `json:"data"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PerPage    int               `json:"per_page"`
	TotalPages int               `json:"total_pages"`
}
