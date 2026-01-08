package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/analytics/model"
)

// AnalyticsRepository defines the interface for analytics data operations
type AnalyticsRepository interface {
	GetStatsByDayOfWeek(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.TimeSlotStats, error)
	GetStatsByHour(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.TimeSlotStats, error)
	GetStatsByDayAndHour(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.TimeSlotStats, error)
	GetOverallStats(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) (*model.AnalyticsSummary, error)
	GetReportMetrics(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.MonthlyMetrics, error)
}

// analyticsRepository implements AnalyticsRepository
type analyticsRepository struct {
	db *gorm.DB
}

// NewAnalyticsRepository creates a new analytics repository
func NewAnalyticsRepository(db *gorm.DB) AnalyticsRepository {
	return &analyticsRepository{db: db}
}

// GetStatsByDayOfWeek retrieves appointment statistics grouped by day of week
func (r *analyticsRepository) GetStatsByDayOfWeek(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.TimeSlotStats, error) {
	var stats []model.TimeSlotStats

	query := r.db.WithContext(ctx).Table("appointments").
		Select(`
			EXTRACT(DOW FROM start_time)::int as day_of_week,
			0 as hour_start,
			COUNT(*) as total_scheduled,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as no_shows
		`).
		Where("clinic_id = ? AND start_time BETWEEN ? AND ? AND deleted_at IS NULL", clinicID, start, end).
		Where("status IN ('completed', 'no_show')") // Only count finalized appointments

	if professionalID != nil {
		query = query.Where("professional_id = ?", *professionalID)
	}

	err := query.
		Group("EXTRACT(DOW FROM start_time)").
		Order("day_of_week").
		Scan(&stats).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Calculate attendance rate for each slot
	for i := range stats {
		total := stats[i].Completed + stats[i].NoShows
		if total > 0 {
			stats[i].AttendanceRate = float64(stats[i].Completed) / float64(total) * 100
		}
	}

	return stats, nil
}

// GetStatsByHour retrieves appointment statistics grouped by hour
func (r *analyticsRepository) GetStatsByHour(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.TimeSlotStats, error) {
	var stats []model.TimeSlotStats

	query := r.db.WithContext(ctx).Table("appointments").
		Select(`
			0 as day_of_week,
			EXTRACT(HOUR FROM start_time)::int as hour_start,
			COUNT(*) as total_scheduled,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as no_shows
		`).
		Where("clinic_id = ? AND start_time BETWEEN ? AND ? AND deleted_at IS NULL", clinicID, start, end).
		Where("status IN ('completed', 'no_show')")

	if professionalID != nil {
		query = query.Where("professional_id = ?", *professionalID)
	}

	err := query.
		Group("EXTRACT(HOUR FROM start_time)").
		Order("hour_start").
		Scan(&stats).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Calculate attendance rate for each slot
	for i := range stats {
		total := stats[i].Completed + stats[i].NoShows
		if total > 0 {
			stats[i].AttendanceRate = float64(stats[i].Completed) / float64(total) * 100
		}
	}

	return stats, nil
}

// GetStatsByDayAndHour retrieves appointment statistics grouped by day of week and hour
func (r *analyticsRepository) GetStatsByDayAndHour(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.TimeSlotStats, error) {
	var stats []model.TimeSlotStats

	query := r.db.WithContext(ctx).Table("appointments").
		Select(`
			EXTRACT(DOW FROM start_time)::int as day_of_week,
			EXTRACT(HOUR FROM start_time)::int as hour_start,
			COUNT(*) as total_scheduled,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as no_shows
		`).
		Where("clinic_id = ? AND start_time BETWEEN ? AND ? AND deleted_at IS NULL", clinicID, start, end).
		Where("status IN ('completed', 'no_show')")

	if professionalID != nil {
		query = query.Where("professional_id = ?", *professionalID)
	}

	err := query.
		Group("EXTRACT(DOW FROM start_time), EXTRACT(HOUR FROM start_time)").
		Order("day_of_week, hour_start").
		Scan(&stats).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Calculate attendance rate for each slot
	for i := range stats {
		total := stats[i].Completed + stats[i].NoShows
		if total > 0 {
			stats[i].AttendanceRate = float64(stats[i].Completed) / float64(total) * 100
		}
	}

	return stats, nil
}

// GetOverallStats retrieves overall appointment statistics
func (r *analyticsRepository) GetOverallStats(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) (*model.AnalyticsSummary, error) {
	var summary model.AnalyticsSummary

	query := r.db.WithContext(ctx).Table("appointments").
		Select(`
			COUNT(*) as total_appointments,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as total_completed,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as total_no_shows
		`).
		Where("clinic_id = ? AND start_time BETWEEN ? AND ? AND deleted_at IS NULL", clinicID, start, end).
		Where("status IN ('completed', 'no_show')")

	if professionalID != nil {
		query = query.Where("professional_id = ?", *professionalID)
	}

	err := query.Scan(&summary).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Calculate overall attendance rate
	total := summary.TotalCompleted + summary.TotalNoShows
	if total > 0 {
		summary.OverallAttendance = float64(summary.TotalCompleted) / float64(total) * 100
	}

	return &summary, nil
}

// GetReportMetrics retrieves monthly metrics for "Relatórios que DOEM"
func (r *analyticsRepository) GetReportMetrics(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.MonthlyMetrics, error) {
	var metrics []model.MonthlyMetrics

	query := `
		SELECT
			EXTRACT(YEAR FROM start_time)::int as year,
			EXTRACT(MONTH FROM start_time)::int as month,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as no_shows,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_sessions,
			COALESCE(SUM(CASE WHEN status = 'no_show' THEN EXTRACT(EPOCH FROM (end_time - start_time))/3600 ELSE 0 END), 0) as hours_lost,
			COALESCE(SUM(CASE WHEN status = 'no_show' THEN price ELSE 0 END), 0) as lost_revenue
		FROM appointments
		WHERE clinic_id = ?
			AND start_time BETWEEN ? AND ?
			AND status IN ('no_show', 'completed')
			AND deleted_at IS NULL
		GROUP BY year, month
		ORDER BY year DESC, month DESC
	`

	err := r.db.WithContext(ctx).Raw(query, clinicID, start, end).Scan(&metrics).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Add month names
	for i := range metrics {
		metrics[i].MonthName = model.GetMonthName(metrics[i].Month)
	}

	return metrics, nil
}
