package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/finance/model"
)

type CategoryRepository interface {
	Create(ctx context.Context, c *model.FinancialCategory) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.FinancialCategory, error)
	Update(ctx context.Context, c *model.FinancialCategory) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	ListByClinic(ctx context.Context, clinicID uuid.UUID) ([]model.FinancialCategory, error)
	CountByClinic(ctx context.Context, clinicID uuid.UUID) (int64, error)
	CreateDefaults(ctx context.Context, clinicID uuid.UUID) error
}

type categoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) Create(ctx context.Context, c *model.FinancialCategory) error {
	if err := r.db.WithContext(ctx).Create(c).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *categoryRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.FinancialCategory, error) {
	var c model.FinancialCategory
	err := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).
		First(&c).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &c, nil
}

func (r *categoryRepository) Update(ctx context.Context, c *model.FinancialCategory) error {
	res := r.db.WithContext(ctx).
		Model(&model.FinancialCategory{}).
		Where("id = ? AND clinic_id = ?", c.ID, c.ClinicID).
		Updates(map[string]interface{}{
			"name":  c.Name,
			"color": c.Color,
		})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	return nil
}

func (r *categoryRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.FinancialCategory{})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	return nil
}

func (r *categoryRepository) ListByClinic(ctx context.Context, clinicID uuid.UUID) ([]model.FinancialCategory, error) {
	var list []model.FinancialCategory
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID).
		Order("type ASC, name ASC").
		Find(&list).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return list, nil
}

func (r *categoryRepository) CountByClinic(ctx context.Context, clinicID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.FinancialCategory{}).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID).
		Count(&count).Error
	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return count, nil
}

// CreateDefaults seeds a fresh clinic with a starter category list covering
// the most common income and expense flows seen in small clinics. Called
// lazily by the service on the first List to keep bootstrap implicit.
func (r *categoryRepository) CreateDefaults(ctx context.Context, clinicID uuid.UUID) error {
	defaults := []model.FinancialCategory{
		// Income
		{ClinicID: clinicID, Name: "Consulta", Type: model.TransactionTypeIncome, Color: "#10B981", IsDefault: true},
		{ClinicID: clinicID, Name: "Plano de sessões", Type: model.TransactionTypeIncome, Color: "#3B82F6", IsDefault: true},
		{ClinicID: clinicID, Name: "Outras receitas", Type: model.TransactionTypeIncome, Color: "#6366F1", IsDefault: true},
		// Expense
		{ClinicID: clinicID, Name: "Aluguel", Type: model.TransactionTypeExpense, Color: "#EF4444", IsDefault: true},
		{ClinicID: clinicID, Name: "Contas (água, luz, internet)", Type: model.TransactionTypeExpense, Color: "#F59E0B", IsDefault: true},
		{ClinicID: clinicID, Name: "Insumos e materiais", Type: model.TransactionTypeExpense, Color: "#8B5CF6", IsDefault: true},
		{ClinicID: clinicID, Name: "Taxas e impostos", Type: model.TransactionTypeExpense, Color: "#F43F5E", IsDefault: true},
		{ClinicID: clinicID, Name: "Outras despesas", Type: model.TransactionTypeExpense, Color: "#6B7280", IsDefault: true},
	}
	if err := r.db.WithContext(ctx).Create(&defaults).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}
