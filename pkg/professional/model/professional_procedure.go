package model

import (
	"time"

	"github.com/google/uuid"
)

// ProfessionalProcedure links a professional to a procedure with commission settings.
// This defines both which procedures a professional can perform and their commission rate.
type ProfessionalProcedure struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID        uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	ProfessionalID  uuid.UUID  `json:"professional_id" gorm:"type:uuid;not null;uniqueIndex:idx_prof_proc"`
	ProcedureID     uuid.UUID  `json:"procedure_id" gorm:"type:uuid;not null;uniqueIndex:idx_prof_proc"`
	CommissionType  string     `json:"commission_type" gorm:"type:varchar(10);not null"` // "percentage" | "fixed"
	CommissionValue float64    `json:"commission_value" gorm:"type:decimal(10,2);not null"`
	IsActive        bool       `json:"is_active" gorm:"default:true"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

func (ProfessionalProcedure) TableName() string {
	return "professional_procedures"
}

// ProfessionalProcedureResponse is the API response including procedure details
type ProfessionalProcedureResponse struct {
	ID              uuid.UUID `json:"id"`
	ProfessionalID  uuid.UUID `json:"professional_id"`
	ProcedureID     uuid.UUID `json:"procedure_id"`
	ProcedureName   string    `json:"procedure_name"`
	ProcedurePrice  float64   `json:"procedure_price"`
	CommissionType  string    `json:"commission_type"`
	CommissionValue float64   `json:"commission_value"`
	IsActive        bool      `json:"is_active"`
}

// BulkUpsertItem represents a single item in a bulk upsert request
type BulkUpsertItem struct {
	ProcedureID     uuid.UUID `json:"procedure_id" binding:"required"`
	CommissionType  string    `json:"commission_type" binding:"required"`
	CommissionValue float64   `json:"commission_value" binding:"required,min=0"`
}

// BulkUpsertProfessionalProcedureRequest is the request body for bulk upserting commissions
type BulkUpsertProfessionalProcedureRequest struct {
	Items []BulkUpsertItem `json:"items" binding:"required,min=1,dive"`
}
