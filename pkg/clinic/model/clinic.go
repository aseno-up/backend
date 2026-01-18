package model

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// SubscriptionStatus represents the subscription state
type SubscriptionStatus string

const (
	SubscriptionStatusTrial    SubscriptionStatus = "trial"
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusPastDue  SubscriptionStatus = "past_due"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusExpired  SubscriptionStatus = "expired"
)

// PlanType represents the subscription plan
type PlanType string

const (
	PlanTypeBasic        PlanType = "basic"
	PlanTypeProfessional PlanType = "professional"
	PlanTypeAdvanced     PlanType = "advanced"
)

// ClinicType represents the type of clinic account
type ClinicType string

const (
	ClinicTypeStandard ClinicType = "standard"
	ClinicTypeDev      ClinicType = "dev"
)

// Clinic represents a clinic/tenant in the system
type Clinic struct {
	ID                 uuid.UUID          `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name               string             `json:"name" gorm:"not null"`
	Slug               string             `json:"slug" gorm:"uniqueIndex;not null"`
	Type               ClinicType         `json:"type" gorm:"default:'standard'"`
	Email              string             `json:"email" gorm:"uniqueIndex;not null"`
	Phone              string             `json:"phone"`
	Document           string             `json:"document"` // CNPJ or CPF
	Address            string             `json:"address"`
	City               string             `json:"city"`
	State              string             `json:"state"`
	ZipCode            string             `json:"zip_code"`
	LogoURL            string             `json:"logo_url"`
	PrimaryColor       string             `json:"primary_color" gorm:"default:'#7C3AED'"`
	Plan               PlanType           `json:"plan" gorm:"default:'basic'"`
	SubscriptionStatus SubscriptionStatus `json:"subscription_status" gorm:"default:'trial'"`
	TrialEndsAt        *time.Time         `json:"trial_ends_at"`
	SubscriptionEndsAt *time.Time         `json:"subscription_ends_at"`
	AsaasCustomerID    string             `json:"-"` // Asaas customer ID (hidden from JSON)
	WhatsAppInstance   string             `json:"-"` // Evolution API instance name (hidden from JSON)
	WhatsAppConnected  bool               `json:"whatsapp_connected" gorm:"default:false"`
	GoogleConnected    bool               `json:"google_connected" gorm:"default:false"`
	Settings           ClinicSettings     `json:"settings" gorm:"type:jsonb;serializer:json"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	DeletedAt          *time.Time         `json:"deleted_at,omitempty" gorm:"index"`
}

// DaySchedule define o horário de um dia específico
type DaySchedule struct {
	Enabled   bool   `json:"enabled"`
	StartTime string `json:"start_time"` // Formato: "08:00"
	EndTime   string `json:"end_time"`   // Formato: "18:00"
}

// BusinessHoursConfig define o horário de atendimento da IA
type BusinessHoursConfig struct {
	Monday    *DaySchedule `json:"monday,omitempty"`
	Tuesday   *DaySchedule `json:"tuesday,omitempty"`
	Wednesday *DaySchedule `json:"wednesday,omitempty"`
	Thursday  *DaySchedule `json:"thursday,omitempty"`
	Friday    *DaySchedule `json:"friday,omitempty"`
	Saturday  *DaySchedule `json:"saturday,omitempty"`
	Sunday    *DaySchedule `json:"sunday,omitempty"`
}

// ClinicSettings contains clinic-specific settings
type ClinicSettings struct {
	Timezone                  string `json:"timezone"`
	DefaultAppointmentMinutes int    `json:"default_appointment_minutes"`
	ReminderHoursBefore       int    `json:"reminder_hours_before"`
	AllowOnlineBooking        bool   `json:"allow_online_booking"`
	RequireConfirmation       bool   `json:"require_confirmation"`
	// Notification settings
	WhatsAppRemindersEnabled bool `json:"whatsapp_reminders_enabled"`
	EmailRemindersEnabled    bool `json:"email_reminders_enabled"`

	// AI/WhatsApp settings
	AIMinDelayBetweenMessages *int                 `json:"ai_min_delay_between_messages,omitempty"`
	AIMinDelayBetweenPatients *int                 `json:"ai_min_delay_between_patients,omitempty"`
	AIBusinessHours           *BusinessHoursConfig `json:"ai_business_hours,omitempty"`
}

// TableName returns the table name for GORM
func (Clinic) TableName() string {
	return "clinics"
}

// ClinicResponse represents the API response for a clinic
type ClinicResponse struct {
	ID                 uuid.UUID          `json:"id"`
	Name               string             `json:"name"`
	Slug               string             `json:"slug"`
	Type               ClinicType         `json:"type"`
	Email              string             `json:"email"`
	Phone              string             `json:"phone"`
	Address            string             `json:"address"`
	City               string             `json:"city"`
	State              string             `json:"state"`
	LogoURL            string             `json:"logo_url"`
	PrimaryColor       string             `json:"primary_color"`
	Plan               PlanType           `json:"plan"`
	SubscriptionStatus SubscriptionStatus `json:"subscription_status"`
	TrialEndsAt        *time.Time         `json:"trial_ends_at,omitempty"`
	WhatsAppConnected  bool               `json:"whatsapp_connected"`
	GoogleConnected    bool               `json:"google_connected"`
	Settings           ClinicSettings     `json:"settings"`
	BookingURL         string             `json:"booking_url,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
}

// ToResponse converts Clinic to ClinicResponse
func (c *Clinic) ToResponse() *ClinicResponse {
	return &ClinicResponse{
		ID:                 c.ID,
		Name:               c.Name,
		Slug:               c.Slug,
		Type:               c.Type,
		Email:              c.Email,
		Phone:              c.Phone,
		Address:            c.Address,
		City:               c.City,
		State:              c.State,
		LogoURL:            c.LogoURL,
		PrimaryColor:       c.PrimaryColor,
		Plan:               c.Plan,
		SubscriptionStatus: c.SubscriptionStatus,
		TrialEndsAt:        c.TrialEndsAt,
		WhatsAppConnected:  c.WhatsAppConnected,
		GoogleConnected:    c.GoogleConnected,
		Settings:           c.Settings,
		CreatedAt:          c.CreatedAt,
	}
}

// IsSubscriptionActive checks if the clinic has an active subscription
func (c *Clinic) IsSubscriptionActive() bool {
	switch c.SubscriptionStatus {
	case SubscriptionStatusActive:
		return true
	case SubscriptionStatusTrial:
		if c.TrialEndsAt != nil {
			return time.Now().Before(*c.TrialEndsAt)
		}
		return false
	default:
		return false
	}
}

// CreateClinicRequest represents the request to create a clinic
type CreateClinicRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone"`
	Document string `json:"document"`
}

// UpdateClinicSettingsRequest represents settings that can be updated (using pointers for optional booleans)
type UpdateClinicSettingsRequest struct {
	Timezone                  string `json:"timezone,omitempty"`
	DefaultAppointmentMinutes int    `json:"default_appointment_minutes,omitempty"`
	ReminderHoursBefore       int    `json:"reminder_hours_before,omitempty"`
	AllowOnlineBooking        *bool  `json:"allow_online_booking,omitempty"`
	RequireConfirmation       *bool  `json:"require_confirmation,omitempty"`
	WhatsAppRemindersEnabled  *bool  `json:"whatsapp_reminders_enabled,omitempty"`
	EmailRemindersEnabled     *bool  `json:"email_reminders_enabled,omitempty"`

	// AI/WhatsApp settings
	AIMinDelayBetweenMessages *int                 `json:"ai_min_delay_between_messages,omitempty"`
	AIMinDelayBetweenPatients *int                 `json:"ai_min_delay_between_patients,omitempty"`
	AIBusinessHours           *BusinessHoursConfig `json:"ai_business_hours,omitempty"`
}

// UpdateClinicRequest represents the request to update a clinic
type UpdateClinicRequest struct {
	Name         string                       `json:"name" binding:"omitempty,min=2,max=100"`
	Phone        string                       `json:"phone"`
	Address      string                       `json:"address"`
	City         string                       `json:"city"`
	State        string                       `json:"state"`
	ZipCode      string                       `json:"zip_code"`
	LogoURL      string                       `json:"logo_url"`
	PrimaryColor string                       `json:"primary_color"`
	Settings     *UpdateClinicSettingsRequest `json:"settings"`
}

// Validate valida a configuração de horário
func (b *BusinessHoursConfig) Validate() error {
	if b == nil {
		return nil
	}

	days := []*DaySchedule{b.Monday, b.Tuesday, b.Wednesday, b.Thursday, b.Friday, b.Saturday, b.Sunday}
	dayNames := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	timeRegex := regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

	for i, day := range days {
		if day == nil || !day.Enabled {
			continue
		}

		if !timeRegex.MatchString(day.StartTime) {
			return fmt.Errorf("%s start_time has invalid format (expected HH:MM)", dayNames[i])
		}
		if !timeRegex.MatchString(day.EndTime) {
			return fmt.Errorf("%s end_time has invalid format (expected HH:MM)", dayNames[i])
		}
		if day.StartTime >= day.EndTime {
			return fmt.Errorf("%s start_time must be before end_time", dayNames[i])
		}
	}

	return nil
}

// IsWithinBusinessHours verifica se está no horário de atendimento
func (c *Clinic) IsWithinBusinessHours(now time.Time) bool {
	// Obter timezone da clínica
	loc := time.UTC
	if c.Settings.Timezone != "" {
		if tz, err := time.LoadLocation(c.Settings.Timezone); err == nil {
			loc = tz
		}
	}

	localNow := now.In(loc)
	weekday := localNow.Weekday()
	currentTime := localNow.Format("15:04")

	// Se AIBusinessHours não está configurado, usar HORÁRIO PADRÃO
	if c.Settings.AIBusinessHours == nil {
		// Padrão: Segunda a Sábado, 08:00 às 18:00
		if weekday == time.Sunday {
			return false
		}
		return currentTime >= "08:00" && currentTime <= "18:00"
	}

	// Usar configuração customizada
	var schedule *DaySchedule
	switch weekday {
	case time.Monday:
		schedule = c.Settings.AIBusinessHours.Monday
	case time.Tuesday:
		schedule = c.Settings.AIBusinessHours.Tuesday
	case time.Wednesday:
		schedule = c.Settings.AIBusinessHours.Wednesday
	case time.Thursday:
		schedule = c.Settings.AIBusinessHours.Thursday
	case time.Friday:
		schedule = c.Settings.AIBusinessHours.Friday
	case time.Saturday:
		schedule = c.Settings.AIBusinessHours.Saturday
	case time.Sunday:
		schedule = c.Settings.AIBusinessHours.Sunday
	}

	if schedule == nil || !schedule.Enabled {
		return false
	}

	return currentTime >= schedule.StartTime && currentTime <= schedule.EndTime
}

// GetMinDelayBetweenMessages retorna o delay entre mensagens configurado (clínica > env)
func (c *Clinic) GetMinDelayBetweenMessages(defaultDelay int) int {
	if c.Settings.AIMinDelayBetweenMessages != nil && *c.Settings.AIMinDelayBetweenMessages > 0 {
		return *c.Settings.AIMinDelayBetweenMessages
	}
	return defaultDelay
}

// GetMinDelayBetweenPatients retorna o delay entre pacientes configurado (clínica > env)
func (c *Clinic) GetMinDelayBetweenPatients(defaultDelay int) int {
	if c.Settings.AIMinDelayBetweenPatients != nil && *c.Settings.AIMinDelayBetweenPatients > 0 {
		return *c.Settings.AIMinDelayBetweenPatients
	}
	return defaultDelay
}
