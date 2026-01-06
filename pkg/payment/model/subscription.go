package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Subscription represents a recurring subscription in Asaas
type Subscription struct {
	ID                   uuid.UUID          `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID             uuid.UUID          `json:"clinic_id" gorm:"type:uuid;not null;index"`
	CustomerID           uuid.UUID          `json:"customer_id" gorm:"type:uuid;not null;index"`
	AsaasSubscriptionID  string             `json:"asaas_subscription_id" gorm:"uniqueIndex;not null"`
	Value                float64            `json:"value" gorm:"not null"`
	Status               SubscriptionStatus `json:"status" gorm:"not null;default:'ACTIVE'"`
	BillingType          BillingType        `json:"billing_type" gorm:"not null"`
	Cycle                BillingCycle       `json:"cycle" gorm:"not null;default:'MONTHLY'"`
	NextDueDate          time.Time          `json:"next_due_date"`
	Description          string             `json:"description"`
	ExternalRef          string             `json:"external_ref"`
	EndDate              *time.Time         `json:"end_date,omitempty"`
	MaxPayments          *int               `json:"max_payments,omitempty"`
	CreatedAt            time.Time          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt            gorm.DeletedAt     `json:"-" gorm:"index"`

	// Relations
	Customer *Customer  `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`
	Payments []Payment  `json:"payments,omitempty" gorm:"foreignKey:SubscriptionID"`
}

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "ACTIVE"
	SubscriptionStatusInactive SubscriptionStatus = "INACTIVE"
	SubscriptionStatusExpired  SubscriptionStatus = "EXPIRED"
)

// BillingCycle represents the billing cycle
type BillingCycle string

const (
	BillingCycleWeekly     BillingCycle = "WEEKLY"
	BillingCycleBiweekly   BillingCycle = "BIWEEKLY"
	BillingCycleMonthly    BillingCycle = "MONTHLY"
	BillingCycleQuarterly  BillingCycle = "QUARTERLY"
	BillingCycleSemiannual BillingCycle = "SEMIANNUALLY"
	BillingCycleYearly     BillingCycle = "YEARLY"
)

// CreateSubscriptionRequest represents a request to create a subscription in Asaas
type CreateSubscriptionRequest struct {
	CustomerID    string       `json:"customer" binding:"required"`
	BillingType   BillingType  `json:"billingType" binding:"required"`
	Value         float64      `json:"value" binding:"required"`
	NextDueDate   string       `json:"nextDueDate" binding:"required"` // Format: YYYY-MM-DD
	Cycle         BillingCycle `json:"cycle" binding:"required"`
	Description   string       `json:"description"`
	ExternalRef   string       `json:"externalReference"`
	EndDate       string       `json:"endDate,omitempty"`
	MaxPayments   *int         `json:"maxPayments,omitempty"`
}

// AsaasSubscriptionResponse represents the response from Asaas API
type AsaasSubscriptionResponse struct {
	ID            string  `json:"id"`
	Customer      string  `json:"customer"`
	Value         float64 `json:"value"`
	Status        string  `json:"status"`
	BillingType   string  `json:"billingType"`
	Cycle         string  `json:"cycle"`
	NextDueDate   string  `json:"nextDueDate"`
	Description   string  `json:"description"`
	ExternalRef   string  `json:"externalReference"`
	EndDate       string  `json:"endDate,omitempty"`
	MaxPayments   *int    `json:"maxPayments,omitempty"`
	DateCreated   string  `json:"dateCreated"`
}

// SubscriptionListResponse represents a paginated list of subscriptions
type SubscriptionListResponse struct {
	Data       []AsaasSubscriptionResponse `json:"data"`
	HasMore    bool                        `json:"hasMore"`
	TotalCount int                         `json:"totalCount"`
	Limit      int                         `json:"limit"`
	Offset     int                         `json:"offset"`
}
