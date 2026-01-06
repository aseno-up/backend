package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/whatsapp/model"
	"github.com/saas/backend/pkg/whatsapp/service"
)

// messageRepository implements service.MessageRepository
type messageRepository struct {
	db *gorm.DB
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *gorm.DB) service.MessageRepository {
	return &messageRepository{db: db}
}

// Create creates a new message log
func (r *messageRepository) Create(ctx context.Context, message *model.Message) error {
	result := r.db.WithContext(ctx).Create(message)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a message by ID
func (r *messageRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	var message model.Message
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&message)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Message not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &message, nil
}

// UpdateStatus updates the status of a message
func (r *messageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.MessageStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}

	switch status {
	case model.MessageStatusDelivered:
		now := time.Now()
		updates["delivered_at"] = &now
	case model.MessageStatusRead:
		now := time.Now()
		updates["read_at"] = &now
	}

	result := r.db.WithContext(ctx).
		Model(&model.Message{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByAppointmentID retrieves messages for an appointment
func (r *messageRepository) GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) ([]*model.Message, error) {
	var messages []*model.Message
	result := r.db.WithContext(ctx).
		Where("appointment_id = ?", appointmentID).
		Order("created_at DESC").
		Find(&messages)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return messages, nil
}

// GetPendingReminders retrieves messages that need to be sent before a given time
func (r *messageRepository) GetPendingReminders(ctx context.Context, before time.Time) ([]*model.Message, error) {
	var messages []*model.Message
	result := r.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", model.MessageStatusPending, before).
		Find(&messages)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return messages, nil
}

// GetByClinicAndPhone retrieves recent messages for a phone number in a clinic
func (r *messageRepository) GetByClinicAndPhone(ctx context.Context, clinicID uuid.UUID, phone string, limit int) ([]*model.Message, error) {
	var messages []*model.Message
	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND phone LIKE ?", clinicID, "%"+phone+"%").
		Order("created_at DESC").
		Limit(limit).
		Find(&messages)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return messages, nil
}

// GetLatestOutboundForAppointment gets the latest outbound message for an appointment
func (r *messageRepository) GetLatestOutboundForAppointment(ctx context.Context, appointmentID uuid.UUID) (*model.Message, error) {
	var message model.Message
	result := r.db.WithContext(ctx).
		Where("appointment_id = ? AND direction = ?", appointmentID, model.MessageDirectionOutbound).
		Order("created_at DESC").
		First(&message)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &message, nil
}
