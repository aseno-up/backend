package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/sessionplan/model"
)

// PlanPaymentRepository defines the interface for plan payment data operations
type PlanPaymentRepository interface {
	Create(ctx context.Context, payment *model.SessionPlanPayment) error
	GetByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanPayment, error)
	GetByEnrollment(ctx context.Context, clinicID, enrollmentID uuid.UUID) ([]*model.SessionPlanPayment, error)
	GetTotalPaidByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (float64, error)
	GetTotalPaidByEnrollment(ctx context.Context, clinicID, enrollmentID uuid.UUID) (float64, error)
	GetTotalPaidByEnrollmentAndDate(ctx context.Context, clinicID, enrollmentID uuid.UUID, sessionDate string) (float64, error)
	Delete(ctx context.Context, clinicID, id uuid.UUID) error
}

type planPaymentRepository struct {
	db *gorm.DB
}

// NewPlanPaymentRepository creates a new plan payment repository
func NewPlanPaymentRepository(db *gorm.DB) PlanPaymentRepository {
	return &planPaymentRepository{db: db}
}

func (r *planPaymentRepository) Create(ctx context.Context, payment *model.SessionPlanPayment) error {
	if err := r.db.WithContext(ctx).Create(payment).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *planPaymentRepository) GetByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanPayment, error) {
	var payments []*model.SessionPlanPayment
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND session_plan_id = ? AND enrollment_id IS NULL", clinicID, sessionPlanID).
		Order("paid_at DESC").
		Find(&payments).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	for _, p := range payments {
		r.loadRelations(ctx, p)
	}
	return payments, nil
}

func (r *planPaymentRepository) GetByEnrollment(ctx context.Context, clinicID, enrollmentID uuid.UUID) ([]*model.SessionPlanPayment, error) {
	var payments []*model.SessionPlanPayment
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND enrollment_id = ?", clinicID, enrollmentID).
		Order("paid_at DESC").
		Find(&payments).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	for _, p := range payments {
		r.loadRelations(ctx, p)
	}
	return payments, nil
}

func (r *planPaymentRepository) GetTotalPaidByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).
		Model(&model.SessionPlanPayment{}).
		Where("clinic_id = ? AND session_plan_id = ? AND enrollment_id IS NULL AND deleted_at IS NULL", clinicID, sessionPlanID).
		Select("COALESCE(SUM(amount + discount), 0)").
		Scan(&total).Error

	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return total, nil
}

func (r *planPaymentRepository) GetTotalPaidByEnrollment(ctx context.Context, clinicID, enrollmentID uuid.UUID) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).
		Model(&model.SessionPlanPayment{}).
		Where("clinic_id = ? AND enrollment_id = ? AND deleted_at IS NULL", clinicID, enrollmentID).
		Select("COALESCE(SUM(amount + discount), 0)").
		Scan(&total).Error

	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return total, nil
}

func (r *planPaymentRepository) GetTotalPaidByEnrollmentAndDate(ctx context.Context, clinicID, enrollmentID uuid.UUID, sessionDate string) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).
		Model(&model.SessionPlanPayment{}).
		Where("clinic_id = ? AND enrollment_id = ? AND session_date = ? AND deleted_at IS NULL", clinicID, enrollmentID, sessionDate).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error

	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return total, nil
}

func (r *planPaymentRepository) Delete(ctx context.Context, clinicID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Delete(&model.SessionPlanPayment{})

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFoundInstance.WithMessage("Payment not found")
	}
	return nil
}

func (r *planPaymentRepository) loadRelations(ctx context.Context, payment *model.SessionPlanPayment) {
	var methodName string
	r.db.WithContext(ctx).
		Table("payment_methods").
		Select("name").
		Where("id = ?", payment.PaymentMethodID).
		Scan(&methodName)
	payment.PaymentMethodName = methodName
}
