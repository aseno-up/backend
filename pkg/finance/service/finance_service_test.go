package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	appointmentModel "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/finance/model"
	"github.com/aseno/backend/pkg/finance/service"
)

// --- Mocks -----------------------------------------------------------------

type mockTransactionRepo struct{ mock.Mock }

func (m *mockTransactionRepo) Create(ctx context.Context, t *model.FinancialTransaction) error {
	return m.Called(ctx, t).Error(0)
}

func (m *mockTransactionRepo) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.FinancialTransaction, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.FinancialTransaction), args.Error(1)
}

func (m *mockTransactionRepo) Update(ctx context.Context, t *model.FinancialTransaction) error {
	return m.Called(ctx, t).Error(0)
}

func (m *mockTransactionRepo) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return m.Called(ctx, id, clinicID).Error(0)
}

func (m *mockTransactionRepo) List(ctx context.Context, clinicID uuid.UUID, filter model.TransactionFilter) ([]model.FinancialTransaction, error) {
	args := m.Called(ctx, clinicID, filter)
	return args.Get(0).([]model.FinancialTransaction), args.Error(1)
}

func (m *mockTransactionRepo) GetByAppointmentPaymentID(ctx context.Context, appointmentPaymentID, clinicID uuid.UUID) (*model.FinancialTransaction, error) {
	args := m.Called(ctx, appointmentPaymentID, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.FinancialTransaction), args.Error(1)
}

func (m *mockTransactionRepo) DeleteByAppointmentPaymentID(ctx context.Context, appointmentPaymentID, clinicID uuid.UUID) error {
	return m.Called(ctx, appointmentPaymentID, clinicID).Error(0)
}

func (m *mockTransactionRepo) GetBySessionPlanPaymentID(ctx context.Context, sessionPlanPaymentID, clinicID uuid.UUID) (*model.FinancialTransaction, error) {
	args := m.Called(ctx, sessionPlanPaymentID, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.FinancialTransaction), args.Error(1)
}

func (m *mockTransactionRepo) DeleteBySessionPlanPaymentID(ctx context.Context, sessionPlanPaymentID, clinicID uuid.UUID) error {
	return m.Called(ctx, sessionPlanPaymentID, clinicID).Error(0)
}

func (m *mockTransactionRepo) SumByPeriod(ctx context.Context, clinicID uuid.UUID, from, to time.Time) (income, expense float64, err error) {
	args := m.Called(ctx, clinicID, from, to)
	return args.Get(0).(float64), args.Get(1).(float64), args.Error(2)
}

type mockCategoryRepo struct{ mock.Mock }

func (m *mockCategoryRepo) Create(ctx context.Context, c *model.FinancialCategory) error {
	return m.Called(ctx, c).Error(0)
}

func (m *mockCategoryRepo) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.FinancialCategory, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.FinancialCategory), args.Error(1)
}

func (m *mockCategoryRepo) Update(ctx context.Context, c *model.FinancialCategory) error {
	return m.Called(ctx, c).Error(0)
}

func (m *mockCategoryRepo) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return m.Called(ctx, id, clinicID).Error(0)
}

func (m *mockCategoryRepo) ListByClinic(ctx context.Context, clinicID uuid.UUID) ([]model.FinancialCategory, error) {
	args := m.Called(ctx, clinicID)
	return args.Get(0).([]model.FinancialCategory), args.Error(1)
}

func (m *mockCategoryRepo) CountByClinic(ctx context.Context, clinicID uuid.UUID) (int64, error) {
	args := m.Called(ctx, clinicID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockCategoryRepo) CreateDefaults(ctx context.Context, clinicID uuid.UUID) error {
	return m.Called(ctx, clinicID).Error(0)
}

// --- Helpers ---------------------------------------------------------------

var (
	testClinicID = uuid.New()
	testUserID   = uuid.New()
	testAptID    = uuid.New()
)

func newTestService() (service.FinanceService, *mockTransactionRepo, *mockCategoryRepo) {
	tx := new(mockTransactionRepo)
	cat := new(mockCategoryRepo)
	svc := service.NewFinanceService(tx, cat)
	return svc, tx, cat
}

func ptrTime(t time.Time) *time.Time { return &t }

// --- CreateTransaction (manual) -------------------------------------------

func TestCreateTransaction_Income(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	req := &model.CreateTransactionRequest{
		Type:           model.TransactionTypeIncome,
		Amount:         250,
		Description:    "Venda avulsa",
		CompetenceDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		PaidAt:         ptrTime(time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)),
	}

	txRepo.On("Create", ctx, mock.MatchedBy(func(ftx *model.FinancialTransaction) bool {
		return ftx.ClinicID == testClinicID &&
			ftx.Type == model.TransactionTypeIncome &&
			ftx.Amount == 250 &&
			ftx.Description == "Venda avulsa" &&
			ftx.Source == model.SourceManual &&
			ftx.CreatedBy != nil && *ftx.CreatedBy == testUserID
	})).Return(nil)

	resp, err := svc.CreateTransaction(ctx, req, testClinicID, testUserID)

	assert.NoError(t, err)
	assert.Equal(t, model.TransactionTypeIncome, resp.Type)
	assert.Equal(t, model.SourceManual, resp.Source)
	txRepo.AssertExpectations(t)
}

func TestCreateTransaction_Expense(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	req := &model.CreateTransactionRequest{
		Type:           model.TransactionTypeExpense,
		Amount:         2500,
		Description:    "Aluguel abril",
		CompetenceDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}

	txRepo.On("Create", ctx, mock.MatchedBy(func(ftx *model.FinancialTransaction) bool {
		return ftx.Type == model.TransactionTypeExpense && ftx.Amount == 2500
	})).Return(nil)

	_, err := svc.CreateTransaction(ctx, req, testClinicID, testUserID)
	assert.NoError(t, err)
}

func TestCreateTransaction_AmountMustBePositive(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	req := &model.CreateTransactionRequest{
		Type:           model.TransactionTypeIncome,
		Amount:         -10,
		Description:    "negativo",
		CompetenceDate: time.Now(),
	}

	_, err := svc.CreateTransaction(ctx, req, testClinicID, testUserID)
	assert.Error(t, err, "amount <= 0 deve falhar no service mesmo se binding passar")
	txRepo.AssertNotCalled(t, "Create")
}

func TestCreateTransaction_RejectsWhenCategoryTypeMismatches(t *testing.T) {
	svc, txRepo, catRepo := newTestService()
	ctx := context.Background()

	catID := uuid.New()
	// Category is an expense category, but user is creating an income.
	catRepo.On("GetByID", ctx, catID, testClinicID).
		Return(&model.FinancialCategory{
			ID: catID, ClinicID: testClinicID, Type: model.TransactionTypeExpense,
		}, nil)

	req := &model.CreateTransactionRequest{
		Type:           model.TransactionTypeIncome,
		CategoryID:     &catID,
		Amount:         100,
		Description:    "teste",
		CompetenceDate: time.Now(),
	}

	_, err := svc.CreateTransaction(ctx, req, testClinicID, testUserID)
	assert.Error(t, err, "categoria de expense não pode ser usada em transação de income")
	txRepo.AssertNotCalled(t, "Create")
}

// --- UpdateTransaction ----------------------------------------------------

func TestUpdateTransaction_OnlyManual(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	txID := uuid.New()
	existing := &model.FinancialTransaction{
		ID:       txID,
		ClinicID: testClinicID,
		Source:   model.SourceAppointment, // originada de agendamento
		Amount:   100,
	}
	txRepo.On("GetByID", ctx, txID, testClinicID).Return(existing, nil)

	newAmount := 999.0
	_, err := svc.UpdateTransaction(ctx, txID, &model.UpdateTransactionRequest{
		Amount: &newAmount,
	}, testClinicID)

	assert.Error(t, err, "transações automáticas não podem ser editadas manualmente")
	txRepo.AssertNotCalled(t, "Update")
}

func TestUpdateTransaction_ManualHappy(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	txID := uuid.New()
	existing := &model.FinancialTransaction{
		ID:       txID,
		ClinicID: testClinicID,
		Source:   model.SourceManual,
		Amount:   100,
	}
	txRepo.On("GetByID", ctx, txID, testClinicID).Return(existing, nil)
	txRepo.On("Update", ctx, mock.MatchedBy(func(tx *model.FinancialTransaction) bool {
		return tx.ID == txID && tx.Amount == 250
	})).Return(nil)

	newAmount := 250.0
	resp, err := svc.UpdateTransaction(ctx, txID, &model.UpdateTransactionRequest{
		Amount: &newAmount,
	}, testClinicID)

	assert.NoError(t, err)
	assert.Equal(t, 250.0, resp.Amount)
}

// --- DeleteTransaction ----------------------------------------------------

func TestDeleteTransaction_BlockedForAutoSource(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	txID := uuid.New()
	existing := &model.FinancialTransaction{
		ID: txID, ClinicID: testClinicID, Source: model.SourceAppointment,
	}
	txRepo.On("GetByID", ctx, txID, testClinicID).Return(existing, nil)

	err := svc.DeleteTransaction(ctx, txID, testClinicID)
	assert.Error(t, err, "não pode apagar transação originada de agendamento")
	txRepo.AssertNotCalled(t, "Delete")
}

func TestDeleteTransaction_ManualOK(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	txID := uuid.New()
	existing := &model.FinancialTransaction{ID: txID, ClinicID: testClinicID, Source: model.SourceManual}
	txRepo.On("GetByID", ctx, txID, testClinicID).Return(existing, nil)
	txRepo.On("Delete", ctx, txID, testClinicID).Return(nil)

	err := svc.DeleteTransaction(ctx, txID, testClinicID)
	assert.NoError(t, err)
}

// --- UpsertFromAppointmentPayment (auto hook) -----------------------------

func TestUpsertFromAppointmentPayment_CreatesWhenNoExisting(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	apt := &appointmentModel.Appointment{
		ID:        testAptID,
		ClinicID:  testClinicID,
		Price:     200,
		StartTime: time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC),
		Patient:   &appointmentModel.PatientInfo{Name: "Fulana"},
	}
	pmID := uuid.New()
	paymentID := uuid.New()
	payment := &appointmentModel.AppointmentPayment{
		ID:              paymentID,
		ClinicID:        testClinicID,
		AppointmentID:   testAptID,
		PaymentMethodID: pmID,
		Amount:          200,
		PaidAt:          time.Date(2026, 4, 23, 14, 0, 0, 0, time.UTC),
	}

	txRepo.On("GetByAppointmentPaymentID", ctx, paymentID, testClinicID).Return(nil, nil)
	txRepo.On("Create", ctx, mock.MatchedBy(func(tx *model.FinancialTransaction) bool {
		return tx.Source == model.SourceAppointment &&
			tx.AppointmentID != nil && *tx.AppointmentID == testAptID &&
			tx.AppointmentPaymentID != nil && *tx.AppointmentPaymentID == paymentID &&
			tx.Amount == 200 &&
			tx.Type == model.TransactionTypeIncome &&
			tx.PaidAt != nil && tx.PaidAt.Equal(payment.PaidAt) &&
			tx.CompetenceDate.Equal(payment.PaidAt) &&
			tx.PaymentMethodID != nil && *tx.PaymentMethodID == pmID
	})).Return(nil)

	err := svc.UpsertFromAppointmentPayment(ctx, apt, payment, testClinicID)
	assert.NoError(t, err)
	txRepo.AssertExpectations(t)
}

func TestUpsertFromAppointmentPayment_RoutesToSessionPlanWhenLinked(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	planID := uuid.New()
	apt := &appointmentModel.Appointment{
		ID:            testAptID,
		ClinicID:      testClinicID,
		Price:         150,
		SessionPlanID: &planID,
	}
	paymentID := uuid.New()
	payment := &appointmentModel.AppointmentPayment{
		ID: paymentID, ClinicID: testClinicID, AppointmentID: testAptID,
		PaymentMethodID: uuid.New(), Amount: 150, PaidAt: time.Now(),
	}

	txRepo.On("GetByAppointmentPaymentID", ctx, paymentID, testClinicID).Return(nil, nil)
	txRepo.On("Create", ctx, mock.MatchedBy(func(tx *model.FinancialTransaction) bool {
		return tx.Source == model.SourceSessionPlan &&
			tx.SessionPlanID != nil && *tx.SessionPlanID == planID
	})).Return(nil)

	err := svc.UpsertFromAppointmentPayment(ctx, apt, payment, testClinicID)
	assert.NoError(t, err)
	txRepo.AssertExpectations(t)
}

func TestUpsertFromAppointmentPayment_UpdatesWhenAmountChanges(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	apt := &appointmentModel.Appointment{ID: testAptID, ClinicID: testClinicID}
	pmID := uuid.New()
	paymentID := uuid.New()
	payment := &appointmentModel.AppointmentPayment{
		ID: paymentID, ClinicID: testClinicID, AppointmentID: testAptID,
		PaymentMethodID: pmID, Amount: 200, PaidAt: time.Now(),
	}

	existingID := uuid.New()
	existing := &model.FinancialTransaction{
		ID: existingID, ClinicID: testClinicID, AppointmentID: &testAptID,
		AppointmentPaymentID: &paymentID, Source: model.SourceAppointment,
		Amount: 100, PaymentMethodID: &pmID,
	}
	txRepo.On("GetByAppointmentPaymentID", ctx, paymentID, testClinicID).Return(existing, nil)
	txRepo.On("Update", ctx, mock.MatchedBy(func(tx *model.FinancialTransaction) bool {
		return tx.ID == existingID && tx.Amount == 200
	})).Return(nil)

	err := svc.UpsertFromAppointmentPayment(ctx, apt, payment, testClinicID)
	assert.NoError(t, err)
}

func TestDeleteFromAppointmentPayment_RemovesRow(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	paymentID := uuid.New()
	txRepo.On("DeleteByAppointmentPaymentID", ctx, paymentID, testClinicID).Return(nil)

	err := svc.DeleteFromAppointmentPayment(ctx, paymentID, testClinicID)
	assert.NoError(t, err)
	txRepo.AssertExpectations(t)
}

// --- CreateFromCommissionBatch (auto hook) --------------------------------

func TestCreateFromCommissionBatch_InsertsOneRowPerEvent(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	profID := uuid.New()
	earliest := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	latest := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)

	txRepo.On("Create", ctx, mock.MatchedBy(func(tx *model.FinancialTransaction) bool {
		return tx.Source == model.SourceCommission &&
			tx.Type == model.TransactionTypeExpense &&
			tx.Amount == 3500 &&
			tx.ProfessionalID != nil && *tx.ProfessionalID == profID &&
			tx.PaidAt != nil &&
			// Description carries session count + date range so user can read
			// the row without drilling into Repasse.
			tx.Description == "Repasse Dra. Ana — 5 sessões (10/04 a 15/04)"
	})).Return(nil)

	err := svc.CreateFromCommissionBatch(ctx, testClinicID, profID, 3500, "Dra. Ana", 5, earliest, latest)
	assert.NoError(t, err)
	txRepo.AssertExpectations(t)
}

func TestCreateFromCommissionBatch_NoopWhenZeroCount(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	err := svc.CreateFromCommissionBatch(ctx, testClinicID, uuid.New(), 0, "Ana", 0, time.Time{}, time.Time{})
	assert.NoError(t, err)
	txRepo.AssertNotCalled(t, "Create")
}

func TestCreateFromCommissionBatch_SingleDayFormatsAsSingle(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	day := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	txRepo.On("Create", ctx, mock.MatchedBy(func(tx *model.FinancialTransaction) bool {
		return tx.Description == "Repasse Dra. Ana — 1 sessão (23/04)"
	})).Return(nil)

	err := svc.CreateFromCommissionBatch(ctx, testClinicID, uuid.New(), 300, "Dra. Ana", 1, day, day)
	assert.NoError(t, err)
	txRepo.AssertExpectations(t)
}

// --- Cash flow summary ----------------------------------------------------

func TestGetCashFlowSummary(t *testing.T) {
	svc, txRepo, _ := newTestService()
	ctx := context.Background()

	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	txRepo.On("SumByPeriod", ctx, testClinicID, from, to).Return(1500.0, 800.0, nil)

	res, err := svc.GetCashFlowSummary(ctx, testClinicID, from, to)
	assert.NoError(t, err)
	assert.Equal(t, 1500.0, res.TotalIncome)
	assert.Equal(t, 800.0, res.TotalExpense)
	assert.Equal(t, 700.0, res.Balance)
}

// --- Categories -----------------------------------------------------------

func TestListCategories_SeedsDefaultsWhenEmpty(t *testing.T) {
	svc, _, catRepo := newTestService()
	ctx := context.Background()

	catRepo.On("CountByClinic", ctx, testClinicID).Return(int64(0), nil)
	catRepo.On("CreateDefaults", ctx, testClinicID).Return(nil)
	catRepo.On("ListByClinic", ctx, testClinicID).
		Return([]model.FinancialCategory{
			{ID: uuid.New(), Name: "Consulta", Type: model.TransactionTypeIncome, IsDefault: true},
		}, nil)

	cats, err := svc.ListCategories(ctx, testClinicID)
	assert.NoError(t, err)
	assert.Len(t, cats, 1)
	catRepo.AssertCalled(t, "CreateDefaults", ctx, testClinicID)
}

func TestListCategories_SkipsSeedWhenAlreadyHas(t *testing.T) {
	svc, _, catRepo := newTestService()
	ctx := context.Background()

	catRepo.On("CountByClinic", ctx, testClinicID).Return(int64(5), nil)
	catRepo.On("ListByClinic", ctx, testClinicID).
		Return([]model.FinancialCategory{{ID: uuid.New(), Name: "X"}}, nil)

	_, err := svc.ListCategories(ctx, testClinicID)
	assert.NoError(t, err)
	catRepo.AssertNotCalled(t, "CreateDefaults")
}

func TestDeleteCategory_BlockedWhenDefault(t *testing.T) {
	svc, _, catRepo := newTestService()
	ctx := context.Background()

	catID := uuid.New()
	catRepo.On("GetByID", ctx, catID, testClinicID).
		Return(&model.FinancialCategory{
			ID: catID, ClinicID: testClinicID, IsDefault: true,
		}, nil)

	err := svc.DeleteCategory(ctx, catID, testClinicID)
	assert.Error(t, err)
	catRepo.AssertNotCalled(t, "Delete")
}

func TestDeleteCategory_OKWhenCustom(t *testing.T) {
	svc, _, catRepo := newTestService()
	ctx := context.Background()

	catID := uuid.New()
	catRepo.On("GetByID", ctx, catID, testClinicID).
		Return(&model.FinancialCategory{
			ID: catID, ClinicID: testClinicID, IsDefault: false,
		}, nil)
	catRepo.On("Delete", ctx, catID, testClinicID).Return(nil)

	err := svc.DeleteCategory(ctx, catID, testClinicID)
	assert.NoError(t, err)
}
