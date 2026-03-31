package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	account_model "github.com/aseno/backend/pkg/account/model"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	"github.com/aseno/backend/pkg/payment/model"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	settings_model "github.com/aseno/backend/pkg/settings/model"
)

// Mock implementations

type mockPaymentClient struct {
	mock.Mock
}

func (m *mockPaymentClient) CreateCustomer(ctx context.Context, req *model.CreateCustomerRequest) (*model.AsaasCustomerResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AsaasCustomerResponse), args.Error(1)
}

func (m *mockPaymentClient) GetCustomer(ctx context.Context, customerID string) (*model.AsaasCustomerResponse, error) {
	args := m.Called(ctx, customerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AsaasCustomerResponse), args.Error(1)
}

func (m *mockPaymentClient) UpdateCustomer(ctx context.Context, customerID string, req *model.CreateCustomerRequest) (*model.AsaasCustomerResponse, error) {
	args := m.Called(ctx, customerID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AsaasCustomerResponse), args.Error(1)
}

func (m *mockPaymentClient) DeleteCustomer(ctx context.Context, customerID string) error {
	return m.Called(ctx, customerID).Error(0)
}

func (m *mockPaymentClient) CreatePayment(ctx context.Context, req *model.CreatePaymentRequest) (*model.AsaasPaymentResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AsaasPaymentResponse), args.Error(1)
}

func (m *mockPaymentClient) GetPayment(ctx context.Context, paymentID string) (*model.AsaasPaymentResponse, error) {
	args := m.Called(ctx, paymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AsaasPaymentResponse), args.Error(1)
}

func (m *mockPaymentClient) GetPaymentsByCustomer(ctx context.Context, customerID string, limit, offset int) (*model.PaymentListResponse, error) {
	args := m.Called(ctx, customerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaymentListResponse), args.Error(1)
}

func (m *mockPaymentClient) DeletePayment(ctx context.Context, paymentID string) error {
	return m.Called(ctx, paymentID).Error(0)
}

func (m *mockPaymentClient) GetPixQRCode(ctx context.Context, paymentID string) (*model.AsaasPixQRCodeResponse, error) {
	args := m.Called(ctx, paymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AsaasPixQRCodeResponse), args.Error(1)
}

func (m *mockPaymentClient) CreateSubscription(ctx context.Context, req *model.CreateSubscriptionRequest) (*model.AsaasSubscriptionResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AsaasSubscriptionResponse), args.Error(1)
}

func (m *mockPaymentClient) GetSubscription(ctx context.Context, subscriptionID string) (*model.AsaasSubscriptionResponse, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AsaasSubscriptionResponse), args.Error(1)
}

func (m *mockPaymentClient) GetSubscriptionsByCustomer(ctx context.Context, customerID string) (*model.SubscriptionListResponse, error) {
	args := m.Called(ctx, customerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.SubscriptionListResponse), args.Error(1)
}

func (m *mockPaymentClient) UpdateSubscription(ctx context.Context, subscriptionID string, req *model.CreateSubscriptionRequest) (*model.AsaasSubscriptionResponse, error) {
	args := m.Called(ctx, subscriptionID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AsaasSubscriptionResponse), args.Error(1)
}

func (m *mockPaymentClient) DeleteSubscription(ctx context.Context, subscriptionID string) error {
	return m.Called(ctx, subscriptionID).Error(0)
}

func (m *mockPaymentClient) GetSubscriptionPayments(ctx context.Context, subscriptionID string) (*model.PaymentListResponse, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PaymentListResponse), args.Error(1)
}

// Mock CustomerRepository
type mockCustomerRepo struct {
	mock.Mock
}

func (m *mockCustomerRepo) Create(ctx context.Context, customer *model.Customer) error {
	return m.Called(ctx, customer).Error(0)
}

func (m *mockCustomerRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Customer, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Customer), args.Error(1)
}

func (m *mockCustomerRepo) GetByAccountID(ctx context.Context, accountID uuid.UUID) (*model.Customer, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Customer), args.Error(1)
}

func (m *mockCustomerRepo) GetByAsaasID(ctx context.Context, asaasID string) (*model.Customer, error) {
	args := m.Called(ctx, asaasID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Customer), args.Error(1)
}

func (m *mockCustomerRepo) Update(ctx context.Context, customer *model.Customer) error {
	return m.Called(ctx, customer).Error(0)
}

func (m *mockCustomerRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// Mock PaymentRepository
type mockPaymentRepo struct {
	mock.Mock
}

func (m *mockPaymentRepo) Create(ctx context.Context, payment *model.Payment) error {
	return m.Called(ctx, payment).Error(0)
}

func (m *mockPaymentRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Payment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetByAsaasID(ctx context.Context, asaasID string) (*model.Payment, error) {
	args := m.Called(ctx, asaasID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*model.Payment, int64, error) {
	args := m.Called(ctx, accountID, limit, offset)
	return args.Get(0).([]*model.Payment), args.Get(1).(int64), args.Error(2)
}

func (m *mockPaymentRepo) GetByCustomerID(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*model.Payment, int64, error) {
	args := m.Called(ctx, customerID, limit, offset)
	return args.Get(0).([]*model.Payment), args.Get(1).(int64), args.Error(2)
}

func (m *mockPaymentRepo) Update(ctx context.Context, payment *model.Payment) error {
	return m.Called(ctx, payment).Error(0)
}

func (m *mockPaymentRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.PaymentStatus) error {
	return m.Called(ctx, id, status).Error(0)
}

// Mock SubscriptionRepository
type mockSubscriptionRepo struct {
	mock.Mock
}

func (m *mockSubscriptionRepo) Create(ctx context.Context, subscription *model.Subscription) error {
	return m.Called(ctx, subscription).Error(0)
}

func (m *mockSubscriptionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Subscription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Subscription), args.Error(1)
}

func (m *mockSubscriptionRepo) GetByAsaasID(ctx context.Context, asaasID string) (*model.Subscription, error) {
	args := m.Called(ctx, asaasID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Subscription), args.Error(1)
}

func (m *mockSubscriptionRepo) GetByAccountID(ctx context.Context, accountID uuid.UUID) (*model.Subscription, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Subscription), args.Error(1)
}

func (m *mockSubscriptionRepo) GetByCustomerID(ctx context.Context, customerID uuid.UUID) ([]*model.Subscription, error) {
	args := m.Called(ctx, customerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Subscription), args.Error(1)
}

func (m *mockSubscriptionRepo) Update(ctx context.Context, subscription *model.Subscription) error {
	return m.Called(ctx, subscription).Error(0)
}

func (m *mockSubscriptionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.SubscriptionStatus) error {
	return m.Called(ctx, id, status).Error(0)
}

// Mock ClinicRepository
type mockClinicRepo struct {
	mock.Mock
}

func (m *mockClinicRepo) Create(ctx context.Context, clinic *clinic_model.Clinic) error {
	return m.Called(ctx, clinic).Error(0)
}

func (m *mockClinicRepo) GetByID(ctx context.Context, id uuid.UUID) (*clinic_model.Clinic, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clinic_model.Clinic), args.Error(1)
}

func (m *mockClinicRepo) GetBySlug(ctx context.Context, slug string) (*clinic_model.Clinic, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clinic_model.Clinic), args.Error(1)
}

func (m *mockClinicRepo) GetByAccountID(ctx context.Context, accountID uuid.UUID) (*clinic_model.Clinic, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clinic_model.Clinic), args.Error(1)
}

func (m *mockClinicRepo) Update(ctx context.Context, clinic *clinic_model.Clinic) error {
	return m.Called(ctx, clinic).Error(0)
}

func (m *mockClinicRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockClinicRepo) GetByEmail(ctx context.Context, email string) (*clinic_model.Clinic, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clinic_model.Clinic), args.Error(1)
}

func (m *mockClinicRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *mockClinicRepo) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	args := m.Called(ctx, slug)
	return args.Bool(0), args.Error(1)
}

func (m *mockClinicRepo) List(ctx context.Context, limit, offset int) ([]*clinic_model.Clinic, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*clinic_model.Clinic), args.Get(1).(int64), args.Error(2)
}

// Mock AccountRepository
type mockAccountRepo struct {
	mock.Mock
}

func (m *mockAccountRepo) Create(ctx context.Context, account *account_model.Account) error {
	return m.Called(ctx, account).Error(0)
}

func (m *mockAccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*account_model.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account_model.Account), args.Error(1)
}

func (m *mockAccountRepo) GetByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]account_model.Account, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]account_model.Account), args.Error(1)
}

func (m *mockAccountRepo) Update(ctx context.Context, account *account_model.Account) error {
	return m.Called(ctx, account).Error(0)
}

func (m *mockAccountRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockAccountRepo) ListAll(ctx context.Context) ([]account_model.Account, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]account_model.Account), args.Error(1)
}

type mockSettingsRepo struct {
	mock.Mock
}

func (m *mockSettingsRepo) GetAll(ctx context.Context) ([]settings_model.SystemSetting, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]settings_model.SystemSetting), args.Error(1)
}

func (m *mockSettingsRepo) Get(ctx context.Context, key string) (*settings_model.SystemSetting, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*settings_model.SystemSetting), args.Error(1)
}

func (m *mockSettingsRepo) Set(ctx context.Context, key, value string) error {
	return m.Called(ctx, key, value).Error(0)
}

func (m *mockSettingsRepo) SeedDefaults(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// Mock PlanService
type mockPlanService struct {
	mock.Mock
}

func (m *mockPlanService) ListPlans(ctx context.Context) ([]plan_model.Plan, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]plan_model.Plan), args.Error(1)
}
func (m *mockPlanService) CalculateBestPrice(ctx context.Context, plan *plan_model.Plan, offer *plan_model.Offer, cycleType plan_model.CycleType) (float64, error) {
	args := m.Called(ctx, plan, offer, cycleType)
	return args.Get(0).(float64), args.Error(1)
}

// Add dummy methods to satisfy the interface if necessary
func (m *mockPlanService) GetPlanByID(ctx context.Context, id uuid.UUID) (*plan_model.Plan, error) {
	return nil, nil
}
func (m *mockPlanService) ValidateOffer(ctx context.Context, token string) (*plan_model.Offer, error) {
	return nil, nil
}
func (m *mockPlanService) UseOffer(ctx context.Context, offerID uuid.UUID) error          { return nil }
func (m *mockPlanService) CreatePlan(ctx context.Context, plan *plan_model.Plan) error    { return nil }
func (m *mockPlanService) UpdatePlan(ctx context.Context, plan *plan_model.Plan) error    { return nil }
func (m *mockPlanService) CreateOffer(ctx context.Context, offer *plan_model.Offer) error { return nil }
func (m *mockPlanService) ListOffers(ctx context.Context) ([]plan_model.Offer, error) {
	return nil, nil
}
func (m *mockPlanService) UpdateOffer(ctx context.Context, offer *plan_model.Offer) error { return nil }

// Test helper to create service with mocks
func setupTestService() (*paymentService, *mockPaymentClient, *mockCustomerRepo, *mockPaymentRepo, *mockSubscriptionRepo, *mockClinicRepo, *mockAccountRepo, *mockPlanService) {
	client := new(mockPaymentClient)
	customerRepo := new(mockCustomerRepo)
	paymentRepo := new(mockPaymentRepo)
	subscriptionRepo := new(mockSubscriptionRepo)
	clinicRepo := new(mockClinicRepo)
	accountRepo := new(mockAccountRepo)
	planService := new(mockPlanService)

	svc := &paymentService{
		client:           client,
		customerRepo:     customerRepo,
		paymentRepo:      paymentRepo,
		subscriptionRepo: subscriptionRepo,
		clinicRepo:       clinicRepo,
		accountRepo:      accountRepo,
		planService:      planService,
	}

	return svc, client, customerRepo, paymentRepo, subscriptionRepo, clinicRepo, accountRepo, planService
}

// --- SwitchPlan Tests ---

func TestSwitchPlan_NewSubscription_FromTrial(t *testing.T) {
	svc, client, customerRepo, _, subscriptionRepo, clinicRepo, accountRepo, planServiceMock := setupTestService()
	ctx := context.Background()

	clinicID := uuid.New()
	accountID := uuid.New()
	customerID := uuid.New()

	// Mock clinic lookup
	clinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		ID:        clinicID,
		AccountID: accountID,
	}, nil)

	// Mock customer lookup
	customerRepo.On("GetByAccountID", ctx, accountID).Return(&model.Customer{
		ID:              customerID,
		AsaasCustomerID: "cus_asaas123",
	}, nil)

	// No existing subscription (trial user)
	subscriptionRepo.On("GetByAccountID", ctx, accountID).Return(nil, assert.AnError)
	// Mock plans
	professionalPlan := plan_model.Plan{ID: uuid.New(), Slug: "professional", Name: "Plano Profissional"}
	planServiceMock.On("ListPlans", ctx).Return([]plan_model.Plan{professionalPlan}, nil)
	planServiceMock.On("CalculateBestPrice", ctx, mock.Anything, mock.Anything, mock.Anything).Return(199.00, nil)

	// Mock Asaas subscription creation

	client.On("CreateSubscription", ctx, mock.AnythingOfType("*model.CreateSubscriptionRequest")).Return(&model.AsaasSubscriptionResponse{
		ID:          "sub_asaas_new",
		Status:      "ACTIVE",
		BillingType: "PIX",
		Value:       199.00,
		Cycle:       "MONTHLY",
		NextDueDate: time.Now().Format("2006-01-02"),
		Description: "Plano Profissional - Aseno",
	}, nil)

	// Mock subscription save
	subscriptionRepo.On("Create", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	// Mock account update
	accountRepo.On("GetByID", ctx, accountID).Return(&account_model.Account{
		ID:                 accountID,
		Plan:               &plan_model.Plan{Slug: "professional"},
		SubscriptionStatus: account_model.SubscriptionStatusTrial,
	}, nil)
	accountRepo.On("Update", ctx, mock.AnythingOfType("*model.Account")).Return(nil)

	// Execute
	result, err := svc.SwitchPlan(ctx, clinicID, "professional", "PIX", "MONTHLY", nil, nil, nil)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "sub_asaas_new", result.AsaasSubscriptionID)
	assert.Equal(t, float64(199.00), result.Value)

	// Verify Asaas was not asked to delete (no existing subscription)
	client.AssertNotCalled(t, "DeleteSubscription", mock.Anything, mock.Anything)

	// Verify subscription was created
	subscriptionRepo.AssertCalled(t, "Create", ctx, mock.AnythingOfType("*model.Subscription"))
}

func TestSwitchPlan_ExistingActiveSubscription_UsesNextDueDate(t *testing.T) {
	svc, client, customerRepo, _, subscriptionRepo, clinicRepo, accountRepo, planServiceMock := setupTestService()
	ctx := context.Background()

	clinicID := uuid.New()
	accountID := uuid.New()
	customerID := uuid.New()
	existingSubID := uuid.New()
	futureDate := time.Now().AddDate(0, 1, 0) // 1 month ahead

	// Mock clinic lookup
	clinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		ID:        clinicID,
		AccountID: accountID,
	}, nil)

	// Mock customer lookup
	customerRepo.On("GetByAccountID", ctx, accountID).Return(&model.Customer{
		ID:              customerID,
		AsaasCustomerID: "cus_active",
	}, nil)

	// Existing active subscription
	subscriptionRepo.On("GetByAccountID", ctx, accountID).Return(&model.Subscription{
		ID:                  existingSubID,
		AsaasSubscriptionID: "sub_old",
		Status:              model.SubscriptionStatusActive,
		NextDueDate:         futureDate,
	}, nil)

	// Mock plans
	advancedPlan := plan_model.Plan{ID: uuid.New(), Slug: "advanced", Name: "Plano Avançado"}
	planServiceMock.On("ListPlans", ctx).Return([]plan_model.Plan{advancedPlan}, nil)
	planServiceMock.On("CalculateBestPrice", ctx, mock.Anything, mock.Anything, mock.Anything).Return(349.00, nil)

	// Mock cancel old subscription in Asaas
	client.On("DeleteSubscription", ctx, "sub_old").Return(nil)

	// Mock status update on old subscription
	subscriptionRepo.On("UpdateStatus", ctx, existingSubID, model.SubscriptionStatusInactive).Return(nil)

	// Mock Asaas creates new subscription with future nextDueDate

	client.On("CreateSubscription", ctx, mock.MatchedBy(func(req *model.CreateSubscriptionRequest) bool {
		// Verify nextDueDate is set to the existing subscription's next due date
		return req.NextDueDate == futureDate.Format("2006-01-02") && req.Value == 349.00
	})).Return(&model.AsaasSubscriptionResponse{
		ID:          "sub_new_advanced",
		Status:      "ACTIVE",
		BillingType: "CREDIT_CARD",
		Value:       349.00,
		Cycle:       "MONTHLY",
		NextDueDate: futureDate.Format("2006-01-02"),
		Description: "Plano Avançado - Aseno",
	}, nil)

	// Mock subscription save
	subscriptionRepo.On("Create", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	// Mock account update
	accountRepo.On("GetByID", ctx, accountID).Return(&account_model.Account{
		ID:                 accountID,
		Plan:               &plan_model.Plan{Slug: "professional"},
		SubscriptionStatus: account_model.SubscriptionStatusActive,
	}, nil)
	accountRepo.On("Update", ctx, mock.AnythingOfType("*model.Account")).Return(nil)

	// Execute
	result, err := svc.SwitchPlan(ctx, clinicID, "advanced", "CREDIT_CARD", "MONTHLY", nil, nil, nil)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "sub_new_advanced", result.AsaasSubscriptionID)
	assert.Equal(t, float64(349.00), result.Value)

	// Verify old subscription was cancelled
	client.AssertCalled(t, "DeleteSubscription", ctx, "sub_old")
	subscriptionRepo.AssertCalled(t, "UpdateStatus", ctx, existingSubID, model.SubscriptionStatusInactive)
}

func TestSwitchPlan_InvalidPlan_ReturnsError(t *testing.T) {
	svc, _, customerRepo, _, _, clinicRepo, _, planServiceMock := setupTestService()
	ctx := context.Background()

	clinicID := uuid.New()
	accountID := uuid.New()

	clinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		ID:        clinicID,
		AccountID: accountID,
	}, nil)

	customerRepo.On("GetByAccountID", ctx, accountID).Return(&model.Customer{
		ID:              uuid.New(),
		AsaasCustomerID: "cus_test",
	}, nil)

	planServiceMock.On("ListPlans", ctx).Return([]plan_model.Plan{}, nil)

	// Execute with invalid plan
	result, err := svc.SwitchPlan(ctx, clinicID, "invalid_plan", "PIX", "MONTHLY", nil, nil, nil)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "Invalid plan")
}

func TestSwitchPlan_YearlyCycle_CalculatesAnnualPrice(t *testing.T) {
	svc, client, customerRepo, _, subscriptionRepo, clinicRepo, accountRepo, planServiceMock := setupTestService()
	ctx := context.Background()

	clinicID := uuid.New()
	accountID := uuid.New()

	clinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		ID:        clinicID,
		AccountID: accountID,
	}, nil)

	customerRepo.On("GetByAccountID", ctx, accountID).Return(&model.Customer{
		ID:              uuid.New(),
		AsaasCustomerID: "cus_yearly",
	}, nil)

	subscriptionRepo.On("GetByAccountID", ctx, accountID).Return(nil, assert.AnError)

	basicPlan := plan_model.Plan{ID: uuid.New(), Slug: "basic", Name: "Plano Básico"}
	planServiceMock.On("ListPlans", ctx).Return([]plan_model.Plan{basicPlan}, nil)
	planServiceMock.On("CalculateBestPrice", ctx, mock.Anything, mock.Anything, mock.Anything).Return(990.00, nil)

	// Verify annual price: basic = 99 * 10 = 990 (2 months free)

	client.On("CreateSubscription", ctx, mock.MatchedBy(func(req *model.CreateSubscriptionRequest) bool {
		return req.Value == 990.00 && req.Cycle == model.BillingCycleYearly
	})).Return(&model.AsaasSubscriptionResponse{
		ID:          "sub_yearly",
		Status:      "ACTIVE",
		BillingType: "PIX",
		Value:       990.00,
		Cycle:       "YEARLY",
		NextDueDate: time.Now().Format("2006-01-02"),
		Description: "Plano Básico - Aseno",
	}, nil)

	subscriptionRepo.On("Create", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	accountRepo.On("GetByID", ctx, accountID).Return(&account_model.Account{
		ID:                 accountID,
		SubscriptionStatus: account_model.SubscriptionStatusTrial,
	}, nil)
	accountRepo.On("Update", ctx, mock.AnythingOfType("*model.Account")).Return(nil)

	// Execute
	result, err := svc.SwitchPlan(ctx, clinicID, "basic", "PIX", "YEARLY", nil, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, float64(990.00), result.Value)
}

// --- HandleWebhook Tests ---

func TestHandleWebhook_PaymentConfirmed_ActivatesAccount(t *testing.T) {
	svc, _, _, paymentRepo, subscriptionRepo, _, accountRepo, _ := setupTestService()
	ctx := context.Background()

	accountID := uuid.New()
	subscriptionID := uuid.New()
	paymentID := uuid.New()

	// Payment exists in local DB
	paymentRepo.On("GetByAsaasID", ctx, "pay_confirmed123").Return(&model.Payment{
		ID:        paymentID,
		AccountID: accountID,
	}, nil)
	paymentRepo.On("UpdateStatus", ctx, paymentID, model.PaymentStatus("CONFIRMED")).Return(nil)

	// Subscription lookup for account resolution
	subscriptionRepo.On("GetByAsaasID", ctx, "sub_test123").Return(&model.Subscription{
		ID:        subscriptionID,
		AccountID: accountID,
	}, nil)

	// Account lookup and update
	account := &account_model.Account{
		ID:                 accountID,
		SubscriptionStatus: account_model.SubscriptionStatusPending,
	}
	accountRepo.On("GetByID", ctx, accountID).Return(account, nil)
	accountRepo.On("Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusActive
	})).Return(nil)

	payload := &model.AsaasWebhookPayload{
		Event: model.WebhookEventPaymentConfirmed,
		Payment: &model.WebhookPayment{
			ID:           "pay_confirmed123",
			Customer:     "cus_test",
			Subscription: "sub_test123",
			Status:       "CONFIRMED",
			Value:        199.00,
		},
	}

	err := svc.HandleWebhook(ctx, payload)
	assert.NoError(t, err)
	accountRepo.AssertCalled(t, "Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusActive
	}))
}

func TestHandleWebhook_PaymentOverdue_SetsPastDue(t *testing.T) {
	svc, _, _, paymentRepo, subscriptionRepo, _, accountRepo, _ := setupTestService()
	ctx := context.Background()

	accountID := uuid.New()
	subscriptionID := uuid.New()
	paymentID := uuid.New()

	paymentRepo.On("GetByAsaasID", ctx, "pay_overdue123").Return(&model.Payment{
		ID:        paymentID,
		AccountID: accountID,
	}, nil)
	paymentRepo.On("UpdateStatus", ctx, paymentID, model.PaymentStatus("OVERDUE")).Return(nil)

	subscriptionRepo.On("GetByAsaasID", ctx, "sub_overdue").Return(&model.Subscription{
		ID:        subscriptionID,
		AccountID: accountID,
	}, nil)

	account := &account_model.Account{
		ID:                 accountID,
		SubscriptionStatus: account_model.SubscriptionStatusActive,
	}
	accountRepo.On("GetByID", ctx, accountID).Return(account, nil)
	accountRepo.On("Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusPastDue
	})).Return(nil)

	payload := &model.AsaasWebhookPayload{
		Event: model.WebhookEventPaymentOverdue,
		Payment: &model.WebhookPayment{
			ID:           "pay_overdue123",
			Customer:     "cus_test",
			Subscription: "sub_overdue",
			Status:       "OVERDUE",
		},
	}

	err := svc.HandleWebhook(ctx, payload)
	assert.NoError(t, err)
	accountRepo.AssertCalled(t, "Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusPastDue
	}))
}

func TestHandleWebhook_PaymentRefunded_CancelsAccount(t *testing.T) {
	svc, _, _, paymentRepo, subscriptionRepo, _, accountRepo, _ := setupTestService()
	ctx := context.Background()

	accountID := uuid.New()
	subscriptionID := uuid.New()
	paymentID := uuid.New()

	paymentRepo.On("GetByAsaasID", ctx, "pay_refund123").Return(&model.Payment{
		ID:        paymentID,
		AccountID: accountID,
	}, nil)
	paymentRepo.On("UpdateStatus", ctx, paymentID, model.PaymentStatus("REFUNDED")).Return(nil)

	subscriptionRepo.On("GetByAsaasID", ctx, "sub_refund").Return(&model.Subscription{
		ID:        subscriptionID,
		AccountID: accountID,
	}, nil)

	account := &account_model.Account{
		ID:                 accountID,
		SubscriptionStatus: account_model.SubscriptionStatusActive,
	}
	accountRepo.On("GetByID", ctx, accountID).Return(account, nil)
	accountRepo.On("Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusCanceled
	})).Return(nil)

	payload := &model.AsaasWebhookPayload{
		Event: model.WebhookEventPaymentRefunded,
		Payment: &model.WebhookPayment{
			ID:           "pay_refund123",
			Customer:     "cus_test",
			Subscription: "sub_refund",
			Status:       "REFUNDED",
		},
	}

	err := svc.HandleWebhook(ctx, payload)
	assert.NoError(t, err)
	accountRepo.AssertCalled(t, "Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusCanceled
	}))
}

func TestSwitchPlan_SetsPendingNotActive(t *testing.T) {
	svc, client, customerRepo, _, subscriptionRepo, clinicRepo, accountRepo, planServiceMock := setupTestService()
	ctx := context.Background()

	clinicID := uuid.New()
	accountID := uuid.New()

	clinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		ID:        clinicID,
		AccountID: accountID,
	}, nil)

	customerRepo.On("GetByAccountID", ctx, accountID).Return(&model.Customer{
		ID:              uuid.New(),
		AsaasCustomerID: "cus_pending_test",
	}, nil)

	subscriptionRepo.On("GetByAccountID", ctx, accountID).Return(nil, assert.AnError)

	basicPlan := plan_model.Plan{ID: uuid.New(), Slug: "basic", Name: "Plano Básico"}
	planServiceMock.On("ListPlans", ctx).Return([]plan_model.Plan{basicPlan}, nil)
	planServiceMock.On("CalculateBestPrice", ctx, mock.Anything, mock.Anything, mock.Anything).Return(99.00, nil)

	client.On("CreateSubscription", ctx, mock.AnythingOfType("*model.CreateSubscriptionRequest")).Return(&model.AsaasSubscriptionResponse{
		ID:          "sub_pending_test",
		Status:      "ACTIVE",
		BillingType: "PIX",
		Value:       99.00,
		Cycle:       "MONTHLY",
		NextDueDate: time.Now().Format("2006-01-02"),
		Description: "Plano Básico - Aseno",
	}, nil)

	subscriptionRepo.On("Create", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	accountRepo.On("GetByID", ctx, accountID).Return(&account_model.Account{
		ID:                 accountID,
		SubscriptionStatus: account_model.SubscriptionStatusTrial,
	}, nil)

	// Verify that Update is called with PENDING status, not ACTIVE
	accountRepo.On("Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusPending
	})).Return(nil)

	result, err := svc.SwitchPlan(ctx, clinicID, "basic", "PIX", "MONTHLY", nil, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify the account was updated to pending, NOT active
	accountRepo.AssertCalled(t, "Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusPending
	}))
}

func TestSwitchPlan_EmptyAsaasID_ReturnsError(t *testing.T) {
	svc, client, customerRepo, _, subscriptionRepo, clinicRepo, _, planServiceMock := setupTestService()
	ctx := context.Background()

	clinicID := uuid.New()
	accountID := uuid.New()

	clinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		ID:        clinicID,
		AccountID: accountID,
	}, nil)

	customerRepo.On("GetByAccountID", ctx, accountID).Return(&model.Customer{
		ID:              uuid.New(),
		AsaasCustomerID: "cus_empty_test",
	}, nil)

	subscriptionRepo.On("GetByAccountID", ctx, accountID).Return(nil, assert.AnError)

	basicPlan := plan_model.Plan{ID: uuid.New(), Slug: "basic", Name: "Plano Básico"}
	planServiceMock.On("ListPlans", ctx).Return([]plan_model.Plan{basicPlan}, nil)
	planServiceMock.On("CalculateBestPrice", ctx, mock.Anything, mock.Anything, mock.Anything).Return(99.00, nil)

	// Asaas returns empty subscription ID
	client.On("CreateSubscription", ctx, mock.AnythingOfType("*model.CreateSubscriptionRequest")).Return(&model.AsaasSubscriptionResponse{
		ID:          "", // Empty ID — this is the bug scenario
		Status:      "ACTIVE",
		BillingType: "PIX",
		Value:       0,
		Cycle:       "MONTHLY",
		NextDueDate: "",
	}, nil)

	result, err := svc.SwitchPlan(ctx, clinicID, "basic", "PIX", "MONTHLY", nil, nil, nil)

	// Should return error, not try to save with empty ID
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "empty subscription ID")

	// Verify subscription was NOT saved
	subscriptionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSwitchPlan_PopulatesPlanFields(t *testing.T) {
	svc, client, customerRepo, _, subscriptionRepo, clinicRepo, accountRepo, planServiceMock := setupTestService()
	ctx := context.Background()

	clinicID := uuid.New()
	accountID := uuid.New()
	planID := uuid.New()

	clinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		ID:        clinicID,
		AccountID: accountID,
	}, nil)

	customerRepo.On("GetByAccountID", ctx, accountID).Return(&model.Customer{
		ID:              uuid.New(),
		AsaasCustomerID: "cus_plan_fields",
	}, nil)

	subscriptionRepo.On("GetByAccountID", ctx, accountID).Return(nil, assert.AnError)

	basicPlan := plan_model.Plan{
		ID:                     planID,
		Slug:                   "basic",
		Name:                   "Plano Básico",
		MaxClinics:             2,
		MaxProfessionals:       5,
		MaxWhatsappConnections: 3,
		MaxGoogleCalendars:     2,
	}
	planServiceMock.On("ListPlans", ctx).Return([]plan_model.Plan{basicPlan}, nil)
	planServiceMock.On("CalculateBestPrice", ctx, mock.Anything, mock.Anything, mock.Anything).Return(99.00, nil)

	client.On("CreateSubscription", ctx, mock.AnythingOfType("*model.CreateSubscriptionRequest")).Return(&model.AsaasSubscriptionResponse{
		ID:          "sub_plan_fields_test",
		Status:      "ACTIVE",
		BillingType: "PIX",
		Value:       99.00,
		Cycle:       "MONTHLY",
		NextDueDate: time.Now().Format("2006-01-02"),
		Description: "Plano Básico - Aseno",
	}, nil)

	// Capture the subscription passed to Create
	var savedSubscription *model.Subscription
	subscriptionRepo.On("Create", ctx, mock.AnythingOfType("*model.Subscription")).Run(func(args mock.Arguments) {
		savedSubscription = args.Get(1).(*model.Subscription)
	}).Return(nil)

	accountRepo.On("GetByID", ctx, accountID).Return(&account_model.Account{
		ID:                 accountID,
		SubscriptionStatus: account_model.SubscriptionStatusTrial,
	}, nil)
	accountRepo.On("Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusPending
	})).Return(nil)

	result, err := svc.SwitchPlan(ctx, clinicID, "basic", "PIX", "MONTHLY", nil, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify PlanID and Price Lock fields are populated
	assert.NotNil(t, savedSubscription, "subscription should have been saved")
	assert.NotNil(t, savedSubscription.PlanID, "PlanID should be set")
	assert.Equal(t, planID, *savedSubscription.PlanID)
	assert.Equal(t, 99.00, savedSubscription.PriceAtContract)
	assert.Equal(t, 2, savedSubscription.MaxClinicsAtContract)
	assert.Equal(t, 5, savedSubscription.MaxProfessionalsAtContract)
	assert.Equal(t, 3, savedSubscription.MaxWhatsappAtContract)
	assert.Equal(t, 2, savedSubscription.MaxGoogleAtContract)
}

func TestSwitchPlan_GhostSubscription_SkipsAsaasDelete(t *testing.T) {
	svc, client, customerRepo, _, subscriptionRepo, clinicRepo, accountRepo, planServiceMock := setupTestService()
	ctx := context.Background()

	clinicID := uuid.New()
	accountID := uuid.New()
	ghostSubID := uuid.New()

	clinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		ID:        clinicID,
		AccountID: accountID,
	}, nil)

	customerRepo.On("GetByAccountID", ctx, accountID).Return(&model.Customer{
		ID:              uuid.New(),
		AsaasCustomerID: "cus_ghost_test",
	}, nil)

	// Ghost subscription: ACTIVE but with empty AsaasSubscriptionID and zero NextDueDate
	subscriptionRepo.On("GetByAccountID", ctx, accountID).Return(&model.Subscription{
		ID:                  ghostSubID,
		AsaasSubscriptionID: "", // Ghost — no Asaas ID
		Status:              model.SubscriptionStatusActive,
		NextDueDate:         time.Time{}, // Zero value
	}, nil)

	// Mark ghost as inactive
	subscriptionRepo.On("UpdateStatus", ctx, ghostSubID, model.SubscriptionStatusInactive).Return(nil)

	basicPlan := plan_model.Plan{ID: uuid.New(), Slug: "basic", Name: "Plano Básico"}
	planServiceMock.On("ListPlans", ctx).Return([]plan_model.Plan{basicPlan}, nil)
	planServiceMock.On("CalculateBestPrice", ctx, mock.Anything, mock.Anything, mock.Anything).Return(99.00, nil)

	// Asaas should create new subscription with today's date (not the ghost's zero date)
	client.On("CreateSubscription", ctx, mock.MatchedBy(func(req *model.CreateSubscriptionRequest) bool {
		return req.NextDueDate == time.Now().Format("2006-01-02")
	})).Return(&model.AsaasSubscriptionResponse{
		ID:          "sub_after_ghost",
		Status:      "ACTIVE",
		BillingType: "PIX",
		Value:       99.00,
		Cycle:       "MONTHLY",
		NextDueDate: time.Now().Format("2006-01-02"),
		Description: "Plano Básico - Aseno",
	}, nil)

	subscriptionRepo.On("Create", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	accountRepo.On("GetByID", ctx, accountID).Return(&account_model.Account{
		ID:                 accountID,
		SubscriptionStatus: account_model.SubscriptionStatusTrial,
	}, nil)
	accountRepo.On("Update", ctx, mock.MatchedBy(func(acc *account_model.Account) bool {
		return acc.SubscriptionStatus == account_model.SubscriptionStatusPending
	})).Return(nil)

	result, err := svc.SwitchPlan(ctx, clinicID, "basic", "PIX", "MONTHLY", nil, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "sub_after_ghost", result.AsaasSubscriptionID)

	// Verify Asaas DeleteSubscription was NOT called (ghost has no Asaas ID)
	client.AssertNotCalled(t, "DeleteSubscription", mock.Anything, mock.Anything)

	// Verify ghost was marked as inactive
	subscriptionRepo.AssertCalled(t, "UpdateStatus", ctx, ghostSubID, model.SubscriptionStatusInactive)

	// Verify new subscription was created
	subscriptionRepo.AssertCalled(t, "Create", ctx, mock.AnythingOfType("*model.Subscription"))
}
