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
	GetReportMetrics(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.MonthlyMetrics, error)
	GetWhatsAppMetrics(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) (*model.WhatsAppMetrics, error)
	GetAppointmentsSummary(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) (*model.AppointmentsSummary, error)
	GetFinancialSummary(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) (*model.FinancialSummary, error)
	GetDailyReceivables(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.DailyReceivable, error)
	GetPaymentMethodBreakdown(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.PaymentMethodBreakdown, error)
	GetUnconciliatedAppointments(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.UnconciliatedAppointment, error)
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
func (r *analyticsRepository) GetReportMetrics(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.MonthlyMetrics, error) {
	var metrics []model.MonthlyMetrics

	baseQuery := `
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
	`

	var err error
	if professionalID != nil {
		query := baseQuery + " AND professional_id = ? GROUP BY year, month ORDER BY year DESC, month DESC"
		err = r.db.WithContext(ctx).Raw(query, clinicID, start, end, *professionalID).Scan(&metrics).Error
	} else {
		query := baseQuery + " GROUP BY year, month ORDER BY year DESC, month DESC"
		err = r.db.WithContext(ctx).Raw(query, clinicID, start, end).Scan(&metrics).Error
	}

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Add month names
	for i := range metrics {
		metrics[i].MonthName = model.GetMonthName(metrics[i].Month)
	}

	return metrics, nil
}

// GetWhatsAppMetrics retrieves WhatsApp message statistics
func (r *analyticsRepository) GetWhatsAppMetrics(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) (*model.WhatsAppMetrics, error) {
	var metrics model.WhatsAppMetrics

	baseQuery := `
		SELECT
			COUNT(CASE WHEN event = 'sent' THEN 1 END) as messages_sent,
			COUNT(CASE WHEN event = 'delivered' THEN 1 END) as delivered,
			COUNT(CASE WHEN event = 'replied' THEN 1 END) as replied,
			COUNT(CASE WHEN event = 'failed' THEN 1 END) as failed,
			COUNT(CASE WHEN event = 'confirmation_timeout' THEN 1 END) as confirmation_timeout
		FROM message_logs
		WHERE clinic_id = ?
			AND created_at BETWEEN ? AND ?
	`

	var err error
	if professionalID != nil {
		// Join with appointments to filter by professional
		query := `
			SELECT
				COUNT(CASE WHEN ml.event = 'sent' THEN 1 END) as messages_sent,
				COUNT(CASE WHEN ml.event = 'delivered' THEN 1 END) as delivered,
				COUNT(CASE WHEN ml.event = 'replied' THEN 1 END) as replied,
				COUNT(CASE WHEN ml.event = 'failed' THEN 1 END) as failed,
				COUNT(CASE WHEN ml.event = 'confirmation_timeout' THEN 1 END) as confirmation_timeout
			FROM message_logs ml
			LEFT JOIN appointments a ON ml.appointment_id = a.id
			WHERE ml.clinic_id = ?
				AND ml.created_at BETWEEN ? AND ?
				AND (a.professional_id = ? OR ml.appointment_id IS NULL)
		`
		err = r.db.WithContext(ctx).Raw(query, clinicID, start, end, *professionalID).Scan(&metrics).Error
	} else {
		err = r.db.WithContext(ctx).Raw(baseQuery, clinicID, start, end).Scan(&metrics).Error
	}

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return &metrics, nil
}

// GetAppointmentsSummary retrieves appointments summary by status
func (r *analyticsRepository) GetAppointmentsSummary(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) (*model.AppointmentsSummary, error) {
	var summary model.AppointmentsSummary

	query := r.db.WithContext(ctx).Table("appointments").
		Select(`
			COUNT(*) as total,
			COUNT(CASE WHEN status = 'confirmed' THEN 1 END) as confirmed,
			COUNT(CASE WHEN status = 'canceled' THEN 1 END) as canceled,
			COUNT(CASE WHEN status = 'no_response' THEN 1 END) as no_response,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as no_show,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed
		`).
		Where("clinic_id = ? AND start_time BETWEEN ? AND ? AND deleted_at IS NULL", clinicID, start, end)

	if professionalID != nil {
		query = query.Where("professional_id = ?", *professionalID)
	}

	err := query.Scan(&summary).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return &summary, nil
}

// GetFinancialSummary retrieves aggregated financial metrics for a period
func (r *analyticsRepository) GetFinancialSummary(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) (*model.FinancialSummary, error) {
	var summary model.FinancialSummary

	// Get appointment-level metrics (expected, sessions, paid/pending counts)
	appointmentQuery := `
		SELECT
			COALESCE(SUM(price), 0) as total_expected,
			COUNT(*) as total_sessions,
			COUNT(CASE WHEN payment_status = 'paid' THEN 1 END) as paid_sessions,
			COUNT(CASE WHEN payment_status IN ('pending', 'partial') THEN 1 END) as pending_sessions
		FROM appointments
		WHERE clinic_id = ?
			AND start_time BETWEEN ? AND ?
			AND status = 'completed'
			AND deleted_at IS NULL
	`

	if professionalID != nil {
		appointmentQuery += " AND professional_id = ?"
		err := r.db.WithContext(ctx).Raw(appointmentQuery, clinicID, start, end, *professionalID).Scan(&summary).Error
		if err != nil {
			return nil, errors.ErrInternalInstance.WithError(err)
		}
	} else {
		err := r.db.WithContext(ctx).Raw(appointmentQuery, clinicID, start, end).Scan(&summary).Error
		if err != nil {
			return nil, errors.ErrInternalInstance.WithError(err)
		}
	}

	// Get actual received amounts from appointment_payments + session_plan_payments
	paymentQuery := `
		SELECT
			COALESCE(SUM(total_received), 0) as total_received,
			COALESCE(SUM(total_discount), 0) as total_discount
		FROM (
			SELECT
				COALESCE(SUM(ap.amount), 0) as total_received,
				COALESCE(SUM(ap.discount), 0) as total_discount
			FROM appointment_payments ap
			INNER JOIN appointments a ON ap.appointment_id = a.id
			WHERE a.clinic_id = ?
				AND a.start_time BETWEEN ? AND ?
				AND a.status = 'completed'
				AND a.deleted_at IS NULL
				AND ap.deleted_at IS NULL
			UNION ALL
			SELECT
				COALESCE(SUM(spp.amount), 0) as total_received,
				COALESCE(SUM(spp.discount), 0) as total_discount
			FROM session_plan_payments spp
			INNER JOIN session_plans sp ON spp.session_plan_id = sp.id
			WHERE spp.clinic_id = ?
				AND spp.paid_at BETWEEN ? AND ?
				AND spp.deleted_at IS NULL
				AND sp.deleted_at IS NULL
		) combined
	`

	var paymentData struct {
		TotalReceived float64 `gorm:"column:total_received"`
		TotalDiscount float64 `gorm:"column:total_discount"`
	}

	if professionalID != nil {
		// For professional filter, we need to add the condition to both sub-queries.
		// Rebuild the query with professional filter on both branches.
		paymentQuery = `
			SELECT
				COALESCE(SUM(total_received), 0) as total_received,
				COALESCE(SUM(total_discount), 0) as total_discount
			FROM (
				SELECT
					COALESCE(SUM(ap.amount), 0) as total_received,
					COALESCE(SUM(ap.discount), 0) as total_discount
				FROM appointment_payments ap
				INNER JOIN appointments a ON ap.appointment_id = a.id
				WHERE a.clinic_id = ?
					AND a.start_time BETWEEN ? AND ?
					AND a.status = 'completed'
					AND a.deleted_at IS NULL
					AND ap.deleted_at IS NULL
					AND a.professional_id = ?
				UNION ALL
				SELECT
					COALESCE(SUM(spp.amount), 0) as total_received,
					COALESCE(SUM(spp.discount), 0) as total_discount
				FROM session_plan_payments spp
				INNER JOIN session_plans sp ON spp.session_plan_id = sp.id
				WHERE spp.clinic_id = ?
					AND spp.paid_at BETWEEN ? AND ?
					AND spp.deleted_at IS NULL
					AND sp.deleted_at IS NULL
					AND sp.professional_id = ?
			) combined
		`
		err := r.db.WithContext(ctx).Raw(paymentQuery, clinicID, start, end, *professionalID, clinicID, start, end, *professionalID).Scan(&paymentData).Error
		if err != nil {
			return nil, errors.ErrInternalInstance.WithError(err)
		}
	} else {
		err := r.db.WithContext(ctx).Raw(paymentQuery, clinicID, start, end, clinicID, start, end).Scan(&paymentData).Error
		if err != nil {
			return nil, errors.ErrInternalInstance.WithError(err)
		}
	}

	summary.TotalReceived = paymentData.TotalReceived
	summary.TotalDiscount = paymentData.TotalDiscount
	summary.TotalPending = summary.TotalExpected - summary.TotalReceived - summary.TotalDiscount
	if summary.TotalPending < 0 {
		summary.TotalPending = 0
	}

	if summary.TotalSessions > 0 {
		summary.ConciliationRate = float64(summary.PaidSessions) / float64(summary.TotalSessions) * 100
	}

	return &summary, nil
}

// GetDailyReceivables retrieves financial data grouped by day
// Includes both appointment_payments and session_plan_payments.
func (r *analyticsRepository) GetDailyReceivables(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.DailyReceivable, error) {
	// Combine appointment data (sessions, expected, received) with session_plan_payments received
	// using a CTE-like approach with UNION ALL and then aggregate by date.
	query := `
		SELECT
			date,
			SUM(sessions) as sessions,
			SUM(expected) as expected,
			SUM(received) as received
		FROM (
			SELECT
				TO_CHAR(a.start_time, 'YYYY-MM-DD') as date,
				COUNT(*) as sessions,
				COALESCE(SUM(a.price), 0) as expected,
				COALESCE(SUM(
					(SELECT COALESCE(SUM(ap.amount), 0) FROM appointment_payments ap WHERE ap.appointment_id = a.id AND ap.deleted_at IS NULL)
				), 0) as received
			FROM appointments a
			WHERE a.clinic_id = ?
				AND a.start_time BETWEEN ? AND ?
				AND a.status = 'completed'
				AND a.deleted_at IS NULL
	`

	if professionalID != nil {
		query += " AND a.professional_id = ?"
	}

	query += `
			GROUP BY TO_CHAR(a.start_time, 'YYYY-MM-DD')
			UNION ALL
			SELECT
				TO_CHAR(spp.paid_at, 'YYYY-MM-DD') as date,
				0 as sessions,
				0 as expected,
				COALESCE(SUM(spp.amount), 0) as received
			FROM session_plan_payments spp
			INNER JOIN session_plans sp ON spp.session_plan_id = sp.id
			WHERE spp.clinic_id = ?
				AND spp.paid_at BETWEEN ? AND ?
				AND spp.deleted_at IS NULL
				AND sp.deleted_at IS NULL
	`

	if professionalID != nil {
		query += " AND sp.professional_id = ?"
	}

	query += `
			GROUP BY TO_CHAR(spp.paid_at, 'YYYY-MM-DD')
		) combined
		GROUP BY date
		ORDER BY date DESC
	`

	var results []model.DailyReceivable
	var err error

	if professionalID != nil {
		err = r.db.WithContext(ctx).Raw(query, clinicID, start, end, *professionalID, clinicID, start, end, *professionalID).Scan(&results).Error
	} else {
		err = r.db.WithContext(ctx).Raw(query, clinicID, start, end, clinicID, start, end).Scan(&results).Error
	}

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	for i := range results {
		results[i].Pending = results[i].Expected - results[i].Received
		if results[i].Pending < 0 {
			results[i].Pending = 0
		}
	}
	return results, nil
}

// GetPaymentMethodBreakdown retrieves revenue breakdown by payment method
// Includes both appointment_payments and session_plan_payments.
func (r *analyticsRepository) GetPaymentMethodBreakdown(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.PaymentMethodBreakdown, error) {
	query := `
		SELECT
			method_name,
			method_type,
			SUM(total) as total,
			SUM(count) as count
		FROM (
			SELECT
				pm.name as method_name,
				pm.type as method_type,
				COALESCE(SUM(ap.amount), 0) as total,
				COUNT(*) as count
			FROM appointment_payments ap
			INNER JOIN appointments a ON ap.appointment_id = a.id
			INNER JOIN payment_methods pm ON ap.payment_method_id = pm.id
			WHERE a.clinic_id = ?
				AND a.start_time BETWEEN ? AND ?
				AND a.status = 'completed'
				AND a.deleted_at IS NULL
				AND ap.deleted_at IS NULL
			GROUP BY pm.id, pm.name, pm.type
			UNION ALL
			SELECT
				pm.name as method_name,
				pm.type as method_type,
				COALESCE(SUM(spp.amount), 0) as total,
				COUNT(*) as count
			FROM session_plan_payments spp
			INNER JOIN session_plans sp ON spp.session_plan_id = sp.id
			INNER JOIN payment_methods pm ON spp.payment_method_id = pm.id
			WHERE spp.clinic_id = ?
				AND spp.paid_at BETWEEN ? AND ?
				AND spp.deleted_at IS NULL
				AND sp.deleted_at IS NULL
			GROUP BY pm.id, pm.name, pm.type
		) combined
		GROUP BY method_name, method_type
		ORDER BY total DESC
	`

	var results []model.PaymentMethodBreakdown
	err := r.db.WithContext(ctx).Raw(query, clinicID, start, end, clinicID, start, end).Scan(&results).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Calculate percentages
	var grandTotal float64
	for _, r := range results {
		grandTotal += r.Total
	}
	if grandTotal > 0 {
		for i := range results {
			results[i].Percentage = results[i].Total / grandTotal * 100
		}
	}

	return results, nil
}

// GetUnconciliatedAppointments retrieves completed appointments without full payment
func (r *analyticsRepository) GetUnconciliatedAppointments(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, start, end time.Time) ([]model.UnconciliatedAppointment, error) {
	query := `
		SELECT
			a.id::text as appointment_id,
			TO_CHAR(a.start_time, 'YYYY-MM-DD') as date,
			p.name as patient_name,
			pr.name as professional_name,
			a.price,
			a.payment_status
		FROM appointments a
		INNER JOIN patients p ON a.patient_id = p.id
		INNER JOIN professionals pr ON a.professional_id = pr.id
		WHERE a.clinic_id = ?
			AND a.start_time BETWEEN ? AND ?
			AND a.status = 'completed'
			AND a.payment_status != 'paid'
			AND a.deleted_at IS NULL
	`

	if professionalID != nil {
		query += " AND a.professional_id = ?"
		query += " ORDER BY a.start_time DESC"
		var results []model.UnconciliatedAppointment
		err := r.db.WithContext(ctx).Raw(query, clinicID, start, end, *professionalID).Scan(&results).Error
		if err != nil {
			return nil, errors.ErrInternalInstance.WithError(err)
		}
		return results, nil
	}

	query += " ORDER BY a.start_time DESC"
	var results []model.UnconciliatedAppointment
	err := r.db.WithContext(ctx).Raw(query, clinicID, start, end).Scan(&results).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return results, nil
}
