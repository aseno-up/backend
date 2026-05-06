package model

import (
	"time"

	"github.com/google/uuid"
)

// PaymentMethodType categorizes a payment method for reports
type PaymentMethodType string

const (
	PaymentMethodTypePix      PaymentMethodType = "pix"
	PaymentMethodTypeCredit   PaymentMethodType = "credit"
	PaymentMethodTypeDebit    PaymentMethodType = "debit"
	PaymentMethodTypeCash     PaymentMethodType = "cash"
	PaymentMethodTypeTransfer PaymentMethodType = "transfer"
	PaymentMethodTypeOther    PaymentMethodType = "other"
)

// PaymentMethod represents a clinic's payment method (e.g., "Maquininha Stone", "PIX Banco Inter")
type PaymentMethod struct {
	ID                  uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID            uuid.UUID         `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Name                string            `json:"name" gorm:"not null"`
	Type                PaymentMethodType `json:"type" gorm:"not null;default:'other'"`
	Active              bool              `json:"active" gorm:"default:true"`
	SettlementDays      int               `json:"settlement_days" gorm:"not null;default:0"`
	InstallmentsAllowed bool              `json:"installments_allowed" gorm:"not null;default:false"`
	MaxInstallments     int               `json:"max_installments" gorm:"not null;default:1"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	DeletedAt           *time.Time        `json:"deleted_at,omitempty" gorm:"index"`
}

func (PaymentMethod) TableName() string {
	return "payment_methods"
}

// PaymentMethodResponse represents the API response
type PaymentMethodResponse struct {
	ID                  uuid.UUID         `json:"id"`
	ClinicID            uuid.UUID         `json:"clinic_id"`
	Name                string            `json:"name"`
	Type                PaymentMethodType `json:"type"`
	Active              bool              `json:"active"`
	SettlementDays      int               `json:"settlement_days"`
	InstallmentsAllowed bool              `json:"installments_allowed"`
	MaxInstallments     int               `json:"max_installments"`
	CreatedAt           time.Time         `json:"created_at"`
}

func (p *PaymentMethod) ToResponse() *PaymentMethodResponse {
	return &PaymentMethodResponse{
		ID:                  p.ID,
		ClinicID:            p.ClinicID,
		Name:                p.Name,
		Type:                p.Type,
		Active:              p.Active,
		SettlementDays:      p.SettlementDays,
		InstallmentsAllowed: p.InstallmentsAllowed,
		MaxInstallments:     p.MaxInstallments,
		CreatedAt:           p.CreatedAt,
	}
}

// CreatePaymentMethodRequest represents the request to create a payment method
type CreatePaymentMethodRequest struct {
	Name                string            `json:"name" binding:"required,min=2"`
	Type                PaymentMethodType `json:"type" binding:"required"`
	SettlementDays      *int              `json:"settlement_days" binding:"omitempty,min=0"`
	InstallmentsAllowed *bool             `json:"installments_allowed"`
	MaxInstallments     *int              `json:"max_installments" binding:"omitempty,min=1"`
}

// UpdatePaymentMethodRequest represents the request to update a payment method
type UpdatePaymentMethodRequest struct {
	Name                *string            `json:"name" binding:"omitempty,min=2"`
	Type                *PaymentMethodType `json:"type"`
	Active              *bool              `json:"active"`
	SettlementDays      *int               `json:"settlement_days" binding:"omitempty,min=0"`
	InstallmentsAllowed *bool              `json:"installments_allowed"`
	MaxInstallments     *int               `json:"max_installments" binding:"omitempty,min=1"`
}
