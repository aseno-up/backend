package model

import (
	"time"

	"github.com/google/uuid"
)

// Specialty represents a clinic's specialty (e.g., "Psicologia", "Fisioterapia", "Odontologia")
type Specialty struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID  uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Name      string     `json:"name" gorm:"not null"`
	Active    bool       `json:"active" gorm:"default:true"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

func (Specialty) TableName() string {
	return "specialties"
}

// SpecialtyResponse represents the API response
type SpecialtyResponse struct {
	ID        uuid.UUID `json:"id"`
	ClinicID  uuid.UUID `json:"clinic_id"`
	Name      string    `json:"name"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Specialty) ToResponse() *SpecialtyResponse {
	return &SpecialtyResponse{
		ID:        s.ID,
		ClinicID:  s.ClinicID,
		Name:      s.Name,
		Active:    s.Active,
		CreatedAt: s.CreatedAt,
	}
}

// CreateSpecialtyRequest represents the request to create a specialty
type CreateSpecialtyRequest struct {
	Name string `json:"name" binding:"required,min=2"`
}

// UpdateSpecialtyRequest represents the request to update a specialty
type UpdateSpecialtyRequest struct {
	Name   *string `json:"name" binding:"omitempty,min=2"`
	Active *bool   `json:"active"`
}
