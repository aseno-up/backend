package model

import (
	"time"

	"github.com/google/uuid"
)

// PatientRecord represents the clinical record of a patient
type PatientRecord struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID         uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID        uuid.UUID  `json:"patient_id" gorm:"type:uuid;not null;uniqueIndex"`
	ChiefComplaint   string     `json:"chief_complaint"`   // Queixa principal
	MedicalHistory   string     `json:"medical_history"`   // Historico medico
	FamilyHistory    string     `json:"family_history"`    // Historico familiar
	Allergies        string     `json:"allergies"`         // Alergias
	Medications      string     `json:"medications"`       // Medicamentos em uso
	SurgicalHistory  string     `json:"surgical_history"`  // Historico cirurgico
	SocialHistory    string     `json:"social_history"`    // Historico social (fumo, alcool, etc)
	InitialDiagnosis string     `json:"initial_diagnosis"` // Diagnostico inicial
	TreatmentPlan    string     `json:"treatment_plan"`    // Plano de tratamento
	Observations     string     `json:"observations"`      // Observacoes gerais
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (PatientRecord) TableName() string {
	return "patient_records"
}

// PatientRecordResponse represents the API response for a patient record
type PatientRecordResponse struct {
	ID               uuid.UUID `json:"id"`
	ClinicID         uuid.UUID `json:"clinic_id"`
	PatientID        uuid.UUID `json:"patient_id"`
	ChiefComplaint   string    `json:"chief_complaint"`
	MedicalHistory   string    `json:"medical_history"`
	FamilyHistory    string    `json:"family_history"`
	Allergies        string    `json:"allergies"`
	Medications      string    `json:"medications"`
	SurgicalHistory  string    `json:"surgical_history"`
	SocialHistory    string    `json:"social_history"`
	InitialDiagnosis string    `json:"initial_diagnosis"`
	TreatmentPlan    string    `json:"treatment_plan"`
	Observations     string    `json:"observations"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ToResponse converts PatientRecord to PatientRecordResponse
func (r *PatientRecord) ToResponse() *PatientRecordResponse {
	return &PatientRecordResponse{
		ID:               r.ID,
		ClinicID:         r.ClinicID,
		PatientID:        r.PatientID,
		ChiefComplaint:   r.ChiefComplaint,
		MedicalHistory:   r.MedicalHistory,
		FamilyHistory:    r.FamilyHistory,
		Allergies:        r.Allergies,
		Medications:      r.Medications,
		SurgicalHistory:  r.SurgicalHistory,
		SocialHistory:    r.SocialHistory,
		InitialDiagnosis: r.InitialDiagnosis,
		TreatmentPlan:    r.TreatmentPlan,
		Observations:     r.Observations,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

// CreatePatientRecordRequest represents the request to create a patient record
type CreatePatientRecordRequest struct {
	ChiefComplaint   string `json:"chief_complaint"`
	MedicalHistory   string `json:"medical_history"`
	FamilyHistory    string `json:"family_history"`
	Allergies        string `json:"allergies"`
	Medications      string `json:"medications"`
	SurgicalHistory  string `json:"surgical_history"`
	SocialHistory    string `json:"social_history"`
	InitialDiagnosis string `json:"initial_diagnosis"`
	TreatmentPlan    string `json:"treatment_plan"`
	Observations     string `json:"observations"`
}

// UpdatePatientRecordRequest represents the request to update a patient record
type UpdatePatientRecordRequest struct {
	ChiefComplaint   *string `json:"chief_complaint"`
	MedicalHistory   *string `json:"medical_history"`
	FamilyHistory    *string `json:"family_history"`
	Allergies        *string `json:"allergies"`
	Medications      *string `json:"medications"`
	SurgicalHistory  *string `json:"surgical_history"`
	SocialHistory    *string `json:"social_history"`
	InitialDiagnosis *string `json:"initial_diagnosis"`
	TreatmentPlan    *string `json:"treatment_plan"`
	Observations     *string `json:"observations"`
}

// SessionNote represents a clinical note for an appointment/session
type SessionNote struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID       uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID      uuid.UUID  `json:"patient_id" gorm:"type:uuid;not null;index"`
	AppointmentID  *uuid.UUID `json:"appointment_id,omitempty" gorm:"type:uuid;index"` // Optional link to appointment
	ProfessionalID uuid.UUID  `json:"professional_id" gorm:"type:uuid;not null;index"`
	SessionDate    time.Time  `json:"session_date" gorm:"not null"`
	SessionType    string     `json:"session_type"`                    // Tipo de sessão (ex: avaliação, alta, terapia manual)
	Subjective     string     `json:"subjective"`                      // Queixa do paciente (S do SOAP)
	Objective      string     `json:"objective"`                       // Exame fisico/observacoes (O do SOAP)
	Assessment     string     `json:"assessment"`                      // Avaliacao/diagnostico (A do SOAP)
	Plan           string     `json:"plan"`                            // Plano de tratamento (P do SOAP)
	Notes          string     `json:"notes"`                           // Observacoes livres da sessao
	IsPrivate      bool       `json:"is_private" gorm:"default:false"` // Visivel apenas para o profissional
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (SessionNote) TableName() string {
	return "session_notes"
}

// SessionNoteResponse represents the API response for a session note
type SessionNoteResponse struct {
	ID               uuid.UUID  `json:"id"`
	ClinicID         uuid.UUID  `json:"clinic_id"`
	PatientID        uuid.UUID  `json:"patient_id"`
	AppointmentID    *uuid.UUID `json:"appointment_id,omitempty"`
	ProfessionalID   uuid.UUID  `json:"professional_id"`
	ProfessionalName string     `json:"professional_name,omitempty"`
	SessionDate      time.Time  `json:"session_date"`
	SessionType      string     `json:"session_type"`
	Subjective       string     `json:"subjective"`
	Objective        string     `json:"objective"`
	Assessment       string     `json:"assessment"`
	Plan             string     `json:"plan"`
	Notes            string     `json:"notes"`
	IsPrivate        bool       `json:"is_private"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ToResponse converts SessionNote to SessionNoteResponse
func (n *SessionNote) ToResponse() *SessionNoteResponse {
	return &SessionNoteResponse{
		ID:             n.ID,
		ClinicID:       n.ClinicID,
		PatientID:      n.PatientID,
		AppointmentID:  n.AppointmentID,
		ProfessionalID: n.ProfessionalID,
		SessionDate:    n.SessionDate,
		SessionType:    n.SessionType,
		Subjective:     n.Subjective,
		Objective:      n.Objective,
		Assessment:     n.Assessment,
		Plan:           n.Plan,
		Notes:          n.Notes,
		IsPrivate:      n.IsPrivate,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
	}
}

// CreateSessionNoteRequest represents the request to create a session note
type CreateSessionNoteRequest struct {
	AppointmentID *uuid.UUID `json:"appointment_id"`
	SessionDate   time.Time  `json:"session_date" binding:"required"`
	SessionType   string     `json:"session_type"`
	Subjective    string     `json:"subjective"`
	Objective     string     `json:"objective"`
	Assessment    string     `json:"assessment"`
	Plan          string     `json:"plan"`
	Notes         string     `json:"notes"`
	IsPrivate     bool       `json:"is_private"`
}

// UpdateSessionNoteRequest represents the request to update a session note
type UpdateSessionNoteRequest struct {
	SessionDate *time.Time `json:"session_date"`
	SessionType *string    `json:"session_type"`
	Subjective  *string    `json:"subjective"`
	Objective   *string    `json:"objective"`
	Assessment  *string    `json:"assessment"`
	Plan        *string    `json:"plan"`
	Notes       *string    `json:"notes"`
	IsPrivate   *bool      `json:"is_private"`
}

// SessionNoteListResponse represents a paginated list of session notes
type SessionNoteListResponse struct {
	Data       []*SessionNoteResponse `json:"data"`
	Total      int64                  `json:"total"`
	Page       int                    `json:"page"`
	PerPage    int                    `json:"per_page"`
	TotalPages int                    `json:"total_pages"`
}

// FullPatientRecordResponse includes record and session notes
type FullPatientRecordResponse struct {
	Record       *PatientRecordResponse `json:"record"`
	SessionNotes []*SessionNoteResponse `json:"session_notes"`
	Patient      *PatientBasicInfo      `json:"patient"`
}

// PatientBasicInfo contains basic patient info for the record view
type PatientBasicInfo struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Phone     string     `json:"phone"`
	BirthDate *time.Time `json:"birth_date,omitempty"`
	Gender    string     `json:"gender,omitempty"`
}
