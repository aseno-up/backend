package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SalesPackagePayment represents one installment / payment row attached to a
// sale. A sale with three installments on a credit card creates three rows;
// a sale paid in cash up-front creates one. The finance hook upserts a
// FinancialTransaction per row so the cash-flow screen sees each event on the
// day it actually happens (DueDate for accounts receivable, PaidAt once
// settled).
type SalesPackagePayment struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	SalesPackageID uuid.UUID `json:"sales_package_id" gorm:"type:uuid;not null;index"`
	ClinicID       uuid.UUID `json:"clinic_id" gorm:"type:uuid;not null;index"`
	// Nullable: an open receivable (sale not yet paid) has no method until the
	// patient settles it. The user fills it in later via RegisterPayment.
	PaymentMethodID   *uuid.UUID     `json:"payment_method_id,omitempty" gorm:"type:uuid;index"`
	Amount            float64        `json:"amount" gorm:"type:decimal(12,2);not null"`
	Discount          float64        `json:"discount" gorm:"type:decimal(12,2);not null;default:0"`
	Installment       int            `json:"installment" gorm:"not null;default:1"`        // 1-based: 1, 2, 3...
	TotalInstallments int            `json:"total_installments" gorm:"not null;default:1"` // total parcels in this payment line (always 1 for non-credit)
	DueDate           time.Time      `json:"due_date" gorm:"type:date;not null;index"`
	PaidAt            *time.Time     `json:"paid_at,omitempty" gorm:"index"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

func (SalesPackagePayment) TableName() string {
	return "sales_package_payments"
}

// IsPaid reports whether the row has been settled.
func (p *SalesPackagePayment) IsPaid() bool {
	return p.PaidAt != nil
}

// CreateSalesPackagePaymentInput is one user-supplied payment line at sale time.
// Installments > 1 expand into N rows on persistence (one per parcel).
// PaymentMethodID is required when this payload is used directly — the
// service generates a method-less open receivable internally for any unpaid
// remainder.
type CreateSalesPackagePaymentInput struct {
	PaymentMethodID uuid.UUID  `json:"payment_method_id" binding:"required"`
	Amount          float64    `json:"amount" binding:"required,gt=0"`
	Discount        float64    `json:"discount" binding:"min=0"`
	Installments    int        `json:"installments" binding:"omitempty,min=1"` // defaults to 1; > 1 only valid for installments_allowed methods
	DueDate         *time.Time `json:"due_date"`                               // optional: when nil, falls back to sale_date + settlement_days
	PaidAt          *time.Time `json:"paid_at"`                                // optional: filled when received already
}

// RegisterPaymentRequest covers settling an existing scheduled payment.
type RegisterPaymentRequest struct {
	PaidAt          time.Time  `json:"paid_at" binding:"required"`
	PaymentMethodID *uuid.UUID `json:"payment_method_id"` // optional override
	Amount          *float64   `json:"amount" binding:"omitempty,gt=0"`
}

// SalesPackagePaymentResponse is the serialized view of one payment row.
type SalesPackagePaymentResponse struct {
	ID                uuid.UUID  `json:"id"`
	SalesPackageID    uuid.UUID  `json:"sales_package_id"`
	PaymentMethodID   *uuid.UUID `json:"payment_method_id,omitempty"`
	Amount            float64    `json:"amount"`
	Discount          float64    `json:"discount"`
	Installment       int        `json:"installment"`
	TotalInstallments int        `json:"total_installments"`
	DueDate           time.Time  `json:"due_date"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

func (p *SalesPackagePayment) ToResponse() *SalesPackagePaymentResponse {
	resp := &SalesPackagePaymentResponse{
		ID:                p.ID,
		SalesPackageID:    p.SalesPackageID,
		Amount:            p.Amount,
		Discount:          p.Discount,
		Installment:       p.Installment,
		TotalInstallments: p.TotalInstallments,
		DueDate:           p.DueDate,
		PaidAt:            p.PaidAt,
		CreatedAt:         p.CreatedAt,
	}
	if p.PaymentMethodID != nil {
		id := *p.PaymentMethodID
		resp.PaymentMethodID = &id
	}
	return resp
}
