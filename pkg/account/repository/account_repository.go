package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/account/model"
)

// AccountRepository defines the interface for account DB operations
type AccountRepository interface {
	Create(ctx context.Context, account *model.Account) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error)
	GetByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]model.Account, error)
	ListAll(ctx context.Context) ([]model.Account, error)
	Update(ctx context.Context, account *model.Account) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type accountRepository struct {
	db *gorm.DB
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &accountRepository{db: db}
}

func (r *accountRepository) Create(ctx context.Context, account *model.Account) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *accountRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	var account model.Account
	err := r.db.WithContext(ctx).First(&account, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *accountRepository) GetByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]model.Account, error) {
	var accounts []model.Account
	err := r.db.WithContext(ctx).Where("owner_id = ?", ownerID).Find(&accounts).Error
	return accounts, err
}

func (r *accountRepository) ListAll(ctx context.Context) ([]model.Account, error) {
	var accounts []model.Account
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Find(&accounts).Error
	return accounts, err
}

func (r *accountRepository) Update(ctx context.Context, account *model.Account) error {
	return r.db.WithContext(ctx).Save(account).Error
}

func (r *accountRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Account{}, "id = ?", id).Error
}
