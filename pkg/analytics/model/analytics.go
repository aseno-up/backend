package model

import "github.com/google/uuid"

// TimeSlotStats holds statistics for a specific time slot
type TimeSlotStats struct {
	DayOfWeek      int     `json:"day_of_week"`      // 0=Sunday, 1=Monday...
	HourStart      int     `json:"hour_start"`       // 8, 9, 10...
	TotalScheduled int     `json:"total_scheduled"`
	Completed      int     `json:"completed"`
	NoShows        int     `json:"no_shows"`
	AttendanceRate float64 `json:"attendance_rate"`
}

// SlotSuggestion represents a recommended time slot
type SlotSuggestion struct {
	DayOfWeek      int     `json:"day_of_week"`
	DayName        string  `json:"day_name"`
	HourStart      int     `json:"hour_start"`
	HourEnd        int     `json:"hour_end"`
	AttendanceRate float64 `json:"attendance_rate"`
	TotalScheduled int     `json:"total_scheduled"`
	Rank           int     `json:"rank"`
	Recommendation string  `json:"recommendation"` // "Excelente", "Bom", "Evitar"
}

// AnalyticsResponse represents the complete analytics response
type AnalyticsResponse struct {
	BestSlots      []SlotSuggestion  `json:"best_slots"`
	WorstSlots     []SlotSuggestion  `json:"worst_slots"`
	ByDayOfWeek    []TimeSlotStats   `json:"by_day_of_week"`
	ByHour         []TimeSlotStats   `json:"by_hour"`
	Summary        *AnalyticsSummary `json:"summary"`
	ProfessionalID *uuid.UUID        `json:"professional_id,omitempty"`
}

// AnalyticsSummary provides overall statistics
type AnalyticsSummary struct {
	TotalAppointments   int     `json:"total_appointments"`
	TotalCompleted      int     `json:"total_completed"`
	TotalNoShows        int     `json:"total_no_shows"`
	OverallAttendance   float64 `json:"overall_attendance_rate"`
	BestDay             string  `json:"best_day"`
	WorstDay            string  `json:"worst_day"`
	BestHour            int     `json:"best_hour"`
	WorstHour           int     `json:"worst_hour"`
}

// AnalyticsFilter represents filter parameters for analytics queries
type AnalyticsFilter struct {
	ProfessionalID *uuid.UUID `form:"professional_id"`
	Period         string     `form:"period"` // "3m", "6m", "1y", "all"
}

// DayNames maps day of week numbers to Portuguese names
var DayNames = map[int]string{
	0: "Domingo",
	1: "Segunda-feira",
	2: "Terça-feira",
	3: "Quarta-feira",
	4: "Quinta-feira",
	5: "Sexta-feira",
	6: "Sábado",
}

// GetDayName returns the Portuguese name for a day of week
func GetDayName(dayOfWeek int) string {
	if name, ok := DayNames[dayOfWeek]; ok {
		return name
	}
	return "Desconhecido"
}

// GetRecommendation returns a recommendation based on attendance rate
func GetRecommendation(rate float64) string {
	if rate >= 90 {
		return "Excelente"
	} else if rate >= 80 {
		return "Muito bom"
	} else if rate >= 70 {
		return "Bom"
	} else if rate >= 60 {
		return "Regular"
	}
	return "Evitar"
}

// ReportMetrics represents the "Relatórios que DOEM" metrics
type ReportMetrics struct {
	Period               string           `json:"period"`
	NoShows              int              `json:"no_shows"`
	HoursLost            float64          `json:"hours_lost"`
	CompletedSessions    int              `json:"completed_sessions"`
	EstimatedLostRevenue float64          `json:"estimated_lost_revenue"`
	TotalRevenue         float64          `json:"total_revenue"`
	AttendanceRate       float64          `json:"attendance_rate"`
	MonthlyBreakdown     []MonthlyMetrics `json:"monthly_breakdown"`
}

// MonthlyMetrics represents monthly breakdown of metrics
type MonthlyMetrics struct {
	Year              int     `json:"year"`
	Month             int     `json:"month"`
	MonthName         string  `json:"month_name"`
	NoShows           int     `json:"no_shows"`
	HoursLost         float64 `json:"hours_lost"`
	CompletedSessions int     `json:"completed_sessions"`
	LostRevenue       float64 `json:"lost_revenue"`
}

// ReportFilter represents filter parameters for report queries
type ReportFilter struct {
	Period string `form:"period"` // "1m", "3m", "6m", "1y"
}

// MonthNames maps month numbers to Portuguese names
var MonthNames = map[int]string{
	1:  "Janeiro",
	2:  "Fevereiro",
	3:  "Março",
	4:  "Abril",
	5:  "Maio",
	6:  "Junho",
	7:  "Julho",
	8:  "Agosto",
	9:  "Setembro",
	10: "Outubro",
	11: "Novembro",
	12: "Dezembro",
}

// GetMonthName returns the Portuguese name for a month
func GetMonthName(month int) string {
	if name, ok := MonthNames[month]; ok {
		return name
	}
	return "Desconhecido"
}
