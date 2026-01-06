package model

import (
	"time"

	"github.com/google/uuid"
)

// Gender represents patient gender
type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

// Patient represents a patient in the clinic
type Patient struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID    uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Name        string     `json:"name" gorm:"not null"`
	Email       string     `json:"email" gorm:"index"`
	Phone       string     `json:"phone" gorm:"index"`
	WhatsApp    string     `json:"whatsapp" gorm:"column:whatsapp"` // May differ from phone
	Document    string     `json:"document"` // CPF
	BirthDate   *time.Time `json:"birth_date"`
	Gender      Gender     `json:"gender"`
	Address     string     `json:"address"`
	City        string     `json:"city"`
	State       string     `json:"state"`
	ZipCode     string     `json:"zip_code"`
	Notes       string     `json:"notes"`                          // General notes
	IsActive    bool       `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (Patient) TableName() string {
	return "patients"
}

// PatientResponse represents the API response for a patient
type PatientResponse struct {
	ID        uuid.UUID  `json:"id"`
	ClinicID  uuid.UUID  `json:"clinic_id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Phone     string     `json:"phone"`
	WhatsApp  string     `json:"whatsapp"`
	Document  string     `json:"document"`
	BirthDate *time.Time `json:"birth_date,omitempty"`
	Gender    Gender     `json:"gender,omitempty"`
	Address   string     `json:"address"`
	City      string     `json:"city"`
	State     string     `json:"state"`
	ZipCode   string     `json:"zip_code"`
	Notes     string     `json:"notes"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
}

// ToResponse converts Patient to PatientResponse
func (p *Patient) ToResponse() *PatientResponse {
	return &PatientResponse{
		ID:        p.ID,
		ClinicID:  p.ClinicID,
		Name:      p.Name,
		Email:     p.Email,
		Phone:     p.Phone,
		WhatsApp:  p.WhatsApp,
		Document:  p.Document,
		BirthDate: p.BirthDate,
		Gender:    p.Gender,
		Address:   p.Address,
		City:      p.City,
		State:     p.State,
		ZipCode:   p.ZipCode,
		Notes:     p.Notes,
		IsActive:  p.IsActive,
		CreatedAt: p.CreatedAt,
	}
}

// GetWhatsAppNumber returns the WhatsApp number or phone if not set
func (p *Patient) GetWhatsAppNumber() string {
	if p.WhatsApp != "" {
		return p.WhatsApp
	}
	return p.Phone
}

// CreatePatientRequest represents the request to create a patient
type CreatePatientRequest struct {
	Name      string     `json:"name" binding:"required,min=2,max=100"`
	Email     string     `json:"email" binding:"omitempty,email"`
	Phone     string     `json:"phone" binding:"required"`
	WhatsApp  string     `json:"whatsapp"`
	Document  string     `json:"document"`
	BirthDate *time.Time `json:"birth_date"`
	Gender    Gender     `json:"gender" binding:"omitempty,oneof=male female other"`
	Address   string     `json:"address"`
	City      string     `json:"city"`
	State     string     `json:"state"`
	ZipCode   string     `json:"zip_code"`
	Notes     string     `json:"notes"`
}

// UpdatePatientRequest represents the request to update a patient
type UpdatePatientRequest struct {
	Name      string     `json:"name" binding:"omitempty,min=2,max=100"`
	Email     string     `json:"email" binding:"omitempty,email"`
	Phone     string     `json:"phone"`
	WhatsApp  string     `json:"whatsapp"`
	Document  string     `json:"document"`
	BirthDate *time.Time `json:"birth_date"`
	Gender    Gender     `json:"gender" binding:"omitempty,oneof=male female other"`
	Address   string     `json:"address"`
	City      string     `json:"city"`
	State     string     `json:"state"`
	ZipCode   string     `json:"zip_code"`
	Notes     string     `json:"notes"`
	IsActive  *bool      `json:"is_active"`
}

// PatientListResponse represents a paginated list of patients
type PatientListResponse struct {
	Data       []*PatientResponse `json:"data"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PerPage    int                `json:"per_page"`
	TotalPages int                `json:"total_pages"`
}

// PatientSearchParams represents search parameters for patients
type PatientSearchParams struct {
	Query    string `form:"q"`       // Search by name, phone, email
	Page     int    `form:"page"`
	PerPage  int    `form:"per_page"`
}
