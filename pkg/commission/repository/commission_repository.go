package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/commission/model"
)

// CommissionRepository defines data operations for commissions
type CommissionRepository interface {
	Create(ctx context.Context, commission *model.Commission) error
	GetByAppointmentID(ctx context.Context, appointmentID, clinicID uuid.UUID) (*model.Commission, error)
	GetBySessionPlanAndDate(ctx context.Context, sessionPlanID, clinicID uuid.UUID, date string) ([]model.Commission, error)
	ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID, from, to time.Time) ([]model.Commission, error)
	Summary(ctx context.Context, clinicID uuid.UUID, from, to time.Time) ([]model.ProfessionalCommissionSummary, error)
	BulkMarkPaid(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error
	DeleteByAppointmentID(ctx context.Context, appointmentID, clinicID uuid.UUID) error
	DeleteBySessionPlanAndDate(ctx context.Context, sessionPlanID, clinicID uuid.UUID, date string) error
}

type commissionRepository struct {
	db *gorm.DB
}

func NewCommissionRepository(db *gorm.DB) CommissionRepository {
	return &commissionRepository{db: db}
}

func (r *commissionRepository) Create(ctx context.Context, commission *model.Commission) error {
	if err := r.db.WithContext(ctx).Create(commission).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *commissionRepository) GetByAppointmentID(ctx context.Context, appointmentID, clinicID uuid.UUID) (*model.Commission, error) {
	var c model.Commission
	err := r.db.WithContext(ctx).
		Where("appointment_id = ? AND clinic_id = ? AND deleted_at IS NULL", appointmentID, clinicID).
		First(&c).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &c, nil
}

func (r *commissionRepository) ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID, from, to time.Time) ([]model.Commission, error) {
	var results []model.Commission
	err := r.db.WithContext(ctx).
		Where("professional_id = ? AND clinic_id = ? AND deleted_at IS NULL AND created_at >= ? AND created_at < ?",
			professionalID, clinicID, from, to).
		Order("created_at ASC").
		Find(&results).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return results, nil
}

// Summary returns a consolidated view of commissions per professional for a period
func (r *commissionRepository) Summary(ctx context.Context, clinicID uuid.UUID, from, to time.Time) ([]model.ProfessionalCommissionSummary, error) {
	var results []model.ProfessionalCommissionSummary

	err := r.db.WithContext(ctx).
		Table("commissions c").
		Select(`
			c.professional_id,
			p.name AS professional_name,
			p.color AS professional_color,
			COUNT(c.id) AS total_sessions,
			COALESCE(SUM(c.commission_value), 0) AS total_commission,
			COALESCE(SUM(CASE WHEN c.status = 'paid' THEN c.commission_value ELSE 0 END), 0) AS total_paid,
			COALESCE(SUM(CASE WHEN c.status = 'pending' THEN c.commission_value ELSE 0 END), 0) AS total_pending
		`).
		Joins("JOIN professionals p ON p.id = c.professional_id").
		Where("c.clinic_id = ? AND c.deleted_at IS NULL AND c.created_at >= ? AND c.created_at < ?", clinicID, from, to).
		Group("c.professional_id, p.name, p.color").
		Order("p.name ASC").
		Scan(&results).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return results, nil
}

func (r *commissionRepository) BulkMarkPaid(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.Commission{}).
		Where("id IN ? AND clinic_id = ? AND status = ? AND deleted_at IS NULL", ids, clinicID, model.CommissionStatusPending).
		Updates(map[string]interface{}{
			"status":  model.CommissionStatusPaid,
			"paid_at": &now,
		})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *commissionRepository) DeleteByAppointmentID(ctx context.Context, appointmentID, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("appointment_id = ? AND clinic_id = ?", appointmentID, clinicID).
		Delete(&model.Commission{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *commissionRepository) GetBySessionPlanAndDate(ctx context.Context, sessionPlanID, clinicID uuid.UUID, date string) ([]model.Commission, error) {
	var results []model.Commission
	err := r.db.WithContext(ctx).
		Where("session_plan_id = ? AND clinic_id = ? AND session_date = ? AND deleted_at IS NULL", sessionPlanID, clinicID, date).
		Find(&results).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return results, nil
}

func (r *commissionRepository) DeleteBySessionPlanAndDate(ctx context.Context, sessionPlanID, clinicID uuid.UUID, date string) error {
	result := r.db.WithContext(ctx).
		Where("session_plan_id = ? AND clinic_id = ? AND session_date = ?", sessionPlanID, clinicID, date).
		Delete(&model.Commission{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
