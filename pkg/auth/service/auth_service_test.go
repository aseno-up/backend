package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aseno/backend/internal/errors"
	account_model "github.com/aseno/backend/pkg/account/model"
	account_repository "github.com/aseno/backend/pkg/account/repository"
	"github.com/aseno/backend/pkg/auth/model"
	auth_repository "github.com/aseno/backend/pkg/auth/repository"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	email_service "github.com/aseno/backend/pkg/email/service"
	payment_model "github.com/aseno/backend/pkg/payment/model"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	plan_repo "github.com/aseno/backend/pkg/plan/repository"
	user_model "github.com/aseno/backend/pkg/user/model"
	user_repository "github.com/aseno/backend/pkg/user/repository"
)

// MockPaymentService is a mock for PaymentService
type MockPaymentService struct {
	PaymentService
	mock.Mock
}

func (m *MockPaymentService) CreateCustomerForClinic(ctx context.Context, clinicID uuid.UUID, req *payment_model.CreateCustomerRequest) error {
	args := m.Called(ctx, clinicID, req)
	return args.Error(0)
}

func (m *MockPaymentService) CreateSubscriptionForClinic(ctx context.Context, clinicID uuid.UUID, planID uuid.UUID, offerToken *string, billing string) error {
	args := m.Called(ctx, clinicID, planID, offerToken, billing)
	return args.Error(0)
}

// MockUserRepository
type MockUserRepository struct {
	user_repository.UserRepository
	mock.Mock
}

func (m *MockUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}
func (m *MockUserRepository) Create(ctx context.Context, user *user_model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}
func (m *MockUserRepository) UpdateRefreshToken(ctx context.Context, userID uuid.UUID, token string) error {
	args := m.Called(ctx, userID, token)
	return args.Error(0)
}
func (m *MockUserRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}
func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*user_model.User, error) {
	args := m.Called(ctx, email)
	var user *user_model.User
	if args.Get(0) != nil {
		user = args.Get(0).(*user_model.User)
	}
	return user, args.Error(1)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*user_model.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*user_model.User), args.Error(1)
}

// MockClinicRepository
type MockClinicRepository struct {
	clinic_repository.ClinicRepository
	mock.Mock
}

func (m *MockClinicRepository) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	args := m.Called(ctx, slug)
	return args.Bool(0), args.Error(1)
}
func (m *MockClinicRepository) Create(ctx context.Context, clinic *clinic_model.Clinic) error {
	args := m.Called(ctx, clinic)
	return args.Error(0)
}
func (m *MockClinicRepository) GetByID(ctx context.Context, id uuid.UUID) (*clinic_model.Clinic, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*clinic_model.Clinic), args.Error(1)
}

// MockAccountRepository
type MockAccountRepository struct {
	account_repository.AccountRepository
	mock.Mock
}

// MockPlanRepository
type MockPlanRepository struct {
	plan_repo.PlanRepository
	mock.Mock
}

func (m *MockPlanRepository) GetBySlug(ctx context.Context, slug string) (*plan_model.Plan, error) {
	args := m.Called(ctx, slug)
	var plan *plan_model.Plan
	if args.Get(0) != nil {
		plan = args.Get(0).(*plan_model.Plan)
	}
	return plan, args.Error(1)
}

func (m *MockAccountRepository) Create(ctx context.Context, account *account_model.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

// MockJWTService
type MockJWTService struct {
	JWTService
	mock.Mock
}

func (m *MockJWTService) GenerateTokenPair(user *user_model.User) (*model.TokenPair, error) {
	args := m.Called(user)
	return args.Get(0).(*model.TokenPair), args.Error(1)
}

// MockEmailService
type MockEmailService struct {
	email_service.EmailService
	mock.Mock
}

func (m *MockEmailService) SendEmailVerification(ctx context.Context, data *email_service.EmailVerificationData) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

// MockEmailVerificationRepository
type MockEmailVerificationRepository struct {
	auth_repository.EmailVerificationRepository
	mock.Mock
}

func (m *MockEmailVerificationRepository) Create(ctx context.Context, verification *model.EmailVerification) error {
	args := m.Called(ctx, verification)
	return args.Error(0)
}

// MockPasswordResetRepository
type MockPasswordResetRepository struct {
	auth_repository.PasswordResetRepository
	mock.Mock
}

// MockGoogleAuthClient
type MockGoogleAuthClient struct {
	GoogleAuthClient
	mock.Mock
}

func (m *MockGoogleAuthClient) VerifyIDToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	args := m.Called(ctx, idToken)
	var info *GoogleUserInfo
	if args.Get(0) != nil {
		info = args.Get(0).(*GoogleUserInfo)
	}
	return info, args.Error(1)
}

func TestAuthService_Register_CreatesAsaasCustomerUnconditionally(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAccountRepo := new(MockAccountRepository)
	mockJWTService := new(MockJWTService)
	mockEmailService := new(MockEmailService)
	mockEmailVerifRepo := new(MockEmailVerificationRepository)
	mockPassResetRepo := new(MockPasswordResetRepository)
	mockPaymentService := new(MockPaymentService)
	mockPlanRepo := new(MockPlanRepository)

	svc := NewAuthService(
		mockUserRepo,
		mockClinicRepo,
		mockAccountRepo,
		mockPassResetRepo,
		mockEmailVerifRepo,
		mockPlanRepo,
		mockJWTService,
		mockEmailService,
		"http://localhost:3000",
	).(*authService)

	svc.SetPaymentService(mockPaymentService)

	ctx := context.Background()

	req := &model.RegisterRequest{
		Email:      "test@example.com",
		ClinicName: "Test Clinic",
		Name:       "Test User",
		Phone:      "11999999999",
		Password:   "password123",
		Plan:       "trial",
		// Document explicitly empty to test unconditional creation
	}

	// Setup mocks
	mockPlanRepo.On("GetBySlug", ctx, "basic").Return(&plan_model.Plan{ID: uuid.New(), Slug: "basic", Name: "Basic"}, nil)
	mockUserRepo.On("ExistsByEmail", ctx, req.Email).Return(false, nil)
	mockClinicRepo.On("ExistsBySlug", ctx, mock.Anything).Return(false, nil)
	mockAccountRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockClinicRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockJWTService.On("GenerateTokenPair", mock.Anything).Return(&model.TokenPair{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresIn:    3600,
	}, nil)
	mockUserRepo.On("UpdateRefreshToken", ctx, mock.Anything, "refresh").Return(nil)
	mockUserRepo.On("UpdateLastLogin", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("GetByID", mock.Anything, mock.Anything).Return(&user_model.User{Email: req.Email, Name: req.Name}, nil)
	mockUserRepo.On("GetByID", mock.Anything, mock.Anything).Return(&user_model.User{Email: req.Email, Name: req.Name}, nil)

	// Here is our main assertion: Customer should be created even if plan is trial and doc is empty!
	mockPaymentService.On("CreateCustomerForClinic", ctx, mock.Anything, mock.MatchedBy(func(r *payment_model.CreateCustomerRequest) bool {
		return r.Email == req.Email && r.Name == req.Name
	})).Return(nil)

	mockEmailVerifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockEmailService.On("SendEmailVerification", mock.Anything, mock.Anything).Return(nil)

	resp, err := svc.Register(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Wait for async email send goroutine
	time.Sleep(100 * time.Millisecond)

	mockPaymentService.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_GoogleLogin_CreatesAsaasCustomerUnconditionally(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAccountRepo := new(MockAccountRepository)
	mockJWTService := new(MockJWTService)
	mockEmailService := new(MockEmailService)
	mockEmailVerifRepo := new(MockEmailVerificationRepository)
	mockPassResetRepo := new(MockPasswordResetRepository)
	mockPaymentService := new(MockPaymentService)
	mockGoogleAuth := new(MockGoogleAuthClient)
	mockPlanRepo := new(MockPlanRepository)

	svc := NewAuthServiceWithGoogle(
		mockUserRepo,
		mockClinicRepo,
		mockAccountRepo,
		mockPassResetRepo,
		mockEmailVerifRepo,
		mockPlanRepo,
		mockJWTService,
		mockGoogleAuth,
		mockEmailService,
		"http://localhost:3000",
	).(*authService)

	svc.SetPaymentService(mockPaymentService)

	ctx := context.Background()

	req := &model.GoogleLoginRequest{
		IDToken: "valid_token",
	}

	googleUser := &GoogleUserInfo{
		ID:            "g123",
		Email:         "google@example.com",
		Name:          "Google User",
		VerifiedEmail: true,
	}

	// Setup mocks
	mockPlanRepo.On("GetBySlug", ctx, "basic").Return(&plan_model.Plan{ID: uuid.New(), Slug: "basic", Name: "Basic"}, nil)
	mockGoogleAuth.On("VerifyIDToken", ctx, req.IDToken).Return(googleUser, nil)

	mockUserRepo.On("GetByEmail", ctx, googleUser.Email).Return(nil, errors.ErrNotFoundInstance)
	mockClinicRepo.On("ExistsBySlug", ctx, mock.Anything).Return(false, nil)
	mockAccountRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockClinicRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockJWTService.On("GenerateTokenPair", mock.Anything).Return(&model.TokenPair{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresIn:    3600,
	}, nil)
	mockUserRepo.On("UpdateRefreshToken", ctx, mock.Anything, "refresh").Return(nil)
	mockUserRepo.On("UpdateLastLogin", ctx, mock.Anything).Return(nil)
	mockClinicRepo.On("GetByID", ctx, mock.Anything).Return(&clinic_model.Clinic{Name: "Test Clinic"}, nil)

	// Assert Asaas Customer creation happens unconditionally
	mockPaymentService.On("CreateCustomerForClinic", ctx, mock.Anything, mock.MatchedBy(func(r *payment_model.CreateCustomerRequest) bool {
		return r.Email == googleUser.Email && r.Name == googleUser.Name
	})).Return(nil)

	resp, err := svc.GoogleLogin(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	mockPaymentService.AssertExpectations(t)
}
