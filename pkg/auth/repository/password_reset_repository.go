package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/auth/model"
)

// PasswordResetRepository defines the interface for password reset operations
type PasswordResetRepository interface {
	Create(ctx context.Context, reset *model.PasswordReset) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.PasswordReset, error)
	MarkAsUsed(ctx context.Context, id uuid.UUID) error
}

// passwordResetRepository implements PasswordResetRepository
type passwordResetRepository struct {
	db *gorm.DB
}

// NewPasswordResetRepository creates a new password reset repository
func NewPasswordResetRepository(db *gorm.DB) PasswordResetRepository {
	return &passwordResetRepository{db: db}
}

// Create creates a new password reset record
func (r *passwordResetRepository) Create(ctx context.Context, reset *model.PasswordReset) error {
	return r.db.WithContext(ctx).Create(reset).Error
}

// GetByTokenHash retrieves a password reset record by token hash
func (r *passwordResetRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.PasswordReset, error) {
	var reset model.PasswordReset
	if err := r.db.WithContext(ctx).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, time.Now()).
		First(&reset).Error; err != nil {
		return nil, err
	}
	return &reset, nil
}

// MarkAsUsed marks a password reset token as used
func (r *passwordResetRepository) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.PasswordReset{}).
		Where("id = ?", id).
		Update("used_at", now).Error
}
