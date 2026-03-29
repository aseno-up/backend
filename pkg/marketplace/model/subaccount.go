package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SubaccountStatus string

const (
	SubaccountStatusPending   SubaccountStatus = "pending"
	SubaccountStatusActive    SubaccountStatus = "active"
	SubaccountStatusRejected  SubaccountStatus = "rejected"
	SubaccountStatusSuspended SubaccountStatus = "suspended"
)

// Subaccount represents an Asaas subaccount linked to a clinic
type Subaccount struct {
	ID              uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID        uuid.UUID        `json:"clinic_id" gorm:"type:uuid;uniqueIndex;not null"`
	AccountID       uuid.UUID        `json:"account_id" gorm:"type:uuid;index;not null"`
	AsaasAccountID  string           `json:"asaas_account_id" gorm:"uniqueIndex"`
	WalletID        string           `json:"wallet_id"`
	EncryptedAPIKey string           `json:"-" gorm:"not null"`
	Status          SubaccountStatus `json:"status" gorm:"not null;default:'pending'"`
	CompanyName     string           `json:"company_name" gorm:"not null"`
	CpfCnpj         string           `json:"cpf_cnpj" gorm:"not null"`
	CompanyType     string           `json:"company_type"` // MEI, LIMITED, INDIVIDUAL, etc.
	IncomeValue     float64          `json:"income_value"`
	Email           string           `json:"email" gorm:"not null"`
	Phone           string           `json:"phone"`
	PostalCode      string           `json:"postal_code"`
	Address         string           `json:"address"`
	AddressNumber   string           `json:"address_number"`
	Province        string           `json:"province"`
	City            string           `json:"city"`
	State           string           `json:"state"`
	CreatedAt       time.Time        `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time        `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt       gorm.DeletedAt   `json:"-" gorm:"index"`
}
