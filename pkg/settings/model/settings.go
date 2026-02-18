package model

import (
	"time"

	"github.com/google/uuid"
)

// SystemSetting represents a system-wide configuration setting
type SystemSetting struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Key         string    `json:"key" gorm:"uniqueIndex;size:100;not null"`
	Value       string    `json:"value" gorm:"not null"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName returns the table name for GORM
func (SystemSetting) TableName() string {
	return "system_settings"
}

// UpdateSettingRequest represents the request to update a setting
type UpdateSettingRequest struct {
	Value string `json:"value" binding:"required"`
}

// SettingsResponse represents the API response for all settings
type SettingsResponse struct {
	Settings map[string]SettingDetail `json:"settings"`
}

// SettingDetail represents a single setting with its metadata
type SettingDetail struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// Default setting keys
const (
	SettingPlanBasicLimit       = "plan_basic_limit"
	SettingPlanProLimit         = "plan_pro_limit"
	SettingPlanAdvancedLimit    = "plan_advanced_limit"
	SettingPlanExcessPrice      = "plan_excess_price"
	SettingExcessBillingEnabled = "excess_billing_enabled"
	SettingPlanBasicPrice       = "plan_basic_price"
	SettingPlanProPrice         = "plan_pro_price"
	SettingPlanAdvancedPrice    = "plan_advanced_price"
	SettingTrialDays            = "trial_days"

	// WhatsApp Rate Limiting & Message Segmentation
	SettingWhatsAppMinDelayBetweenPatients = "whatsapp_min_delay_between_patients"
	SettingWhatsAppMinDelayBetweenMessages = "whatsapp_min_delay_between_messages"
	SettingWhatsAppMaxDelayBetweenMessages = "whatsapp_max_delay_between_messages"
	SettingWhatsAppSegmentationEnabled     = "whatsapp_segmentation_enabled"
)

// DefaultSettings returns the default settings to seed the database
func DefaultSettings() []SystemSetting {
	return []SystemSetting{
		{Key: SettingPlanBasicLimit, Value: "5", Description: "Limite de profissionais do plano Básico"},
		{Key: SettingPlanProLimit, Value: "15", Description: "Limite de profissionais do plano Profissional"},
		{Key: SettingPlanAdvancedLimit, Value: "-1", Description: "Limite do plano Avançado (-1 = ilimitado)"},
		{Key: SettingPlanExcessPrice, Value: "20", Description: "Preço por profissional excedente (BRL)"},
		{Key: SettingExcessBillingEnabled, Value: "false", Description: "Cobrança de excedente ativa"},
		{Key: SettingPlanBasicPrice, Value: "99", Description: "Preço mensal plano Básico (BRL)"},
		{Key: SettingPlanProPrice, Value: "199", Description: "Preço mensal plano Profissional (BRL)"},
		{Key: SettingPlanAdvancedPrice, Value: "349", Description: "Preço mensal plano Avançado (BRL)"},
		{Key: SettingTrialDays, Value: "14", Description: "Dias de trial gratuito"},

		// WhatsApp Rate Limiting & Message Segmentation
		{Key: SettingWhatsAppMinDelayBetweenPatients, Value: "60", Description: "Delay mínimo entre mensagens para pacientes diferentes (segundos)"},
		{Key: SettingWhatsAppMinDelayBetweenMessages, Value: "3", Description: "Delay mínimo entre mensagens segmentadas (segundos)"},
		{Key: SettingWhatsAppMaxDelayBetweenMessages, Value: "10", Description: "Delay máximo entre mensagens segmentadas (segundos)"},
		{Key: SettingWhatsAppSegmentationEnabled, Value: "true", Description: "Habilitar segmentação de mensagens do WhatsApp"},
	}
}
