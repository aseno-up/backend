package model

import (
	"time"

	"github.com/google/uuid"
)

// UsageMetric represents a persisted usage metric per clinic per day
type UsageMetric struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID uuid.UUID `json:"clinic_id" gorm:"type:uuid;not null;uniqueIndex:idx_metric_clinic_date_key"`
	Metric   string    `json:"metric" gorm:"type:varchar(100);not null;uniqueIndex:idx_metric_clinic_date_key"`
	Value    float64   `json:"value" gorm:"not null;default:0"`
	Date     time.Time `json:"date" gorm:"type:date;not null;uniqueIndex:idx_metric_clinic_date_key;index"`
}

// TableName returns the table name
func (UsageMetric) TableName() string {
	return "usage_metrics"
}

// Common metric keys
const (
	MetricAppointmentsCreated   = "appointments_created"
	MetricAppointmentsCancelled = "appointments_cancelled"
	MetricAppointmentsNoShow    = "appointments_no_show"
	MetricAppointmentsCompleted = "appointments_completed"
	MetricMessagesSent          = "messages_sent"
	MetricMessagesDelivered     = "messages_delivered"
	MetricMessagesFailed        = "messages_failed"
	MetricMessagesConfirmed     = "messages_confirmed"
	MetricAIConversations       = "ai_conversations"
	MetricAITokensUsed          = "ai_tokens_used"
	MetricPatientsCreated       = "patients_created"
	MetricActivePatients        = "active_patients"
)

// UsageMetricFilter represents filters for listing usage metrics
type UsageMetricFilter struct {
	Page      int    `form:"page"`
	Limit     int    `form:"limit"`
	ClinicID  string `form:"clinic_id"`
	Metric    string `form:"metric"`
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
}

// GetLimit ensures a valid limit
func (f *UsageMetricFilter) GetLimit() int {
	if f.Limit <= 0 {
		return 50
	}
	if f.Limit > 200 {
		return 200
	}
	return f.Limit
}

// GetPage ensures a valid page
func (f *UsageMetricFilter) GetPage() int {
	if f.Page <= 0 {
		return 1
	}
	return f.Page
}

// GetOffset calculates SQL offset
func (f *UsageMetricFilter) GetOffset() int {
	return (f.GetPage() - 1) * f.GetLimit()
}

// UsageMetricSummary represents aggregated metrics for a clinic
type UsageMetricSummary struct {
	ClinicID uuid.UUID              `json:"clinic_id"`
	Period   string                 `json:"period"`
	Metrics  map[string]float64     `json:"metrics"`
}

// UsageMetricResponse represents the response
type UsageMetricResponse struct {
	Data []UsageMetric `json:"data"`
	Meta struct {
		Page       int   `json:"page"`
		Limit      int   `json:"limit"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"total_pages"`
	} `json:"meta"`
}

// UsageSummaryResponse represents the summary response
type UsageSummaryResponse struct {
	Data []UsageMetricSummary `json:"data"`
}
