package repository

import (
	"context"
	"math"

	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/systemlog/model"
)

// SystemLogRepository defines the interface for system log persistence
type SystemLogRepository interface {
	Create(ctx context.Context, log *model.SystemLog) error
	List(ctx context.Context, filter *model.SystemLogFilter) (*model.SystemLogResponse, error)
}

type systemLogRepository struct {
	db *gorm.DB
}

// NewSystemLogRepository creates a new system log repository
func NewSystemLogRepository(db *gorm.DB) SystemLogRepository {
	return &systemLogRepository{db: db}
}

func (r *systemLogRepository) Create(ctx context.Context, log *model.SystemLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *systemLogRepository) List(ctx context.Context, filter *model.SystemLogFilter) (*model.SystemLogResponse, error) {
	var logs []model.SystemLog
	var total int64

	query := r.db.WithContext(ctx).Model(&model.SystemLog{})

	if filter.Service != "" {
		query = query.Where("service = ?", filter.Service)
	}
	if filter.Event != "" {
		query = query.Where("event = ?", filter.Event)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.ClinicID != "" {
		query = query.Where("clinic_id = ?", filter.ClinicID)
	}
	if filter.StartDate != "" {
		query = query.Where("created_at >= ?", filter.StartDate)
	}
	if filter.EndDate != "" {
		query = query.Where("created_at <= ?", filter.EndDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	limit := filter.GetLimit()
	page := filter.GetPage()
	offset := filter.GetOffset()

	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return &model.SystemLogResponse{
		Data: logs,
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
