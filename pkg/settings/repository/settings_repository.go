package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aseno/backend/pkg/settings/model"
)

// SettingsRepository defines the interface for settings data access
type SettingsRepository interface {
	GetAll(ctx context.Context) ([]model.SystemSetting, error)
	Get(ctx context.Context, key string) (*model.SystemSetting, error)
	Set(ctx context.Context, key, value string) error
	SeedDefaults(ctx context.Context) error
}

// settingsRepository implements SettingsRepository
type settingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository creates a new settings repository
func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &settingsRepository{db: db}
}

// GetAll retrieves all settings
func (r *settingsRepository) GetAll(ctx context.Context) ([]model.SystemSetting, error) {
	var settings []model.SystemSetting
	if err := r.db.WithContext(ctx).Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

// Get retrieves a single setting by key
func (r *settingsRepository) Get(ctx context.Context, key string) (*model.SystemSetting, error) {
	var setting model.SystemSetting
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

// Set updates or creates a setting
func (r *settingsRepository) Set(ctx context.Context, key, value string) error {
	result := r.db.WithContext(ctx).
		Model(&model.SystemSetting{}).
		Where("key = ?", key).
		Updates(map[string]interface{}{
			"value":      value,
			"updated_at": r.db.NowFunc(),
		})

	if result.Error != nil {
		return result.Error
	}

	// If no rows affected, the key doesn't exist - create it
	if result.RowsAffected == 0 {
		setting := &model.SystemSetting{
			Key:   key,
			Value: value,
		}
		return r.db.WithContext(ctx).Create(setting).Error
	}

	return nil
}

// SeedDefaults creates default settings if they don't exist
func (r *settingsRepository) SeedDefaults(ctx context.Context) error {
	defaults := model.DefaultSettings()

	// Use ON CONFLICT DO NOTHING to skip existing keys
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoNothing: true,
		}).
		Create(&defaults).Error
}
