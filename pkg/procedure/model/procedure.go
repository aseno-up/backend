package model

import (
	"time"

	"github.com/google/uuid"
)

// Procedure represents a clinic's procedure catalog item
type Procedure struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID        uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Name            string     `json:"name" gorm:"not null"`
	SpecialtyID     *uuid.UUID `json:"specialty_id,omitempty" gorm:"type:uuid;index"` // FK to specialties table
	Price           float64    `json:"price" gorm:"type:decimal(10,2)"`               // Default price
	DurationMinutes int        `json:"duration_minutes"`                              // Optional estimated duration
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (Procedure) TableName() string {
	return "procedures"
}

// ProcedureResponse represents the API response for a procedure
type ProcedureResponse struct {
	ID              uuid.UUID  `json:"id"`
	ClinicID        uuid.UUID  `json:"clinic_id"`
	Name            string     `json:"name"`
	SpecialtyID     *uuid.UUID `json:"specialty_id,omitempty"`
	Price           float64    `json:"price"`
	DurationMinutes int        `json:"duration_minutes"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ToResponse converts Procedure to ProcedureResponse
func (p *Procedure) ToResponse() *ProcedureResponse {
	return &ProcedureResponse{
		ID:              p.ID,
		ClinicID:        p.ClinicID,
		Name:            p.Name,
		SpecialtyID:     p.SpecialtyID,
		Price:           p.Price,
		DurationMinutes: p.DurationMinutes,
		CreatedAt:       p.CreatedAt,
	}
}

// CreateProcedureRequest represents the request to create a procedure
type CreateProcedureRequest struct {
	Name            string     `json:"name" binding:"required,min=2"`
	SpecialtyID     *uuid.UUID `json:"specialty_id"`
	Price           *float64   `json:"price" binding:"required,min=0"`
	DurationMinutes *int       `json:"duration_minutes" binding:"required,min=1"`
}

// UpdateProcedureRequest represents the request to update a procedure
type UpdateProcedureRequest struct {
	Name            *string    `json:"name" binding:"omitempty,min=2"`
	SpecialtyID     *uuid.UUID `json:"specialty_id"`
	Price           *float64   `json:"price" binding:"omitempty,min=0"`
	DurationMinutes *int       `json:"duration_minutes" binding:"omitempty,min=1"`
}

// ProcedureListResponse represents a paginated list of procedures
type ProcedureListResponse struct {
	Data       []*ProcedureResponse `json:"data"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PerPage    int                  `json:"per_page"`
	TotalPages int                  `json:"total_pages"`
}
