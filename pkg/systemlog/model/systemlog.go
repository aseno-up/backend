package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LogStatus represents the status of a system event
type LogStatus string

const (
	LogStatusSuccess LogStatus = "success"
	LogStatusFailed  LogStatus = "failed"
	LogStatusWarning LogStatus = "warning"
)

// SystemLog represents a system-level log entry
type SystemLog struct {
	ID        uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID  *uuid.UUID      `json:"clinic_id,omitempty" gorm:"type:uuid;index"`
	Service   string          `json:"service" gorm:"type:varchar(100);not null;index"`
	Event     string          `json:"event" gorm:"type:varchar(100);not null;index"`
	Status    LogStatus       `json:"status" gorm:"type:varchar(20);not null;index"`
	Payload   json.RawMessage `json:"payload,omitempty" gorm:"type:jsonb"`
	Error     string          `json:"error,omitempty" gorm:"type:text"`
	Duration  int             `json:"duration_ms,omitempty" gorm:"column:duration_ms"`
	CreatedAt time.Time       `json:"created_at" gorm:"index"`
}

// TableName returns the table name
func (SystemLog) TableName() string {
	return "system_logs"
}

// SystemLogFilter represents filters for listing system logs
type SystemLogFilter struct {
	Page      int       `form:"page"`
	Limit     int       `form:"limit"`
	Service   string    `form:"service"`
	Event     string    `form:"event"`
	Status    LogStatus `form:"status"`
	ClinicID  string    `form:"clinic_id"`
	StartDate string    `form:"start_date"`
	EndDate   string    `form:"end_date"`
}

// GetLimit ensures a valid limit
func (f *SystemLogFilter) GetLimit() int {
	if f.Limit <= 0 {
		return 20
	}
	if f.Limit > 100 {
		return 100
	}
	return f.Limit
}

// GetPage ensures a valid page
func (f *SystemLogFilter) GetPage() int {
	if f.Page <= 0 {
		return 1
	}
	return f.Page
}

// GetOffset calculates SQL offset
func (f *SystemLogFilter) GetOffset() int {
	return (f.GetPage() - 1) * f.GetLimit()
}

// SystemLogResponse represents the paginated response
type SystemLogResponse struct {
	Data []SystemLog `json:"data"`
	Meta struct {
		Page       int   `json:"page"`
		Limit      int   `json:"limit"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"total_pages"`
	} `json:"meta"`
}
