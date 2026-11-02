package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/marketplace/model"
	"github.com/aseno/backend/pkg/marketplace/service"
)

type clinicPaymentRepository struct {
	db *gorm.DB
}

func NewClinicPaymentRepository(db *gorm.DB) service.ClinicPaymentRepository {
	return &clinicPaymentRepository{db: db}
}

func (r *clinicPaymentRepository) Create(ctx context.Context, payment *model.ClinicPayment) error {
	result := r.db.WithContext(ctx).Create(payment)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *clinicPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ClinicPayment, error) {
	var payment model.ClinicPayment
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&payment)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPaymentNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &payment, nil
}

func (r *clinicPaymentRepository) GetByAsaasID(ctx context.Context, asaasID string) (*model.ClinicPayment, error) {
	var payment model.ClinicPayment
	result := r.db.WithContext(ctx).Where("asaas_payment_id = ?", asaasID).First(&payment)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPaymentNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &payment, nil
}

func (r *clinicPaymentRepository) GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*model.ClinicPayment, int64, error) {
	var payments []*model.ClinicPayment
	var total int64

	query := r.db.WithContext(ctx).Where("clinic_id = ?", clinicID)
	query.Model(&model.ClinicPayment{}).Count(&total)

	result := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&payments)
	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return payments, total, nil
}

func (r *clinicPaymentRepository) GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) (*model.ClinicPayment, error) {
	var payment model.ClinicPayment
	result := r.db.WithContext(ctx).Where("appointment_id = ?", appointmentID).First(&payment)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPaymentNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &payment, nil
}

func (r *clinicPaymentRepository) ListWithFilters(ctx context.Context, clinicID uuid.UUID, filters model.PaymentFilters) ([]*model.ClinicPayment, int64, error) {
	var payments []*model.ClinicPayment
	var total int64

	query := r.db.WithContext(ctx).Where("clinic_id = ?", clinicID)

	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.BillingType != "" {
		query = query.Where("billing_type = ?", filters.BillingType)
	}
	if filters.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", filters.DateFrom); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if filters.DateTo != "" {
		if t, err := time.Parse("2006-01-02", filters.DateTo); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}

	query.Model(&model.ClinicPayment{}).Count(&total)

	limit := filters.PerPage
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := filters.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	result := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&payments)
	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return payments, total, nil
}

func (r *clinicPaymentRepository) Update(ctx context.Context, payment *model.ClinicPayment) error {
	result := r.db.WithContext(ctx).Save(payment)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *clinicPaymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	result := r.db.WithContext(ctx).Model(&model.ClinicPayment{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

func (r *clinicPaymentRepository) GetSummary(ctx context.Context, clinicID uuid.UUID, dateFrom, dateTo time.Time) (*model.FinancialSummaryResponse, error) {
	summary := &model.FinancialSummaryResponse{
		PaymentsByMethod: make(map[string]float64),
	}

	// Total received
	r.db.WithContext(ctx).Model(&model.ClinicPayment{}).
		Where("clinic_id = ? AND status IN (?) AND (COALESCE(payment_date, created_at) BETWEEN ? AND ?)", clinicID, []string{"RECEIVED", "CONFIRMED"}, dateFrom, dateTo).
		Select("COALESCE(SUM(net_value), 0)").Scan(&summary.TotalReceived)

	// Total pending
	r.db.WithContext(ctx).Model(&model.ClinicPayment{}).
		Where("clinic_id = ? AND status = ? AND (COALESCE(payment_date, created_at) BETWEEN ? AND ?)", clinicID, "PENDING", dateFrom, dateTo).
		Select("COALESCE(SUM(value), 0)").Scan(&summary.TotalPending)

	// Total fees
	r.db.WithContext(ctx).Model(&model.ClinicPayment{}).
		Where("clinic_id = ? AND status IN (?) AND (COALESCE(payment_date, created_at) BETWEEN ? AND ?)", clinicID, []string{"RECEIVED", "CONFIRMED"}, dateFrom, dateTo).
		Select("COALESCE(SUM(split_fee_value), 0)").Scan(&summary.TotalFees)

	summary.NetReceived = summary.TotalReceived - summary.TotalFees

	// Payment count
	r.db.WithContext(ctx).Model(&model.ClinicPayment{}).
		Where("clinic_id = ? AND (COALESCE(payment_date, created_at) BETWEEN ? AND ?)", clinicID, dateFrom, dateTo).
		Count(&summary.PaymentCount)

	// Payments by method
	type methodSum struct {
		BillingType string
		Total       float64
	}
	var methods []methodSum
	r.db.WithContext(ctx).Model(&model.ClinicPayment{}).
		Where("clinic_id = ? AND status IN (?) AND (COALESCE(payment_date, created_at) BETWEEN ? AND ?)", clinicID, []string{"RECEIVED", "CONFIRMED"}, dateFrom, dateTo).
		Select("billing_type, COALESCE(SUM(net_value), 0) as total").
		Group("billing_type").Scan(&methods)

	for _, m := range methods {
		summary.PaymentsByMethod[m.BillingType] = m.Total
	}

	return summary, nil
}
