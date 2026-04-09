package model

import (
	"time"

	"github.com/google/uuid"
)

// CommissionStatus represents the status of a commission payment
type CommissionStatus string

const (
	CommissionStatusPending CommissionStatus = "pending"
	CommissionStatusPaid    CommissionStatus = "paid"
)

// Commission records the calculated commission for a completed appointment or group session.
// Values are snapshots at the time of calculation — changes to commission config
// do not retroactively affect existing records.
type Commission struct {
	ID              uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID        uuid.UUID        `json:"clinic_id" gorm:"type:uuid;not null;index"`
	AppointmentID   *uuid.UUID       `json:"appointment_id,omitempty" gorm:"type:uuid;index"`              // Set for individual appointment commissions
	SessionPlanID   *uuid.UUID       `json:"session_plan_id,omitempty" gorm:"type:uuid;index"`             // Set for group session commissions
	SessionDate     string           `json:"session_date,omitempty" gorm:"size:10"`                        // YYYY-MM-DD, set for group session commissions
	ProfessionalID  uuid.UUID        `json:"professional_id" gorm:"type:uuid;not null;index"`
	ProcedureID     uuid.UUID        `json:"procedure_id" gorm:"type:uuid;not null;index"`
	ProcedureName   string           `json:"procedure_name" gorm:"size:255"`                               // snapshot: procedure name at creation time (for group sessions)
	SessionPrice    float64          `json:"session_price" gorm:"type:decimal(10,2);not null"`             // snapshot: procedure price at session time
	CommissionType  string           `json:"commission_type" gorm:"type:varchar(10);not null"`             // snapshot: "percentage" | "fixed"
	CommissionRate  float64          `json:"commission_rate" gorm:"type:decimal(10,4);not null"`           // snapshot: the rate/value configured
	CommissionValue float64          `json:"commission_value" gorm:"type:decimal(10,2);not null"`          // calculated commission amount
	Status          CommissionStatus `json:"status" gorm:"type:varchar(10);not null;default:'pending';index"`
	PaidAt          *time.Time       `json:"paid_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	DeletedAt       *time.Time       `json:"deleted_at,omitempty" gorm:"index"`
}

func (Commission) TableName() string {
	return "commissions"
}

// CommissionDetailResponse is a single commission row for the detailed report
type CommissionDetailResponse struct {
	ID              uuid.UUID        `json:"id"`
	AppointmentID   *uuid.UUID       `json:"appointment_id,omitempty"`
	SessionPlanID   *uuid.UUID       `json:"session_plan_id,omitempty"`
	SessionDate     string           `json:"session_date,omitempty"`
	AppointmentDate time.Time        `json:"appointment_date"`
	PatientName     string           `json:"patient_name"`
	ProcedureName   string           `json:"procedure_name"`
	SessionPrice    float64          `json:"session_price"`
	CommissionType  string           `json:"commission_type"`
	CommissionRate  float64          `json:"commission_rate"`
	CommissionValue float64          `json:"commission_value"`
	Status          CommissionStatus `json:"status"`
	PaidAt          *time.Time       `json:"paid_at,omitempty"`
	Source          string           `json:"source"` // "appointment" or "group_session"
}

// ProfessionalCommissionSummary is one row in the consolidated report
type ProfessionalCommissionSummary struct {
	ProfessionalID    uuid.UUID `json:"professional_id"`
	ProfessionalName  string    `json:"professional_name"`
	ProfessionalColor string    `json:"professional_color"`
	TotalSessions     int       `json:"total_sessions"`
	TotalCommission   float64   `json:"total_commission"`
	TotalPaid         float64   `json:"total_paid"`
	TotalPending      float64   `json:"total_pending"`
}

// CommissionSummaryResponse is the full consolidated report response
type CommissionSummaryResponse struct {
	Professionals   []ProfessionalCommissionSummary `json:"professionals"`
	TotalSessions   int                             `json:"total_sessions"`
	TotalCommission float64                         `json:"total_commission"`
	TotalPaid       float64                         `json:"total_paid"`
	TotalPending    float64                         `json:"total_pending"`
}
