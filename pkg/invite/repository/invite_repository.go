package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/invite/model"
)

// InviteRepository defines the interface for invite data access
type InviteRepository interface {
	Create(ctx context.Context, invite *model.Invite) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Invite, error)
	GetByToken(ctx context.Context, token string) (*model.Invite, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*model.Invite, int64, error)
	GetPendingByEmail(ctx context.Context, email string, clinicID uuid.UUID) (*model.Invite, error)
	Update(ctx context.Context, invite *model.Invite) error
	Delete(ctx context.Context, id uuid.UUID) error
	ExpireOldInvites(ctx context.Context) error
}

// inviteRepository implements InviteRepository
type inviteRepository struct {
	db *gorm.DB
}

// NewInviteRepository creates a new invite repository
func NewInviteRepository(db *gorm.DB) InviteRepository {
	return &inviteRepository{db: db}
}

// Create creates a new invite
func (r *inviteRepository) Create(ctx context.Context, invite *model.Invite) error {
	result := r.db.WithContext(ctx).Create(invite)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves an invite by ID
func (r *inviteRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Invite, error) {
	var invite model.Invite
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&invite)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrInviteNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &invite, nil
}

// GetByToken retrieves an invite by token
func (r *inviteRepository) GetByToken(ctx context.Context, token string) (*model.Invite, error) {
	var invite model.Invite
	result := r.db.WithContext(ctx).Where("token = ?", token).First(&invite)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrInviteNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &invite, nil
}

// GetByClinicID retrieves invites for a clinic with pagination
func (r *inviteRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*model.Invite, int64, error) {
	var invites []*model.Invite
	var total int64

	baseQuery := r.db.WithContext(ctx).Model(&model.Invite{}).
		Where("clinic_id = ?", clinicID)

	// Count total
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Get paginated results
	result := r.db.WithContext(ctx).
		Where("clinic_id = ?", clinicID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&invites)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return invites, total, nil
}

// GetPendingByEmail retrieves a pending invite by email for a specific clinic
func (r *inviteRepository) GetPendingByEmail(ctx context.Context, email string, clinicID uuid.UUID) (*model.Invite, error) {
	var invite model.Invite
	result := r.db.WithContext(ctx).
		Where("email = ? AND clinic_id = ? AND status = ?", email, clinicID, model.InviteStatusPending).
		First(&invite)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // No pending invite, not an error
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &invite, nil
}

// Update updates an invite
func (r *inviteRepository) Update(ctx context.Context, invite *model.Invite) error {
	result := r.db.WithContext(ctx).Save(invite)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete deletes an invite
func (r *inviteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&model.Invite{}, "id = ?", id)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrInviteNotFoundInstance
	}
	return nil
}

// ExpireOldInvites marks expired invites as expired
func (r *inviteRepository) ExpireOldInvites(ctx context.Context) error {
	result := r.db.WithContext(ctx).
		Model(&model.Invite{}).
		Where("status = ? AND expires_at < ?", model.InviteStatusPending, time.Now()).
		Update("status", model.InviteStatusExpired)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
