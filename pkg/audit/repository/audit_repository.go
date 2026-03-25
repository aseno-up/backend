package repository

import (
	"context"
	"math"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/audit/model"
)

// AuditRepository defines the interface for audit log persistence
type AuditRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
	List(ctx context.Context, clinicID uuid.UUID, filter *model.AuditLogFilter) (*model.AuditLogResponse, error)
}

type auditRepository struct {
	db *gorm.DB
}

// NewAuditRepository creates a new audit repository
func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) Create(ctx context.Context, log *model.AuditLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *auditRepository) List(ctx context.Context, clinicID uuid.UUID, filter *model.AuditLogFilter) (*model.AuditLogResponse, error) {
	var logs []model.AuditLog
	var total int64

	query := r.db.WithContext(ctx).Where("clinic_id = ?", clinicID)

	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.EntityType != "" {
		query = query.Where("entity_type = ?", filter.EntityType)
	}
	if filter.UserID != "" {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.EntityID != "" {
		query = query.Where("entity_id = ?", filter.EntityID)
	}
	if filter.StartDate != "" {
		query = query.Where("created_at >= ?", filter.StartDate)
	}
	if filter.EndDate != "" {
		query = query.Where("created_at <= ?", filter.EndDate)
	}

	if err := query.Model(&model.AuditLog{}).Count(&total).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	limit := filter.GetLimit()
	page := filter.GetPage()
	offset := filter.GetOffset()

	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return &model.AuditLogResponse{
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
