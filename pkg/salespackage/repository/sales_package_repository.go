package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/salespackage/model"
)

// SalesPackageRepository owns persistence for the sale, its items, and its
// payments. All queries are tenant-scoped by clinic_id.
type SalesPackageRepository interface {
	// CRUD
	Create(ctx context.Context, pkg *model.SalesPackage) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SalesPackage, error)
	Update(ctx context.Context, pkg *model.SalesPackage) error
	UpdateStatus(ctx context.Context, id, clinicID uuid.UUID, status model.SalesPackageStatus) error
	List(ctx context.Context, clinicID uuid.UUID, filter model.SalesPackageFilter) ([]model.SalesPackage, int64, error)

	// Patient view + consumption
	ListByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]model.SalesPackage, error)
	GetBalancesByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]model.ProcedureBalance, error)
	GetItemUsage(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]int, error)

	// FIFO: pick the oldest active item with remaining balance for (patient, procedure).
	// Returns nil, nil when nothing is available (caller treats as ad-hoc).
	PickFIFOItem(ctx context.Context, clinicID, patientID, procedureID uuid.UUID) (*model.SalesPackageItem, error)

	// Code generator
	NextCodeForYear(ctx context.Context, clinicID uuid.UUID, year int) (int, error)
	// AcquireCodeLock takes a postgres advisory lock keyed by (clinic, year)
	// so concurrent Create calls serialize when picking the next sale code.
	// The lock is transaction-scoped — auto-released at commit/rollback.
	// Must be called inside WithTx.
	AcquireCodeLock(ctx context.Context, clinicID uuid.UUID, year int) error

	// Payments
	CreatePayments(ctx context.Context, payments []model.SalesPackagePayment) error
	GetPaymentByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SalesPackagePayment, error)
	UpdatePayment(ctx context.Context, payment *model.SalesPackagePayment) error
	ListPaymentsByPackage(ctx context.Context, packageID, clinicID uuid.UUID) ([]model.SalesPackagePayment, error)
	DeletePayment(ctx context.Context, id, clinicID uuid.UUID) error
	DeletePaymentsByPackage(ctx context.Context, packageID, clinicID uuid.UUID) error

	// Transaction wrapper used by the service layer to keep create-with-items-and-payments atomic.
	WithTx(ctx context.Context, fn func(txRepo SalesPackageRepository) error) error
}

type salesPackageRepository struct {
	db *gorm.DB
}

func NewSalesPackageRepository(db *gorm.DB) SalesPackageRepository {
	return &salesPackageRepository{db: db}
}

// --- CRUD -----------------------------------------------------------------

func (r *salesPackageRepository) Create(ctx context.Context, pkg *model.SalesPackage) error {
	// GORM cascades the Items + Payments relations because they're declared on
	// the parent struct. ClinicID is propagated to payments by the service before
	// calling Create.
	if err := r.db.WithContext(ctx).Create(pkg).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *salesPackageRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SalesPackage, error) {
	var pkg model.SalesPackage
	err := r.db.WithContext(ctx).
		Preload("Patient").
		Preload("Items.Procedure").
		Preload("Payments", func(db *gorm.DB) *gorm.DB {
			return db.Order("installment ASC, due_date ASC")
		}).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		First(&pkg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &pkg, nil
}

func (r *salesPackageRepository) Update(ctx context.Context, pkg *model.SalesPackage) error {
	res := r.db.WithContext(ctx).
		Model(&model.SalesPackage{}).
		Where("id = ? AND clinic_id = ?", pkg.ID, pkg.ClinicID).
		Updates(map[string]interface{}{
			"sale_date":   pkg.SaleDate,
			"valid_until": pkg.ValidUntil,
			"discount":    pkg.Discount,
			"total":       pkg.Total,
			"notes":       pkg.Notes,
		})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *salesPackageRepository) UpdateStatus(ctx context.Context, id, clinicID uuid.UUID, status model.SalesPackageStatus) error {
	res := r.db.WithContext(ctx).
		Model(&model.SalesPackage{}).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Update("status", status)
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *salesPackageRepository) List(ctx context.Context, clinicID uuid.UUID, filter model.SalesPackageFilter) ([]model.SalesPackage, int64, error) {
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	q := r.db.WithContext(ctx).
		Model(&model.SalesPackage{}).
		Where("sales_packages.clinic_id = ?", clinicID)

	if filter.Status != nil {
		q = q.Where("sales_packages.status = ?", *filter.Status)
	}
	if filter.PatientID != nil {
		q = q.Where("sales_packages.patient_id = ?", *filter.PatientID)
	}
	if filter.From != nil {
		q = q.Where("sales_packages.sale_date >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("sales_packages.sale_date <= ?", *filter.To)
	}
	if strings.TrimSpace(filter.Query) != "" {
		// Search across patient name/phone, package code, and any item's procedure name.
		// LEFT JOIN avoids dropping packages whose items haven't been preloaded yet.
		like := "%" + strings.ToLower(strings.TrimSpace(filter.Query)) + "%"
		q = q.Joins("LEFT JOIN patients pt ON pt.id = sales_packages.patient_id").
			Joins("LEFT JOIN sales_package_items spi ON spi.sales_package_id = sales_packages.id").
			Joins("LEFT JOIN procedures pr ON pr.id = spi.procedure_id").
			Where(`
				LOWER(pt.name) LIKE ? OR
				pt.phone LIKE ? OR
				LOWER(sales_packages.code) LIKE ? OR
				LOWER(pr.name) LIKE ?
			`, like, "%"+strings.TrimSpace(filter.Query)+"%", like, like).
			Distinct("sales_packages.id")
	}

	var total int64
	countQ := q.Session(&gorm.Session{})
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	var packages []model.SalesPackage
	err := q.
		Preload("Patient").
		Preload("Items.Procedure").
		Preload("Payments").
		Order("sales_packages.sale_date DESC, sales_packages.created_at DESC").
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(&packages).Error
	if err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}
	return packages, total, nil
}

// --- Patient view + consumption -------------------------------------------

func (r *salesPackageRepository) ListByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]model.SalesPackage, error) {
	var packages []model.SalesPackage
	err := r.db.WithContext(ctx).
		Preload("Patient").
		Preload("Items.Procedure").
		Preload("Payments").
		Where("clinic_id = ? AND patient_id = ?", clinicID, patientID).
		Order("sale_date DESC, created_at DESC").
		Find(&packages).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return packages, nil
}

// GetBalancesByPatient returns one row per procedure aggregating quantity and
// usage across all of the patient's ACTIVE packages. Expired packages
// (valid_until < today) and quote/canceled/completed are excluded — they
// don't contribute to bookable balance.
func (r *salesPackageRepository) GetBalancesByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]model.ProcedureBalance, error) {
	type row struct {
		ProcedureID   uuid.UUID
		ProcedureName string
		QuantityTotal int
		UsedTotal     int
	}

	today := time.Now().Format("2006-01-02")

	var rows []row
	err := r.db.WithContext(ctx).
		Raw(`
			SELECT
				spi.procedure_id        AS procedure_id,
				pr.name                 AS procedure_name,
				SUM(spi.quantity)       AS quantity_total,
				COALESCE(SUM(used.cnt), 0) AS used_total
			FROM sales_package_items spi
			JOIN sales_packages sp ON sp.id = spi.sales_package_id
			JOIN procedures pr ON pr.id = spi.procedure_id
			LEFT JOIN LATERAL (
				SELECT COUNT(*)::int AS cnt
				FROM appointment_procedures ap
				JOIN appointments a ON a.id = ap.appointment_id
				WHERE ap.sales_package_item_id = spi.id
				  AND a.status <> 'canceled'
				  AND a.deleted_at IS NULL
			) used ON TRUE
			WHERE sp.clinic_id = ?
			  AND sp.patient_id = ?
			  AND sp.status = 'active'
			  AND sp.deleted_at IS NULL
			  AND (sp.valid_until IS NULL OR sp.valid_until >= ?::date)
			GROUP BY spi.procedure_id, pr.name
			ORDER BY pr.name ASC
		`, clinicID, patientID, today).
		Scan(&rows).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	out := make([]model.ProcedureBalance, len(rows))
	for i, r := range rows {
		out[i] = model.ProcedureBalance{
			ProcedureID:   r.ProcedureID,
			ProcedureName: r.ProcedureName,
			UsedTotal:     r.UsedTotal,
			QuantityTotal: r.QuantityTotal,
		}
	}
	return out, nil
}

// GetItemUsage returns count of non-canceled appointment_procedures per item id.
func (r *salesPackageRepository) GetItemUsage(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	out := make(map[uuid.UUID]int, len(itemIDs))
	if len(itemIDs) == 0 {
		return out, nil
	}

	type row struct {
		SalesPackageItemID uuid.UUID
		Cnt                int
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Raw(`
			SELECT ap.sales_package_item_id, COUNT(*)::int AS cnt
			FROM appointment_procedures ap
			JOIN appointments a ON a.id = ap.appointment_id
			WHERE ap.sales_package_item_id IN ?
			  AND a.status <> 'canceled'
			  AND a.deleted_at IS NULL
			GROUP BY ap.sales_package_item_id
		`, itemIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	for _, r := range rows {
		out[r.SalesPackageItemID] = r.Cnt
	}
	for _, id := range itemIDs {
		if _, ok := out[id]; !ok {
			out[id] = 0
		}
	}
	return out, nil
}

// PickFIFOItem returns the oldest active item with remaining balance for
// (patient, procedure). nil, nil means no balance — caller falls back to ad-hoc.
func (r *salesPackageRepository) PickFIFOItem(ctx context.Context, clinicID, patientID, procedureID uuid.UUID) (*model.SalesPackageItem, error) {
	today := time.Now().Format("2006-01-02")

	var item model.SalesPackageItem
	err := r.db.WithContext(ctx).
		Raw(`
			SELECT spi.*
			FROM sales_package_items spi
			JOIN sales_packages sp ON sp.id = spi.sales_package_id
			WHERE sp.clinic_id = ?
			  AND sp.patient_id = ?
			  AND sp.status = 'active'
			  AND sp.deleted_at IS NULL
			  AND spi.procedure_id = ?
			  AND (sp.valid_until IS NULL OR sp.valid_until >= ?::date)
			  AND (
				SELECT COUNT(*) FROM appointment_procedures ap
				JOIN appointments a ON a.id = ap.appointment_id
				WHERE ap.sales_package_item_id = spi.id
				  AND a.status <> 'canceled'
				  AND a.deleted_at IS NULL
			  ) < spi.quantity
			ORDER BY sp.sale_date ASC, spi.created_at ASC
			LIMIT 1
		`, clinicID, patientID, procedureID, today).
		Scan(&item).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	if item.ID == uuid.Nil {
		return nil, nil
	}
	return &item, nil
}

// --- Code generator -------------------------------------------------------

// acquireCodeLockSQL uses the single-argument form of pg_advisory_xact_lock,
// which takes a bigint. The two-argument form requires (integer, integer),
// not (bigint, bigint) — passing two bigints (e.g. from hashtextextended +
// ?::bigint) raises "function pg_advisory_xact_lock(bigint, bigint) does not
// exist (SQLSTATE 42883)" at runtime. To keep keying by (clinic, year), we
// concatenate them into one text key and hash it to a single bigint.
const acquireCodeLockSQL = "SELECT pg_advisory_xact_lock(hashtextextended(? || ':' || ?::text, 0))"

// AcquireCodeLock takes a transaction-scoped postgres advisory lock keyed
// by (clinic, year). Concurrent Create calls for the same clinic+year wait
// here, so the MAX(code)+1 read can't race with another insert. Lock auto-
// releases at commit/rollback.
func (r *salesPackageRepository) AcquireCodeLock(ctx context.Context, clinicID uuid.UUID, year int) error {
	if err := r.db.WithContext(ctx).Exec(
		acquireCodeLockSQL,
		clinicID.String(), year,
	).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// NextCodeForYear returns the next sequential number for a clinic within a
// given year, based on existing codes that match V-{year}-NNN.
func (r *salesPackageRepository) NextCodeForYear(ctx context.Context, clinicID uuid.UUID, year int) (int, error) {
	prefix := fmt.Sprintf("V-%d-", year)
	var maxNum int
	err := r.db.WithContext(ctx).
		Raw(`
			SELECT COALESCE(MAX(NULLIF(REGEXP_REPLACE(SUBSTRING(code FROM ?), '[^0-9]', '', 'g'), '')::int), 0)
			FROM sales_packages
			WHERE clinic_id = ?
			  AND code LIKE ?
			  AND deleted_at IS NULL
		`, len(prefix)+1, clinicID, prefix+"%").
		Scan(&maxNum).Error
	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return maxNum + 1, nil
}

// --- Payments -------------------------------------------------------------

func (r *salesPackageRepository) CreatePayments(ctx context.Context, payments []model.SalesPackagePayment) error {
	if len(payments) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&payments).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

func (r *salesPackageRepository) GetPaymentByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SalesPackagePayment, error) {
	var p model.SalesPackagePayment
	err := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &p, nil
}

func (r *salesPackageRepository) UpdatePayment(ctx context.Context, payment *model.SalesPackagePayment) error {
	res := r.db.WithContext(ctx).
		Model(&model.SalesPackagePayment{}).
		Where("id = ? AND clinic_id = ?", payment.ID, payment.ClinicID).
		Updates(map[string]interface{}{
			"amount":            payment.Amount,
			"discount":          payment.Discount,
			"payment_method_id": payment.PaymentMethodID,
			"due_date":          payment.DueDate,
			"paid_at":           payment.PaidAt,
		})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *salesPackageRepository) ListPaymentsByPackage(ctx context.Context, packageID, clinicID uuid.UUID) ([]model.SalesPackagePayment, error) {
	var out []model.SalesPackagePayment
	err := r.db.WithContext(ctx).
		Where("sales_package_id = ? AND clinic_id = ?", packageID, clinicID).
		Order("installment ASC, due_date ASC").
		Find(&out).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return out, nil
}

// DeletePayment soft-deletes a single SalesPackagePayment row. Used by
// AddPayment when an open receivable is fully consumed by the new payment.
func (r *salesPackageRepository) DeletePayment(ctx context.Context, id, clinicID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.SalesPackagePayment{})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	return nil
}

func (r *salesPackageRepository) DeletePaymentsByPackage(ctx context.Context, packageID, clinicID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("sales_package_id = ? AND clinic_id = ?", packageID, clinicID).
		Delete(&model.SalesPackagePayment{})
	if res.Error != nil {
		return errors.ErrInternalInstance.WithError(res.Error)
	}
	return nil
}

// --- Transaction wrapper --------------------------------------------------

func (r *salesPackageRepository) WithTx(ctx context.Context, fn func(txRepo SalesPackageRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&salesPackageRepository{db: tx})
	})
}
