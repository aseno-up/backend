package service

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/aseno/backend/pkg/settings/model"
	"github.com/aseno/backend/pkg/settings/repository"
)

// SettingsService defines the interface for settings business logic
type SettingsService interface {
	GetAll(ctx context.Context) (map[string]model.SettingDetail, error)
	Get(ctx context.Context, key string) (string, error)
	Update(ctx context.Context, key, value string) error
	RefreshCache(ctx context.Context) error

	// Typed helpers for plan configuration
	GetPlanLimit(ctx context.Context, planType string) int
	GetPlanPrice(ctx context.Context, planType string) int
	GetTrialDays(ctx context.Context) int
	GetExcessPrice(ctx context.Context) int
	IsExcessBillingEnabled(ctx context.Context) bool
	IsUnlimited(limit int) bool
}

// settingsService implements SettingsService with in-memory cache
type settingsService struct {
	repo       repository.SettingsRepository
	cache      map[string]string
	cacheMu    sync.RWMutex
	cacheTime  time.Time
	cacheTTL   time.Duration
}

// NewSettingsService creates a new settings service
func NewSettingsService(repo repository.SettingsRepository) SettingsService {
	return &settingsService{
		repo:     repo,
		cache:    make(map[string]string),
		cacheTTL: 5 * time.Minute, // Cache TTL of 5 minutes
	}
}

// GetAll retrieves all settings
func (s *settingsService) GetAll(ctx context.Context) (map[string]model.SettingDetail, error) {
	settings, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]model.SettingDetail)
	for _, setting := range settings {
		result[setting.Key] = model.SettingDetail{
			Value:       setting.Value,
			Description: setting.Description,
		}
	}

	return result, nil
}

// Get retrieves a single setting value with cache
func (s *settingsService) Get(ctx context.Context, key string) (string, error) {
	// Check cache first
	s.cacheMu.RLock()
	if val, ok := s.cache[key]; ok && time.Since(s.cacheTime) < s.cacheTTL {
		s.cacheMu.RUnlock()
		return val, nil
	}
	s.cacheMu.RUnlock()

	// Cache miss or expired, refresh cache
	if err := s.RefreshCache(ctx); err != nil {
		return "", err
	}

	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return s.cache[key], nil
}

// Update updates a setting and invalidates cache
func (s *settingsService) Update(ctx context.Context, key, value string) error {
	log.Printf("[Settings] Update: key=%s, value=%s", key, value)

	if err := s.repo.Set(ctx, key, value); err != nil {
		log.Printf("[Settings] Update: repo.Set error: %v", err)
		return err
	}

	// Invalidate cache completely
	s.cacheMu.Lock()
	s.cache = make(map[string]string) // Clear cache
	s.cacheTime = time.Time{}         // Force refresh on next read
	s.cacheMu.Unlock()

	log.Printf("[Settings] Update: cache cleared, refreshing...")

	// Force immediate refresh
	if err := s.RefreshCache(ctx); err != nil {
		log.Printf("[Settings] Update: RefreshCache error: %v", err)
	}

	return nil
}

// RefreshCache reloads all settings into cache
func (s *settingsService) RefreshCache(ctx context.Context) error {
	settings, err := s.repo.GetAll(ctx)
	if err != nil {
		log.Printf("[Settings] RefreshCache error: %v", err)
		return err
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.cache = make(map[string]string)
	for _, setting := range settings {
		s.cache[setting.Key] = setting.Value
		log.Printf("[Settings] Cached: %s = %s", setting.Key, setting.Value)
	}
	s.cacheTime = time.Now()

	log.Printf("[Settings] Cache refreshed with %d settings", len(settings))
	return nil
}

// GetPlanLimit returns the professional limit for a plan type
func (s *settingsService) GetPlanLimit(ctx context.Context, planType string) int {
	var key string
	var defaultVal int

	switch planType {
	case "basic":
		key = model.SettingPlanBasicLimit
		defaultVal = 5
	case "professional":
		key = model.SettingPlanProLimit
		defaultVal = 15
	case "advanced":
		key = model.SettingPlanAdvancedLimit
		defaultVal = -1 // unlimited
	default:
		key = model.SettingPlanBasicLimit
		defaultVal = 5
	}

	val, err := s.Get(ctx, key)
	if err != nil || val == "" {
		return defaultVal
	}

	limit, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}

	return limit
}

// GetPlanPrice returns the monthly price for a plan type
func (s *settingsService) GetPlanPrice(ctx context.Context, planType string) int {
	var key string
	var defaultVal int

	switch planType {
	case "basic":
		key = model.SettingPlanBasicPrice
		defaultVal = 99
	case "professional":
		key = model.SettingPlanProPrice
		defaultVal = 199
	case "advanced":
		key = model.SettingPlanAdvancedPrice
		defaultVal = 349
	default:
		key = model.SettingPlanBasicPrice
		defaultVal = 99
	}

	val, err := s.Get(ctx, key)
	if err != nil || val == "" {
		return defaultVal
	}

	price, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}

	return price
}

// GetTrialDays returns the trial period in days
func (s *settingsService) GetTrialDays(ctx context.Context) int {
	val, err := s.Get(ctx, model.SettingTrialDays)
	log.Printf("[Settings] GetTrialDays: key=%s, val=%s, err=%v", model.SettingTrialDays, val, err)
	if err != nil || val == "" {
		log.Printf("[Settings] GetTrialDays: using default 14")
		return 14 // default
	}

	days, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("[Settings] GetTrialDays: parse error, using default 14")
		return 14
	}

	log.Printf("[Settings] GetTrialDays: returning %d", days)
	return days
}

// GetExcessPrice returns the price per excess professional
func (s *settingsService) GetExcessPrice(ctx context.Context) int {
	val, err := s.Get(ctx, model.SettingPlanExcessPrice)
	if err != nil || val == "" {
		return 20 // default
	}

	price, err := strconv.Atoi(val)
	if err != nil {
		return 20
	}

	return price
}

// IsExcessBillingEnabled checks if excess billing is enabled
func (s *settingsService) IsExcessBillingEnabled(ctx context.Context) bool {
	val, err := s.Get(ctx, model.SettingExcessBillingEnabled)
	if err != nil || val == "" {
		return false // default: disabled
	}

	return val == "true"
}

// IsUnlimited checks if a limit value represents unlimited
func (s *settingsService) IsUnlimited(limit int) bool {
	return limit < 0
}
