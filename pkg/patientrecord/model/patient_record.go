package model

import (
	"time"

	"github.com/google/uuid"
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSON is a custom type for JSONB columns (objects only)
type JSON map[string]interface{}

// Value implements driver.Valuer for JSON
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner for JSON
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for JSON")
	}

	return json.Unmarshal(data, j)
}

// RawJSON accepts any valid JSON (arrays, objects, etc.)
type RawJSON json.RawMessage

// Value implements driver.Valuer for RawJSON
func (r RawJSON) Value() (driver.Value, error) {
	if len(r) == 0 {
		return nil, nil
	}
	return []byte(r), nil
}

// Scan implements sql.Scanner for RawJSON
func (r *RawJSON) Scan(value interface{}) error {
	if value == nil {
		*r = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*r = append((*r)[0:0], v...)
	case string:
		*r = []byte(v)
	default:
		return errors.New("invalid type for RawJSON")
	}
	return nil
}

// MarshalJSON implements json.Marshaler
func (r RawJSON) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return []byte(r), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (r *RawJSON) UnmarshalJSON(data []byte) error {
	*r = append((*r)[0:0], data...)
	return nil
}

// NewRawJSON creates a RawJSON from any serializable value
func NewRawJSON(v interface{}) RawJSON {
	data, _ := json.Marshal(v)
	return RawJSON(data)
}

// PatientRecord represents the clinical record of a patient
type PatientRecord struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID     uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	PatientID    uuid.UUID  `json:"patient_id" gorm:"type:uuid;not null;index"`
	TemplateID   *uuid.UUID `json:"template_id,omitempty" gorm:"type:uuid;index"`
	CustomFields JSON       `json:"custom_fields,omitempty" gorm:"type:jsonb"`
	RecordedAt   time.Time  `json:"recorded_at" gorm:"not null;default:now()"`
	RecordedBy   uuid.UUID  `json:"recorded_by" gorm:"type:uuid;not null"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// AnamneseTemplate represents a template for clinical records
type AnamneseTemplate struct {
	ID        uuid.UUID                `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID  uuid.UUID                `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Name      string                   `json:"name" gorm:"not null"`
	IsDefault bool                     `json:"is_default" gorm:"not null;default:false"`
	Fields    []AnamneseTemplateField  `json:"fields,omitempty" gorm:"foreignKey:TemplateID"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
	DeletedAt *time.Time               `json:"deleted_at,omitempty" gorm:"index"`
}

// AnamneseTemplateField represents a single field in a template
type AnamneseTemplateField struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TemplateID   uuid.UUID  `json:"template_id" gorm:"type:uuid;not null;index"`
	Label        string     `json:"label" gorm:"not null"`
	FieldKey     string     `json:"field_key" gorm:"not null"`
	FieldType    string     `json:"field_type" gorm:"not null"` // text, textarea, select, multiselect, scale, section_header
	Options      RawJSON    `json:"options,omitempty" gorm:"type:jsonb"`
	Required     bool       `json:"required" gorm:"not null;default:false"`
	DisplayOrder int        `json:"display_order" gorm:"not null;default:0"`
	Placeholder  string     `json:"placeholder"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// AnamneseTemplateSpecialty represents the many-to-many relationship between templates and specialties
type AnamneseTemplateSpecialty struct {
	TemplateID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	SpecialtyID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

// TableName returns the table names for GORM
func (PatientRecord) TableName() string             { return "patient_records" }
func (AnamneseTemplate) TableName() string          { return "anamnese_templates" }
func (AnamneseTemplateField) TableName() string     { return "anamnese_template_fields" }
func (AnamneseTemplateSpecialty) TableName() string { return "anamnese_template_specialties" }

// PatientRecordResponse represents the API response for a patient record
type PatientRecordResponse struct {
	ID           uuid.UUID             `json:"id"`
	ClinicID     uuid.UUID             `json:"clinic_id"`
	PatientID    uuid.UUID             `json:"patient_id"`
	TemplateID   *uuid.UUID            `json:"template_id,omitempty"`
	Template     *AnamneseTemplate     `json:"template,omitempty"`
	CustomFields JSON                  `json:"custom_fields,omitempty"`
	RecordedAt   time.Time             `json:"recorded_at"`
	RecordedBy   uuid.UUID             `json:"recorded_by"`
	Professional *PatientBasicInfo     `json:"professional,omitempty"` // Basic info of the person who recorded it
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

// ToResponse converts PatientRecord to PatientRecordResponse
func (r *PatientRecord) ToResponse() *PatientRecordResponse {
	return &PatientRecordResponse{
		ID:           r.ID,
		ClinicID:     r.ClinicID,
		PatientID:    r.PatientID,
		TemplateID:   r.TemplateID,
		CustomFields: r.CustomFields,
		RecordedAt:   r.RecordedAt,
		RecordedBy:   r.RecordedBy,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

// CreatePatientRecordRequest represents the request to create a patient record
type CreatePatientRecordRequest struct {
	TemplateID   *uuid.UUID `json:"template_id"`
	CustomFields JSON       `json:"custom_fields" binding:"required"`
}

// UpdatePatientRecordRequest represents the request to update a patient record
// Note: In append-only, this actually creates a new version
type UpdatePatientRecordRequest struct {
	TemplateID   *uuid.UUID `json:"template_id"`
	CustomFields JSON       `json:"custom_fields" binding:"required"`
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
