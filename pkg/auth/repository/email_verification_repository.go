package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/auth/model"
)

// EmailVerificationRepository defines the interface for email verification data access
type EmailVerificationRepository interface {
	Create(ctx context.Context, verification *model.EmailVerification) error
	GetByToken(ctx context.Context, token string) (*model.EmailVerification, error)
	GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*model.EmailVerification, error)
	MarkAsUsed(ctx context.Context, id uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

type emailVerificationRepository struct {
	db *gorm.DB
}

// NewEmailVerificationRepository creates a new email verification repository
func NewEmailVerificationRepository(db *gorm.DB) EmailVerificationRepository {
	return &emailVerificationRepository{db: db}
}

// Create creates a new email verification record
func (r *emailVerificationRepository) Create(ctx context.Context, verification *model.EmailVerification) error {
	return r.db.WithContext(ctx).Create(verification).Error
}

// GetByToken retrieves a verification by its token
func (r *emailVerificationRepository) GetByToken(ctx context.Context, token string) (*model.EmailVerification, error) {
	var verification model.EmailVerification
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&verification).Error
	if err != nil {
		return nil, err
	}
	return &verification, nil
}

// GetLatestByUserID retrieves the latest verification for a user
func (r *emailVerificationRepository) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*model.EmailVerification, error) {
	var verification model.EmailVerification
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&verification).Error
	if err != nil {
		return nil, err
	}
	return &verification, nil
}

// MarkAsUsed marks a verification as used
func (r *emailVerificationRepository) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.EmailVerification{}).
		Where("id = ?", id).
		Update("used_at", now).Error
}

// DeleteByUserID deletes all verifications for a user
func (r *emailVerificationRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.EmailVerification{}).Error
}

// DeleteExpired deletes all expired verifications
func (r *emailVerificationRepository) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&model.EmailVerification{}).Error
}
