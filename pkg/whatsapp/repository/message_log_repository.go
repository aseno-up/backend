package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// messageLogRepository implements service.MessageLogRepository
type messageLogRepository struct {
	db *gorm.DB
}

// NewMessageLogRepository creates a new message log repository
func NewMessageLogRepository(db *gorm.DB) service.MessageLogRepository {
	return &messageLogRepository{db: db}
}

// Create creates a new message log entry
func (r *messageLogRepository) Create(ctx context.Context, log *model.MessageLog) error {
	result := r.db.WithContext(ctx).Create(log)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByMessageID retrieves all logs for a message ordered by created_at
func (r *messageLogRepository) GetByMessageID(ctx context.Context, messageID uuid.UUID) ([]*model.MessageLog, error) {
	var logs []*model.MessageLog
	result := r.db.WithContext(ctx).
		Where("message_id = ?", messageID).
		Order("created_at ASC").
		Find(&logs)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return logs, nil
}

// GetByAppointmentID retrieves all logs for an appointment
func (r *messageLogRepository) GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) ([]*model.MessageLog, error) {
	var logs []*model.MessageLog
	result := r.db.WithContext(ctx).
		Where("appointment_id = ?", appointmentID).
		Order("created_at DESC").
		Find(&logs)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return logs, nil
}

// GetByPatientID retrieves all logs for a patient
func (r *messageLogRepository) GetByPatientID(ctx context.Context, patientID uuid.UUID, limit int) ([]*model.MessageLog, error) {
	var logs []*model.MessageLog
	query := r.db.WithContext(ctx).
		Where("patient_id = ?", patientID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	result := query.Find(&logs)
	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return logs, nil
}

// GetByClinicID retrieves recent logs for a clinic
func (r *messageLogRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit int) ([]*model.MessageLog, error) {
	var logs []*model.MessageLog
	query := r.db.WithContext(ctx).
		Where("clinic_id = ?", clinicID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	result := query.Find(&logs)
	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return logs, nil
}
