package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/finance/model"
)

// TransactionRepository handles persistence of financial_transactions.
// Every query is scoped by clinic_id to preserve multi-tenant isolation.
type TransactionRepository interface {
	Create(ctx context.Context, t *model.FinancialTransaction) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.FinancialTransaction, error)
	Update(ctx context.Context, t *model.FinancialTransaction) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	List(ctx context.Context, clinicID uuid.UUID, filter model.TransactionFilter) ([]model.FinancialTransaction, error)

	// Per-payment keys — one finance row per individual payment event.
	GetByAppointmentPaymentID(ctx context.Context, appointmentPaymentID, clinicID uuid.UUID) (*model.FinancialTransaction, error)
	DeleteByAppointmentPaymentID(ctx context.Context, appointmentPaymentID, clinicID uuid.UUID) error

	GetBySessionPlanPaymentID(ctx context.Context, sessionPlanPaymentID, clinicID uuid.UUID) (*model.FinancialTransaction, error)
	DeleteBySessionPlanPaymentID(ctx context.Context, sessionPlanPaymentID, clinicID uuid.UUID) error

	SumByPeriod(ctx context.Context, clinicID uuid.UUID, from, to time.Time) (income, expense float64, err error)
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(ctx context.Context, t *model.FinancialTransaction) error {
	if err := r.db.WithContext(ctx).Create(t).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *transactionRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.FinancialTransaction, error) {
	var tx model.FinancialTransaction
	err := r.db.WithContext(ctx).
		Preload("Category").
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).
		First(&tx).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &tx, nil
}

func (r *transactionRepository) Update(ctx context.Context, t *model.FinancialTransaction) error {
	// Explicit tenant scope on update — defense in depth against a caller
	// who might construct the struct with a foreign ClinicID.
	res := r.db.WithContext(ctx).
		Model(&model.FinancialTransaction{}).
		Where("id = ? AND clinic_id = ?", t.ID, t.ClinicID).
		Updates(map[string]interface{}{
			"type":              t.Type,
			"category_id":       t.CategoryID,
			"amount":            t.Amount,
			"description":       t.Description,
			"competence_date":   t.CompetenceDate,
			"due_date":          t.DueDate,
			"paid_at":           t.PaidAt,
			"payment_method_id": t.PaymentMethodID,
		})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	return nil
}

func (r *transactionRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.FinancialTransaction{})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	return nil
}

func (r *transactionRepository) List(ctx context.Context, clinicID uuid.UUID, filter model.TransactionFilter) ([]model.FinancialTransaction, error) {
	q := r.db.WithContext(ctx).
		Preload("Category").
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID)

	if filter.From != nil {
		q = q.Where("competence_date >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("competence_date < ?", *filter.To)
	}
	if filter.Type != nil {
		q = q.Where("type = ?", *filter.Type)
	}
	if filter.CategoryID != nil {
		q = q.Where("category_id = ?", *filter.CategoryID)
	}
	if filter.Source != nil {
		q = q.Where("source = ?", *filter.Source)
	}
	if filter.OnlyPaid {
		q = q.Where("paid_at IS NOT NULL")
	}

	var out []model.FinancialTransaction
	if err := q.Order("competence_date DESC, created_at DESC").Find(&out).Error; err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return out, nil
}

func (r *transactionRepository) GetByAppointmentPaymentID(ctx context.Context, appointmentPaymentID, clinicID uuid.UUID) (*model.FinancialTransaction, error) {
	var tx model.FinancialTransaction
	err := r.db.WithContext(ctx).
		Preload("Category").
		Where("appointment_payment_id = ? AND clinic_id = ? AND deleted_at IS NULL", appointmentPaymentID, clinicID).
		First(&tx).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &tx, nil
}

func (r *transactionRepository) DeleteByAppointmentPaymentID(ctx context.Context, appointmentPaymentID, clinicID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("appointment_payment_id = ? AND clinic_id = ?", appointmentPaymentID, clinicID).
		Delete(&model.FinancialTransaction{})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	return nil
}

func (r *transactionRepository) GetBySessionPlanPaymentID(ctx context.Context, sessionPlanPaymentID, clinicID uuid.UUID) (*model.FinancialTransaction, error) {
	var tx model.FinancialTransaction
	err := r.db.WithContext(ctx).
		Preload("Category").
		Where("session_plan_payment_id = ? AND clinic_id = ? AND deleted_at IS NULL", sessionPlanPaymentID, clinicID).
		First(&tx).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &tx, nil
}

func (r *transactionRepository) DeleteBySessionPlanPaymentID(ctx context.Context, sessionPlanPaymentID, clinicID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("session_plan_payment_id = ? AND clinic_id = ?", sessionPlanPaymentID, clinicID).
		Delete(&model.FinancialTransaction{})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	return nil
}

// SumByPeriod sums income and expense for a period using competence_date.
// Only paid transactions count — pending bills (paid_at IS NULL) are
// future cash flow, not realized cash.
func (r *transactionRepository) SumByPeriod(ctx context.Context, clinicID uuid.UUID, from, to time.Time) (float64, float64, error) {
	var row struct {
		TotalIncome  float64
		TotalExpense float64
	}
	err := r.db.WithContext(ctx).
		Table("financial_transactions").
		Select(`
			COALESCE(SUM(CASE WHEN type = 'income'  THEN amount ELSE 0 END), 0) AS total_income,
			COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0) AS total_expense
		`).
		Where("clinic_id = ? AND deleted_at IS NULL AND paid_at IS NOT NULL AND competence_date >= ? AND competence_date < ?",
			clinicID, from, to).
		Scan(&row).Error
	if err != nil {
		return 0, 0, errors.ErrInternalInstance.WithError(err)
	}
	return row.TotalIncome, row.TotalExpense, nil
}
