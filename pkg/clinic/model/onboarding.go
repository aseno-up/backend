package model

// OnboardingStep represents a step in the onboarding process
type OnboardingStep string

const (
	StepClinicInfo     OnboardingStep = "clinic_info"
	StepProfessionals  OnboardingStep = "professionals"
	StepSchedule       OnboardingStep = "schedule"
	StepWhatsApp       OnboardingStep = "whatsapp"
	StepGoogleCalendar OnboardingStep = "google_calendar"
	StepFirstAppt      OnboardingStep = "first_appointment"
	StepComplete       OnboardingStep = "complete"
)

// OnboardingStatus represents the current state of clinic onboarding
type OnboardingStatus struct {
	ClinicInfoComplete    bool   `json:"clinic_info_complete"`
	ProfessionalsCount    int    `json:"professionals_count"`
	HasScheduleConfigured bool   `json:"has_schedule_configured"`
	WhatsAppConnected     bool   `json:"whatsapp_connected"`
	GoogleConnected       bool   `json:"google_connected"`
	AppointmentsCount     int    `json:"appointments_count"`
	Progress              int    `json:"progress"` // 0-100
	NextStep              string `json:"next_step"`
	IsComplete            bool   `json:"is_complete"`
}
