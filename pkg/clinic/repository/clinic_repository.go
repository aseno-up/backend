package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/clinic/model"
)

// ClinicRepository defines the interface for clinic data operations
type ClinicRepository interface {
	Create(ctx context.Context, clinic *model.Clinic) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Clinic, error)
	GetBySlug(ctx context.Context, slug string) (*model.Clinic, error)
	GetByEmail(ctx context.Context, email string) (*model.Clinic, error)
	Update(ctx context.Context, clinic *model.Clinic) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*model.Clinic, int64, error)
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	CountByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error)
}

// clinicRepository implements ClinicRepository
type clinicRepository struct {
	db *gorm.DB
}

// NewClinicRepository creates a new clinic repository
func NewClinicRepository(db *gorm.DB) ClinicRepository {
	return &clinicRepository{db: db}
}

// clinicWithConnections is a helper struct for scanning JOIN results
type clinicWithConnections struct {
	model.Clinic
	WaConnected *bool `gorm:"column:wa_connected"`
	GcConnected *bool `gorm:"column:gc_connected"`
}

// getWithConnections executes a JOIN query to populate computed connection fields
func (r *clinicRepository) getWithConnections(ctx context.Context, whereClause string, args ...any) (*model.Clinic, error) {
	var row clinicWithConnections
	result := r.db.WithContext(ctx).
		Table("clinics c").
		Preload("Account").
		Preload("Account.Plan").
		Select(`c.*,
			(SELECT connected FROM whatsapp_connections wc
			 WHERE wc.clinic_id = c.id AND wc.is_active = true AND wc.deleted_at IS NULL
			 LIMIT 1) AS wa_connected,
			(SELECT CASE WHEN COUNT(*) > 0 THEN true ELSE false END
			 FROM calendar_integrations ci
			 WHERE ci.clinic_id = c.id AND ci.deleted_at IS NULL) AS gc_connected`).
		Where(whereClause, args...).
		Where("c.deleted_at IS NULL").
		First(&row)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrClinicNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}

	clinic := &row.Clinic
	if row.WaConnected != nil {
		clinic.WhatsAppConnected = *row.WaConnected
	}
	if row.GcConnected != nil {
		clinic.GoogleConnected = *row.GcConnected
	}
	return clinic, nil
}

// Create creates a new clinic
func (r *clinicRepository) Create(ctx context.Context, clinic *model.Clinic) error {
	result := r.db.WithContext(ctx).Create(clinic)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a clinic by ID (with computed WhatsAppConnected and GoogleConnected)
func (r *clinicRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Clinic, error) {
	return r.getWithConnections(ctx, "c.id = ?", id)
}

// GetBySlug retrieves a clinic by slug (with computed WhatsAppConnected and GoogleConnected)
func (r *clinicRepository) GetBySlug(ctx context.Context, slug string) (*model.Clinic, error) {
	return r.getWithConnections(ctx, "c.slug = ?", slug)
}

// GetByEmail retrieves a clinic by email
func (r *clinicRepository) GetByEmail(ctx context.Context, email string) (*model.Clinic, error) {
	var clinic model.Clinic
	result := r.db.WithContext(ctx).Preload("Account").Where("email = ? AND deleted_at IS NULL", email).First(&clinic)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrClinicNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &clinic, nil
}

// Update updates a clinic
func (r *clinicRepository) Update(ctx context.Context, clinic *model.Clinic) error {
	result := r.db.WithContext(ctx).Save(clinic)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft-deletes a clinic
func (r *clinicRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Clinic{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrClinicNotFoundInstance
	}
	return nil
}

// List retrieves a paginated list of clinics
func (r *clinicRepository) List(ctx context.Context, limit, offset int) ([]*model.Clinic, int64, error) {
	var clinics []*model.Clinic
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).Model(&model.Clinic{}).Where("deleted_at IS NULL").Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	result := r.db.WithContext(ctx).
		Preload("Account").
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&clinics)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return clinics, total, nil
}

// CountByAccountID counts clinics belonging to an account
func (r *clinicRepository) CountByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&model.Clinic{}).Where("account_id = ? AND deleted_at IS NULL", accountID).Count(&count)
	if result.Error != nil {
		return 0, errors.ErrInternalInstance.WithError(result.Error)
	}
	return count, nil
}

// ExistsBySlug checks if a clinic with the given slug exists
func (r *clinicRepository) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&model.Clinic{}).Where("slug = ? AND deleted_at IS NULL", slug).Count(&count)
	if result.Error != nil {
		return false, errors.ErrInternalInstance.WithError(result.Error)
	}
	return count > 0, nil
}

// ExistsByEmail checks if a clinic with the given email exists
func (r *clinicRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&model.Clinic{}).Where("email = ? AND deleted_at IS NULL", email).Count(&count)
	if result.Error != nil {
		return false, errors.ErrInternalInstance.WithError(result.Error)
	}
	return count > 0, nil
}
