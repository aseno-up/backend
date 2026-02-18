package model

// DashboardMetrics represents the aggregated dashboard data
type DashboardMetrics struct {
	// Operational & Financial
	EstimatedRevenue       float64 `json:"estimated_revenue"`
	CompletedConsultations int     `json:"completed_consultations"`
	ConfirmedConsultations int     `json:"confirmed_consultations"`
	NoShows                int     `json:"no_shows"`
	AttendanceRate         float64 `json:"attendance_rate"`
	PreviousAttendanceRate float64 `json:"previous_attendance_rate"` // Comparison
	EstimatedLostRevenue   float64 `json:"estimated_lost_revenue"`

	// Agenda & Occupancy
	AgendaOccupancy  float64 `json:"agenda_occupancy"`
	OpenSlots        int     `json:"open_slots"`
	FutureSessions   int     `json:"future_sessions"`
	PotentialRevenue float64 `json:"potential_revenue"`

	// Automation Impact
	AvoidedNoShows          int     `json:"avoided_no_shows"`
	AutoConfirmed           int     `json:"auto_confirmed"`
	PreservedRevenue        float64 `json:"preserved_revenue"`
	AnnualSavingsProjection float64 `json:"annual_savings_projection"` // Projected annual savings based on current rate

	// Insights
	TopAbsentees  []PatientAbsentee `json:"top_absentees"`
	Insights      []string          `json:"insights"`
	SmartInsights []SmartInsight    `json:"smart_insights"`
}

type PatientAbsentee struct {
	Name        string `json:"name"`
	AbsentCount int    `json:"absent_count"`
}

// TimePatternStats represents no-show statistics by time period
type TimePatternStats struct {
	Period      string  `json:"period"` // "morning", "afternoon", "evening" or day name
	NoShowCount int     `json:"no_show_count"`
	TotalCount  int     `json:"total_count"`
	NoShowRate  float64 `json:"no_show_rate"` // percentage
}

// PatientTimePattern represents a patient's no-show pattern by time
type PatientTimePattern struct {
	PatientID   string  `json:"patient_id"`
	PatientName string  `json:"patient_name"`
	Period      string  `json:"period"` // "morning", "afternoon", "evening"
	NoShowCount int     `json:"no_show_count"`
	TotalCount  int     `json:"total_count"`
	NoShowRate  float64 `json:"no_show_rate"`
}

// ProfessionalDayPattern represents a professional's no-show pattern by day of week
type ProfessionalDayPattern struct {
	ProfessionalID   string  `json:"professional_id"`
	ProfessionalName string  `json:"professional_name"`
	DayOfWeek        string  `json:"day_of_week"`
	NoShowCount      int     `json:"no_show_count"`
	TotalCount       int     `json:"total_count"`
	NoShowRate       float64 `json:"no_show_rate"`
}

// SmartInsight represents an AI-generated insight with context
type SmartInsight struct {
	Type        string `json:"type"`        // "patient_time", "professional_day", "best_time", "worst_time"
	Title       string `json:"title"`       // Short title
	Description string `json:"description"` // Detailed description
	Severity    string `json:"severity"`    // "info", "warning", "critical"
	ActionHint  string `json:"action_hint"` // Suggested action
}
