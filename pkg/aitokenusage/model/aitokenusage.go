package model

import (
	"time"

	"github.com/google/uuid"
)

// AITokenUsage records a single AI generation's resource consumption.
// One row per call so cost can be attributed per-clinic per-model. The daily
// aggregate in usage_metrics.ai_tokens_used remains the source of truth for
// high-level KPIs; this table sits alongside it for granular analysis.
type AITokenUsage struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID       uuid.UUID `json:"clinic_id" gorm:"type:uuid;not null;index"`
	ConversationID uuid.UUID `json:"conversation_id" gorm:"type:uuid;not null;index"`
	OccurredAt     time.Time `json:"occurred_at" gorm:"not null;default:now();index"`
	Date           time.Time `json:"date" gorm:"type:date;not null;index"`
	Provider       string    `json:"provider" gorm:"type:varchar(50);not null;index"`
	Model          string    `json:"model" gorm:"type:varchar(120);not null;index"`
	InputTokens    int       `json:"input_tokens" gorm:"not null;default:0"`
	OutputTokens   int       `json:"output_tokens" gorm:"not null;default:0"`
	TotalTokens    int       `json:"total_tokens" gorm:"not null;default:0"`
	CostBRL        float64   `json:"cost_brl" gorm:"not null;default:0"`
}

// TableName returns the table name
func (AITokenUsage) TableName() string {
	return "ai_token_usage"
}

// ModelSummary aggregates consumption by (provider, model) for a clinic over a period.
type ModelSummary struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Calls        int64   `json:"calls"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	CostBRL      float64 `json:"cost_brl"`
}

// ClinicModelSummary is a per-clinic per-model aggregation row used by the
// admin dashboard to compare which model each clinic actually consumed.
type ClinicModelSummary struct {
	ClinicID     uuid.UUID `json:"clinic_id"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Calls        int64     `json:"calls"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	TotalTokens  int64     `json:"total_tokens"`
	CostBRL      float64   `json:"cost_brl"`
}

// ClinicModelSummaryResponse is the JSON envelope for the admin endpoint.
type ClinicModelSummaryResponse struct {
	Data   []ClinicModelSummary `json:"data"`
	Period string               `json:"period"`
}
