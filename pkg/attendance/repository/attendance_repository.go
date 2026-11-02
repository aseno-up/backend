package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/attendance/model"
)

// AttendanceRepository defines the interface for attendance data operations
type AttendanceRepository interface {
	Create(ctx context.Context, attendance *model.AppointmentAttendance) error
	GetByAppointmentID(ctx context.Context, clinicID, appointmentID uuid.UUID) ([]*model.AppointmentAttendance, error)
	GetByAppointmentAndPatient(ctx context.Context, clinicID, appointmentID, patientID uuid.UUID) (*model.AppointmentAttendance, error)
	Update(ctx context.Context, attendance *model.AppointmentAttendance) error
	Delete(ctx context.Context, clinicID, id uuid.UUID) error
}

type attendanceRepository struct {
	db *gorm.DB
}

// NewAttendanceRepository creates a new attendance repository
func NewAttendanceRepository(db *gorm.DB) AttendanceRepository {
	return &attendanceRepository{db: db}
}

// Create creates a new attendance record
func (r *attendanceRepository) Create(ctx context.Context, attendance *model.AppointmentAttendance) error {
	if err := r.db.WithContext(ctx).Create(attendance).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// GetByAppointmentID retrieves all attendance records for an appointment
func (r *attendanceRepository) GetByAppointmentID(ctx context.Context, clinicID, appointmentID uuid.UUID) ([]*model.AppointmentAttendance, error) {
	var records []*model.AppointmentAttendance
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND appointment_id = ?", clinicID, appointmentID).
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

// GetByAppointmentAndPatient retrieves a specific attendance record
func (r *attendanceRepository) GetByAppointmentAndPatient(ctx context.Context, clinicID, appointmentID, patientID uuid.UUID) (*model.AppointmentAttendance, error) {
	var record model.AppointmentAttendance
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND appointment_id = ? AND patient_id = ?", clinicID, appointmentID, patientID).
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

// Update updates an attendance record
func (r *attendanceRepository) Update(ctx context.Context, attendance *model.AppointmentAttendance) error {
	if err := r.db.WithContext(ctx).Save(attendance).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// Delete deletes an attendance record
func (r *attendanceRepository) Delete(ctx context.Context, clinicID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Delete(&model.AppointmentAttendance{})

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFoundInstance.WithMessage("Attendance record not found")
	}
	return nil
}

// loadRelations loads patient name
func (r *attendanceRepository) loadRelations(ctx context.Context, record *model.AppointmentAttendance) {
	var patientName string
	r.db.WithContext(ctx).
		Table("patients").
		Select("name").
		Where("id = ?", record.PatientID).
		Scan(&patientName)
	record.PatientName = patientName
}
