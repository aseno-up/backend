package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/calendar/model"
	"github.com/aseno/backend/pkg/calendar/service"
)

// syncRepository implements service.SyncRepository
type syncRepository struct {
	db *gorm.DB
}

// NewSyncRepository creates a new sync repository
func NewSyncRepository(db *gorm.DB) service.SyncRepository {
	return &syncRepository{db: db}
}

// Create creates a new calendar sync record
func (r *syncRepository) Create(ctx context.Context, sync *model.CalendarSync) error {
	result := r.db.WithContext(ctx).Create(sync)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByAppointmentID retrieves a sync record by appointment ID
func (r *syncRepository) GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) (*model.CalendarSync, error) {
	var sync model.CalendarSync
	result := r.db.WithContext(ctx).Where("appointment_id = ?", appointmentID).First(&sync)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Calendar sync not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &sync, nil
}

// GetByGoogleEventID retrieves a sync record by Google event ID
func (r *syncRepository) GetByGoogleEventID(ctx context.Context, googleEventID string) (*model.CalendarSync, error) {
	var sync model.CalendarSync
	result := r.db.WithContext(ctx).Where("google_event_id = ?", googleEventID).First(&sync)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Calendar sync not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &sync, nil
}

// Update updates a sync record
func (r *syncRepository) Update(ctx context.Context, sync *model.CalendarSync) error {
	result := r.db.WithContext(ctx).Save(sync)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft deletes a sync record
func (r *syncRepository) Delete(ctx context.Context, appointmentID uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("appointment_id = ?", appointmentID).Delete(&model.CalendarSync{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetPendingSyncs retrieves pending sync records for a clinic
func (r *syncRepository) GetPendingSyncs(ctx context.Context, clinicID uuid.UUID, limit int) ([]*model.CalendarSync, error) {
	var syncs []*model.CalendarSync
	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND sync_status = ?", clinicID, model.SyncStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&syncs)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}

	return syncs, nil
}
