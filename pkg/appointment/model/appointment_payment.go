package model

import (
	"time"

	"github.com/google/uuid"
)

// PaymentStatus represents the payment status of an appointment
type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusPartial PaymentStatus = "partial"
	PaymentStatusPaid    PaymentStatus = "paid"
)

// AppointmentPayment represents a payment record for an appointment (1:N)
type AppointmentPayment struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID        uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	AppointmentID   uuid.UUID  `json:"appointment_id" gorm:"type:uuid;not null;index"`
	PaymentMethodID uuid.UUID  `json:"payment_method_id" gorm:"type:uuid;not null;index"`
	Amount          float64    `json:"amount" gorm:"type:decimal(10,2);not null"`
	Discount        float64    `json:"discount" gorm:"type:decimal(10,2);default:0"`
	Installments    int        `json:"installments" gorm:"default:1"`
	Notes           string     `json:"notes"`
	PaidAt          time.Time  `json:"paid_at" gorm:"not null"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty" gorm:"index"`

	// Relationships
	PaymentMethod *PaymentMethodInfo `json:"payment_method,omitempty" gorm:"foreignKey:PaymentMethodID;references:ID"`

	// Transient fields populated by ListBySessionPlan (read-only, not persisted)
	AppointmentStartTime *time.Time `json:"appointment_start_time,omitempty" gorm:"->"`
	AppointmentPrice     float64    `json:"appointment_price,omitempty" gorm:"->"`
}

func (AppointmentPayment) TableName() string {
	return "appointment_payments"
}

// PaymentMethodInfo is a slim version for embedding in responses
type PaymentMethodInfo struct {
	ID   uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	Name string    `json:"name"`
	Type string    `json:"type"`
}

func (PaymentMethodInfo) TableName() string {
	return "payment_methods"
}

// AppointmentPaymentResponse represents the API response
type AppointmentPaymentResponse struct {
	ID              uuid.UUID          `json:"id"`
	AppointmentID   uuid.UUID          `json:"appointment_id"`
	PaymentMethodID uuid.UUID          `json:"payment_method_id"`
	Amount          float64            `json:"amount"`
	Discount        float64            `json:"discount"`
	Installments    int                `json:"installments"`
	Notes           string             `json:"notes"`
	PaidAt          time.Time          `json:"paid_at"`
	PaymentMethod   *PaymentMethodInfo `json:"payment_method,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
}

func (p *AppointmentPayment) ToResponse() *AppointmentPaymentResponse {
	return &AppointmentPaymentResponse{
		ID:              p.ID,
		AppointmentID:   p.AppointmentID,
		PaymentMethodID: p.PaymentMethodID,
		Amount:          p.Amount,
		Discount:        p.Discount,
		Installments:    p.Installments,
		Notes:           p.Notes,
		PaidAt:          p.PaidAt,
		PaymentMethod:   p.PaymentMethod,
		CreatedAt:       p.CreatedAt,
	}
}

// CreateAppointmentPaymentRequest represents the request to create a payment
type CreateAppointmentPaymentRequest struct {
	PaymentMethodID uuid.UUID `json:"payment_method_id" binding:"required"`
	Amount          float64   `json:"amount" binding:"required,gt=0"`
	Discount        float64   `json:"discount"`
	Installments    int       `json:"installments"`
	Notes           string    `json:"notes"`
}
