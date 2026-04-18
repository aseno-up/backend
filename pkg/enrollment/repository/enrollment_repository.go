package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/enrollment/model"
)

// EnrollmentRepository defines the interface for enrollment data operations
type EnrollmentRepository interface {
	Create(ctx context.Context, enrollment *model.SessionPlanEnrollment) error
	GetByID(ctx context.Context, clinicID, id uuid.UUID) (*model.SessionPlanEnrollment, error)
	GetByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanEnrollment, error)
	GetActiveByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanEnrollment, error)
	GetByPatientAndPlan(ctx context.Context, clinicID, patientID, sessionPlanID uuid.UUID) (*model.SessionPlanEnrollment, error)
	CountActiveByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (int64, error)
	CountActiveByPlanAndWeekDay(ctx context.Context, clinicID, sessionPlanID uuid.UUID, weekDay int) (int64, error)
	UpdateStatus(ctx context.Context, clinicID, id uuid.UUID, status model.EnrollmentStatus) error
	IncrementSessionsDone(ctx context.Context, clinicID, id uuid.UUID) error
	Update(ctx context.Context, enrollment *model.SessionPlanEnrollment) error
}

type enrollmentRepository struct {
	db *gorm.DB
}

// NewEnrollmentRepository creates a new enrollment repository
func NewEnrollmentRepository(db *gorm.DB) EnrollmentRepository {
	return &enrollmentRepository{db: db}
}

func (r *enrollmentRepository) Create(ctx context.Context, enrollment *model.SessionPlanEnrollment) error {
	if err := r.db.WithContext(ctx).Create(enrollment).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *enrollmentRepository) GetByID(ctx context.Context, clinicID, id uuid.UUID) (*model.SessionPlanEnrollment, error) {
	var enrollment model.SessionPlanEnrollment
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		First(&enrollment).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Enrollment not found")
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	r.loadRelations(ctx, &enrollment)
	return &enrollment, nil
}

func (r *enrollmentRepository) GetByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanEnrollment, error) {
	var enrollments []*model.SessionPlanEnrollment
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND session_plan_id = ?", clinicID, sessionPlanID).
		Where("patient_id IN (SELECT id FROM patients WHERE deleted_at IS NULL)").
		Order("joined_at ASC").
		Find(&enrollments).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	for _, e := range enrollments {
		r.loadRelations(ctx, e)
	}
	return enrollments, nil
}

func (r *enrollmentRepository) GetActiveByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanEnrollment, error) {
	var enrollments []*model.SessionPlanEnrollment
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND session_plan_id = ? AND status = ?", clinicID, sessionPlanID, model.EnrollmentStatusActive).
		Where("patient_id IN (SELECT id FROM patients WHERE deleted_at IS NULL)").
		Order("joined_at ASC").
		Find(&enrollments).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	for _, e := range enrollments {
		r.loadRelations(ctx, e)
	}
	return enrollments, nil
}

func (r *enrollmentRepository) GetByPatientAndPlan(ctx context.Context, clinicID, patientID, sessionPlanID uuid.UUID) (*model.SessionPlanEnrollment, error) {
	var enrollment model.SessionPlanEnrollment
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND patient_id = ? AND session_plan_id = ?", clinicID, patientID, sessionPlanID).
		First(&enrollment).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	r.loadRelations(ctx, &enrollment)
	return &enrollment, nil
}

func (r *enrollmentRepository) CountActiveByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.SessionPlanEnrollment{}).
		Where("clinic_id = ? AND session_plan_id = ? AND status = ?", clinicID, sessionPlanID, model.EnrollmentStatusActive).
		Count(&count).Error

	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return count, nil
}

func (r *enrollmentRepository) CountActiveByPlanAndWeekDay(ctx context.Context, clinicID, sessionPlanID uuid.UUID, weekDay int) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.SessionPlanEnrollment{}).
		Where("clinic_id = ? AND session_plan_id = ? AND status = ? AND ? = ANY(week_days)",
			clinicID, sessionPlanID, model.EnrollmentStatusActive, weekDay).
		Count(&count).Error

	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return count, nil
}

func (r *enrollmentRepository) UpdateStatus(ctx context.Context, clinicID, id uuid.UUID, status model.EnrollmentStatus) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}

	if status == model.EnrollmentStatusCancelled {
		now := time.Now()
		updates["left_at"] = now
	}

	result := r.db.WithContext(ctx).
		Model(&model.SessionPlanEnrollment{}).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Updates(updates)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFoundInstance.WithMessage("Enrollment not found")
	}
	return nil
}

func (r *enrollmentRepository) IncrementSessionsDone(ctx context.Context, clinicID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.SessionPlanEnrollment{}).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Update("sessions_done", gorm.Expr("sessions_done + 1"))

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *enrollmentRepository) Update(ctx context.Context, enrollment *model.SessionPlanEnrollment) error {
	if err := r.db.WithContext(ctx).Save(enrollment).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *enrollmentRepository) loadRelations(ctx context.Context, enrollment *model.SessionPlanEnrollment) {
	var patientName string
	r.db.WithContext(ctx).
		Table("patients").
		Select("name").
		Where("id = ?", enrollment.PatientID).
		Scan(&patientName)
	enrollment.PatientName = patientName
}
