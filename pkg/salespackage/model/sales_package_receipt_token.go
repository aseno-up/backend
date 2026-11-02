package model

import (
	"time"

	"github.com/google/uuid"
)

// SalesPackageReceiptToken is the opaque token used to build the public
// receipt link shared with patients (`/recibo/venda/:token`). One token per
// sales_package is enforced at the database level.
type SalesPackageReceiptToken struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID       uuid.UUID `json:"clinic_id" gorm:"type:uuid;not null;index"`
	SalesPackageID uuid.UUID `json:"sales_package_id" gorm:"type:uuid;not null;uniqueIndex"`
	Token          uuid.UUID `json:"token" gorm:"type:uuid;not null;uniqueIndex;default:gen_random_uuid()"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SalesPackageReceiptToken) TableName() string {
	return "sales_package_receipt_tokens"
}

// EnsureReceiptTokenResponse is returned by the authenticated endpoint that
// creates or retrieves the share token for a sale.
type EnsureReceiptTokenResponse struct {
	Token     uuid.UUID `json:"token"`
	PublicURL string    `json:"public_url"`
}

// PublicReceiptItem is one item line on the public sales receipt.
type PublicReceiptItem struct {
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	LineTotal float64 `json:"line_total"`
}

// PublicReceiptPayment is one realized payment line on the public receipt.
// Method is empty for open-receivable rows (unpaid remainder).
type PublicReceiptPayment struct {
	Method   string     `json:"method"`
	Amount   float64    `json:"amount"`
	Discount float64    `json:"discount"`
	PaidAt   *time.Time `json:"paid_at,omitempty"`
	DueDate  time.Time  `json:"due_date"`
}

// PublicReceiptClinic mirrors the appointment receipt's clinic block.
type PublicReceiptClinic struct {
	Name     string `json:"name"`
	Document string `json:"document,omitempty"`
	Address  string `json:"address,omitempty"`
	Phone    string `json:"phone,omitempty"`
}

// PublicSalesReceiptResponse is the payload rendered on the public receipt
// page for a sale. Keeps the same safety guarantees as the appointment
// receipt: no CPF, no notes, no internal IDs.
type PublicSalesReceiptResponse struct {
	ReceiptNumber string                 `json:"receipt_number"`
	Code          string                 `json:"code"`
	Status        SalesPackageStatus     `json:"status"`
	Clinic        PublicReceiptClinic    `json:"clinic"`
	PatientName   string                 `json:"patient_name"`
	SaleDate      time.Time              `json:"sale_date"`
	ValidUntil    *time.Time             `json:"valid_until,omitempty"`
	Items         []PublicReceiptItem    `json:"items"`
	Subtotal      float64                `json:"subtotal"`
	Discount      float64                `json:"discount"`
	Total         float64                `json:"total"`
	Payments      []PublicReceiptPayment `json:"payments"`
	TotalPaid     float64                `json:"total_paid"`
	BalanceDue    float64                `json:"balance_due"`
	GeneratedAt   time.Time              `json:"generated_at"`
}
