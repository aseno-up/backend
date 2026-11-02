package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ClinicPayment represents a patient-to-clinic payment processed via Asaas subaccount
type ClinicPayment struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	AccountID      uuid.UUID      `json:"account_id" gorm:"type:uuid;index;not null"`
	ClinicID       uuid.UUID      `json:"clinic_id" gorm:"type:uuid;index;not null"`
	AppointmentID  *uuid.UUID     `json:"appointment_id,omitempty" gorm:"type:uuid;index"`
	PatientID      *uuid.UUID     `json:"patient_id,omitempty" gorm:"type:uuid;index"`
	AsaasPaymentID string         `json:"asaas_payment_id" gorm:"uniqueIndex;not null"`
	Value          float64        `json:"value" gorm:"not null"`
	NetValue       float64        `json:"net_value"`
	SplitFeeValue  float64        `json:"split_fee_value" gorm:"default:0"`
	Status         string         `json:"status" gorm:"not null;default:'PENDING'"`
	BillingType    string         `json:"billing_type" gorm:"not null"`
	DueDate        time.Time      `json:"due_date" gorm:"not null"`
	PaymentDate    *time.Time     `json:"payment_date,omitempty"`
	InvoiceURL     string         `json:"invoice_url"`
	PixQRCodeImage string         `json:"pix_qrcode_image,omitempty"` // base64
	PixCopyPaste   string         `json:"pix_copy_paste,omitempty"`
	Description    string         `json:"description"`
	ExternalRef    string         `json:"external_ref"`
	Installments   *int           `json:"installments,omitempty"`
	CreatedAt      time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

// ClinicPaymentResponse is the API response for a clinic payment
type ClinicPaymentResponse struct {
	ID             uuid.UUID  `json:"id"`
	ClinicID       uuid.UUID  `json:"clinic_id"`
	AppointmentID  *uuid.UUID `json:"appointment_id,omitempty"`
	PatientID      *uuid.UUID `json:"patient_id,omitempty"`
	AsaasPaymentID string     `json:"asaas_payment_id"`
	Value          float64    `json:"value"`
	NetValue       float64    `json:"net_value"`
	SplitFeeValue  float64    `json:"split_fee_value"`
	Status         string     `json:"status"`
	BillingType    string     `json:"billing_type"`
	DueDate        time.Time  `json:"due_date"`
	PaymentDate    *time.Time `json:"payment_date,omitempty"`
	InvoiceURL     string     `json:"invoice_url,omitempty"`
	PixQRCodeImage string     `json:"pix_qrcode_image,omitempty"`
	PixCopyPaste   string     `json:"pix_copy_paste,omitempty"`
	Description    string     `json:"description,omitempty"`
	Installments   *int       `json:"installments,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (p *ClinicPayment) ToResponse() *ClinicPaymentResponse {
	return &ClinicPaymentResponse{
		ID:             p.ID,
		ClinicID:       p.ClinicID,
		AppointmentID:  p.AppointmentID,
		PatientID:      p.PatientID,
		AsaasPaymentID: p.AsaasPaymentID,
		Value:          p.Value,
		NetValue:       p.NetValue,
		SplitFeeValue:  p.SplitFeeValue,
		Status:         p.Status,
		BillingType:    p.BillingType,
		DueDate:        p.DueDate,
		PaymentDate:    p.PaymentDate,
		InvoiceURL:     p.InvoiceURL,
		PixQRCodeImage: p.PixQRCodeImage,
		PixCopyPaste:   p.PixCopyPaste,
		Description:    p.Description,
		Installments:   p.Installments,
		CreatedAt:      p.CreatedAt,
	}
}
