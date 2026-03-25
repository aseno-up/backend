package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/usagemetric/model"
	"github.com/aseno/backend/pkg/usagemetric/repository"
)

// UsageMetricService defines the interface for usage metric operations
type UsageMetricService interface {
	Record(ctx context.Context, clinicID uuid.UUID, metric string, value float64) error
	Increment(ctx context.Context, clinicID uuid.UUID, metric string, delta float64) error
	List(ctx context.Context, filter *model.UsageMetricFilter) (*model.UsageMetricResponse, error)
	GetSummary(ctx context.Context, clinicID uuid.UUID, startDate, endDate string) (*model.UsageMetricSummary, error)
	GetAllClinicsSummary(ctx context.Context, startDate, endDate string) (*model.UsageSummaryResponse, error)
}

type usageMetricService struct {
	repo repository.UsageMetricRepository
}

// NewUsageMetricService creates a new usage metric service
func NewUsageMetricService(repo repository.UsageMetricRepository) UsageMetricService {
	return &usageMetricService{repo: repo}
}

// Record sets an absolute value for a metric today
func (s *usageMetricService) Record(ctx context.Context, clinicID uuid.UUID, metric string, value float64) error {
	today := time.Now().Truncate(24 * time.Hour)

	go func() {
		if err := s.repo.Upsert(context.Background(), clinicID, metric, value, today); err != nil {
			logger.Printf("[UsageMetric] Failed to record metric %s: %v", metric, err)
		}
	}()

	return nil
}

// Increment adds to a metric counter for today
func (s *usageMetricService) Increment(ctx context.Context, clinicID uuid.UUID, metric string, delta float64) error {
	today := time.Now().Truncate(24 * time.Hour)

	go func() {
		if err := s.repo.Increment(context.Background(), clinicID, metric, delta, today); err != nil {
			logger.Printf("[UsageMetric] Failed to increment metric %s: %v", metric, err)
		}
	}()

	return nil
}

func (s *usageMetricService) List(ctx context.Context, filter *model.UsageMetricFilter) (*model.UsageMetricResponse, error) {
	return s.repo.List(ctx, filter)
}

func (s *usageMetricService) GetSummary(ctx context.Context, clinicID uuid.UUID, startDate, endDate string) (*model.UsageMetricSummary, error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		start = time.Now().AddDate(0, -1, 0)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		end = time.Now()
	}
	return s.repo.GetSummary(ctx, clinicID, start, end)
}

func (s *usageMetricService) GetAllClinicsSummary(ctx context.Context, startDate, endDate string) (*model.UsageSummaryResponse, error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		start = time.Now().AddDate(0, -1, 0)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		end = time.Now()
	}

	summaries, err := s.repo.GetAllClinicsSummary(ctx, start, end)
	if err != nil {
		return nil, err
	}

	return &model.UsageSummaryResponse{Data: summaries}, nil
}
