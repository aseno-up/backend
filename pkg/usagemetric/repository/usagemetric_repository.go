package repository

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/usagemetric/model"
)

// UsageMetricRepository defines the interface for usage metric persistence
type UsageMetricRepository interface {
	Upsert(ctx context.Context, clinicID uuid.UUID, metric string, value float64, date time.Time) error
	Increment(ctx context.Context, clinicID uuid.UUID, metric string, delta float64, date time.Time) error
	List(ctx context.Context, filter *model.UsageMetricFilter) (*model.UsageMetricResponse, error)
	GetSummary(ctx context.Context, clinicID uuid.UUID, startDate, endDate time.Time) (*model.UsageMetricSummary, error)
	GetAllClinicsSummary(ctx context.Context, startDate, endDate time.Time) ([]model.UsageMetricSummary, error)
}

type usageMetricRepository struct {
	db *gorm.DB
}

// NewUsageMetricRepository creates a new usage metric repository
func NewUsageMetricRepository(db *gorm.DB) UsageMetricRepository {
	return &usageMetricRepository{db: db}
}

// Upsert creates or replaces a metric value for a clinic/date/metric combination
func (r *usageMetricRepository) Upsert(ctx context.Context, clinicID uuid.UUID, metric string, value float64, date time.Time) error {
	m := &model.UsageMetric{
		ClinicID: clinicID,
		Metric:   metric,
		Value:    value,
		Date:     date,
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "clinic_id"}, {Name: "metric"}, {Name: "date"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).
		Create(m).Error

	if err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// Increment adds delta to an existing metric value (or creates it)
func (r *usageMetricRepository) Increment(ctx context.Context, clinicID uuid.UUID, metric string, delta float64, date time.Time) error {
	result := r.db.WithContext(ctx).Exec(`
		INSERT INTO usage_metrics (id, clinic_id, metric, value, date)
		VALUES (gen_random_uuid(), ?, ?, ?, ?)
		ON CONFLICT (clinic_id, metric, date)
		DO UPDATE SET value = usage_metrics.value + EXCLUDED.value
	`, clinicID, metric, delta, date)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *usageMetricRepository) List(ctx context.Context, filter *model.UsageMetricFilter) (*model.UsageMetricResponse, error) {
	var metrics []model.UsageMetric
	var total int64

	query := r.db.WithContext(ctx).Model(&model.UsageMetric{})

	if filter.ClinicID != "" {
		query = query.Where("clinic_id = ?", filter.ClinicID)
	}
	if filter.Metric != "" {
		query = query.Where("metric = ?", filter.Metric)
	}
	if filter.StartDate != "" {
		query = query.Where("date >= ?", filter.StartDate)
	}
	if filter.EndDate != "" {
		query = query.Where("date <= ?", filter.EndDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	limit := filter.GetLimit()
	page := filter.GetPage()
	offset := filter.GetOffset()

	if err := query.Order("date DESC, clinic_id, metric").Offset(offset).Limit(limit).Find(&metrics).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return &model.UsageMetricResponse{
		Data: metrics,
		Meta: struct {
			Page       int   `json:"page"`
			Limit      int   `json:"limit"`
			Total      int64 `json:"total"`
			TotalPages int   `json:"total_pages"`
		}{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (r *usageMetricRepository) GetSummary(ctx context.Context, clinicID uuid.UUID, startDate, endDate time.Time) (*model.UsageMetricSummary, error) {
	var results []struct {
		Metric string
		Total  float64
	}

	err := r.db.WithContext(ctx).
		Model(&model.UsageMetric{}).
		Select("metric, SUM(value) as total").
		Where("clinic_id = ? AND date >= ? AND date <= ?", clinicID, startDate, endDate).
		Group("metric").
		Find(&results).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	metrics := make(map[string]float64)
	for _, r := range results {
		metrics[r.Metric] = r.Total
	}

	return &model.UsageMetricSummary{
		ClinicID: clinicID,
		Period:   startDate.Format("2006-01-02") + " to " + endDate.Format("2006-01-02"),
		Metrics:  metrics,
	}, nil
}

func (r *usageMetricRepository) GetAllClinicsSummary(ctx context.Context, startDate, endDate time.Time) ([]model.UsageMetricSummary, error) {
	var results []struct {
		ClinicID uuid.UUID
		Metric   string
		Total    float64
	}

	err := r.db.WithContext(ctx).
		Model(&model.UsageMetric{}).
		Select("clinic_id, metric, SUM(value) as total").
		Where("date >= ? AND date <= ?", startDate, endDate).
		Group("clinic_id, metric").
		Order("clinic_id").
		Find(&results).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Group by clinic
	summaryMap := make(map[uuid.UUID]*model.UsageMetricSummary)
	period := startDate.Format("2006-01-02") + " to " + endDate.Format("2006-01-02")

	for _, r := range results {
		if _, ok := summaryMap[r.ClinicID]; !ok {
			summaryMap[r.ClinicID] = &model.UsageMetricSummary{
				ClinicID: r.ClinicID,
				Period:   period,
				Metrics:  make(map[string]float64),
			}
		}
		summaryMap[r.ClinicID].Metrics[r.Metric] = r.Total
	}

	summaries := make([]model.UsageMetricSummary, 0, len(summaryMap))
	for _, s := range summaryMap {
		summaries = append(summaries, *s)
	}

	return summaries, nil
}
