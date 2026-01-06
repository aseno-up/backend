package model

import (
	"time"

	"github.com/google/uuid"
)

// Professional represents a healthcare professional in the clinic
type Professional struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID        uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	UserID          *uuid.UUID `json:"user_id,omitempty" gorm:"type:uuid;index"` // Optional link to user account
	Name            string     `json:"name" gorm:"not null"`
	Email           string     `json:"email"`
	Phone           string     `json:"phone"`
	Specialty       string     `json:"specialty"`                          // e.g., "Psicólogo", "Fisioterapeuta", "Dentista"
	RegistrationNum string     `json:"registration_num"`                   // CRP, CREFITO, CRO, etc.
	AvatarURL       string     `json:"avatar_url"`
	Bio             string     `json:"bio"`
	Color           string     `json:"color" gorm:"default:'#7C3AED'"`     // Color for calendar display
	IsActive        bool       `json:"is_active" gorm:"default:true"`
	WorkingHours    WorkingHours `json:"working_hours" gorm:"serializer:json"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// WorkingHours defines the professional's available hours
type WorkingHours struct {
	Monday    []TimeSlot `json:"monday,omitempty"`
	Tuesday   []TimeSlot `json:"tuesday,omitempty"`
	Wednesday []TimeSlot `json:"wednesday,omitempty"`
	Thursday  []TimeSlot `json:"thursday,omitempty"`
	Friday    []TimeSlot `json:"friday,omitempty"`
	Saturday  []TimeSlot `json:"saturday,omitempty"`
	Sunday    []TimeSlot `json:"sunday,omitempty"`
}

// TimeSlot represents a time range
type TimeSlot struct {
	Start string `json:"start"` // Format: "09:00"
	End   string `json:"end"`   // Format: "18:00"
}

// TableName returns the table name for GORM
func (Professional) TableName() string {
	return "professionals"
}

// ProfessionalResponse represents the API response for a professional
type ProfessionalResponse struct {
	ID              uuid.UUID    `json:"id"`
	ClinicID        uuid.UUID    `json:"clinic_id"`
	UserID          *uuid.UUID   `json:"user_id,omitempty"`
	Name            string       `json:"name"`
	Email           string       `json:"email"`
	Phone           string       `json:"phone"`
	Specialty       string       `json:"specialty"`
	RegistrationNum string       `json:"registration_num"`
	AvatarURL       string       `json:"avatar_url"`
	Bio             string       `json:"bio"`
	Color           string       `json:"color"`
	IsActive        bool         `json:"is_active"`
	WorkingHours    WorkingHours `json:"working_hours"`
	CreatedAt       time.Time    `json:"created_at"`
}

// ToResponse converts Professional to ProfessionalResponse
func (p *Professional) ToResponse() *ProfessionalResponse {
	return &ProfessionalResponse{
		ID:              p.ID,
		ClinicID:        p.ClinicID,
		UserID:          p.UserID,
		Name:            p.Name,
		Email:           p.Email,
		Phone:           p.Phone,
		Specialty:       p.Specialty,
		RegistrationNum: p.RegistrationNum,
		AvatarURL:       p.AvatarURL,
		Bio:             p.Bio,
		Color:           p.Color,
		IsActive:        p.IsActive,
		WorkingHours:    p.WorkingHours,
		CreatedAt:       p.CreatedAt,
	}
}

// CreateProfessionalRequest represents the request to create a professional
type CreateProfessionalRequest struct {
	Name            string        `json:"name" binding:"required,min=2,max=100"`
	Email           string        `json:"email" binding:"omitempty,email"`
	Phone           string        `json:"phone"`
	Specialty       string        `json:"specialty"`
	RegistrationNum string        `json:"registration_num"`
	Bio             string        `json:"bio"`
	Color           string        `json:"color"`
	WorkingHours    *WorkingHours `json:"working_hours"`
}

// UpdateProfessionalRequest represents the request to update a professional
type UpdateProfessionalRequest struct {
	Name            string        `json:"name" binding:"omitempty,min=2,max=100"`
	Email           string        `json:"email" binding:"omitempty,email"`
	Phone           string        `json:"phone"`
	Specialty       string        `json:"specialty"`
	RegistrationNum string        `json:"registration_num"`
	AvatarURL       string        `json:"avatar_url"`
	Bio             string        `json:"bio"`
	Color           string        `json:"color"`
	IsActive        *bool         `json:"is_active"`
	WorkingHours    *WorkingHours `json:"working_hours"`
}

// ProfessionalListResponse represents a paginated list of professionals
type ProfessionalListResponse struct {
	Data       []*ProfessionalResponse `json:"data"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PerPage    int                     `json:"per_page"`
	TotalPages int                     `json:"total_pages"`
}
