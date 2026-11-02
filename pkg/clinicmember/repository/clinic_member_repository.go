package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/clinicmember/model"
)

// ClinicMemberRepository defines the interface for clinic member data operations
type ClinicMemberRepository interface {
	Create(ctx context.Context, member *model.ClinicMember) error
	GetByUserAndClinic(ctx context.Context, userID, clinicID uuid.UUID) (*model.ClinicMember, error)
	GetClinicsByUserID(ctx context.Context, userID uuid.UUID) ([]*model.ClinicMember, error)
	GetMembersByClinicID(ctx context.Context, clinicID uuid.UUID) ([]*model.ClinicMember, error)
	Update(ctx context.Context, member *model.ClinicMember) error
	SoftDelete(ctx context.Context, userID, clinicID uuid.UUID) error
	ClearUserID(ctx context.Context, userID, clinicID uuid.UUID) error
}

type clinicMemberRepository struct {
	db *gorm.DB
}

// NewClinicMemberRepository creates a new clinic member repository
func NewClinicMemberRepository(db *gorm.DB) ClinicMemberRepository {
	return &clinicMemberRepository{db: db}
}

// Create creates a new clinic membership
func (r *clinicMemberRepository) Create(ctx context.Context, member *model.ClinicMember) error {
	result := r.db.WithContext(ctx).Create(member)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByUserAndClinic retrieves a membership for a specific user and clinic
func (r *clinicMemberRepository) GetByUserAndClinic(ctx context.Context, userID, clinicID uuid.UUID) (*model.ClinicMember, error) {
	var member model.ClinicMember
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND clinic_id = ? AND deleted_at IS NULL", userID, clinicID).
		First(&member)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Membership not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &member, nil
}

// GetClinicsByUserID retrieves all active clinic memberships for a user
func (r *clinicMemberRepository) GetClinicsByUserID(ctx context.Context, userID uuid.UUID) ([]*model.ClinicMember, error) {
	var members []*model.ClinicMember
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = true AND deleted_at IS NULL", userID).
		Order("created_at ASC").
		Find(&members)
	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return members, nil
}

// GetMembersByClinicID retrieves all active members for a clinic
func (r *clinicMemberRepository) GetMembersByClinicID(ctx context.Context, clinicID uuid.UUID) ([]*model.ClinicMember, error) {
	var members []*model.ClinicMember
	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND is_active = true AND deleted_at IS NULL", clinicID).
		Find(&members)
	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return members, nil
}

// Update updates a clinic membership
func (r *clinicMemberRepository) Update(ctx context.Context, member *model.ClinicMember) error {
	result := r.db.WithContext(ctx).Save(member)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// SoftDelete soft-deletes a clinic membership
func (r *clinicMemberRepository) SoftDelete(ctx context.Context, userID, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND clinic_id = ?", userID, clinicID).
		Delete(&model.ClinicMember{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// ClearUserID is a no-op placeholder to satisfy interface patterns (membership deletion is via SoftDelete)
func (r *clinicMemberRepository) ClearUserID(ctx context.Context, userID, clinicID uuid.UUID) error {
	return r.SoftDelete(ctx, userID, clinicID)
}
