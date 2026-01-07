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
	AvoidedNoShows   int     `json:"avoided_no_shows"`
	AutoConfirmed    int     `json:"auto_confirmed"`
	PreservedRevenue float64 `json:"preserved_revenue"`

	// Insights
	TopAbsentees []PatientAbsentee `json:"top_absentees"`
	Insights     []string          `json:"insights"`
}

type PatientAbsentee struct {
	Name        string `json:"name"`
	AbsentCount int    `json:"absent_count"`
}
