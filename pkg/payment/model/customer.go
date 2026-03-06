package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Customer represents an Asaas customer linked to a clinic
type Customer struct {
	ID               uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	AccountID        uuid.UUID      `json:"account_id" gorm:"type:uuid;index"`
	AsaasCustomerID  string         `json:"asaas_customer_id" gorm:"uniqueIndex;not null"`
	Name             string         `json:"name" gorm:"not null"`
	Email            string         `json:"email" gorm:"not null"`
	Phone            string         `json:"phone"`
	Document         string         `json:"document"` // CPF or CNPJ
	DocumentType     DocumentType   `json:"document_type" gorm:"default:'CPF'"`
	PostalCode       string         `json:"postal_code"`
	Address          string         `json:"address"`
	AddressNumber    string         `json:"address_number"`
	Complement       string         `json:"complement"`
	Province         string         `json:"province"` // Bairro
	City             string         `json:"city"`
	State            string         `json:"state"`
	ExternalRef      string         `json:"external_ref"` // Reference to clinic ID
	AdditionalEmails string         `json:"additional_emails"`
	CreatedAt        time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

// DocumentType represents the type of document
type DocumentType string

const (
	DocumentTypeCPF  DocumentType = "CPF"
	DocumentTypeCNPJ DocumentType = "CNPJ"
)

// CreateCustomerRequest represents a request to create a customer in Asaas
type CreateCustomerRequest struct {
	Name             string `json:"name" binding:"required"`
	Email            string `json:"email" binding:"required,email"`
	Phone            string `json:"phone"`
	Document         string `json:"cpfCnpj" binding:"required"`
	PostalCode       string `json:"postalCode"`
	Address          string `json:"address"`
	AddressNumber    string `json:"addressNumber"`
	Complement       string `json:"complement"`
	Province         string `json:"province"`
	City             string `json:"city,omitempty"`
	State            string `json:"state,omitempty"`
	ExternalRef      string `json:"externalReference"`
	AdditionalEmails string `json:"additionalEmails,omitempty"`
}

// AsaasCustomerResponse represents the response from Asaas API when creating/getting a customer
type AsaasCustomerResponse struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	Email            string      `json:"email"`
	Phone            string      `json:"phone"`
	MobilePhone      string      `json:"mobilePhone"`
	Document         string      `json:"cpfCnpj"`
	PostalCode       string      `json:"postalCode"`
	Address          string      `json:"address"`
	AddressNumber    string      `json:"addressNumber"`
	Complement       string      `json:"complement"`
	Province         string      `json:"province"`
	City             interface{} `json:"city"`
	State            interface{} `json:"state"`
	ExternalRef      string      `json:"externalReference"`
	AdditionalEmails string      `json:"additionalEmails"`
	DateCreated      string      `json:"dateCreated"`
}
