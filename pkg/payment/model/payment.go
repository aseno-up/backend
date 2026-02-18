package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Payment represents a payment/charge in Asaas
type Payment struct {
	ID                uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID          uuid.UUID      `json:"clinic_id" gorm:"type:uuid;not null;index"`
	CustomerID        uuid.UUID      `json:"customer_id" gorm:"type:uuid;not null;index"`
	SubscriptionID    *uuid.UUID     `json:"subscription_id,omitempty" gorm:"type:uuid;index"`
	AsaasPaymentID    string         `json:"asaas_payment_id" gorm:"uniqueIndex;not null"`
	Value             float64        `json:"value" gorm:"not null"`
	NetValue          float64        `json:"net_value"`
	Status            PaymentStatus  `json:"status" gorm:"not null;default:'PENDING'"`
	BillingType       BillingType    `json:"billing_type" gorm:"not null"`
	DueDate           time.Time      `json:"due_date" gorm:"not null"`
	PaymentDate       *time.Time     `json:"payment_date,omitempty"`
	ClientPaymentDate *time.Time     `json:"client_payment_date,omitempty"`
	InvoiceURL        string         `json:"invoice_url"`
	BankSlipURL       string         `json:"bank_slip_url"`
	PixQRCodeURL      string         `json:"pix_qrcode_url"`
	PixCopyPaste      string         `json:"pix_copy_paste"`
	Description       string         `json:"description"`
	ExternalRef       string         `json:"external_ref"`
	CreatedAt         time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Customer *Customer `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`
}

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusPending              PaymentStatus = "PENDING"
	PaymentStatusReceived             PaymentStatus = "RECEIVED"
	PaymentStatusConfirmed            PaymentStatus = "CONFIRMED"
	PaymentStatusOverdue              PaymentStatus = "OVERDUE"
	PaymentStatusRefunded             PaymentStatus = "REFUNDED"
	PaymentStatusReceivedInCash       PaymentStatus = "RECEIVED_IN_CASH"
	PaymentStatusRefundRequested      PaymentStatus = "REFUND_REQUESTED"
	PaymentStatusRefundInProgress     PaymentStatus = "REFUND_IN_PROGRESS"
	PaymentStatusChargebackRequested  PaymentStatus = "CHARGEBACK_REQUESTED"
	PaymentStatusChargebackDispute    PaymentStatus = "CHARGEBACK_DISPUTE"
	PaymentStatusAwaitingChargeback   PaymentStatus = "AWAITING_CHARGEBACK_REVERSAL"
	PaymentStatusDunningRequested     PaymentStatus = "DUNNING_REQUESTED"
	PaymentStatusDunningReceived      PaymentStatus = "DUNNING_RECEIVED"
	PaymentStatusAwaitingRiskAnalysis PaymentStatus = "AWAITING_RISK_ANALYSIS"
)

// BillingType represents the payment method
type BillingType string

const (
	BillingTypeBoleto     BillingType = "BOLETO"
	BillingTypeCreditCard BillingType = "CREDIT_CARD"
	BillingTypeDebitCard  BillingType = "DEBIT_CARD"
	BillingTypePix        BillingType = "PIX"
	BillingTypeUndefined  BillingType = "UNDEFINED"
)

// CreatePaymentRequest represents a request to create a payment in Asaas
type CreatePaymentRequest struct {
	CustomerID    string      `json:"customer" binding:"required"`
	BillingType   BillingType `json:"billingType" binding:"required"`
	Value         float64     `json:"value" binding:"required"`
	DueDate       string      `json:"dueDate" binding:"required"` // Format: YYYY-MM-DD
	Description   string      `json:"description"`
	ExternalRef   string      `json:"externalReference"`
	PostalService bool        `json:"postalService,omitempty"`
}

// AsaasPaymentResponse represents the response from Asaas API when creating/getting a payment
type AsaasPaymentResponse struct {
	ID                string  `json:"id"`
	Customer          string  `json:"customer"`
	Subscription      string  `json:"subscription,omitempty"`
	Value             float64 `json:"value"`
	NetValue          float64 `json:"netValue"`
	Status            string  `json:"status"`
	BillingType       string  `json:"billingType"`
	DueDate           string  `json:"dueDate"`
	PaymentDate       string  `json:"paymentDate,omitempty"`
	ClientPaymentDate string  `json:"clientPaymentDate,omitempty"`
	InvoiceURL        string  `json:"invoiceUrl"`
	BankSlipURL       string  `json:"bankSlipUrl"`
	Description       string  `json:"description"`
	ExternalRef       string  `json:"externalReference"`
	DateCreated       string  `json:"dateCreated"`
}

// AsaasPixQRCodeResponse represents the PIX QR Code response
type AsaasPixQRCodeResponse struct {
	EncodedImage   string `json:"encodedImage"`
	Payload        string `json:"payload"`
	ExpirationDate string `json:"expirationDate"`
}

// PaymentListResponse represents a paginated list of payments
type PaymentListResponse struct {
	Data       []AsaasPaymentResponse `json:"data"`
	HasMore    bool                   `json:"hasMore"`
	TotalCount int                    `json:"totalCount"`
	Limit      int                    `json:"limit"`
	Offset     int                    `json:"offset"`
}
