package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/aitokenusage/model"
)

// Repository defines AI token usage persistence operations.
type Repository interface {
	Create(ctx context.Context, u *model.AITokenUsage) error
	SummaryByModel(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.ModelSummary, error)
	SummaryByClinicAndModel(ctx context.Context, start, end time.Time) ([]model.ClinicModelSummary, error)
}

type repo struct {
	db *gorm.DB
}

// NewRepository creates a new AI token usage repository.
func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}

func (r *repo) Create(ctx context.Context, u *model.AITokenUsage) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *repo) SummaryByModel(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.ModelSummary, error) {
	var rows []model.ModelSummary
	err := r.db.WithContext(ctx).
		Model(&model.AITokenUsage{}).
		Select(`
			provider,
			model,
			COUNT(*) AS calls,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(cost_brl), 0) AS cost_brl
		`).
		Where("clinic_id = ? AND date >= ? AND date <= ?", clinicID, start, end).
		Group("provider, model").
		Order("total_tokens DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return rows, nil
}

func (r *repo) SummaryByClinicAndModel(ctx context.Context, start, end time.Time) ([]model.ClinicModelSummary, error) {
	var rows []model.ClinicModelSummary
	err := r.db.WithContext(ctx).
		Model(&model.AITokenUsage{}).
		Select(`
			clinic_id,
			provider,
			model,
			COUNT(*) AS calls,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(cost_brl), 0) AS cost_brl
		`).
		Where("date >= ? AND date <= ?", start, end).
		Group("clinic_id, provider, model").
		Order("clinic_id, total_tokens DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return rows, nil
}
