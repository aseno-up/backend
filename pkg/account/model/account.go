package model

import (
	"time"

	"github.com/google/uuid"
)

// SubscriptionStatus represents the subscription state
type SubscriptionStatus string

const (
	SubscriptionStatusTrial    SubscriptionStatus = "trial"
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusPastDue  SubscriptionStatus = "past_due"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusExpired  SubscriptionStatus = "expired"
)

// PlanType represents the subscription plan
type PlanType string

const (
	PlanTypeBasic        PlanType = "basic"
	PlanTypeProfessional PlanType = "professional"
	PlanTypeAdvanced     PlanType = "advanced"
)

// Account represents a multi-tenant billing profile
type Account struct {
	ID                 uuid.UUID          `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OwnerID            uuid.UUID          `json:"owner_id" gorm:"type:uuid;not null;index"`
	AsaasCustomerID    string             `json:"-"`
	Plan               PlanType           `json:"plan" gorm:"default:'basic'"`
	SubscriptionStatus SubscriptionStatus `json:"subscription_status" gorm:"default:'trial'"`
	TrialEndsAt        *time.Time         `json:"trial_ends_at"`
	SubscriptionEndsAt *time.Time         `json:"subscription_ends_at"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	DeletedAt          *time.Time         `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (Account) TableName() string {
	return "accounts"
}

// IsSubscriptionActive checks if the account has an active subscription
func (a *Account) IsSubscriptionActive() bool {
	switch a.SubscriptionStatus {
	case SubscriptionStatusActive:
		return true
	case SubscriptionStatusTrial:
		if a.TrialEndsAt != nil {
			return time.Now().Before(*a.TrialEndsAt)
		}
		return false
	default:
		return false
	}
}

// AccountResponse represents the API response for an account
type AccountResponse struct {
	ID                 uuid.UUID          `json:"id"`
	OwnerID            uuid.UUID          `json:"owner_id"`
	Plan               PlanType           `json:"plan"`
	SubscriptionStatus SubscriptionStatus `json:"subscription_status"`
	TrialEndsAt        *time.Time         `json:"trial_ends_at,omitempty"`
	SubscriptionEndsAt *time.Time         `json:"subscription_ends_at,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
}

// ToResponse converts Account to AccountResponse
func (a *Account) ToResponse() *AccountResponse {
	return &AccountResponse{
		ID:                 a.ID,
		OwnerID:            a.OwnerID,
		Plan:               a.Plan,
		SubscriptionStatus: a.SubscriptionStatus,
		TrialEndsAt:        a.TrialEndsAt,
		SubscriptionEndsAt: a.SubscriptionEndsAt,
		CreatedAt:          a.CreatedAt,
	}
}
