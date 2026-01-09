package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/notification/model"
)

// NotificationRepository defines the interface for notification data access
type NotificationRepository interface {
	Create(ctx context.Context, notification *model.Notification) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Notification, error)
	List(ctx context.Context, clinicID, userID uuid.UUID, filter *model.NotificationFilter) ([]*model.Notification, error)
	CountUnread(ctx context.Context, clinicID, userID uuid.UUID) (int64, error)
	MarkAsRead(ctx context.Context, id, clinicID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, clinicID, userID uuid.UUID) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	DeleteOlderThan(ctx context.Context, before time.Time) error
}

type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

// Create creates a new notification
func (r *notificationRepository) Create(ctx context.Context, notification *model.Notification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}

// GetByID retrieves a notification by ID
func (r *notificationRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Notification, error) {
	var notification model.Notification
	err := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		First(&notification).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

// List retrieves notifications for a user in a clinic
func (r *notificationRepository) List(ctx context.Context, clinicID, userID uuid.UUID, filter *model.NotificationFilter) ([]*model.Notification, error) {
	query := r.db.WithContext(ctx).
		Where("clinic_id = ?", clinicID).
		Where("user_id IS NULL OR user_id = ?", userID).
		Order("created_at DESC")

	if filter != nil {
		if filter.Type != nil {
			query = query.Where("type = ?", *filter.Type)
		}
		if filter.Read != nil {
			if *filter.Read {
				query = query.Where("read_at IS NOT NULL")
			} else {
				query = query.Where("read_at IS NULL")
			}
		}
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		} else {
			query = query.Limit(50) // Default limit
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	} else {
		query = query.Limit(50) // Default limit
	}

	var notifications []*model.Notification
	if err := query.Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

// CountUnread counts unread notifications for a user
func (r *notificationRepository) CountUnread(ctx context.Context, clinicID, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("clinic_id = ?", clinicID).
		Where("user_id IS NULL OR user_id = ?", userID).
		Where("read_at IS NULL").
		Count(&count).Error
	return count, err
}

// MarkAsRead marks a notification as read
func (r *notificationRepository) MarkAsRead(ctx context.Context, id, clinicID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Update("read_at", now).Error
}

// MarkAllAsRead marks all notifications as read for a user
func (r *notificationRepository) MarkAllAsRead(ctx context.Context, clinicID, userID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("clinic_id = ?", clinicID).
		Where("user_id IS NULL OR user_id = ?", userID).
		Where("read_at IS NULL").
		Update("read_at", now).Error
}

// Delete deletes a notification
func (r *notificationRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.Notification{}).Error
}

// DeleteOlderThan deletes notifications older than a given time
func (r *notificationRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&model.Notification{}).Error
}
