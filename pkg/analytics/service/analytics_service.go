package service

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/analytics/model"
	"github.com/aseno/backend/pkg/analytics/repository"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
)

// AnalyticsService defines the interface for analytics business logic
type AnalyticsService interface {
	GetSlotAnalytics(ctx context.Context, clinicID uuid.UUID, filter *model.AnalyticsFilter) (*model.AnalyticsResponse, error)
	GetBestSlots(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, limit int) ([]model.SlotSuggestion, error)
	GetWorstSlots(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, limit int) ([]model.SlotSuggestion, error)
	GetReportMetrics(ctx context.Context, clinicID uuid.UUID, filter *model.ReportFilter) (*model.ReportMetrics, error)
}

// analyticsService implements AnalyticsService
type analyticsService struct {
	repo       repository.AnalyticsRepository
	clinicRepo clinic_repository.ClinicRepository
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(repo repository.AnalyticsRepository, clinicRepo clinic_repository.ClinicRepository) AnalyticsService {
	return &analyticsService{
		repo:       repo,
		clinicRepo: clinicRepo,
	}
}

// checkPlanRestriction checks if the clinic plan allows access to advanced features
func (s *analyticsService) checkPlanRestriction(ctx context.Context, clinicID uuid.UUID) error {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return err
	}

	if clinic.Account != nil && clinic.Account.Plan != nil && !clinic.Account.Plan.HasApiAccess {
		return errors.ErrUpgradeRequiredInstance.WithMessage("Relatórios avançados disponíveis apenas nos planos Profissional e Avançado")
	}

	return nil
}

// GetSlotAnalytics returns complete slot analytics
func (s *analyticsService) GetSlotAnalytics(ctx context.Context, clinicID uuid.UUID, filter *model.AnalyticsFilter) (*model.AnalyticsResponse, error) {
	if err := s.checkPlanRestriction(ctx, clinicID); err != nil {
		return nil, err
	}

	start, end := s.getPeriodRange(filter.Period)

	// Get stats by day of week
	byDay, err := s.repo.GetStatsByDayOfWeek(ctx, clinicID, filter.ProfessionalID, start, end)
	if err != nil {
		return nil, err
	}

	// Get stats by hour
	byHour, err := s.repo.GetStatsByHour(ctx, clinicID, filter.ProfessionalID, start, end)
	if err != nil {
		return nil, err
	}

	// Get overall summary
	summary, err := s.repo.GetOverallStats(ctx, clinicID, filter.ProfessionalID, start, end)
	if err != nil {
		return nil, err
	}

	// Find best and worst days
	s.findBestWorstDay(byDay, summary)
	s.findBestWorstHour(byHour, summary)

	// Get best and worst slots (combined day+hour)
	bestSlots, err := s.GetBestSlots(ctx, clinicID, filter.ProfessionalID, 5)
	if err != nil {
		return nil, err
	}

	worstSlots, err := s.GetWorstSlots(ctx, clinicID, filter.ProfessionalID, 5)
	if err != nil {
		return nil, err
	}

	return &model.AnalyticsResponse{
		BestSlots:      bestSlots,
		WorstSlots:     worstSlots,
		ByDayOfWeek:    byDay,
		ByHour:         byHour,
		Summary:        summary,
		ProfessionalID: filter.ProfessionalID,
	}, nil
}

// GetBestSlots returns the top N best performing time slots
func (s *analyticsService) GetBestSlots(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, limit int) ([]model.SlotSuggestion, error) {
	if err := s.checkPlanRestriction(ctx, clinicID); err != nil {
		return nil, err
	}

	start, end := s.getPeriodRange("6m")

	stats, err := s.repo.GetStatsByDayAndHour(ctx, clinicID, professionalID, start, end)
	if err != nil {
		return nil, err
	}

	// Filter slots with enough data (at least 3 appointments)
	var validStats []model.TimeSlotStats
	for _, stat := range stats {
		if stat.TotalScheduled >= 3 {
			validStats = append(validStats, stat)
		}
	}

	// Sort by attendance rate (descending)
	sort.Slice(validStats, func(i, j int) bool {
		return validStats[i].AttendanceRate > validStats[j].AttendanceRate
	})

	// Take top N
	if len(validStats) > limit {
		validStats = validStats[:limit]
	}

	return s.statsToSuggestions(validStats, true), nil
}

// GetWorstSlots returns the top N worst performing time slots
func (s *analyticsService) GetWorstSlots(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, limit int) ([]model.SlotSuggestion, error) {
	if err := s.checkPlanRestriction(ctx, clinicID); err != nil {
		return nil, err
	}

	start, end := s.getPeriodRange("6m")

	stats, err := s.repo.GetStatsByDayAndHour(ctx, clinicID, professionalID, start, end)
	if err != nil {
		return nil, err
	}

	// Filter slots with enough data (at least 3 appointments)
	var validStats []model.TimeSlotStats
	for _, stat := range stats {
		if stat.TotalScheduled >= 3 {
			validStats = append(validStats, stat)
		}
	}

	// Sort by attendance rate (ascending - worst first)
	sort.Slice(validStats, func(i, j int) bool {
		return validStats[i].AttendanceRate < validStats[j].AttendanceRate
	})

	// Take top N worst
	if len(validStats) > limit {
		validStats = validStats[:limit]
	}

	return s.statsToSuggestions(validStats, false), nil
}

// statsToSuggestions converts TimeSlotStats to SlotSuggestion
func (s *analyticsService) statsToSuggestions(stats []model.TimeSlotStats, isBest bool) []model.SlotSuggestion {
	suggestions := make([]model.SlotSuggestion, len(stats))

	for i, stat := range stats {
		suggestions[i] = model.SlotSuggestion{
			DayOfWeek:      stat.DayOfWeek,
			DayName:        model.GetDayName(stat.DayOfWeek),
			HourStart:      stat.HourStart,
			HourEnd:        stat.HourStart + 1,
			AttendanceRate: stat.AttendanceRate,
			TotalScheduled: stat.TotalScheduled,
			Rank:           i + 1,
			Recommendation: model.GetRecommendation(stat.AttendanceRate),
		}
	}

	return suggestions
}

// getPeriodRange returns the start and end time for a given period
func (s *analyticsService) getPeriodRange(period string) (time.Time, time.Time) {
	now := time.Now()
	end := now

	var start time.Time
	switch period {
	case "3m":
		start = now.AddDate(0, -3, 0)
	case "6m":
		start = now.AddDate(0, -6, 0)
	case "1y":
		start = now.AddDate(-1, 0, 0)
	case "all":
		start = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	default:
		start = now.AddDate(0, -6, 0) // Default to 6 months
	}

	return start, end
}

// findBestWorstDay finds and sets the best and worst day in the summary
func (s *analyticsService) findBestWorstDay(stats []model.TimeSlotStats, summary *model.AnalyticsSummary) {
	if len(stats) == 0 {
		return
	}

	bestRate := -1.0
	worstRate := 101.0
	bestDay := 0
	worstDay := 0

	for _, stat := range stats {
		if stat.TotalScheduled >= 3 { // Minimum sample size
			if stat.AttendanceRate > bestRate {
				bestRate = stat.AttendanceRate
				bestDay = stat.DayOfWeek
			}
			if stat.AttendanceRate < worstRate {
				worstRate = stat.AttendanceRate
				worstDay = stat.DayOfWeek
			}
		}
	}

	if bestRate >= 0 {
		summary.BestDay = model.GetDayName(bestDay)
	}
	if worstRate <= 100 {
		summary.WorstDay = model.GetDayName(worstDay)
	}
}

// findBestWorstHour finds and sets the best and worst hour in the summary
func (s *analyticsService) findBestWorstHour(stats []model.TimeSlotStats, summary *model.AnalyticsSummary) {
	if len(stats) == 0 {
		return
	}

	bestRate := -1.0
	worstRate := 101.0
	bestHour := 0
	worstHour := 0

	for _, stat := range stats {
		if stat.TotalScheduled >= 3 { // Minimum sample size
			if stat.AttendanceRate > bestRate {
				bestRate = stat.AttendanceRate
				bestHour = stat.HourStart
			}
			if stat.AttendanceRate < worstRate {
				worstRate = stat.AttendanceRate
				worstHour = stat.HourStart
			}
		}
	}

	if bestRate >= 0 {
		summary.BestHour = bestHour
	}
	if worstRate <= 100 {
		summary.WorstHour = worstHour
	}
}

// GetReportMetrics returns the "Relatórios que DOEM" metrics
func (s *analyticsService) GetReportMetrics(ctx context.Context, clinicID uuid.UUID, filter *model.ReportFilter) (*model.ReportMetrics, error) {
	if err := s.checkPlanRestriction(ctx, clinicID); err != nil {
		return nil, err
	}

	start, end := s.getReportPeriodRange(filter.Period)

	// Get monthly breakdown from repository
	monthlyMetrics, err := s.repo.GetReportMetrics(ctx, clinicID, filter.ProfessionalID, start, end)
	if err != nil {
		return nil, err
	}

	// Get WhatsApp metrics
	whatsAppMetrics, err := s.repo.GetWhatsAppMetrics(ctx, clinicID, filter.ProfessionalID, start, end)
	if err != nil {
		return nil, err
	}

	// Get appointments summary
	appointmentsSummary, err := s.repo.GetAppointmentsSummary(ctx, clinicID, filter.ProfessionalID, start, end)
	if err != nil {
		return nil, err
	}

	// Calculate totals from monthly data
	var totalNoShows int
	var totalHoursLost float64
	var totalCompletedSessions int
	var totalLostRevenue float64

	for _, m := range monthlyMetrics {
		totalNoShows += m.NoShows
		totalHoursLost += m.HoursLost
		totalCompletedSessions += m.CompletedSessions
		totalLostRevenue += m.LostRevenue
	}

	// Calculate attendance rate
	totalAppointments := totalNoShows + totalCompletedSessions
	var attendanceRate float64
	if totalAppointments > 0 {
		attendanceRate = float64(totalCompletedSessions) / float64(totalAppointments) * 100
	}

	return &model.ReportMetrics{
		Period:               filter.Period,
		NoShows:              totalNoShows,
		HoursLost:            totalHoursLost,
		CompletedSessions:    totalCompletedSessions,
		EstimatedLostRevenue: totalLostRevenue,
		AttendanceRate:       attendanceRate,
		MonthlyBreakdown:     monthlyMetrics,
		WhatsAppMetrics:      whatsAppMetrics,
		AppointmentsSummary:  appointmentsSummary,
	}, nil
}

// getReportPeriodRange returns the start and end time for report period
func (s *analyticsService) getReportPeriodRange(period string) (time.Time, time.Time) {
	now := time.Now()
	end := now

	var start time.Time
	switch period {
	case "1m":
		start = now.AddDate(0, -1, 0)
	case "3m":
		start = now.AddDate(0, -3, 0)
	case "6m":
		start = now.AddDate(0, -6, 0)
	case "1y":
		start = now.AddDate(-1, 0, 0)
	default:
		start = now.AddDate(0, -3, 0) // Default to 3 months
	}

	return start, end
}
