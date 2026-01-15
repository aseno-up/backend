package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// schedulingIntentRepository implements service.SchedulingIntentRepository
type schedulingIntentRepository struct {
	db *gorm.DB
}

// NewSchedulingIntentRepository creates a new scheduling intent repository
func NewSchedulingIntentRepository(db *gorm.DB) service.SchedulingIntentRepository {
	return &schedulingIntentRepository{db: db}
}

// Create creates a new scheduling intent
func (r *schedulingIntentRepository) Create(ctx context.Context, intent *model.SchedulingIntent) error {
	result := r.db.WithContext(ctx).Create(intent)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a scheduling intent by ID
func (r *schedulingIntentRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.SchedulingIntent, error) {
	var intent model.SchedulingIntent
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&intent)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Scheduling intent not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &intent, nil
}

// GetPendingByPhone retrieves the most recent pending scheduling intent for a phone number
func (r *schedulingIntentRepository) GetPendingByPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*model.SchedulingIntent, error) {
	var intent model.SchedulingIntent
	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND patient_phone LIKE ? AND status = ? AND expires_at > ?",
			clinicID, "%"+phone+"%", model.SchedulingIntentStatusPending, time.Now()).
		Order("created_at DESC").
		First(&intent)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // Not found is not an error for this query
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &intent, nil
}

// Update updates a scheduling intent
func (r *schedulingIntentRepository) Update(ctx context.Context, intent *model.SchedulingIntent) error {
	result := r.db.WithContext(ctx).Save(intent)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// UpdateStatus updates the status of a scheduling intent
func (r *schedulingIntentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.SchedulingIntentStatus) error {
	result := r.db.WithContext(ctx).
		Model(&model.SchedulingIntent{}).
		Where("id = ?", id).
		Update("status", status)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetExpired retrieves all expired pending scheduling intents
func (r *schedulingIntentRepository) GetExpired(ctx context.Context) ([]*model.SchedulingIntent, error) {
	var intents []*model.SchedulingIntent
	result := r.db.WithContext(ctx).
		Where("status = ? AND expires_at < ?", model.SchedulingIntentStatusPending, time.Now()).
		Find(&intents)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return intents, nil
}

// ExpireOld marks all expired pending intents as expired
func (r *schedulingIntentRepository) ExpireOld(ctx context.Context) error {
	result := r.db.WithContext(ctx).
		Model(&model.SchedulingIntent{}).
		Where("status = ? AND expires_at < ?", model.SchedulingIntentStatusPending, time.Now()).
		Update("status", model.SchedulingIntentStatusExpired)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
