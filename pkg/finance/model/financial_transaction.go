package model

import (
	"time"

	"github.com/google/uuid"
)

// TransactionType classifies a transaction as money coming in or going out.
type TransactionType string

const (
	TransactionTypeIncome  TransactionType = "income"
	TransactionTypeExpense TransactionType = "expense"
)

// TransactionSource tracks where the transaction originated from.
// 'manual' is user-entered; 'appointment'/'session_plan' are generated
// automatically when a related entity is paid.
type TransactionSource string

const (
	SourceManual      TransactionSource = "manual"
	SourceAppointment TransactionSource = "appointment"
	SourceSessionPlan TransactionSource = "session_plan"
	SourceCommission  TransactionSource = "commission"
)

// FinancialTransaction represents a single cash-flow event (entrada ou saída)
// for a clinic. Amount is always stored positive — direction is given by Type.
type FinancialTransaction struct {
	ID          uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID    uuid.UUID         `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Type        TransactionType   `json:"type" gorm:"type:varchar(10);not null;index"`
	CategoryID  *uuid.UUID        `json:"category_id,omitempty" gorm:"type:uuid;index"`
	Amount      float64           `json:"amount" gorm:"type:decimal(12,2);not null"`
	Description string            `json:"description" gorm:"not null"`
	Source      TransactionSource `json:"source" gorm:"type:varchar(20);not null;default:'manual';index"`

	AppointmentID        *uuid.UUID `json:"appointment_id,omitempty" gorm:"type:uuid;index"`
	AppointmentPaymentID *uuid.UUID `json:"appointment_payment_id,omitempty" gorm:"type:uuid;index"` // one finance row per appointment_payment event
	SessionPlanID        *uuid.UUID `json:"session_plan_id,omitempty" gorm:"type:uuid;index"`
	SessionPlanPaymentID *uuid.UUID `json:"session_plan_payment_id,omitempty" gorm:"type:uuid;index"` // one finance row per session_plan_payment event
	ProfessionalID       *uuid.UUID `json:"professional_id,omitempty" gorm:"type:uuid;index"`         // set for source='commission'

	CompetenceDate  time.Time  `json:"competence_date" gorm:"type:date;not null;index"`
	DueDate         *time.Time `json:"due_date,omitempty" gorm:"type:date"`
	PaidAt          *time.Time `json:"paid_at,omitempty" gorm:"index"`
	PaymentMethodID *uuid.UUID `json:"payment_method_id,omitempty" gorm:"type:uuid;index"`

	CreatedBy *uuid.UUID `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`

	// Loaded via Preload so the Lançamentos screen can show name/color.
	Category *FinancialCategory `json:"-" gorm:"foreignKey:CategoryID;references:ID"`
}

func (FinancialTransaction) TableName() string {
	return "financial_transactions"
}

// IsPaid reports whether the transaction has already been settled.
// A transaction is "paid" whenever PaidAt is filled — in the MVP every
// realized cash movement fills PaidAt at creation time, so the flag is
// effectively a proxy for "not a pending bill (conta a pagar/receber)".
func (t *FinancialTransaction) IsPaid() bool {
	return t.PaidAt != nil
}

// CreateTransactionRequest is the payload for manual entry creation.
type CreateTransactionRequest struct {
	Type            TransactionType `json:"type" binding:"required,oneof=income expense"`
	CategoryID      *uuid.UUID      `json:"category_id"`
	Amount          float64         `json:"amount" binding:"required,gt=0"`
	Description     string          `json:"description" binding:"required"`
	CompetenceDate  time.Time       `json:"competence_date" binding:"required"`
	DueDate         *time.Time      `json:"due_date"`
	PaidAt          *time.Time      `json:"paid_at"`
	PaymentMethodID *uuid.UUID      `json:"payment_method_id"`
}

// UpdateTransactionRequest is the payload for manual entry update.
// Only manual transactions are editable via the API.
type UpdateTransactionRequest struct {
	Type            *TransactionType `json:"type" binding:"omitempty,oneof=income expense"`
	CategoryID      *uuid.UUID       `json:"category_id"`
	Amount          *float64         `json:"amount" binding:"omitempty,gt=0"`
	Description     *string          `json:"description"`
	CompetenceDate  *time.Time       `json:"competence_date"`
	DueDate         *time.Time       `json:"due_date"`
	PaidAt          *time.Time       `json:"paid_at"`
	PaymentMethodID *uuid.UUID       `json:"payment_method_id"`
}

// TransactionResponse is the serialized view of a financial transaction.
type TransactionResponse struct {
	ID                   uuid.UUID         `json:"id"`
	Type                 TransactionType   `json:"type"`
	CategoryID           *uuid.UUID        `json:"category_id,omitempty"`
	CategoryName         string            `json:"category_name,omitempty"`
	CategoryColor        string            `json:"category_color,omitempty"`
	Amount               float64           `json:"amount"`
	Description          string            `json:"description"`
	Source               TransactionSource `json:"source"`
	AppointmentID        *uuid.UUID        `json:"appointment_id,omitempty"`
	AppointmentPaymentID *uuid.UUID        `json:"appointment_payment_id,omitempty"`
	SessionPlanID        *uuid.UUID        `json:"session_plan_id,omitempty"`
	SessionPlanPaymentID *uuid.UUID        `json:"session_plan_payment_id,omitempty"`
	ProfessionalID       *uuid.UUID        `json:"professional_id,omitempty"`
	CompetenceDate       time.Time         `json:"competence_date"`
	DueDate              *time.Time        `json:"due_date,omitempty"`
	PaidAt               *time.Time        `json:"paid_at,omitempty"`
	PaymentMethodID      *uuid.UUID        `json:"payment_method_id,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
}

// TransactionFilter narrows down listing queries.
type TransactionFilter struct {
	From       *time.Time
	To         *time.Time
	Type       *TransactionType
	CategoryID *uuid.UUID
	Source     *TransactionSource
	OnlyPaid   bool
}

// CashFlowSummary aggregates the cash position for a period.
type CashFlowSummary struct {
	From         time.Time `json:"from"`
	To           time.Time `json:"to"`
	TotalIncome  float64   `json:"total_income"`
	TotalExpense float64   `json:"total_expense"`
	Balance      float64   `json:"balance"`
}

func (t *FinancialTransaction) ToResponse() *TransactionResponse {
	resp := &TransactionResponse{
		ID:                   t.ID,
		Type:                 t.Type,
		CategoryID:           t.CategoryID,
		Amount:               t.Amount,
		Description:          t.Description,
		Source:               t.Source,
		AppointmentID:        t.AppointmentID,
		AppointmentPaymentID: t.AppointmentPaymentID,
		SessionPlanID:        t.SessionPlanID,
		SessionPlanPaymentID: t.SessionPlanPaymentID,
		ProfessionalID:       t.ProfessionalID,
		CompetenceDate:       t.CompetenceDate,
		DueDate:              t.DueDate,
		PaidAt:               t.PaidAt,
		PaymentMethodID:      t.PaymentMethodID,
		CreatedAt:            t.CreatedAt,
	}
	if t.Category != nil {
		resp.CategoryName = t.Category.Name
		resp.CategoryColor = t.Category.Color
	}
	return resp
}
