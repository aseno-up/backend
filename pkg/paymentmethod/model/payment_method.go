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

// SettlementMode determines how the bank pays the clinic for an installment
// sale.
//
// "upfront": the acquirer pays the entire sale as a single payout on
// saleDate + settlement_days, regardless of how many installments the
// customer chose. Produces ONE receivable row.
//
// "monthly": parcel 1 lands on saleDate + settlement_days; each subsequent
// parcel falls one calendar month after the previous one. Calendar-aligned
// so a small settlement_days (e.g. 3) doesn't accidentally schedule 12
// parcels inside one month.
//
// Defaults to "monthly". Single-payment methods (PIX, cash) ignore this
// field entirely.
//
// "fixed_days" is a legacy value: rows persisted before the settlement-mode
// rework may still carry it. The service treats it identically to
// "monthly" — the API no longer accepts it on create/update.
type SettlementMode string

const (
	SettlementModeUpfront   SettlementMode = "upfront"
	SettlementModeMonthly   SettlementMode = "monthly"
	SettlementModeFixedDays SettlementMode = "fixed_days" // legacy — see comment above
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
	SettlementMode      SettlementMode    `json:"settlement_mode" gorm:"type:varchar(20);not null;default:'monthly'"`
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
	SettlementMode      SettlementMode    `json:"settlement_mode"`
	CreatedAt           time.Time         `json:"created_at"`
}

func (p *PaymentMethod) ToResponse() *PaymentMethodResponse {
	// Surface legacy "fixed_days" rows as "monthly" — the API only accepts
	// "upfront" or "monthly" now, and behavior-wise fixed_days collapsed
	// into monthly during the settlement-mode rework.
	mode := p.SettlementMode
	if mode == "" || mode == SettlementModeFixedDays {
		mode = SettlementModeMonthly
	}
	return &PaymentMethodResponse{
		ID:                  p.ID,
		ClinicID:            p.ClinicID,
		Name:                p.Name,
		Type:                p.Type,
		Active:              p.Active,
		SettlementDays:      p.SettlementDays,
		InstallmentsAllowed: p.InstallmentsAllowed,
		MaxInstallments:     p.MaxInstallments,
		SettlementMode:      mode,
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
	SettlementMode      *SettlementMode   `json:"settlement_mode" binding:"omitempty,oneof=upfront monthly"`
}

// UpdatePaymentMethodRequest represents the request to update a payment method
type UpdatePaymentMethodRequest struct {
	Name                *string            `json:"name" binding:"omitempty,min=2"`
	Type                *PaymentMethodType `json:"type"`
	Active              *bool              `json:"active"`
	SettlementDays      *int               `json:"settlement_days" binding:"omitempty,min=0"`
	InstallmentsAllowed *bool              `json:"installments_allowed"`
	MaxInstallments     *int               `json:"max_installments" binding:"omitempty,min=1"`
	SettlementMode      *SettlementMode    `json:"settlement_mode" binding:"omitempty,oneof=upfront monthly"`
}
