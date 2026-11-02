package model

import (
	"time"

	"github.com/google/uuid"

	plan_model "github.com/aseno/backend/pkg/plan/model"
)

// SubscriptionStatus represents the subscription state
type SubscriptionStatus string

const (
	SubscriptionStatusTrial    SubscriptionStatus = "trial"
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusPending  SubscriptionStatus = "pending"
	SubscriptionStatusPastDue  SubscriptionStatus = "past_due"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusExpired  SubscriptionStatus = "expired"
)

// PlanType represents the subscription plan
type PlanType string

// Deprecated: Migrating to dynamic Plan entity instead
const (
	PlanTypeBasic        PlanType = "basic"
	PlanTypeProfessional PlanType = "professional"
	PlanTypeAdvanced     PlanType = "advanced"
)

// Account represents a multi-tenant billing profile
type Account struct {
	ID                     uuid.UUID              `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OwnerID                uuid.UUID              `json:"owner_id" gorm:"type:uuid;not null;index"`
	AsaasCustomerID        string                 `json:"-"`
	PlanID                 *uuid.UUID             `json:"plan_id,omitempty" gorm:"type:uuid;index"`
	Plan                   *plan_model.Plan       `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
	PlanVariant            plan_model.PlanVariant `json:"plan_variant" gorm:"type:varchar(16);not null;default:''"`
	SubscriptionStatus     SubscriptionStatus     `json:"subscription_status" gorm:"default:'trial'"`
	TrialEndsAt            *time.Time             `json:"trial_ends_at"`
	SubscriptionEndsAt     *time.Time             `json:"subscription_ends_at"`
	MaxClinics             int                    `json:"max_clinics"`
	MaxProfessionals       int                    `json:"max_professionals"`
	MaxWhatsappConnections int                    `json:"max_whatsapp_connections"`
	MaxGoogleCalendars     int                    `json:"max_google_calendars"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
	DeletedAt              *time.Time             `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (Account) TableName() string {
	return "accounts"
}

// IsSubscriptionActive checks if the account has an active subscription
// pending is treated as active so the user can reach billing pages to complete payment
func (a *Account) IsSubscriptionActive() bool {
	switch a.SubscriptionStatus {
	case SubscriptionStatusActive, SubscriptionStatusPending:
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
	ID                     uuid.UUID              `json:"id"`
	OwnerID                uuid.UUID              `json:"owner_id"`
	Plan                   *plan_model.Plan       `json:"plan,omitempty"`
	PlanVariant            plan_model.PlanVariant `json:"plan_variant"`
	SubscriptionStatus     SubscriptionStatus     `json:"subscription_status"`
	TrialEndsAt            *time.Time             `json:"trial_ends_at,omitempty"`
	SubscriptionEndsAt     *time.Time             `json:"subscription_ends_at,omitempty"`
	MaxClinics             int                    `json:"max_clinics"`
	MaxProfessionals       int                    `json:"max_professionals"`
	MaxWhatsappConnections int                    `json:"max_whatsapp_connections"`
	MaxGoogleCalendars     int                    `json:"max_google_calendars"`
	CreatedAt              time.Time              `json:"created_at"`
}

// ToResponse converts Account to AccountResponse
func (a *Account) ToResponse() *AccountResponse {
	return &AccountResponse{
		ID:                     a.ID,
		OwnerID:                a.OwnerID,
		Plan:                   a.Plan,
		PlanVariant:            a.PlanVariant,
		SubscriptionStatus:     a.SubscriptionStatus,
		TrialEndsAt:            a.TrialEndsAt,
		SubscriptionEndsAt:     a.SubscriptionEndsAt,
		MaxClinics:             a.MaxClinics,
		MaxProfessionals:       a.MaxProfessionals,
		MaxWhatsappConnections: a.MaxWhatsappConnections,
		MaxGoogleCalendars:     a.MaxGoogleCalendars,
		CreatedAt:              a.CreatedAt,
	}
}
