package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SessionPlanPayment represents a payment made for a session plan (upfront or monthly)
type SessionPlanPayment struct {
	ID              uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClinicID        uuid.UUID      `json:"clinic_id" gorm:"type:uuid;not null;index"`
	SessionPlanID   uuid.UUID      `json:"session_plan_id" gorm:"type:uuid;not null;index"`
	EnrollmentID    *uuid.UUID     `json:"enrollment_id,omitempty" gorm:"type:uuid;index"` // For group upfront/monthly
	PaymentMethodID uuid.UUID      `json:"payment_method_id" gorm:"type:uuid;not null"`
	Amount          float64        `json:"amount" gorm:"type:decimal(10,2);not null"`
	Discount        float64        `json:"discount" gorm:"type:decimal(10,2);default:0"`
	Installments    int            `json:"installments" gorm:"default:1"`
	ReferenceMonth  string         `json:"reference_month,omitempty" gorm:"size:7"` // YYYY-MM for monthly
	Notes           string         `json:"notes" gorm:"type:text"`
	PaidAt          time.Time      `json:"paid_at" gorm:"not null"`
	CreatedAt       time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations (for response)
	PaymentMethodName string `json:"payment_method_name,omitempty" gorm:"-"`

	// Transient fields for per-session aggregation
	AppointmentID        *uuid.UUID `json:"-" gorm:"-"`
	AppointmentStartTime *time.Time `json:"-" gorm:"-"`
	AppointmentPrice     float64    `json:"-" gorm:"-"`
}

// TableName returns the table name
func (SessionPlanPayment) TableName() string {
	return "session_plan_payments"
}

// CreatePlanPaymentRequest represents the request to create a plan payment
type CreatePlanPaymentRequest struct {
	PaymentMethodID uuid.UUID `json:"payment_method_id" binding:"required"`
	Amount          float64   `json:"amount" binding:"required,min=0.01"`
	Discount        float64   `json:"discount"`
	Installments    int       `json:"installments"`
	ReferenceMonth  string    `json:"reference_month"` // YYYY-MM for monthly
	Notes           string    `json:"notes"`
}

// PlanPaymentResponse represents the API response
type PlanPaymentResponse struct {
	ID                uuid.UUID  `json:"id"`
	SessionPlanID     uuid.UUID  `json:"session_plan_id"`
	EnrollmentID      *uuid.UUID `json:"enrollment_id,omitempty"`
	PaymentMethodID   uuid.UUID  `json:"payment_method_id"`
	Amount            float64    `json:"amount"`
	Discount          float64    `json:"discount"`
	Installments      int        `json:"installments"`
	ReferenceMonth    string     `json:"reference_month,omitempty"`
	Notes             string     `json:"notes,omitempty"`
	PaidAt            time.Time  `json:"paid_at"`
	PaymentMethodName string     `json:"payment_method_name,omitempty"`

	// Per-session fields (populated from appointment data)
	AppointmentID        *uuid.UUID `json:"appointment_id,omitempty"`
	AppointmentStartTime *time.Time `json:"appointment_start_time,omitempty"`
	AppointmentPrice     float64    `json:"appointment_price,omitempty"`
}

// ToResponse converts to API response
func (p *SessionPlanPayment) ToResponse() *PlanPaymentResponse {
	return &PlanPaymentResponse{
		ID:                   p.ID,
		SessionPlanID:        p.SessionPlanID,
		EnrollmentID:         p.EnrollmentID,
		PaymentMethodID:      p.PaymentMethodID,
		Amount:               p.Amount,
		Discount:             p.Discount,
		Installments:         p.Installments,
		ReferenceMonth:       p.ReferenceMonth,
		Notes:                p.Notes,
		PaidAt:               p.PaidAt,
		PaymentMethodName:    p.PaymentMethodName,
		AppointmentID:        p.AppointmentID,
		AppointmentStartTime: p.AppointmentStartTime,
		AppointmentPrice:     p.AppointmentPrice,
	}
}
