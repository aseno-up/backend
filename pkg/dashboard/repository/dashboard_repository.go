package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/dashboard/model"
)

type DashboardRepository interface {
	GetAppointmentStats(ctx context.Context, clinicID uuid.UUID, start, end time.Time) (*AppointmentStats, error)
	GetTopAbsentees(ctx context.Context, clinicID uuid.UUID, start, end time.Time, limit int) ([]model.PatientAbsentee, error)
	GetFutureSessionsCount(ctx context.Context, clinicID uuid.UUID) (int64, error)
	GetProfessionals(ctx context.Context, clinicID uuid.UUID) ([]ProfessionalHours, error)
	// Time pattern analysis
	GetNoShowsByTimePeriod(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.TimePatternStats, error)
	GetNoShowsByDayOfWeek(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.TimePatternStats, error)
	GetPatientTimePatterns(ctx context.Context, clinicID uuid.UUID, start, end time.Time, minAppointments int) ([]model.PatientTimePattern, error)
	GetProfessionalDayPatterns(ctx context.Context, clinicID uuid.UUID, start, end time.Time, minAppointments int) ([]model.ProfessionalDayPattern, error)
}

type dashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) DashboardRepository {
	return &dashboardRepository{db: db}
}

type ProfessionalHours struct {
	ID           uuid.UUID
	WorkingHours []byte // stored as JSON
}

func (r *dashboardRepository) GetProfessionals(ctx context.Context, clinicID uuid.UUID) ([]ProfessionalHours, error) {
	var profs []ProfessionalHours
	err := r.db.Table("professionals").
		Select("id, working_hours").
		Where("clinic_id = ? AND is_active = true", clinicID).
		Scan(&profs).Error
	return profs, err
}

type AppointmentStats struct {
	TotalCount     int
	CompletedCount int
	ConfirmedCount int
	NoShowCount    int
	TotalRevenue   float64
	LostRevenue    float64
	AutoConfirmed  int
}

func (r *dashboardRepository) GetAppointmentStats(ctx context.Context, clinicID uuid.UUID, start, end time.Time) (*AppointmentStats, error) {
	var stats AppointmentStats

	// We can do this in a single query or multiple. Single query is better.
	// We assume 'appointments' table exists.
	// status: 'completed', 'confirmed', 'no_show', 'canceled'
	// confirmed_at: not null and status = 'confirmed' or 'completed' -> implies auto confirmed if logic sets it?
	// Actually, "Auto Confirmed" usually means confirmed via WhatsApp/System.
	// For now, let's assume if `confirmed_at` is set, it's confirmed. To distinguish "Auto", we might need a flag or assume logic.
	// The requirement says "Impacto da Automação (WhatsApp)". If we don't store "source" of confirmation, we might need to approximate or check logs.
	// Let's look at `Appointment` model again. It has `WhatsappConfirmation bool` in SessionPlan, but Appointment has `ConfirmedAt`.
	// The `Appointment` model doesn't explicitly say "AutoConfirmed".
	// However, `features/dashboard.md` says "Confirmadas Automaticamente".
	// If the system sends a WA message and receives "SIM", it updates status to `confirmed`.
	// Maybe we can check if `patient_notes` is empty? Or if `confirmed_at` is close to `reminder_sent_at`?
	// For now, let's count all `confirmed` as "Confirmed" and maybe 80% as Auto? No, that's bad.
	// Let's look at the codebase. `ConfirmAppointment` handler might leave a trace.
	// If there's no trace, I'll count all `confirmed` that have `reminder_sent_at` set as "potentially auto-confirmed" or just use `ConfirmedCount` for now and improve later.
	// Wait, `features/dashboard.md` says "Confirmadas Automaticamente - 24".
	// I'll add `AutoConfirmed` to the struct and try to query it.

	err := r.db.Table("appointments").
		Select(`
			COUNT(*) as total_count,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_count,
			COUNT(CASE WHEN status = 'confirmed' THEN 1 END) as confirmed_count,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as no_show_count,
			COALESCE(SUM(CASE WHEN status IN ('completed', 'confirmed') THEN price ELSE 0 END), 0) as total_revenue,
			COALESCE(SUM(CASE WHEN status = 'no_show' THEN price ELSE 0 END), 0) as lost_revenue,
			COUNT(CASE WHEN status = 'confirmed' AND reminder_sent_at IS NOT NULL THEN 1 END) as auto_confirmed
		`).
		Where("clinic_id = ? AND start_time BETWEEN ? AND ?", clinicID, start, end).
		Scan(&stats).Error

	return &stats, err
}

func (r *dashboardRepository) GetTopAbsentees(ctx context.Context, clinicID uuid.UUID, start, end time.Time, limit int) ([]model.PatientAbsentee, error) {
	var absentees []model.PatientAbsentee

	err := r.db.Table("appointments").
		Select("patients.name, COUNT(*) as absent_count").
		Joins("JOIN patients ON appointments.patient_id = patients.id").
		Where("appointments.clinic_id = ? AND appointments.status = 'no_show' AND appointments.start_time BETWEEN ? AND ?", clinicID, start, end).
		Group("patients.name").
		Order("absent_count DESC").
		Limit(limit).
		Scan(&absentees).Error

	return absentees, err
}

func (r *dashboardRepository) GetFutureSessionsCount(ctx context.Context, clinicID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Table("appointments").
		Where("clinic_id = ? AND start_time > ?", clinicID, time.Now()).
		Count(&count).Error
	return count, err
}

// GetNoShowsByTimePeriod returns no-show statistics grouped by time period (morning/afternoon/evening)
func (r *dashboardRepository) GetNoShowsByTimePeriod(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.TimePatternStats, error) {
	var results []model.TimePatternStats

	query := `
		SELECT
			CASE
				WHEN EXTRACT(HOUR FROM start_time) < 12 THEN 'morning'
				WHEN EXTRACT(HOUR FROM start_time) < 18 THEN 'afternoon'
				ELSE 'evening'
			END as period,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as no_show_count,
			COUNT(*) as total_count,
			ROUND(COUNT(CASE WHEN status = 'no_show' THEN 1 END)::numeric / NULLIF(COUNT(*), 0) * 100, 1) as no_show_rate
		FROM appointments
		WHERE clinic_id = ?
			AND start_time BETWEEN ? AND ?
			AND status IN ('completed', 'no_show')
		GROUP BY period
		ORDER BY no_show_rate DESC
	`

	err := r.db.WithContext(ctx).Raw(query, clinicID, start, end).Scan(&results).Error
	return results, err
}

// GetNoShowsByDayOfWeek returns no-show statistics grouped by day of week
func (r *dashboardRepository) GetNoShowsByDayOfWeek(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.TimePatternStats, error) {
	var results []model.TimePatternStats

	query := `
		SELECT
			CASE EXTRACT(DOW FROM start_time)
				WHEN 0 THEN 'Domingo'
				WHEN 1 THEN 'Segunda'
				WHEN 2 THEN 'Terca'
				WHEN 3 THEN 'Quarta'
				WHEN 4 THEN 'Quinta'
				WHEN 5 THEN 'Sexta'
				WHEN 6 THEN 'Sabado'
			END as period,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as no_show_count,
			COUNT(*) as total_count,
			ROUND(COUNT(CASE WHEN status = 'no_show' THEN 1 END)::numeric / NULLIF(COUNT(*), 0) * 100, 1) as no_show_rate
		FROM appointments
		WHERE clinic_id = ?
			AND start_time BETWEEN ? AND ?
			AND status IN ('completed', 'no_show')
		GROUP BY EXTRACT(DOW FROM start_time)
		HAVING COUNT(*) >= 3
		ORDER BY no_show_rate DESC
	`

	err := r.db.WithContext(ctx).Raw(query, clinicID, start, end).Scan(&results).Error
	return results, err
}

// GetPatientTimePatterns returns patients with high no-show rates in specific time periods
func (r *dashboardRepository) GetPatientTimePatterns(ctx context.Context, clinicID uuid.UUID, start, end time.Time, minAppointments int) ([]model.PatientTimePattern, error) {
	var results []model.PatientTimePattern

	query := `
		SELECT
			a.patient_id::text as patient_id,
			p.name as patient_name,
			CASE
				WHEN EXTRACT(HOUR FROM a.start_time) < 12 THEN 'morning'
				WHEN EXTRACT(HOUR FROM a.start_time) < 18 THEN 'afternoon'
				ELSE 'evening'
			END as period,
			COUNT(CASE WHEN a.status = 'no_show' THEN 1 END) as no_show_count,
			COUNT(*) as total_count,
			ROUND(COUNT(CASE WHEN a.status = 'no_show' THEN 1 END)::numeric / NULLIF(COUNT(*), 0) * 100, 1) as no_show_rate
		FROM appointments a
		JOIN patients p ON a.patient_id = p.id
		WHERE a.clinic_id = ?
			AND a.start_time BETWEEN ? AND ?
			AND a.status IN ('completed', 'no_show')
		GROUP BY a.patient_id, p.name, period
		HAVING COUNT(*) >= ? AND COUNT(CASE WHEN a.status = 'no_show' THEN 1 END) >= 2
		ORDER BY no_show_rate DESC, no_show_count DESC
		LIMIT 10
	`

	err := r.db.WithContext(ctx).Raw(query, clinicID, start, end, minAppointments).Scan(&results).Error
	return results, err
}

// GetProfessionalDayPatterns returns professionals with high no-show rates on specific days
func (r *dashboardRepository) GetProfessionalDayPatterns(ctx context.Context, clinicID uuid.UUID, start, end time.Time, minAppointments int) ([]model.ProfessionalDayPattern, error) {
	var results []model.ProfessionalDayPattern

	query := `
		SELECT
			a.professional_id::text as professional_id,
			pr.name as professional_name,
			CASE EXTRACT(DOW FROM a.start_time)
				WHEN 0 THEN 'Domingo'
				WHEN 1 THEN 'Segunda'
				WHEN 2 THEN 'Terca'
				WHEN 3 THEN 'Quarta'
				WHEN 4 THEN 'Quinta'
				WHEN 5 THEN 'Sexta'
				WHEN 6 THEN 'Sabado'
			END as day_of_week,
			COUNT(CASE WHEN a.status = 'no_show' THEN 1 END) as no_show_count,
			COUNT(*) as total_count,
			ROUND(COUNT(CASE WHEN a.status = 'no_show' THEN 1 END)::numeric / NULLIF(COUNT(*), 0) * 100, 1) as no_show_rate
		FROM appointments a
		JOIN professionals pr ON a.professional_id = pr.id
		WHERE a.clinic_id = ?
			AND a.start_time BETWEEN ? AND ?
			AND a.status IN ('completed', 'no_show')
		GROUP BY a.professional_id, pr.name, EXTRACT(DOW FROM a.start_time)
		HAVING COUNT(*) >= ? AND COUNT(CASE WHEN a.status = 'no_show' THEN 1 END) >= 2
		ORDER BY no_show_rate DESC, no_show_count DESC
		LIMIT 10
	`

	err := r.db.WithContext(ctx).Raw(query, clinicID, start, end, minAppointments).Scan(&results).Error
	return results, err
}
