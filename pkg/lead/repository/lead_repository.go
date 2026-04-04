package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/lead/model"
	"github.com/aseno/backend/pkg/pagination"
)

// LeadRepository defines the interface for lead data operations
type LeadRepository interface {
	Create(ctx context.Context, lead *model.Lead) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Lead, error)
	List(ctx context.Context, filter model.LeadFilter) ([]*model.Lead, int64, error)
	Update(ctx context.Context, lead *model.Lead) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByExternalID(ctx context.Context, externalID string) (*model.Lead, error)
}

type leadRepository struct {
	db *gorm.DB
}

func NewLeadRepository(db *gorm.DB) LeadRepository {
	return &leadRepository{db: db}
}

func (r *leadRepository) Create(ctx context.Context, lead *model.Lead) error {
	result := r.db.WithContext(ctx).Create(lead)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *leadRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Lead, error) {
	var lead model.Lead
	result := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&lead)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrLeadNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &lead, nil
}

func (r *leadRepository) List(ctx context.Context, filter model.LeadFilter) ([]*model.Lead, int64, error) {
	var leads []*model.Lead
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Lead{}).
		Where("deleted_at IS NULL")

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where(
			"clinic_name ILIKE ? OR contact_name ILIKE ? OR phone ILIKE ? OR city ILIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern,
		)
	}

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	if filter.Channel != "" {
		query = query.Where("channel = ?", filter.Channel)
	}

	if filter.Source != "" {
		query = query.Where("source = ?", filter.Source)
	}

	if filter.PainScore != "" {
		query = query.Where("pain_score = ?", filter.PainScore)
	}

	if filter.DateFrom != "" {
		query = query.Where("created_at >= ?", filter.DateFrom)
	}

	if filter.DateTo != "" {
		query = query.Where("created_at <= ?", filter.DateTo+"T23:59:59Z")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	order := "created_at DESC"
	switch filter.SortBy {
	case "clinic_name_asc":
		order = "clinic_name ASC"
	case "clinic_name_desc":
		order = "clinic_name DESC"
	case "created_asc":
		order = "created_at ASC"
	case "next_follow_up":
		order = "next_follow_up_at ASC NULLS LAST"
	case "mrr_desc":
		order = "mrr DESC"
	}

	paginationFilter := pagination.Filter{
		Page:  filter.GetPage(),
		Limit: filter.GetLimit(),
	}

	result := query.
		Order(order).
		Scopes(pagination.Paginate(&paginationFilter)).
		Find(&leads)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return leads, total, nil
}

func (r *leadRepository) Update(ctx context.Context, lead *model.Lead) error {
	result := r.db.WithContext(ctx).Save(lead)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *leadRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.Lead{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrLeadNotFoundInstance
	}
	return nil
}

func (r *leadRepository) GetByExternalID(ctx context.Context, externalID string) (*model.Lead, error) {
	var lead model.Lead
	result := r.db.WithContext(ctx).
		Where("external_id = ? AND deleted_at IS NULL", externalID).
		First(&lead)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrLeadNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &lead, nil
}
