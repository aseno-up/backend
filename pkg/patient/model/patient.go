package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Gender represents patient gender
type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

// NoShowRisk represents patient risk level for no-shows
type NoShowRisk string

const (
	NoShowRiskUnknown NoShowRisk = "unknown" // Less than 3 appointments
	NoShowRiskLow     NoShowRisk = "low"     // >= 90% attendance
	NoShowRiskMedium  NoShowRisk = "medium"  // 70-89% attendance
	NoShowRiskHigh    NoShowRisk = "high"    // < 70% attendance
)

// PatientSource represents how the patient was registered
type PatientSource string

const (
	PatientSourceInternal       PatientSource = "INTERNAL"         // Registered by reception/staff
	PatientSourceSelfRegistered PatientSource = "SELF_REGISTERED"  // Registered via public booking link
	PatientSourceWhatsApp       PatientSource = "WHATSAPP_INBOUND" // Auto-created from incoming WhatsApp message
)

// Patient represents a patient in the clinic
type Patient struct {
	ID        uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID  uuid.UUID     `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Name      string        `json:"name" gorm:"not null"`
	Email     string        `json:"email" gorm:"index"`
	Phone     string        `json:"phone" gorm:"index"`
	WhatsApp  string        `json:"whatsapp" gorm:"column:whatsapp"` // May differ from phone
	Document  string        `json:"document"`                        // CPF
	BirthDate *time.Time    `json:"birth_date"`
	Gender    Gender        `json:"gender"`
	Address   string        `json:"address"`
	City      string        `json:"city"`
	State     string        `json:"state"`
	ZipCode   string        `json:"zip_code"`
	Notes     string        `json:"notes"` // General notes
	IsActive  bool          `json:"is_active" gorm:"default:true"`
	Source    PatientSource `json:"source" gorm:"type:varchar(20);not null;default:'INTERNAL'"`
	// Merge tracking
	MergedIntoPatientID *uuid.UUID `json:"merged_into_patient_id,omitempty" gorm:"type:uuid;index"`
	// Risk assessment fields
	NoShowRisk        NoShowRisk `json:"no_show_risk" gorm:"default:'unknown'"`
	TotalAppointments int        `json:"total_appointments" gorm:"default:0"`
	NoShowCount       int        `json:"no_show_count" gorm:"default:0"`
	AttendanceRate    float64    `json:"attendance_rate" gorm:"default:0"`
	RiskUpdatedAt     *time.Time `json:"risk_updated_at,omitempty"`
	// Timestamps
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (Patient) TableName() string {
	return "patients"
}

// PatientResponse represents the API response for a patient
type PatientResponse struct {
	ID                  uuid.UUID     `json:"id"`
	ClinicID            uuid.UUID     `json:"clinic_id"`
	Name                string        `json:"name"`
	Email               string        `json:"email"`
	Phone               string        `json:"phone"`
	WhatsApp            string        `json:"whatsapp"`
	Document            string        `json:"document"`
	BirthDate           *time.Time    `json:"birth_date,omitempty"`
	Gender              Gender        `json:"gender,omitempty"`
	Address             string        `json:"address"`
	City                string        `json:"city"`
	State               string        `json:"state"`
	ZipCode             string        `json:"zip_code"`
	Notes               string        `json:"notes"`
	IsActive            bool          `json:"is_active"`
	Source              PatientSource `json:"source"`
	MergedIntoPatientID *uuid.UUID    `json:"merged_into_patient_id,omitempty"`
	NoShowRisk          NoShowRisk    `json:"no_show_risk"`
	TotalAppointments   int           `json:"total_appointments"`
	NoShowCount         int           `json:"no_show_count"`
	AttendanceRate      float64       `json:"attendance_rate"`
	RiskUpdatedAt       *time.Time    `json:"risk_updated_at,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
}

// ToResponse converts Patient to PatientResponse
func (p *Patient) ToResponse() *PatientResponse {
	return &PatientResponse{
		ID:                  p.ID,
		ClinicID:            p.ClinicID,
		Name:                p.Name,
		Email:               p.Email,
		Phone:               p.Phone,
		WhatsApp:            p.WhatsApp,
		Document:            p.Document,
		BirthDate:           p.BirthDate,
		Gender:              p.Gender,
		Address:             p.Address,
		City:                p.City,
		State:               p.State,
		ZipCode:             p.ZipCode,
		Notes:               p.Notes,
		IsActive:            p.IsActive,
		Source:              p.Source,
		MergedIntoPatientID: p.MergedIntoPatientID,
		NoShowRisk:          p.NoShowRisk,
		TotalAppointments:   p.TotalAppointments,
		NoShowCount:         p.NoShowCount,
		AttendanceRate:      p.AttendanceRate,
		RiskUpdatedAt:       p.RiskUpdatedAt,
		CreatedAt:           p.CreatedAt,
	}
}

// GetWhatsAppNumber returns the WhatsApp number or phone if not set
func (p *Patient) GetWhatsAppNumber() string {
	if p.WhatsApp != "" {
		return p.WhatsApp
	}
	return p.Phone
}

// CreatePatientRequest represents the request to create a patient.
// BirthDate is a string so the handler accepts date-only payloads from the
// HTML `<input type="date">` (e.g. "1975-07-30") without requiring RFC3339.
type CreatePatientRequest struct {
	Name      string        `json:"name" binding:"required,min=2,max=100"`
	Email     string        `json:"email" binding:"omitempty,email"`
	Phone     string        `json:"phone" binding:"required"`
	WhatsApp  string        `json:"whatsapp"`
	Document  string        `json:"document"`
	BirthDate *string       `json:"birth_date"`
	Gender    Gender        `json:"gender" binding:"omitempty,oneof=male female other"`
	Address   string        `json:"address"`
	City      string        `json:"city"`
	State     string        `json:"state"`
	ZipCode   string        `json:"zip_code"`
	Notes     string        `json:"notes"`
	Source    PatientSource `json:"source,omitempty"`
}

// UpdatePatientRequest represents the request to update a patient.
// See CreatePatientRequest for why BirthDate is a string pointer.
type UpdatePatientRequest struct {
	Name      string  `json:"name" binding:"omitempty,min=2,max=100"`
	Email     string  `json:"email" binding:"omitempty,email"`
	Phone     string  `json:"phone"`
	WhatsApp  string  `json:"whatsapp"`
	Document  string  `json:"document"`
	BirthDate *string `json:"birth_date"`
	Gender    Gender  `json:"gender" binding:"omitempty,oneof=male female other"`
	Address   string  `json:"address"`
	City      string  `json:"city"`
	State     string  `json:"state"`
	ZipCode   string  `json:"zip_code"`
	Notes     string  `json:"notes"`
	IsActive  *bool   `json:"is_active"`
}

// ParseBirthDate accepts an empty string, RFC3339, or YYYY-MM-DD and returns
// a parsed time or nil when the caller explicitly sent an empty string.
func ParseBirthDate(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	if *raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, *raw); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02", *raw); err == nil {
		return &t, nil
	}
	return nil, fmt.Errorf("invalid birth_date format %q (expected YYYY-MM-DD)", *raw)
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
	Query   string `form:"q"` // Search by name, phone, email
	Page    int    `form:"page"`
	PerPage int    `form:"per_page"`
}

// MergePatientRequest represents the request to merge a self-registered patient into an existing one
type MergePatientRequest struct {
	SourcePatientID uuid.UUID `json:"source_patient_id" binding:"required"`
}

// BulkMergePatientRequest represents the request to merge multiple provisional patients into a target
type BulkMergePatientRequest struct {
	ProvisionalIDs  []uuid.UUID `json:"provisional_ids" binding:"required,min=1"`
	TargetPatientID uuid.UUID   `json:"target_patient_id" binding:"required"`
}

// BulkMergePatientResponse represents the response after a bulk merge operation
type BulkMergePatientResponse struct {
	AppointmentsReassigned int64     `json:"appointments_reassigned"`
	TargetPatientID        uuid.UUID `json:"target_patient_id"`
	MergedCount            int       `json:"merged_count"`
}

// UnlinkedPatientResponse represents a provisional patient pending verification
type UnlinkedPatientResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CPF       string    `json:"cpf"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
}

// PatientSearchResult represents a search result for merge target selection
type PatientSearchResult struct {
	ID     uuid.UUID     `json:"id"`
	Name   string        `json:"name"`
	CPF    string        `json:"cpf"`
	Email  string        `json:"email"`
	Phone  string        `json:"phone"`
	Source PatientSource `json:"source"`
}

// PatientAppointmentStats holds appointment statistics for risk calculation
type PatientAppointmentStats struct {
	TotalAppointments int `json:"total_appointments"`
	CompletedCount    int `json:"completed_count"`
	NoShowCount       int `json:"no_show_count"`
}

// PatientDeletePreview describes what the UI should show before confirming
// the deletion of a patient — the user just needs to know how many linked
// records exist so they can decide whether to proceed.
type PatientDeletePreview struct {
	AppointmentsCount int `json:"appointments_count"`
	SessionPlansCount int `json:"session_plans_count"`
	EnrollmentsCount  int `json:"enrollments_count"`
	PaymentsCount     int `json:"payments_count"`
}

// PatientRiskResponse represents the risk assessment for a patient
type PatientRiskResponse struct {
	PatientID         uuid.UUID  `json:"patient_id"`
	PatientName       string     `json:"patient_name"`
	NoShowRisk        NoShowRisk `json:"no_show_risk"`
	RiskLabel         string     `json:"risk_label"`
	TotalAppointments int        `json:"total_appointments"`
	NoShowCount       int        `json:"no_show_count"`
	AttendanceRate    float64    `json:"attendance_rate"`
	RiskUpdatedAt     *time.Time `json:"risk_updated_at,omitempty"`
}

// GetRiskLabel returns a human-readable label for the risk level
func GetRiskLabel(risk NoShowRisk) string {
	switch risk {
	case NoShowRiskLow:
		return "Confiável"
	case NoShowRiskMedium:
		return "Médio risco"
	case NoShowRiskHigh:
		return "Alto risco"
	default:
		return "Sem dados suficientes"
	}
}

// ToRiskResponse converts Patient to PatientRiskResponse
func (p *Patient) ToRiskResponse() *PatientRiskResponse {
	return &PatientRiskResponse{
		PatientID:         p.ID,
		PatientName:       p.Name,
		NoShowRisk:        p.NoShowRisk,
		RiskLabel:         GetRiskLabel(p.NoShowRisk),
		TotalAppointments: p.TotalAppointments,
		NoShowCount:       p.NoShowCount,
		AttendanceRate:    p.AttendanceRate,
		RiskUpdatedAt:     p.RiskUpdatedAt,
	}
}
