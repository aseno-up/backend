package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/user/model"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.User, error)
	GetByIDNoTenant(ctx context.Context, id uuid.UUID) (*model.User, error) // auth-internal only (password reset, email verification)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByGoogleID(ctx context.Context, googleID string) (*model.User, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.User, int64, error)
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdateRefreshToken(ctx context.Context, userID uuid.UUID, token string) error
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error
	BulkDelete(ctx context.Context, clinicID uuid.UUID, ids []uuid.UUID) error
}

// userRepository implements UserRepository
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create creates a new user
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	result := r.db.WithContext(ctx).Create(user)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a user by ID within a clinic
func (r *userRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.User, error) {
	var user model.User
	result := r.db.WithContext(ctx).Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrUserNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &user, nil
}

// GetByIDNoTenant retrieves a user by ID without tenant filtering (auth-internal only: password reset, email verification, token refresh)
func (r *userRepository) GetByIDNoTenant(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	result := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrUserNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &user, nil
}

// GetByEmail retrieves a user by email
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	result := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrUserNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &user, nil
}

// GetByGoogleID retrieves a user by Google OAuth ID
func (r *userRepository) GetByGoogleID(ctx context.Context, googleID string) (*model.User, error) {
	var user model.User
	result := r.db.WithContext(ctx).Where("google_id = ? AND deleted_at IS NULL", googleID).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrUserNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &user, nil
}

// GetByClinicID retrieves users belonging to a clinic
func (r *userRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64

	query := r.db.WithContext(ctx).Model(&model.User{}).Where("clinic_id = ? AND deleted_at IS NULL", clinicID)

	if filter.Search != "" {
		searchQuery := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ?", searchQuery, searchQuery)
	}

	if filter.Status != "" && filter.Status != "all" {
		query = query.Where("role = ?", filter.Status) // reusing status argument for role logic in the frontend
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Sorting
	switch filter.SortBy {
	case "created_desc":
		query = query.Order("created_at DESC")
	case "created_asc":
		query = query.Order("created_at ASC")
	case "name_asc":
		query = query.Order("name ASC")
	case "name_desc":
		query = query.Order("name DESC")
	default:
		query = query.Order("created_at DESC")
	}

	// Get paginated results
	result := query.Scopes(pagination.Paginate(&filter)).Find(&users)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return users, total, nil
}

// Update updates a user
func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	result := r.db.WithContext(ctx).Save(user)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft-deletes a user within a clinic
func (r *userRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ? AND clinic_id = ?", id, clinicID).Delete(&model.User{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrUserNotFoundInstance
	}
	return nil
}

// BulkDelete soft deletes multiple users belonging to a clinic
func (r *userRepository) BulkDelete(ctx context.Context, clinicID uuid.UUID, ids []uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id IN ? AND clinic_id = ?", ids, clinicID).
		Delete(&model.User{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// ExistsByEmail checks if an email is already registeredn email exists
func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&model.User{}).Where("email = ? AND deleted_at IS NULL", email).Count(&count)
	if result.Error != nil {
		return false, errors.ErrInternalInstance.WithError(result.Error)
	}
	return count > 0, nil
}

// UpdateRefreshToken updates the user's refresh token
func (r *userRepository) UpdateRefreshToken(ctx context.Context, userID uuid.UUID, token string) error {
	result := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("refresh_token", token)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// UpdateLastLogin updates the user's last login timestamp
func (r *userRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("last_login_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
