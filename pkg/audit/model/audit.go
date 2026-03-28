package model

import (
	"time"

	"encoding/json"

	"github.com/google/uuid"
)

// AuditAction represents the type of action performed
type AuditAction string

const (
	AuditActionCreate AuditAction = "CREATE"
	AuditActionUpdate AuditAction = "UPDATE"
	AuditActionDelete AuditAction = "DELETE"
	AuditActionLogin  AuditAction = "LOGIN"
	AuditActionLogout AuditAction = "LOGOUT"
)

// AuditEntityType represents the type of entity affected
type AuditEntityType string

const (
	AuditEntityUser         AuditEntityType = "USER"
	AuditEntityPatient      AuditEntityType = "PATIENT"
	AuditEntityAppointment  AuditEntityType = "APPOINTMENT"
	AuditEntityProfessional AuditEntityType = "PROFESSIONAL"
	AuditEntityClinic       AuditEntityType = "CLINIC"
	AuditEntitySettings     AuditEntityType = "SETTINGS"
	AuditEntityInvite       AuditEntityType = "INVITE"
	AuditEntityProcedure    AuditEntityType = "PROCEDURE"
	AuditEntitySessionPlan  AuditEntityType = "SESSION_PLAN"
	AuditEntityWaitlist        AuditEntityType = "WAITLIST"
	AuditEntityPaymentMethod   AuditEntityType = "PAYMENT_METHOD"
	AuditEntityAppointmentPayment AuditEntityType = "APPOINTMENT_PAYMENT"
)

// AuditLog represents an audit trail entry
type AuditLog struct {
	ID         uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID   uuid.UUID       `json:"clinic_id" gorm:"type:uuid;not null;index"`
	UserID     uuid.UUID       `json:"user_id" gorm:"type:uuid;not null;index"`
	UserEmail  string          `json:"user_email" gorm:"type:varchar(255)"`
	UserRole   string          `json:"user_role" gorm:"type:varchar(50)"`
	Action     AuditAction     `json:"action" gorm:"type:varchar(50);not null;index"`
	EntityType AuditEntityType `json:"entity_type" gorm:"type:varchar(50);not null;index"`
	EntityID   string          `json:"entity_id" gorm:"type:varchar(255)"`
	OldData    json.RawMessage  `json:"old_data,omitempty" gorm:"type:jsonb"`
	NewData    json.RawMessage  `json:"new_data,omitempty" gorm:"type:jsonb"`
	IP         string          `json:"ip" gorm:"type:varchar(45)"`
	UserAgent  string          `json:"user_agent" gorm:"type:varchar(500)"`
	CreatedAt  time.Time       `json:"created_at" gorm:"index"`
}

// TableName returns the table name
func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditLogFilter represents filters for listing audit logs
type AuditLogFilter struct {
	Page       int             `form:"page"`
	Limit      int             `form:"limit"`
	Action     AuditAction     `form:"action"`
	EntityType AuditEntityType `form:"entity_type"`
	UserID     string          `form:"user_id"`
	EntityID   string          `form:"entity_id"`
	StartDate  string          `form:"start_date"`
	EndDate    string          `form:"end_date"`
}

// GetLimit ensures a valid limit
func (f *AuditLogFilter) GetLimit() int {
	if f.Limit <= 0 {
		return 20
	}
	if f.Limit > 100 {
		return 100
	}
	return f.Limit
}

// GetPage ensures a valid page
func (f *AuditLogFilter) GetPage() int {
	if f.Page <= 0 {
		return 1
	}
	return f.Page
}

// GetOffset calculates SQL offset
func (f *AuditLogFilter) GetOffset() int {
	return (f.GetPage() - 1) * f.GetLimit()
}

// AuditLogResponse represents the paginated response
type AuditLogResponse struct {
	Data []AuditLog `json:"data"`
	Meta struct {
		Page       int   `json:"page"`
		Limit      int   `json:"limit"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"total_pages"`
	} `json:"meta"`
}
