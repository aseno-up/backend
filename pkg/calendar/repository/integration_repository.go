package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/calendar/model"
	"github.com/aseno/backend/pkg/calendar/service"
)

// integrationRepository implements service.IntegrationRepository
type integrationRepository struct {
	db *gorm.DB
}

// NewIntegrationRepository creates a new integration repository
func NewIntegrationRepository(db *gorm.DB) service.IntegrationRepository {
	return &integrationRepository{db: db}
}

// Create creates a new calendar integration
func (r *integrationRepository) Create(ctx context.Context, integration *model.CalendarIntegration) error {
	result := r.db.WithContext(ctx).Create(integration)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByClinicID retrieves a calendar integration by clinic ID
func (r *integrationRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.CalendarIntegration, error) {
	var integration model.CalendarIntegration
	result := r.db.WithContext(ctx).Where("clinic_id = ?", clinicID).First(&integration)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrCalendarNotConfiguredInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &integration, nil
}

// GetByClinicIDIncludingDeleted retrieves a calendar integration including soft-deleted ones
func (r *integrationRepository) GetByClinicIDIncludingDeleted(ctx context.Context, clinicID uuid.UUID) (*model.CalendarIntegration, error) {
	var integration model.CalendarIntegration
	result := r.db.WithContext(ctx).Unscoped().Where("clinic_id = ?", clinicID).First(&integration)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrCalendarNotConfiguredInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &integration, nil
}

// Update updates a calendar integration (including reactivating soft-deleted ones)
func (r *integrationRepository) Update(ctx context.Context, integration *model.CalendarIntegration) error {
	result := r.db.WithContext(ctx).Unscoped().Save(integration)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft deletes a calendar integration
func (r *integrationRepository) Delete(ctx context.Context, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("clinic_id = ?", clinicID).Delete(&model.CalendarIntegration{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// UpdateTokens updates OAuth tokens for an integration
func (r *integrationRepository) UpdateTokens(ctx context.Context, clinicID uuid.UUID, accessToken, refreshToken string, expiry time.Time) error {
	updates := map[string]interface{}{
		"access_token":  accessToken,
		"token_expiry":  expiry,
	}
	if refreshToken != "" {
		updates["refresh_token"] = refreshToken
	}

	result := r.db.WithContext(ctx).
		Model(&model.CalendarIntegration{}).
		Where("clinic_id = ?", clinicID).
		Updates(updates)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// UpdateLastSync updates the last sync timestamp and error
func (r *integrationRepository) UpdateLastSync(ctx context.Context, clinicID uuid.UUID, lastSyncAt time.Time, syncError string) error {
	updates := map[string]interface{}{
		"last_sync_at":    lastSyncAt,
		"last_sync_error": syncError,
	}

	result := r.db.WithContext(ctx).
		Model(&model.CalendarIntegration{}).
		Where("clinic_id = ?", clinicID).
		Updates(updates)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
