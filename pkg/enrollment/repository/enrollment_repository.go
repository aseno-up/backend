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
	// GetActiveByPatient returns every active enrollment for a patient
	// across all session plans in the clinic. Used by the AI agent
	// pre-fetch context so short patient replies ("sim", "imprevisto")
	// can be interpreted against the patient's actual matrículas.
	GetActiveByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.SessionPlanEnrollment, error)
	CountActiveByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (int64, error)
	CountActiveByPlanAndWeekDay(ctx context.Context, clinicID, sessionPlanID uuid.UUID, weekDay int) (int64, error)
	// CountActiveBySlot returns the number of active enrollments that include the given slot.
	CountActiveBySlot(ctx context.Context, clinicID, sessionPlanID, slotID uuid.UUID) (int64, error)
	UpdateStatus(ctx context.Context, clinicID, id uuid.UUID, status model.EnrollmentStatus) error
	IncrementSessionsDone(ctx context.Context, clinicID, id uuid.UUID) error
	Update(ctx context.Context, enrollment *model.SessionPlanEnrollment) error
	// SetEnrollmentSlots replaces all junction rows for an enrollment with the given slot IDs.
	SetEnrollmentSlots(ctx context.Context, enrollmentID uuid.UUID, slotIDs []uuid.UUID) error
	// GetSlotIDsByEnrollment returns the slot IDs currently linked to an enrollment.
	GetSlotIDsByEnrollment(ctx context.Context, enrollmentID uuid.UUID) ([]uuid.UUID, error)
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
		Preload("Slots").
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
		Preload("Slots").
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
		Preload("Slots").
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

// GetActiveByPatient returns every active enrollment for a patient
// across all session plans in the clinic. Loads slots so the AI agent
// pre-fetch context can compute the next class occurrence per matrícula.
func (r *enrollmentRepository) GetActiveByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.SessionPlanEnrollment, error) {
	var enrollments []*model.SessionPlanEnrollment
	err := r.db.WithContext(ctx).
		Preload("Slots").
		Where("clinic_id = ? AND patient_id = ? AND status = ?", clinicID, patientID, model.EnrollmentStatusActive).
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

func (r *enrollmentRepository) CountActiveBySlot(ctx context.Context, clinicID, sessionPlanID, slotID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("enrollment_slots AS es").
		Joins("JOIN session_plan_enrollments AS spe ON spe.id = es.enrollment_id").
		Where("spe.clinic_id = ? AND spe.session_plan_id = ? AND spe.status = ?",
			clinicID, sessionPlanID, model.EnrollmentStatusActive).
		Where("es.session_plan_slot_id = ?", slotID).
		Where("es.deleted_at IS NULL AND spe.deleted_at IS NULL").
		Count(&count).Error
	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return count, nil
}

func (r *enrollmentRepository) SetEnrollmentSlots(ctx context.Context, enrollmentID uuid.UUID, slotIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Hard-delete (Unscoped) so the unique index on
		// (enrollment_id, session_plan_slot_id) doesn't collide with rows that
		// were previously soft-deleted. The junction is a pure relation — its
		// "history" lives on the parent enrollment, not here.
		if err := tx.Unscoped().
			Where("enrollment_id = ?", enrollmentID).
			Delete(&model.EnrollmentSlot{}).Error; err != nil {
			return err
		}
		if len(slotIDs) == 0 {
			return nil
		}
		rows := make([]model.EnrollmentSlot, len(slotIDs))
		for i, sid := range slotIDs {
			rows[i] = model.EnrollmentSlot{
				EnrollmentID:      enrollmentID,
				SessionPlanSlotID: sid,
			}
		}
		return tx.Create(&rows).Error
	})
}

func (r *enrollmentRepository) GetSlotIDsByEnrollment(ctx context.Context, enrollmentID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).
		Model(&model.EnrollmentSlot{}).
		Where("enrollment_id = ?", enrollmentID).
		Pluck("session_plan_slot_id", &ids).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return ids, nil
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
