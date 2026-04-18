package model

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"

	account_model "github.com/aseno/backend/pkg/account/model"
)

// ClinicType represents the type of clinic account
type ClinicType string

const (
	ClinicTypeStandard ClinicType = "standard"
	ClinicTypeDev      ClinicType = "dev"
)

// Clinic represents a clinic/tenant in the system
type Clinic struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name         string     `json:"name" gorm:"not null"`
	Slug         string     `json:"slug" gorm:"uniqueIndex;not null"`
	Type         ClinicType `json:"type" gorm:"default:'standard'"`
	Email        string     `json:"email" gorm:"uniqueIndex;not null"`
	Phone        string     `json:"phone"`
	Document     string     `json:"document"` // CNPJ or CPF
	Address      string     `json:"address"`
	City         string     `json:"city"`
	State        string     `json:"state"`
	ZipCode      string     `json:"zip_code"`
	Instagram    string     `json:"instagram"`
	BusinessInfo string     `json:"business_info" gorm:"type:text"`
	FAQ          string     `json:"faq" gorm:"type:text"`
	LogoURL      string     `json:"logo_url"`
	PrimaryColor string     `json:"primary_color" gorm:"default:'#7C3AED'"`
	AccountID    uuid.UUID  `json:"account_id" gorm:"type:uuid;index"`
	// Computed fields — populated by the repository via JOIN; NOT persisted to clinics table
	Account           *account_model.Account `json:"account,omitempty" gorm:"foreignKey:AccountID"`
	WhatsAppConnected bool                   `json:"whatsapp_connected" gorm:"-"`
	GoogleConnected   bool                   `json:"google_connected" gorm:"-"`
	Settings          ClinicSettings         `json:"settings" gorm:"type:jsonb;serializer:json"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	DeletedAt         *time.Time             `json:"deleted_at,omitempty" gorm:"index"`
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

	// Payment settings
	RequirePaymentBeforeBooking bool `json:"require_payment_before_booking"`

	// Public booking page settings
	AboutUs string `json:"about_us,omitempty"`

	// AI/WhatsApp settings
	AIAgentEnabled            bool                 `json:"ai_agent_enabled"`
	AIMinDelayBetweenMessages *int                 `json:"ai_min_delay_between_messages,omitempty"`
	AIMinDelayBetweenPatients *int                 `json:"ai_min_delay_between_patients,omitempty"`
	AIBusinessHours           *BusinessHoursConfig `json:"ai_business_hours,omitempty"`

	// Per-clinic AI tool toggles. nil means "use default (enabled)" so that
	// existing clinics keep the original behavior after the field was introduced.
	AIToolSchedule   *bool `json:"ai_tool_schedule,omitempty"`
	AIToolConfirm    *bool `json:"ai_tool_confirm,omitempty"`
	AIToolCancel     *bool `json:"ai_tool_cancel,omitempty"`
	AIToolReschedule *bool `json:"ai_tool_reschedule,omitempty"`

	// Canned reply sent when the AI detects an intent the clinic disabled.
	// Empty falls back to a built-in default at the agent layer.
	AIBlockedIntentReply string `json:"ai_blocked_intent_reply,omitempty"`

	// Hours to pause the AI after a human from the clinic replies directly
	// via the WhatsApp app. Nil uses the default (4h). <=0 disables the
	// auto-pause and keeps the AI always on.
	AIHumanTakeoverPauseHours *int `json:"ai_human_takeover_pause_hours,omitempty"`
}

// DefaultHumanTakeoverPauseHours is used when the clinic has no explicit value.
const DefaultHumanTakeoverPauseHours = 4

// GetHumanTakeoverPauseDuration returns how long the AI stays paused after a
// human from the clinic replies directly. Returns 0 when the feature is
// explicitly disabled (value <= 0).
func (s *ClinicSettings) GetHumanTakeoverPauseDuration() time.Duration {
	if s == nil || s.AIHumanTakeoverPauseHours == nil {
		return time.Duration(DefaultHumanTakeoverPauseHours) * time.Hour
	}
	if *s.AIHumanTakeoverPauseHours <= 0 {
		return 0
	}
	return time.Duration(*s.AIHumanTakeoverPauseHours) * time.Hour
}

// IsAIToolScheduleEnabled returns whether AI is allowed to schedule new
// appointments for this clinic. Nil pointer defaults to true to keep existing
// clinics behaving as before the flag was introduced.
func (s *ClinicSettings) IsAIToolScheduleEnabled() bool {
	if s == nil || s.AIToolSchedule == nil {
		return true
	}
	return *s.AIToolSchedule
}

// IsAIToolConfirmEnabled returns whether AI is allowed to handle confirmations.
func (s *ClinicSettings) IsAIToolConfirmEnabled() bool {
	if s == nil || s.AIToolConfirm == nil {
		return true
	}
	return *s.AIToolConfirm
}

// IsAIToolCancelEnabled returns whether AI is allowed to cancel appointments.
func (s *ClinicSettings) IsAIToolCancelEnabled() bool {
	if s == nil || s.AIToolCancel == nil {
		return true
	}
	return *s.AIToolCancel
}

// IsAIToolRescheduleEnabled returns whether AI is allowed to reschedule
// appointments. Independent from Cancel (older code coupled them).
func (s *ClinicSettings) IsAIToolRescheduleEnabled() bool {
	if s == nil || s.AIToolReschedule == nil {
		return true
	}
	return *s.AIToolReschedule
}

// TableName returns the table name for GORM
func (Clinic) TableName() string {
	return "clinics"
}

// ClinicResponse represents the API response for a clinic
type ClinicResponse struct {
	ID                uuid.UUID                      `json:"id"`
	Name              string                         `json:"name"`
	Slug              string                         `json:"slug"`
	Type              ClinicType                     `json:"type"`
	Email             string                         `json:"email"`
	Phone             string                         `json:"phone"`
	Document          string                         `json:"document"`
	Address           string                         `json:"address"`
	City              string                         `json:"city"`
	State             string                         `json:"state"`
	ZipCode           string                         `json:"zip_code"`
	Instagram         string                         `json:"instagram"`
	BusinessInfo      string                         `json:"business_info"`
	FAQ               string                         `json:"faq"`
	LogoURL           string                         `json:"logo_url"`
	PrimaryColor      string                         `json:"primary_color"`
	AccountID         uuid.UUID                      `json:"account_id"`
	Account           *account_model.AccountResponse `json:"account,omitempty"`
	WhatsAppConnected bool                           `json:"whatsapp_connected"`
	GoogleConnected   bool                           `json:"google_connected"`
	Settings          ClinicSettings                 `json:"settings"`
	BookingURL        string                         `json:"booking_url,omitempty"`
	// Subscription fields (flattened from Account for frontend convenience)
	Plan               account_model.PlanType           `json:"plan"`
	SubscriptionStatus account_model.SubscriptionStatus `json:"subscription_status"`
	TrialEndsAt        *time.Time                       `json:"trial_ends_at,omitempty"`
	CreatedAt          time.Time                        `json:"created_at"`
}

// ToResponse converts Clinic to ClinicResponse
func (c *Clinic) ToResponse() *ClinicResponse {
	resp := &ClinicResponse{
		ID:                c.ID,
		Name:              c.Name,
		Slug:              c.Slug,
		Type:              c.Type,
		Email:             c.Email,
		Phone:             c.Phone,
		Document:          c.Document,
		Address:           c.Address,
		City:              c.City,
		State:             c.State,
		ZipCode:           c.ZipCode,
		Instagram:         c.Instagram,
		BusinessInfo:      c.BusinessInfo,
		FAQ:               c.FAQ,
		LogoURL:           c.LogoURL,
		PrimaryColor:      c.PrimaryColor,
		AccountID:         c.AccountID,
		WhatsAppConnected: c.WhatsAppConnected,
		GoogleConnected:   c.GoogleConnected,
		Settings:          c.Settings,
		CreatedAt:         c.CreatedAt,
	}
	if c.Account != nil {
		resp.Account = c.Account.ToResponse()
	}
	return resp
}

// IsSubscriptionActive checks if the clinic has an active subscription via Account
func (c *Clinic) IsSubscriptionActive() bool {
	if c.Account != nil {
		return c.Account.IsSubscriptionActive()
	}
	return false
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

	// Payment settings
	RequirePaymentBeforeBooking *bool `json:"require_payment_before_booking,omitempty"`

	// Public booking page settings
	AboutUs *string `json:"about_us,omitempty"`

	// AI/WhatsApp settings
	AIAgentEnabled            *bool                `json:"ai_agent_enabled,omitempty"`
	AIMinDelayBetweenMessages *int                 `json:"ai_min_delay_between_messages,omitempty"`
	AIMinDelayBetweenPatients *int                 `json:"ai_min_delay_between_patients,omitempty"`
	AIBusinessHours           *BusinessHoursConfig `json:"ai_business_hours,omitempty"`

	AIToolSchedule            *bool   `json:"ai_tool_schedule,omitempty"`
	AIToolConfirm             *bool   `json:"ai_tool_confirm,omitempty"`
	AIToolCancel              *bool   `json:"ai_tool_cancel,omitempty"`
	AIToolReschedule          *bool   `json:"ai_tool_reschedule,omitempty"`
	AIBlockedIntentReply      *string `json:"ai_blocked_intent_reply,omitempty"`
	AIHumanTakeoverPauseHours *int    `json:"ai_human_takeover_pause_hours,omitempty"`
}

// UpdateClinicRequest represents the request to update a clinic
type UpdateClinicRequest struct {
	Name         string                       `json:"name" binding:"omitempty,min=2,max=100"`
	Phone        string                       `json:"phone"`
	Document     *string                      `json:"document,omitempty"`
	Address      string                       `json:"address"`
	City         string                       `json:"city"`
	State        string                       `json:"state"`
	ZipCode      string                       `json:"zip_code"`
	Instagram    *string                      `json:"instagram,omitempty"`
	BusinessInfo *string                      `json:"business_info,omitempty"`
	FAQ          *string                      `json:"faq,omitempty"`
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

// getDaySchedule retorna o DaySchedule para um dia da semana
func (c *Clinic) getDaySchedule(weekday time.Weekday) *DaySchedule {
	if c.Settings.AIBusinessHours == nil {
		// Horário padrão: Segunda a Sábado 08:00-18:00
		if weekday == time.Sunday {
			return nil
		}
		return &DaySchedule{
			Enabled:   true,
			StartTime: "08:00",
			EndTime:   "18:00",
		}
	}

	switch weekday {
	case time.Monday:
		return c.Settings.AIBusinessHours.Monday
	case time.Tuesday:
		return c.Settings.AIBusinessHours.Tuesday
	case time.Wednesday:
		return c.Settings.AIBusinessHours.Wednesday
	case time.Thursday:
		return c.Settings.AIBusinessHours.Thursday
	case time.Friday:
		return c.Settings.AIBusinessHours.Friday
	case time.Saturday:
		return c.Settings.AIBusinessHours.Saturday
	case time.Sunday:
		return c.Settings.AIBusinessHours.Sunday
	}
	return nil
}

// CalculateNextBusinessHour calcula o próximo horário de atendimento
func (c *Clinic) CalculateNextBusinessHour(now time.Time) time.Time {
	// Obter timezone da clínica
	loc := time.UTC
	if c.Settings.Timezone != "" {
		if tz, err := time.LoadLocation(c.Settings.Timezone); err == nil {
			loc = tz
		}
	}

	current := now.In(loc)

	// Se já está dentro do horário de atendimento, retorna agora
	if c.IsWithinBusinessHours(current) {
		return current
	}

	// Verificar se ainda está no mesmo dia mas antes do horário de abertura
	schedule := c.getDaySchedule(current.Weekday())
	if schedule != nil && schedule.Enabled {
		openTime := parseTimeToday(current, schedule.StartTime)
		if current.Before(openTime) {
			return openTime
		}
	}

	// Procurar próximo dia de atendimento (até 7 dias no futuro)
	for i := 1; i <= 7; i++ {
		nextDay := current.Add(time.Duration(i) * 24 * time.Hour)
		nextSchedule := c.getDaySchedule(nextDay.Weekday())

		if nextSchedule != nil && nextSchedule.Enabled {
			return parseTimeToday(nextDay, nextSchedule.StartTime)
		}
	}

	// Fallback: próxima segunda 08:00 (se nenhum dia está habilitado)
	daysUntilMonday := (7 - int(current.Weekday()) + int(time.Monday)) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	nextMonday := current.Add(time.Duration(daysUntilMonday) * 24 * time.Hour)
	return parseTimeToday(nextMonday, "08:00")
}

// parseTimeToday parseia um horário "HH:MM" e retorna time.Time para a data fornecida
func parseTimeToday(date time.Time, timeStr string) time.Time {
	var hour, minute int
	fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location())
}
