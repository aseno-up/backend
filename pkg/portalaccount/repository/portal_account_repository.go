package repository

import (
	"context"

	"github.com/aseno/backend/pkg/portalaccount/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PortalAccountRepository interface {
	Create(ctx context.Context, account *model.PortalAccount) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.PortalAccount, error)
	GetByRegistrationToken(ctx context.Context, token string) (*model.PortalAccount, error)
	GetByEmailVerificationToken(ctx context.Context, token string) (*model.PortalAccount, error)
	GetByEmail(ctx context.Context, email string) (*model.PortalAccount, error)
	GetByEmailAndClinic(ctx context.Context, email string, clinicID uuid.UUID) (*model.PortalAccount, error)
	GetByPatientID(ctx context.Context, patientID uuid.UUID) (*model.PortalAccount, error)
	Update(ctx context.Context, account *model.PortalAccount) error
}

type portalAccountRepository struct {
	db *gorm.DB
}

func NewPortalAccountRepository(db *gorm.DB) PortalAccountRepository {
	return &portalAccountRepository{db: db}
}

func (r *portalAccountRepository) Create(ctx context.Context, account *model.PortalAccount) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *portalAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.PortalAccount, error) {
	var account model.PortalAccount
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&account).Error
	return &account, err
}

func (r *portalAccountRepository) GetByRegistrationToken(ctx context.Context, token string) (*model.PortalAccount, error) {
	var account model.PortalAccount
	err := r.db.WithContext(ctx).Where("registration_token = ?", token).First(&account).Error
	return &account, err
}

func (r *portalAccountRepository) GetByEmailVerificationToken(ctx context.Context, token string) (*model.PortalAccount, error) {
	var account model.PortalAccount
	err := r.db.WithContext(ctx).Where("email_verification_token = ?", token).First(&account).Error
	return &account, err
}

func (r *portalAccountRepository) GetByEmail(ctx context.Context, email string) (*model.PortalAccount, error) {
	var account model.PortalAccount
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&account).Error
	return &account, err
}

func (r *portalAccountRepository) GetByEmailAndClinic(ctx context.Context, email string, clinicID uuid.UUID) (*model.PortalAccount, error) {
	var account model.PortalAccount
	err := r.db.WithContext(ctx).Where("email = ? AND clinic_id = ?", email, clinicID).First(&account).Error
	return &account, err
}

func (r *portalAccountRepository) GetByPatientID(ctx context.Context, patientID uuid.UUID) (*model.PortalAccount, error) {
	var account model.PortalAccount
	err := r.db.WithContext(ctx).Where("patient_id = ?", patientID).First(&account).Error
	return &account, err
}

func (r *portalAccountRepository) Update(ctx context.Context, account *model.PortalAccount) error {
	return r.db.WithContext(ctx).Save(account).Error
}
