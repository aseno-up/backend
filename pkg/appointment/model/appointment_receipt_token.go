package model

import (
	"time"

	"github.com/google/uuid"
)

// AppointmentReceiptToken is the opaque token used to build the public receipt
// link shared with patients (`/recibo/:token`). One token per appointment is
// enforced at the database level so the table can be extended later (expiry,
// rotation, multiple tokens per appointment) without migrating existing rows.
type AppointmentReceiptToken struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID      uuid.UUID `json:"clinic_id" gorm:"type:uuid;not null;index"`
	AppointmentID uuid.UUID `json:"appointment_id" gorm:"type:uuid;not null;uniqueIndex"`
	Token         uuid.UUID `json:"token" gorm:"type:uuid;not null;uniqueIndex;default:gen_random_uuid()"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (AppointmentReceiptToken) TableName() string {
	return "appointment_receipt_tokens"
}

// EnsureTokenResponse is returned by the authenticated endpoint that creates
// or retrieves the share token for an appointment.
type EnsureTokenResponse struct {
	Token     uuid.UUID `json:"token"`
	PublicURL string    `json:"public_url"`
}

// PublicReceiptProcedure is one procedure line on the public receipt.
type PublicReceiptProcedure struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// PublicReceiptPayment is one payment line on the public receipt.
type PublicReceiptPayment struct {
	Method   string    `json:"method"`
	Amount   float64   `json:"amount"`
	Discount float64   `json:"discount"`
	PaidAt   time.Time `json:"paid_at"`
}

// PublicReceiptClinic is the clinic identification block shown on the receipt.
// Only data that is safe to expose publicly (no internal IDs, no user data).
type PublicReceiptClinic struct {
	Name     string `json:"name"`
	Document string `json:"document,omitempty"`
	Address  string `json:"address,omitempty"`
	Phone    string `json:"phone,omitempty"`
}

// PublicReceiptResponse is the full payload rendered on the public /recibo
// page. It never contains sensitive data (CPF, diagnosis, phone, notes, IDs).
type PublicReceiptResponse struct {
	ReceiptNumber    string                   `json:"receipt_number"`
	Clinic           PublicReceiptClinic      `json:"clinic"`
	PatientName      string                   `json:"patient_name"`
	ProfessionalName string                   `json:"professional_name,omitempty"`
	AppointmentDate  time.Time                `json:"appointment_date"`
	Procedures       []PublicReceiptProcedure `json:"procedures"`
	Subtotal         float64                  `json:"subtotal"`
	Discount         float64                  `json:"discount"`
	Total            float64                  `json:"total"`
	Payments         []PublicReceiptPayment   `json:"payments"`
	TotalPaid        float64                  `json:"total_paid"`
	BalanceDue       float64                  `json:"balance_due"`
	GeneratedAt      time.Time                `json:"generated_at"`
}
