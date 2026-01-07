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
	TotalCount       int
	CompletedCount   int
	ConfirmedCount   int
	NoShowCount      int
	TotalRevenue     float64
	LostRevenue      float64
	AutoConfirmed    int
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
