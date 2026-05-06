package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	auditModel "github.com/aseno/backend/pkg/audit/model"
	paymentmethodModel "github.com/aseno/backend/pkg/paymentmethod/model"
	"github.com/aseno/backend/pkg/salespackage/model"
	"github.com/aseno/backend/pkg/salespackage/repository"
	"github.com/aseno/backend/pkg/salespackage/service"
	systemlogModel "github.com/aseno/backend/pkg/systemlog/model"
)

// --- Mocks -----------------------------------------------------------------

type mockRepo struct{ mock.Mock }

func (m *mockRepo) Create(ctx context.Context, pkg *model.SalesPackage) error {
	if pkg.ID == uuid.Nil {
		pkg.ID = uuid.New()
	}
	for i := range pkg.Items {
		if pkg.Items[i].ID == uuid.Nil {
			pkg.Items[i].ID = uuid.New()
		}
		pkg.Items[i].SalesPackageID = pkg.ID
	}
	for i := range pkg.Payments {
		if pkg.Payments[i].ID == uuid.Nil {
			pkg.Payments[i].ID = uuid.New()
		}
		pkg.Payments[i].SalesPackageID = pkg.ID
	}
	return m.Called(ctx, pkg).Error(0)
}
func (m *mockRepo) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SalesPackage, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.SalesPackage), args.Error(1)
}
func (m *mockRepo) Update(ctx context.Context, pkg *model.SalesPackage) error {
	return m.Called(ctx, pkg).Error(0)
}
func (m *mockRepo) UpdateStatus(ctx context.Context, id, clinicID uuid.UUID, status model.SalesPackageStatus) error {
	return m.Called(ctx, id, clinicID, status).Error(0)
}
func (m *mockRepo) List(ctx context.Context, clinicID uuid.UUID, filter model.SalesPackageFilter) ([]model.SalesPackage, int64, error) {
	args := m.Called(ctx, clinicID, filter)
	return args.Get(0).([]model.SalesPackage), args.Get(1).(int64), args.Error(2)
}
func (m *mockRepo) ListByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]model.SalesPackage, error) {
	args := m.Called(ctx, clinicID, patientID)
	return args.Get(0).([]model.SalesPackage), args.Error(1)
}
func (m *mockRepo) GetBalancesByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]model.ProcedureBalance, error) {
	args := m.Called(ctx, clinicID, patientID)
	return args.Get(0).([]model.ProcedureBalance), args.Error(1)
}
func (m *mockRepo) GetItemUsage(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).(map[uuid.UUID]int), args.Error(1)
}
func (m *mockRepo) PickFIFOItem(ctx context.Context, clinicID, patientID, procedureID uuid.UUID) (*model.SalesPackageItem, error) {
	args := m.Called(ctx, clinicID, patientID, procedureID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.SalesPackageItem), args.Error(1)
}
func (m *mockRepo) NextCodeForYear(ctx context.Context, clinicID uuid.UUID, year int) (int, error) {
	args := m.Called(ctx, clinicID, year)
	return args.Int(0), args.Error(1)
}
func (m *mockRepo) CreatePayments(ctx context.Context, payments []model.SalesPackagePayment) error {
	return m.Called(ctx, payments).Error(0)
}
func (m *mockRepo) GetPaymentByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SalesPackagePayment, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.SalesPackagePayment), args.Error(1)
}
func (m *mockRepo) UpdatePayment(ctx context.Context, p *model.SalesPackagePayment) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockRepo) ListPaymentsByPackage(ctx context.Context, packageID, clinicID uuid.UUID) ([]model.SalesPackagePayment, error) {
	args := m.Called(ctx, packageID, clinicID)
	return args.Get(0).([]model.SalesPackagePayment), args.Error(1)
}
func (m *mockRepo) DeletePaymentsByPackage(ctx context.Context, packageID, clinicID uuid.UUID) error {
	return m.Called(ctx, packageID, clinicID).Error(0)
}
func (m *mockRepo) WithTx(ctx context.Context, fn func(repository.SalesPackageRepository) error) error {
	// Run inline against the same mock (fine for unit tests).
	return fn(m)
}

type mockPaymentMethod struct{ mock.Mock }

func (m *mockPaymentMethod) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*paymentmethodModel.PaymentMethod, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*paymentmethodModel.PaymentMethod), args.Error(1)
}

type mockFinance struct{ mock.Mock }

func (m *mockFinance) UpsertFromSalesPackagePayment(ctx context.Context, payment *model.SalesPackagePayment, pkg *model.SalesPackage, patientName string) error {
	return m.Called(ctx, payment, pkg, patientName).Error(0)
}
func (m *mockFinance) DeleteFromSalesPackagePayment(ctx context.Context, paymentID, clinicID uuid.UUID) error {
	return m.Called(ctx, paymentID, clinicID).Error(0)
}

type mockPatient struct{ mock.Mock }

func (m *mockPatient) GetName(ctx context.Context, patientID, clinicID uuid.UUID) (string, error) {
	args := m.Called(ctx, patientID, clinicID)
	return args.String(0), args.Error(1)
}

type mockAudit struct{ mock.Mock }

func (m *mockAudit) Log(ctx context.Context, clinicID, userID uuid.UUID, action auditModel.AuditAction, entityID string, oldData, newData interface{}) {
	m.Called(ctx, clinicID, userID, action, entityID, oldData, newData)
}

type mockSyslog struct{ mock.Mock }

func (m *mockSyslog) Log(ctx context.Context, clinicID *uuid.UUID, event string, status systemlogModel.LogStatus, payload interface{}, errStr string) {
	m.Called(ctx, clinicID, event, status, payload, errStr)
}

// --- Helpers ---------------------------------------------------------------

type deps struct {
	repo    *mockRepo
	pm      *mockPaymentMethod
	fin     *mockFinance
	patient *mockPatient
	audit   *mockAudit
	syslog  *mockSyslog
	svc     service.SalesPackageService
}

func newDeps() *deps {
	d := &deps{
		repo:    &mockRepo{},
		pm:      &mockPaymentMethod{},
		fin:     &mockFinance{},
		patient: &mockPatient{},
		audit:   &mockAudit{},
		syslog:  &mockSyslog{},
	}
	d.svc = service.NewSalesPackageService(d.repo, d.pm, d.fin, d.patient, d.audit, d.syslog)
	// Permissive defaults — most tests don't care about the exact audit/syslog call shape.
	d.audit.On("Log", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	d.syslog.On("Log", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	d.patient.On("GetName", mock.Anything, mock.Anything, mock.Anything).Return("Maria Silva", nil).Maybe()
	return d
}

// pixMethod returns a 0-day, no-installment payment method.
func pixMethod(id uuid.UUID, clinicID uuid.UUID) *paymentmethodModel.PaymentMethod {
	return &paymentmethodModel.PaymentMethod{
		ID:                  id,
		ClinicID:            clinicID,
		Name:                "PIX",
		Type:                paymentmethodModel.PaymentMethodTypePix,
		Active:              true,
		SettlementDays:      0,
		InstallmentsAllowed: false,
		MaxInstallments:     1,
	}
}

// creditCardMethod returns a 30-day, 12x credit card.
func creditCardMethod(id uuid.UUID, clinicID uuid.UUID) *paymentmethodModel.PaymentMethod {
	return &paymentmethodModel.PaymentMethod{
		ID:                  id,
		ClinicID:            clinicID,
		Name:                "Cartão",
		Type:                paymentmethodModel.PaymentMethodTypeCredit,
		Active:              true,
		SettlementDays:      30,
		InstallmentsAllowed: true,
		MaxInstallments:     12,
	}
}

// --- Tests -----------------------------------------------------------------

func TestCreate_ActiveSale_Success(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	userID := uuid.New()
	patientID := uuid.New()
	procedureID := uuid.New()
	pmID := uuid.New()

	d.pm.On("GetByID", ctx, pmID, clinicID).Return(pixMethod(pmID, clinicID), nil)
	d.repo.On("NextCodeForYear", ctx, clinicID, mock.AnythingOfType("int")).Return(1, nil)
	d.repo.On("Create", ctx, mock.AnythingOfType("*model.SalesPackage")).Return(nil)
	// fetchResponse → GetByID after Create.
	d.repo.On("GetByID", ctx, mock.Anything, clinicID).Return(&model.SalesPackage{
		ID:        uuid.New(),
		ClinicID:  clinicID,
		PatientID: patientID,
		Code:      "V-2026-001",
		Status:    model.SalesPackageStatusActive,
		SaleDate:  time.Now().Truncate(24 * time.Hour),
		Subtotal:  500,
		Total:     500,
		Items: []model.SalesPackageItem{{
			ID:          uuid.New(),
			ProcedureID: procedureID,
			UnitPrice:   100,
			Quantity:    5,
			LineTotal:   500,
		}},
	}, nil)
	d.repo.On("GetItemUsage", ctx, mock.AnythingOfType("[]uuid.UUID")).Return(map[uuid.UUID]int{}, nil)
	d.fin.On("UpsertFromSalesPackagePayment", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	resp, err := d.svc.Create(ctx, clinicID, userID, &model.CreateSalesPackageRequest{
		PatientID: patientID,
		Status:    model.SalesPackageStatusActive,
		Items: []model.CreateSalesPackageItemInput{
			{ProcedureID: procedureID, UnitPrice: 100, Quantity: 5},
		},
		Payments: []model.CreateSalesPackagePaymentInput{
			{PaymentMethodID: pmID, Amount: 500},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, model.SalesPackageStatusActive, resp.Status)
	d.fin.AssertCalled(t, "UpsertFromSalesPackagePayment", ctx, mock.Anything, mock.Anything, mock.Anything)
}

func TestCreate_Quote_NoFinanceCall(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	userID := uuid.New()
	patientID := uuid.New()
	procedureID := uuid.New()

	d.repo.On("NextCodeForYear", ctx, clinicID, mock.AnythingOfType("int")).Return(7, nil)
	d.repo.On("Create", ctx, mock.AnythingOfType("*model.SalesPackage")).Return(nil)
	d.repo.On("GetByID", ctx, mock.Anything, clinicID).Return(&model.SalesPackage{
		ID:        uuid.New(),
		ClinicID:  clinicID,
		PatientID: patientID,
		Code:      "V-2026-007",
		Status:    model.SalesPackageStatusQuote,
	}, nil)
	d.repo.On("GetItemUsage", ctx, mock.Anything).Return(map[uuid.UUID]int{}, nil)

	resp, err := d.svc.Create(ctx, clinicID, userID, &model.CreateSalesPackageRequest{
		PatientID: patientID,
		Status:    model.SalesPackageStatusQuote,
		Items: []model.CreateSalesPackageItemInput{
			{ProcedureID: procedureID, UnitPrice: 200, Quantity: 3},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	d.fin.AssertNotCalled(t, "UpsertFromSalesPackagePayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// An active sale with no user-supplied payment becomes an open receivable —
// the service auto-generates a method-less SalesPackagePayment row covering
// the full total, and the finance hook receives that row with PaymentMethodID == nil.
func TestCreate_NoPayments_BecomesOpenReceivable(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	userID := uuid.New()
	procedureID := uuid.New()

	d.repo.On("NextCodeForYear", ctx, clinicID, mock.AnythingOfType("int")).Return(1, nil)

	var capturedPayments []model.SalesPackagePayment
	d.repo.On("Create", ctx, mock.AnythingOfType("*model.SalesPackage")).
		Run(func(args mock.Arguments) {
			pkg := args.Get(1).(*model.SalesPackage)
			capturedPayments = pkg.Payments
		}).Return(nil)
	d.repo.On("GetByID", ctx, mock.Anything, clinicID).Return(&model.SalesPackage{
		ID:       uuid.New(),
		ClinicID: clinicID,
		Status:   model.SalesPackageStatusActive,
	}, nil)
	d.repo.On("GetItemUsage", ctx, mock.Anything).Return(map[uuid.UUID]int{}, nil)
	d.fin.On("UpsertFromSalesPackagePayment", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	_, err := d.svc.Create(ctx, clinicID, userID, &model.CreateSalesPackageRequest{
		PatientID: uuid.New(),
		Status:    model.SalesPackageStatusActive,
		Items: []model.CreateSalesPackageItemInput{
			{ProcedureID: procedureID, UnitPrice: 100, Quantity: 5}, // total = 500
		},
		// no payments
	})

	require.NoError(t, err)
	require.Len(t, capturedPayments, 1, "should auto-generate one open receivable row")
	assert.Equal(t, 500.0, capturedPayments[0].Amount)
	assert.Nil(t, capturedPayments[0].PaymentMethodID, "open receivable has no method")
	assert.Nil(t, capturedPayments[0].PaidAt, "open receivable is not paid")
}

// Sum of payments above total is rejected.
func TestCreate_PaymentSumExceedsTotal_Rejected(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	pmID := uuid.New()

	d.pm.On("GetByID", ctx, pmID, clinicID).Return(pixMethod(pmID, clinicID), nil).Maybe()

	resp, err := d.svc.Create(ctx, clinicID, uuid.New(), &model.CreateSalesPackageRequest{
		PatientID: uuid.New(),
		Status:    model.SalesPackageStatusActive,
		Items: []model.CreateSalesPackageItemInput{
			{ProcedureID: uuid.New(), UnitPrice: 100, Quantity: 5}, // total 500
		},
		Payments: []model.CreateSalesPackagePaymentInput{
			{PaymentMethodID: pmID, Amount: 600}, // sum > total
		},
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maior que o total")
}

func TestCreate_DiscountGreaterThanSubtotal_Rejected(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()

	resp, err := d.svc.Create(ctx, clinicID, uuid.New(), &model.CreateSalesPackageRequest{
		PatientID: uuid.New(),
		Status:    model.SalesPackageStatusQuote,
		Discount:  9999,
		Items: []model.CreateSalesPackageItemInput{
			{ProcedureID: uuid.New(), UnitPrice: 100, Quantity: 1},
		},
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Desconto")
}

// Partial payment is now allowed: the unpaid remainder turns into an
// open-receivable row added by expandPayments. The sale persists with both
// rows (the user-supplied one + the auto-generated remainder).
func TestCreate_PartialPayment_AddsOpenReceivableRemainder(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	pmID := uuid.New()

	d.pm.On("GetByID", ctx, pmID, clinicID).Return(pixMethod(pmID, clinicID), nil)
	d.repo.On("NextCodeForYear", ctx, clinicID, mock.AnythingOfType("int")).Return(1, nil)

	var capturedPayments []model.SalesPackagePayment
	d.repo.On("Create", ctx, mock.AnythingOfType("*model.SalesPackage")).
		Run(func(args mock.Arguments) {
			pkg := args.Get(1).(*model.SalesPackage)
			capturedPayments = pkg.Payments
		}).Return(nil)
	d.repo.On("GetByID", ctx, mock.Anything, clinicID).Return(&model.SalesPackage{
		ID:       uuid.New(),
		ClinicID: clinicID,
		Status:   model.SalesPackageStatusActive,
	}, nil)
	d.repo.On("GetItemUsage", ctx, mock.Anything).Return(map[uuid.UUID]int{}, nil)
	d.fin.On("UpsertFromSalesPackagePayment", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	_, err := d.svc.Create(ctx, clinicID, uuid.New(), &model.CreateSalesPackageRequest{
		PatientID: uuid.New(),
		Status:    model.SalesPackageStatusActive,
		Items: []model.CreateSalesPackageItemInput{
			{ProcedureID: uuid.New(), UnitPrice: 100, Quantity: 5}, // total 500
		},
		Payments: []model.CreateSalesPackagePaymentInput{
			{PaymentMethodID: pmID, Amount: 300},
		},
	})

	require.NoError(t, err)
	require.Len(t, capturedPayments, 2, "1 user payment + 1 auto-generated remainder")
	assert.NotNil(t, capturedPayments[0].PaymentMethodID, "user-supplied row keeps its method")
	assert.Equal(t, 300.0, capturedPayments[0].Amount)
	assert.Nil(t, capturedPayments[1].PaymentMethodID, "remainder is method-less")
	assert.Equal(t, 200.0, capturedPayments[1].Amount)
}

func TestCreate_CreditCard3x_GeneratesThreeRowsWithDifferentDueDates(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	pmID := uuid.New()
	procedureID := uuid.New()

	d.pm.On("GetByID", ctx, pmID, clinicID).Return(creditCardMethod(pmID, clinicID), nil)
	d.repo.On("NextCodeForYear", ctx, clinicID, mock.AnythingOfType("int")).Return(1, nil)

	var capturedPayments []model.SalesPackagePayment
	d.repo.On("Create", ctx, mock.AnythingOfType("*model.SalesPackage")).
		Run(func(args mock.Arguments) {
			pkg := args.Get(1).(*model.SalesPackage)
			capturedPayments = pkg.Payments
		}).Return(nil)
	d.repo.On("GetByID", ctx, mock.Anything, clinicID).Return(&model.SalesPackage{
		ID:       uuid.New(),
		ClinicID: clinicID,
		Status:   model.SalesPackageStatusActive,
	}, nil)
	d.repo.On("GetItemUsage", ctx, mock.Anything).Return(map[uuid.UUID]int{}, nil)
	d.fin.On("UpsertFromSalesPackagePayment", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	saleDate := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	_, err := d.svc.Create(ctx, clinicID, uuid.New(), &model.CreateSalesPackageRequest{
		PatientID: uuid.New(),
		Status:    model.SalesPackageStatusActive,
		SaleDate:  &saleDate,
		Items: []model.CreateSalesPackageItemInput{
			{ProcedureID: procedureID, UnitPrice: 300, Quantity: 1}, // 300
		},
		Payments: []model.CreateSalesPackagePaymentInput{
			{PaymentMethodID: pmID, Amount: 300, Installments: 3},
		},
	})

	require.NoError(t, err)
	require.Len(t, capturedPayments, 3, "should expand 3x credit card into 3 rows")
	for i, p := range capturedPayments {
		assert.Equal(t, i+1, p.Installment, "installment number")
		assert.Equal(t, 3, p.TotalInstallments)
		assert.Nil(t, p.PaidAt, "installments shouldn't be marked paid on creation")
	}
	// Due dates should grow: 30, 60, 90 days from sale date.
	assert.Equal(t, saleDate.AddDate(0, 0, 30), capturedPayments[0].DueDate)
	assert.Equal(t, saleDate.AddDate(0, 0, 60), capturedPayments[1].DueDate)
	assert.Equal(t, saleDate.AddDate(0, 0, 90), capturedPayments[2].DueDate)

	// Equal split with rounding correction on last installment.
	sum := capturedPayments[0].Amount + capturedPayments[1].Amount + capturedPayments[2].Amount
	assert.InDelta(t, 300.0, sum, 0.01)
}

func TestCreate_InstallmentsOnPixMethod_Rejected(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	pmID := uuid.New()

	d.pm.On("GetByID", ctx, pmID, clinicID).Return(pixMethod(pmID, clinicID), nil)

	resp, err := d.svc.Create(ctx, clinicID, uuid.New(), &model.CreateSalesPackageRequest{
		PatientID: uuid.New(),
		Status:    model.SalesPackageStatusActive,
		Items: []model.CreateSalesPackageItemInput{
			{ProcedureID: uuid.New(), UnitPrice: 100, Quantity: 5},
		},
		Payments: []model.CreateSalesPackagePaymentInput{
			{PaymentMethodID: pmID, Amount: 500, Installments: 3},
		},
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parcelamento")
}

func TestCreate_ValidUntilBeforeSaleDate_Rejected(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()

	saleDate := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	validUntil := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	resp, err := d.svc.Create(ctx, clinicID, uuid.New(), &model.CreateSalesPackageRequest{
		PatientID:  uuid.New(),
		Status:     model.SalesPackageStatusQuote,
		SaleDate:   &saleDate,
		ValidUntil: &validUntil,
		Items: []model.CreateSalesPackageItemInput{
			{ProcedureID: uuid.New(), UnitPrice: 100, Quantity: 1},
		},
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validade")
}

func TestConvertQuoteToActive_GeneratesPaymentsAndCallsFinance(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	userID := uuid.New()
	pmID := uuid.New()
	pkgID := uuid.New()

	d.pm.On("GetByID", ctx, pmID, clinicID).Return(pixMethod(pmID, clinicID), nil)
	d.repo.On("GetByID", ctx, pkgID, clinicID).Return(&model.SalesPackage{
		ID:        pkgID,
		ClinicID:  clinicID,
		PatientID: uuid.New(),
		Status:    model.SalesPackageStatusQuote,
		SaleDate:  time.Now().Truncate(24 * time.Hour),
		Subtotal:  300,
		Total:     300,
	}, nil).Once()
	d.repo.On("UpdateStatus", ctx, pkgID, clinicID, model.SalesPackageStatusActive).Return(nil)
	d.repo.On("CreatePayments", ctx, mock.AnythingOfType("[]model.SalesPackagePayment")).Return(nil)
	d.repo.On("GetByID", ctx, pkgID, clinicID).Return(&model.SalesPackage{
		ID:       pkgID,
		ClinicID: clinicID,
		Status:   model.SalesPackageStatusActive,
	}, nil)
	d.repo.On("GetItemUsage", ctx, mock.Anything).Return(map[uuid.UUID]int{}, nil)
	d.fin.On("UpsertFromSalesPackagePayment", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	resp, err := d.svc.ConvertQuoteToActive(ctx, pkgID, clinicID, userID, []model.CreateSalesPackagePaymentInput{
		{PaymentMethodID: pmID, Amount: 300},
	})

	require.NoError(t, err)
	assert.Equal(t, model.SalesPackageStatusActive, resp.Status)
	d.fin.AssertCalled(t, "UpsertFromSalesPackagePayment", ctx, mock.Anything, mock.Anything, mock.Anything)
}

func TestConvertQuoteToActive_OnNonQuote_Rejected(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	pkgID := uuid.New()

	d.repo.On("GetByID", ctx, pkgID, clinicID).Return(&model.SalesPackage{
		ID:       pkgID,
		ClinicID: clinicID,
		Status:   model.SalesPackageStatusActive, // already active
	}, nil)

	resp, err := d.svc.ConvertQuoteToActive(ctx, pkgID, clinicID, uuid.New(), nil)
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "orçamentos")
}

func TestCancel_ChangesStatusButLeavesFinanceUntouched(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	pkgID := uuid.New()

	d.repo.On("GetByID", ctx, pkgID, clinicID).Return(&model.SalesPackage{
		ID:       pkgID,
		ClinicID: clinicID,
		Status:   model.SalesPackageStatusActive,
	}, nil).Once()
	d.repo.On("UpdateStatus", ctx, pkgID, clinicID, model.SalesPackageStatusCanceled).Return(nil)
	d.repo.On("GetByID", ctx, pkgID, clinicID).Return(&model.SalesPackage{
		ID:       pkgID,
		ClinicID: clinicID,
		Status:   model.SalesPackageStatusCanceled,
	}, nil)
	d.repo.On("GetItemUsage", ctx, mock.Anything).Return(map[uuid.UUID]int{}, nil)

	resp, err := d.svc.Cancel(ctx, pkgID, clinicID, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, model.SalesPackageStatusCanceled, resp.Status)

	// Finance is NOT called: refund is a manual reconciliation per spec.
	d.fin.AssertNotCalled(t, "DeleteFromSalesPackagePayment", mock.Anything, mock.Anything, mock.Anything)
}

func TestGetByID_WrongClinic_NotFound(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	pkgID := uuid.New()
	otherClinic := uuid.New()

	d.repo.On("GetByID", ctx, pkgID, otherClinic).Return(nil, gorm.ErrRecordNotFound)

	resp, err := d.svc.GetByID(ctx, pkgID, otherClinic)
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrada")
}

func TestPickFIFOItemID_NoBalance_ReturnsNil(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	patientID := uuid.New()
	procedureID := uuid.New()

	d.repo.On("PickFIFOItem", ctx, clinicID, patientID, procedureID).Return(nil, nil)

	id, err := d.svc.PickFIFOItemID(ctx, clinicID, patientID, procedureID)
	require.NoError(t, err)
	assert.Nil(t, id)
}

func TestPickFIFOItemID_HasBalance_ReturnsItemID(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	patientID := uuid.New()
	procedureID := uuid.New()
	itemID := uuid.New()

	d.repo.On("PickFIFOItem", ctx, clinicID, patientID, procedureID).Return(&model.SalesPackageItem{
		ID:          itemID,
		ProcedureID: procedureID,
	}, nil)

	id, err := d.svc.PickFIFOItemID(ctx, clinicID, patientID, procedureID)
	require.NoError(t, err)
	require.NotNil(t, id)
	assert.Equal(t, itemID, *id)
}

func TestRegisterPayment_SetsPaidAtAndCallsFinance(t *testing.T) {
	d := newDeps()
	ctx := context.Background()
	clinicID := uuid.New()
	pkgID := uuid.New()
	paymentID := uuid.New()

	d.repo.On("GetPaymentByID", ctx, paymentID, clinicID).Return(&model.SalesPackagePayment{
		ID:             paymentID,
		ClinicID:       clinicID,
		SalesPackageID: pkgID,
		Amount:         100,
	}, nil)
	d.repo.On("GetByID", ctx, pkgID, clinicID).Return(&model.SalesPackage{
		ID:        pkgID,
		ClinicID:  clinicID,
		PatientID: uuid.New(),
		Status:    model.SalesPackageStatusActive,
	}, nil)
	d.repo.On("UpdatePayment", ctx, mock.Anything).Return(nil)
	d.fin.On("UpsertFromSalesPackagePayment", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	paidAt := time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC)
	resp, err := d.svc.RegisterPayment(ctx, paymentID, clinicID, uuid.New(), &model.RegisterPaymentRequest{
		PaidAt: paidAt,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.PaidAt)
	d.fin.AssertCalled(t, "UpsertFromSalesPackagePayment", ctx, mock.Anything, mock.Anything, mock.Anything)
}
