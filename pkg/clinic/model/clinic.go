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

// Clinic represents a clinic/tenant in the system
type Clinic struct {
	ID                 uuid.UUID          `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name               string             `json:"name" gorm:"not null"`
	Slug               string             `json:"slug" gorm:"uniqueIndex;not null"`
	Email              string             `json:"email" gorm:"uniqueIndex;not null"`
	Phone              string             `json:"phone"`
	Document           string             `json:"document"` // CNPJ or CPF
	Address            string             `json:"address"`
	City               string             `json:"city"`
	State              string             `json:"state"`
	ZipCode            string             `json:"zip_code"`
	LogoURL            string             `json:"logo_url"`
	PrimaryColor       string             `json:"primary_color" gorm:"default:'#7C3AED'"`
	Plan               PlanType           `json:"plan" gorm:"default:'basic'"`
	SubscriptionStatus SubscriptionStatus `json:"subscription_status" gorm:"default:'trial'"`
	TrialEndsAt        *time.Time         `json:"trial_ends_at"`
	SubscriptionEndsAt *time.Time         `json:"subscription_ends_at"`
	AsaasCustomerID    string             `json:"-"` // Asaas customer ID (hidden from JSON)
	WhatsAppConnected  bool               `json:"whatsapp_connected" gorm:"default:false"`
	GoogleConnected    bool               `json:"google_connected" gorm:"default:false"`
	Settings           ClinicSettings     `json:"settings" gorm:"serializer:json"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	DeletedAt          *time.Time         `json:"deleted_at,omitempty" gorm:"index"`
}

// ClinicSettings contains clinic-specific settings
type ClinicSettings struct {
	Timezone                  string `json:"timezone"`
	DefaultAppointmentMinutes int    `json:"default_appointment_minutes"`
	ReminderHoursBefore       int    `json:"reminder_hours_before"`
	AllowOnlineBooking        bool   `json:"allow_online_booking"`
	RequireConfirmation       bool   `json:"require_confirmation"`
}

// TableName returns the table name for GORM
func (Clinic) TableName() string {
	return "clinics"
}

// ClinicResponse represents the API response for a clinic
type ClinicResponse struct {
	ID                 uuid.UUID          `json:"id"`
	Name               string             `json:"name"`
	Slug               string             `json:"slug"`
	Email              string             `json:"email"`
	Phone              string             `json:"phone"`
	Address            string             `json:"address"`
	City               string             `json:"city"`
	State              string             `json:"state"`
	LogoURL            string             `json:"logo_url"`
	PrimaryColor       string             `json:"primary_color"`
	Plan               PlanType           `json:"plan"`
	SubscriptionStatus SubscriptionStatus `json:"subscription_status"`
	TrialEndsAt        *time.Time         `json:"trial_ends_at,omitempty"`
	WhatsAppConnected  bool               `json:"whatsapp_connected"`
	GoogleConnected    bool               `json:"google_connected"`
	Settings           ClinicSettings     `json:"settings"`
	CreatedAt          time.Time          `json:"created_at"`
}

// ToResponse converts Clinic to ClinicResponse
func (c *Clinic) ToResponse() *ClinicResponse {
	return &ClinicResponse{
		ID:                 c.ID,
		Name:               c.Name,
		Slug:               c.Slug,
		Email:              c.Email,
		Phone:              c.Phone,
		Address:            c.Address,
		City:               c.City,
		State:              c.State,
		LogoURL:            c.LogoURL,
		PrimaryColor:       c.PrimaryColor,
		Plan:               c.Plan,
		SubscriptionStatus: c.SubscriptionStatus,
		TrialEndsAt:        c.TrialEndsAt,
		WhatsAppConnected:  c.WhatsAppConnected,
		GoogleConnected:    c.GoogleConnected,
		Settings:           c.Settings,
		CreatedAt:          c.CreatedAt,
	}
}

// IsSubscriptionActive checks if the clinic has an active subscription
func (c *Clinic) IsSubscriptionActive() bool {
	switch c.SubscriptionStatus {
	case SubscriptionStatusActive:
		return true
	case SubscriptionStatusTrial:
		if c.TrialEndsAt != nil {
			return time.Now().Before(*c.TrialEndsAt)
		}
		return false
	default:
		return false
	}
}

// CreateClinicRequest represents the request to create a clinic
type CreateClinicRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone"`
	Document string `json:"document"`
}

// UpdateClinicRequest represents the request to update a clinic
type UpdateClinicRequest struct {
	Name         string          `json:"name" binding:"omitempty,min=2,max=100"`
	Phone        string          `json:"phone"`
	Address      string          `json:"address"`
	City         string          `json:"city"`
	State        string          `json:"state"`
	ZipCode      string          `json:"zip_code"`
	LogoURL      string          `json:"logo_url"`
	PrimaryColor string          `json:"primary_color"`
	Settings     *ClinicSettings `json:"settings"`
}
