package model

import (
	"time"

	procedureModel "github.com/aseno/backend/pkg/procedure/model"
	"github.com/google/uuid"
)

// SalesPackageItem is one procedure line within a sale. Each item carries its
// own quantity and unit price so a single sale can mix procedures with
// different session counts and prices (override allowed at sale time).
type SalesPackageItem struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	SalesPackageID uuid.UUID `json:"sales_package_id" gorm:"type:uuid;not null;index"`
	ProcedureID    uuid.UUID `json:"procedure_id" gorm:"type:uuid;not null;index"`
	UnitPrice      float64   `json:"unit_price" gorm:"type:decimal(12,2);not null"`
	Quantity       int       `json:"quantity" gorm:"not null"`
	LineTotal      float64   `json:"line_total" gorm:"type:decimal(12,2);not null"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Procedure *procedureModel.Procedure `json:"procedure,omitempty" gorm:"foreignKey:ProcedureID;references:ID"`
}

func (SalesPackageItem) TableName() string {
	return "sales_package_items"
}

// CreateSalesPackageItemInput is the payload to add an item to a sale.
type CreateSalesPackageItemInput struct {
	ProcedureID uuid.UUID `json:"procedure_id" binding:"required"`
	UnitPrice   float64   `json:"unit_price" binding:"required,min=0"`
	Quantity    int       `json:"quantity" binding:"required,min=1"`
}

// SalesPackageItemResponse is the serialized view of an item.
type SalesPackageItemResponse struct {
	ID             uuid.UUID                         `json:"id"`
	SalesPackageID uuid.UUID                         `json:"sales_package_id"`
	ProcedureID    uuid.UUID                         `json:"procedure_id"`
	UnitPrice      float64                           `json:"unit_price"`
	Quantity       int                               `json:"quantity"`
	LineTotal      float64                           `json:"line_total"`
	CreatedAt      time.Time                         `json:"created_at"`
	Procedure      *procedureModel.ProcedureResponse `json:"procedure,omitempty"`
}

func (i *SalesPackageItem) ToResponse() *SalesPackageItemResponse {
	resp := &SalesPackageItemResponse{
		ID:             i.ID,
		SalesPackageID: i.SalesPackageID,
		ProcedureID:    i.ProcedureID,
		UnitPrice:      i.UnitPrice,
		Quantity:       i.Quantity,
		LineTotal:      i.LineTotal,
		CreatedAt:      i.CreatedAt,
	}
	if i.Procedure != nil {
		resp.Procedure = i.Procedure.ToResponse()
	}
	return resp
}
