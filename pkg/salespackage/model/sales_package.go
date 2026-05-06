package model

import (
	"time"

	patientModel "github.com/aseno/backend/pkg/patient/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SalesPackageStatus is the lifecycle of a sale.
//
// quote     → orçamento, ainda não confirmado; não gera FinancialTransaction.
// active    → venda confirmada; FinancialTransactions geradas; saldo disponível.
// completed → todas as sessões foram consumidas (pode ser apurado on-the-fly).
// canceled  → venda cancelada; saldo bloqueado.
type SalesPackageStatus string

const (
	SalesPackageStatusQuote     SalesPackageStatus = "quote"
	SalesPackageStatusActive    SalesPackageStatus = "active"
	SalesPackageStatusCompleted SalesPackageStatus = "completed"
	SalesPackageStatusCanceled  SalesPackageStatus = "canceled"
)

// SalesPackage represents a sale of one or more procedure packages to a patient.
// Differs from SessionPlan: dates are NOT scheduled at sale time — sessions are
// consumed on-the-fly when appointments are created (FIFO across packages).
type SalesPackage struct {
	ID         uuid.UUID          `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID   uuid.UUID          `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID  uuid.UUID          `json:"patient_id" gorm:"type:uuid;not null;index"`
	Code       string             `json:"code" gorm:"type:varchar(40);not null;index"` // sequential per clinic, e.g. "V-2026-001"
	Status     SalesPackageStatus `json:"status" gorm:"type:varchar(20);not null;default:'active';index"`
	SaleDate   time.Time          `json:"sale_date" gorm:"type:date;not null"` // editable by user, defaults to today
	ValidUntil *time.Time         `json:"valid_until,omitempty" gorm:"type:date"`
	Subtotal   float64            `json:"subtotal" gorm:"type:decimal(12,2);not null;default:0"`
	Discount   float64            `json:"discount" gorm:"type:decimal(12,2);not null;default:0"`
	Total      float64            `json:"total" gorm:"type:decimal(12,2);not null;default:0"`
	Notes      string             `json:"notes" gorm:"type:text"`
	CreatedBy  *uuid.UUID         `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
	DeletedAt  gorm.DeletedAt     `json:"deleted_at,omitempty" gorm:"index"`

	Items    []SalesPackageItem    `json:"items,omitempty" gorm:"foreignKey:SalesPackageID;references:ID"`
	Payments []SalesPackagePayment `json:"payments,omitempty" gorm:"foreignKey:SalesPackageID;references:ID"`

	// Loaded via Preload by the repository so the response can carry the
	// patient name/phone without a second roundtrip per row.
	Patient *patientModel.Patient `json:"-" gorm:"foreignKey:PatientID;references:ID"`
}

func (SalesPackage) TableName() string {
	return "sales_packages"
}

// SalesPackageItemBalance carries the live consumption snapshot for one item.
// Used (X) is computed on-the-fly: COUNT(appointment_procedure WHERE
// sales_package_item_id = item.id AND appointment.status != 'canceled').
type SalesPackageItemBalance struct {
	Item     SalesPackageItem `json:"item"`
	Used     int              `json:"used"`
	Quantity int              `json:"quantity"`
}

// ProcedureBalance aggregates balance across all of a patient's packages for
// the same procedure. The agenda screen uses this to render "Procedimento -
// (X/Y)" without exposing per-package complexity.
type ProcedureBalance struct {
	ProcedureID   uuid.UUID `json:"procedure_id"`
	ProcedureName string    `json:"procedure_name"`
	UsedTotal     int       `json:"used_total"`
	QuantityTotal int       `json:"quantity_total"`
}

// CreateSalesPackageRequest is the payload for POST /sales-packages.
// PaymentInputs are optional for status='quote' (orçamento) and required for
// status='active' (venda confirmada — generates FinancialTransactions).
type CreateSalesPackageRequest struct {
	PatientID  uuid.UUID                        `json:"patient_id" binding:"required"`
	Status     SalesPackageStatus               `json:"status" binding:"required,oneof=quote active"`
	SaleDate   *time.Time                       `json:"sale_date"` // defaults to today if omitted
	ValidUntil *time.Time                       `json:"valid_until"`
	Discount   float64                          `json:"discount" binding:"min=0"`
	Notes      string                           `json:"notes"`
	Items      []CreateSalesPackageItemInput    `json:"items" binding:"required,min=1,dive"`
	Payments   []CreateSalesPackagePaymentInput `json:"payments" binding:"omitempty,dive"`
}

// UpdateSalesPackageRequest covers the editable fields after creation.
// Items and payments are managed via dedicated sub-resources (not in this DTO).
type UpdateSalesPackageRequest struct {
	SaleDate   *time.Time `json:"sale_date"`
	ValidUntil *time.Time `json:"valid_until"`
	Discount   *float64   `json:"discount" binding:"omitempty,min=0"`
	Notes      *string    `json:"notes"`
}

// SalesPackageResponse is the serialized view of a sale.
type SalesPackageResponse struct {
	ID           uuid.UUID                          `json:"id"`
	ClinicID     uuid.UUID                          `json:"clinic_id"`
	PatientID    uuid.UUID                          `json:"patient_id"`
	PatientName  string                             `json:"patient_name,omitempty"`
	PatientPhone string                             `json:"patient_phone,omitempty"`
	Code         string                             `json:"code"`
	Status       SalesPackageStatus                 `json:"status"`
	SaleDate     time.Time                          `json:"sale_date"`
	ValidUntil   *time.Time                         `json:"valid_until,omitempty"`
	Subtotal     float64                            `json:"subtotal"`
	Discount     float64                            `json:"discount"`
	Total        float64                            `json:"total"`
	Notes        string                             `json:"notes,omitempty"`
	CreatedBy    *uuid.UUID                         `json:"created_by,omitempty"`
	CreatedAt    time.Time                          `json:"created_at"`
	Items        []*SalesPackageItemResponse        `json:"items,omitempty"`
	Payments     []*SalesPackagePaymentResponse     `json:"payments,omitempty"`
	ItemsBalance []*SalesPackageItemBalanceResponse `json:"items_balance,omitempty"`
}

// SalesPackageItemBalanceResponse mirrors SalesPackageItemBalance for JSON.
type SalesPackageItemBalanceResponse struct {
	ItemID      uuid.UUID `json:"item_id"`
	ProcedureID uuid.UUID `json:"procedure_id"`
	Used        int       `json:"used"`
	Quantity    int       `json:"quantity"`
}

// SalesPackageListResponse is the paginated index view.
type SalesPackageListResponse struct {
	Data       []*SalesPackageResponse `json:"data"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PerPage    int                     `json:"per_page"`
	TotalPages int                     `json:"total_pages"`
}

// SalesPackageFilter narrows down listing queries.
// Query is a free-text search applied to: patient name, patient phone, code,
// and any item's procedure name.
type SalesPackageFilter struct {
	Page    int                 `form:"page"`
	PerPage int                 `form:"per_page"`
	Status  *SalesPackageStatus `form:"status"`
	Query   string              `form:"q"`
	From    *time.Time          `form:"from"`
	To      *time.Time          `form:"to"`
}

// ToResponse builds a SalesPackageResponse without computed balances.
// Use repository.AttachBalances to fill items_balance.
func (s *SalesPackage) ToResponse() *SalesPackageResponse {
	resp := &SalesPackageResponse{
		ID:         s.ID,
		ClinicID:   s.ClinicID,
		PatientID:  s.PatientID,
		Code:       s.Code,
		Status:     s.Status,
		SaleDate:   s.SaleDate,
		ValidUntil: s.ValidUntil,
		Subtotal:   s.Subtotal,
		Discount:   s.Discount,
		Total:      s.Total,
		Notes:      s.Notes,
		CreatedBy:  s.CreatedBy,
		CreatedAt:  s.CreatedAt,
	}
	if s.Patient != nil {
		resp.PatientName = s.Patient.Name
		resp.PatientPhone = s.Patient.Phone
	}
	if len(s.Items) > 0 {
		resp.Items = make([]*SalesPackageItemResponse, len(s.Items))
		for i := range s.Items {
			resp.Items[i] = s.Items[i].ToResponse()
		}
	}
	if len(s.Payments) > 0 {
		resp.Payments = make([]*SalesPackagePaymentResponse, len(s.Payments))
		for i := range s.Payments {
			resp.Payments[i] = s.Payments[i].ToResponse()
		}
	}
	return resp
}
