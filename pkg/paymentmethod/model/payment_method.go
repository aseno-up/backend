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
	ID        uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID  uuid.UUID         `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Name      string            `json:"name" gorm:"not null"`
	Type      PaymentMethodType `json:"type" gorm:"not null;default:'other'"`
	Active    bool              `json:"active" gorm:"default:true"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	DeletedAt *time.Time        `json:"deleted_at,omitempty" gorm:"index"`
}

func (PaymentMethod) TableName() string {
	return "payment_methods"
}

// PaymentMethodResponse represents the API response
type PaymentMethodResponse struct {
	ID        uuid.UUID         `json:"id"`
	ClinicID  uuid.UUID         `json:"clinic_id"`
	Name      string            `json:"name"`
	Type      PaymentMethodType `json:"type"`
	Active    bool              `json:"active"`
	CreatedAt time.Time         `json:"created_at"`
}

func (p *PaymentMethod) ToResponse() *PaymentMethodResponse {
	return &PaymentMethodResponse{
		ID:        p.ID,
		ClinicID:  p.ClinicID,
		Name:      p.Name,
		Type:      p.Type,
		Active:    p.Active,
		CreatedAt: p.CreatedAt,
	}
}

// CreatePaymentMethodRequest represents the request to create a payment method
type CreatePaymentMethodRequest struct {
	Name string            `json:"name" binding:"required,min=2"`
	Type PaymentMethodType `json:"type" binding:"required"`
}

// UpdatePaymentMethodRequest represents the request to update a payment method
type UpdatePaymentMethodRequest struct {
	Name   *string            `json:"name" binding:"omitempty,min=2"`
	Type   *PaymentMethodType `json:"type"`
	Active *bool              `json:"active"`
}
