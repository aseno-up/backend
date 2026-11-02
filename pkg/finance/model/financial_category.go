package model

import (
	"time"

	"github.com/google/uuid"
)

// FinancialCategory classifies a transaction (e.g. "Aluguel", "Consulta").
// Categories are per-clinic. Defaults (is_default = true) are seeded on first
// access so that a clinic never sees an empty list.
type FinancialCategory struct {
	ID        uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID  uuid.UUID       `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Name      string          `json:"name" gorm:"size:100;not null"`
	Type      TransactionType `json:"type" gorm:"type:varchar(10);not null;index"`
	Color     string          `json:"color" gorm:"size:7"`
	IsDefault bool            `json:"is_default" gorm:"default:false"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty" gorm:"index"`
}

func (FinancialCategory) TableName() string {
	return "financial_categories"
}

// CreateCategoryRequest is the payload for creating a new category.
type CreateCategoryRequest struct {
	Name  string          `json:"name" binding:"required"`
	Type  TransactionType `json:"type" binding:"required,oneof=income expense"`
	Color string          `json:"color"`
}

// UpdateCategoryRequest is the payload for editing a category. Type is locked
// after creation to keep existing transactions coherent.
type UpdateCategoryRequest struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}
