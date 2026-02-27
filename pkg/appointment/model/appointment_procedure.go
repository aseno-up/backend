package model

import (
	"time"

	procedureModel "github.com/aseno/backend/pkg/procedure/model"
	"github.com/google/uuid"
)

// AppointmentProcedure represents the N:N relationship between an appointment and a procedure
type AppointmentProcedure struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	AppointmentID uuid.UUID `json:"appointment_id" gorm:"type:uuid;not null;index"`
	ProcedureID   uuid.UUID `json:"procedure_id" gorm:"type:uuid;not null;index"`
	Price         float64   `json:"price" gorm:"type:decimal(10,2)"` // Historical price at the time of appointment
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Relationships for preloading
	Procedure *procedureModel.Procedure `json:"procedure,omitempty" gorm:"foreignKey:ProcedureID;references:ID"`
}

// TableName returns the table name for GORM
func (AppointmentProcedure) TableName() string {
	return "appointment_procedures"
}

// AppointmentProcedureInput represents the input payload for associating a procedure with an appointment
type AppointmentProcedureInput struct {
	ProcedureID uuid.UUID `json:"procedure_id" binding:"required"`
	Price       *float64  `json:"price"` // Optional override price, otherwise takes Procedure default
}

// AppointmentProcedureResponse represents the localized API response for historical appointment procedures
type AppointmentProcedureResponse struct {
	ID          uuid.UUID                         `json:"id"`
	ProcedureID uuid.UUID                         `json:"procedure_id"`
	Price       float64                           `json:"price"`
	Procedure   *procedureModel.ProcedureResponse `json:"procedure,omitempty"`
}

// ToResponse converts an AppointmentProcedure to its Response representation
func (ap *AppointmentProcedure) ToResponse() *AppointmentProcedureResponse {
	resp := &AppointmentProcedureResponse{
		ID:          ap.ID,
		ProcedureID: ap.ProcedureID,
		Price:       ap.Price,
	}

	if ap.Procedure != nil {
		resp.Procedure = ap.Procedure.ToResponse()
	}

	return resp
}
