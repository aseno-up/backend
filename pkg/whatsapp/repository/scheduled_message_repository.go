package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/whatsapp/model"
)

// ScheduledMessageRepository defines the interface for scheduled message operations
type ScheduledMessageRepository interface {
	Create(ctx context.Context, msg *model.ScheduledMessage) error
	GetPendingMessages(ctx context.Context, limit int) ([]*model.ScheduledMessage, error)
	GetLastScheduledForClinic(ctx context.Context, clinicID uuid.UUID) (*model.ScheduledMessage, error)
	MarkAsSent(ctx context.Context, id uuid.UUID) error
	MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error
	DeletePendingByPhoneAndType(ctx context.Context, clinicID uuid.UUID, phone string, messageType string) error
}

type scheduledMessageRepository struct {
	db *gorm.DB
}

// NewScheduledMessageRepository creates a new scheduled message repository
func NewScheduledMessageRepository(db *gorm.DB) ScheduledMessageRepository {
	return &scheduledMessageRepository{db: db}
}

// Create creates a new scheduled message
func (r *scheduledMessageRepository) Create(ctx context.Context, msg *model.ScheduledMessage) error {
	return r.db.WithContext(ctx).Create(msg).Error
}

// GetPendingMessages retrieves ALL messages for the FIRST pending patient of EACH clinic
func (r *scheduledMessageRepository) GetPendingMessages(ctx context.Context, limit int) ([]*model.ScheduledMessage, error) {
	var messages []*model.ScheduledMessage

	// Get the first pending patient (clinic_id + phone) for each clinic
	// Then return ALL messages for those patients
	// This processes multiple clinics in parallel while keeping segmented messages together
	err := r.db.WithContext(ctx).
		Raw(`
			WITH first_patient_per_clinic AS (
				SELECT DISTINCT ON (clinic_id) clinic_id, phone
				FROM scheduled_messages
				WHERE sent = false AND scheduled_at <= ?
				ORDER BY clinic_id, scheduled_at ASC
				LIMIT ?
			)
			SELECT sm.*
			FROM scheduled_messages sm
			INNER JOIN first_patient_per_clinic fp
			  ON sm.clinic_id = fp.clinic_id AND sm.phone = fp.phone
			WHERE sm.sent = false AND sm.scheduled_at <= ?
			ORDER BY sm.clinic_id, sm.scheduled_at ASC
		`, time.Now(), limit, time.Now()).
		Scan(&messages).Error

	return messages, err
}

// GetLastScheduledForClinic retrieves the last scheduled message for a clinic (sent or not)
func (r *scheduledMessageRepository) GetLastScheduledForClinic(ctx context.Context, clinicID uuid.UUID) (*model.ScheduledMessage, error) {
	var msg model.ScheduledMessage
	err := r.db.WithContext(ctx).
		Where("clinic_id = ?", clinicID).
		Order("scheduled_at DESC").
		First(&msg).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &msg, err
}

// MarkAsSent marks a scheduled message as sent
func (r *scheduledMessageRepository) MarkAsSent(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.ScheduledMessage{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"sent":    true,
			"sent_at": now,
		}).Error
}

// MarkAsFailed marks a scheduled message as failed with error
func (r *scheduledMessageRepository) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	return r.db.WithContext(ctx).
		Model(&model.ScheduledMessage{}).
		Where("id = ?", id).
		Update("error", errorMsg).Error
}

// DeletePendingByPhoneAndType deletes all pending scheduled messages for a patient matching the specific type
func (r *scheduledMessageRepository) DeletePendingByPhoneAndType(ctx context.Context, clinicID uuid.UUID, phone string, messageType string) error {
	return r.db.WithContext(ctx).
		Where("clinic_id = ? AND phone = ? AND message_type = ? AND sent = false", clinicID, phone, messageType).
		Delete(&model.ScheduledMessage{}).Error
}
