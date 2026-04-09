package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/attendance/model"
)

// GroupAttendanceRepository defines the interface for group session attendance data operations
type GroupAttendanceRepository interface {
	Create(ctx context.Context, attendance *model.GroupSessionAttendance) error
	GetByPlanAndDate(ctx context.Context, clinicID, sessionPlanID uuid.UUID, date string) ([]*model.GroupSessionAttendance, error)
	GetByPlanDateAndPatient(ctx context.Context, clinicID, sessionPlanID, patientID uuid.UUID, date string) (*model.GroupSessionAttendance, error)
	Update(ctx context.Context, attendance *model.GroupSessionAttendance) error
}

type groupAttendanceRepository struct {
	db *gorm.DB
}

// NewGroupAttendanceRepository creates a new group attendance repository
func NewGroupAttendanceRepository(db *gorm.DB) GroupAttendanceRepository {
	return &groupAttendanceRepository{db: db}
}

// Create creates a new group attendance record
func (r *groupAttendanceRepository) Create(ctx context.Context, attendance *model.GroupSessionAttendance) error {
	if err := r.db.WithContext(ctx).Create(attendance).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// GetByPlanAndDate retrieves all attendance records for a group session on a specific date
func (r *groupAttendanceRepository) GetByPlanAndDate(ctx context.Context, clinicID, sessionPlanID uuid.UUID, date string) ([]*model.GroupSessionAttendance, error) {
	var records []*model.GroupSessionAttendance
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND session_plan_id = ? AND session_date = ?", clinicID, sessionPlanID, date).
		Order("created_at ASC").
		Find(&records).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	for _, record := range records {
		r.loadRelations(ctx, record)
	}

	return records, nil
}

// GetByPlanDateAndPatient retrieves a specific attendance record
func (r *groupAttendanceRepository) GetByPlanDateAndPatient(ctx context.Context, clinicID, sessionPlanID, patientID uuid.UUID, date string) (*model.GroupSessionAttendance, error) {
	var record model.GroupSessionAttendance
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND session_plan_id = ? AND patient_id = ? AND session_date = ?",
			clinicID, sessionPlanID, patientID, date).
		First(&record).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	r.loadRelations(ctx, &record)
	return &record, nil
}

// Update updates a group attendance record
func (r *groupAttendanceRepository) Update(ctx context.Context, attendance *model.GroupSessionAttendance) error {
	if err := r.db.WithContext(ctx).Save(attendance).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// loadRelations loads patient name
func (r *groupAttendanceRepository) loadRelations(ctx context.Context, record *model.GroupSessionAttendance) {
	var patientName string
	r.db.WithContext(ctx).
		Table("patients").
		Select("name").
		Where("id = ?", record.PatientID).
		Scan(&patientName)
	record.PatientName = patientName
}
