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
	"github.com/aseno/backend/pkg/pagination"
	payment_model "github.com/aseno/backend/pkg/payment/model"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	plan_repo "github.com/aseno/backend/pkg/plan/repository"
	professional_model "github.com/aseno/backend/pkg/professional/model"
	professional_repository "github.com/aseno/backend/pkg/professional/repository"
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

func (m *MockPaymentService) CreateSubscriptionForClinic(ctx context.Context, clinicID uuid.UUID, planID uuid.UUID, offerToken *string, billing string, units int) error {
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

func (m *MockUserRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*user_model.User, error) {
	args := m.Called(ctx, id, clinicID)
	return args.Get(0).(*user_model.User), args.Error(1)
}
func (m *MockUserRepository) GetByIDNoTenant(ctx context.Context, id uuid.UUID) (*user_model.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*user_model.User), args.Error(1)
}
func (m *MockUserRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	args := m.Called(ctx, id, clinicID)
	return args.Error(0)
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

func (m *MockAccountRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockClinicRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockJWTService
type MockJWTService struct {
	JWTService
	mock.Mock
}

func (m *MockJWTService) GenerateTokenPair(tc *TokenContext) (*model.TokenPair, error) {
	args := m.Called(tc)
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

// MockProfessionalRepository
type MockProfessionalRepository struct {
	professional_repository.ProfessionalRepository
	mock.Mock
}

func (m *MockProfessionalRepository) Create(ctx context.Context, professional *professional_model.Professional) error {
	args := m.Called(ctx, professional)
	return args.Error(0)
}

func (m *MockProfessionalRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*professional_model.Professional, error) {
	args := m.Called(ctx, id, clinicID)
	var prof *professional_model.Professional
	if args.Get(0) != nil {
		prof = args.Get(0).(*professional_model.Professional)
	}
	return prof, args.Error(1)
}

func (m *MockProfessionalRepository) GetByUserID(ctx context.Context, userID, clinicID uuid.UUID) (*professional_model.Professional, error) {
	args := m.Called(ctx, userID, clinicID)
	var prof *professional_model.Professional
	if args.Get(0) != nil {
		prof = args.Get(0).(*professional_model.Professional)
	}
	return prof, args.Error(1)
}

func (m *MockProfessionalRepository) Update(ctx context.Context, professional *professional_model.Professional) error {
	args := m.Called(ctx, professional)
	return args.Error(0)
}

func (m *MockProfessionalRepository) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*professional_model.Professional, int64, error) {
	args := m.Called(ctx, clinicID, filter)
	return args.Get(0).([]*professional_model.Professional), args.Get(1).(int64), args.Error(2)
}

func (m *MockProfessionalRepository) CountActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (int64, error) {
	args := m.Called(ctx, clinicID)
	return args.Get(0).(int64), args.Error(1)
}

func TestAuthService_Register_CreatesProfessionalLinkedToUser(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAccountRepo := new(MockAccountRepository)
	mockJWTService := new(MockJWTService)
	mockEmailService := new(MockEmailService)
	mockEmailVerifRepo := new(MockEmailVerificationRepository)
	mockPassResetRepo := new(MockPasswordResetRepository)
	mockPlanRepo := new(MockPlanRepository)
	mockProfRepo := new(MockProfessionalRepository)

	svc := NewAuthService(
		mockUserRepo,
		mockClinicRepo,
		mockAccountRepo,
		mockPassResetRepo,
		mockEmailVerifRepo,
		mockPlanRepo,
		mockProfRepo,
		nil, // clinicMemberRepo
		mockJWTService,
		mockEmailService,
		"http://localhost:3000",
	).(*authService)

	ctx := context.Background()

	req := &model.RegisterRequest{
		Email:      "test@example.com",
		ClinicName: "Test Clinic",
		Name:       "Test User",
		Phone:      "11999999999",
		Password:   "password123",
		Plan:       "trial",
	}

	planID := uuid.New()

	// Setup mocks
	mockPlanRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(&plan_model.Plan{ID: planID, Slug: "professional", Name: "Professional"}, nil)
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
	mockUserRepo.On("GetByIDNoTenant", mock.Anything, mock.Anything).Return(&user_model.User{Email: req.Email, Name: req.Name}, nil)
	mockEmailVerifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockEmailService.On("SendEmailVerification", mock.Anything, mock.Anything).Return(nil)

	// KEY ASSERTION: Professional must be created with UserID linking to the new user
	var createdProf *professional_model.Professional
	mockProfRepo.On("Create", ctx, mock.MatchedBy(func(p *professional_model.Professional) bool {
		createdProf = p
		return p.UserID != nil &&
			p.Name == req.Name &&
			p.Email == req.Email &&
			p.Phone == req.Phone &&
			p.IsActive == true &&
			p.Color == "#7C3AED"
	})).Return(nil)

	// buildTokenContext will call GetByUserID to set professional ID in token
	mockProfRepo.On("GetByUserID", ctx, mock.Anything, mock.Anything).Return(nil, errors.ErrProfessionalNotFoundInstance).Maybe()

	resp, err := svc.Register(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, createdProf)

	// Wait for async goroutine
	time.Sleep(100 * time.Millisecond)

	mockProfRepo.AssertExpectations(t)
}

func TestAuthService_Register_ProfessionalInheritsUserData(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAccountRepo := new(MockAccountRepository)
	mockJWTService := new(MockJWTService)
	mockEmailService := new(MockEmailService)
	mockEmailVerifRepo := new(MockEmailVerificationRepository)
	mockPassResetRepo := new(MockPasswordResetRepository)
	mockPlanRepo := new(MockPlanRepository)
	mockProfRepo := new(MockProfessionalRepository)

	svc := NewAuthService(
		mockUserRepo,
		mockClinicRepo,
		mockAccountRepo,
		mockPassResetRepo,
		mockEmailVerifRepo,
		mockPlanRepo,
		mockProfRepo,
		nil,
		mockJWTService,
		mockEmailService,
		"http://localhost:3000",
	).(*authService)

	ctx := context.Background()

	req := &model.RegisterRequest{
		Email:      "maria@example.com",
		ClinicName: "Clinica Maria",
		Name:       "Maria Silva",
		Phone:      "11888888888",
		Password:   "password123",
		Plan:       "trial",
	}

	mockPlanRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(&plan_model.Plan{ID: uuid.New(), Slug: "professional", Name: "Professional"}, nil)
	mockUserRepo.On("ExistsByEmail", ctx, req.Email).Return(false, nil)
	mockClinicRepo.On("ExistsBySlug", ctx, mock.Anything).Return(false, nil)
	mockAccountRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockClinicRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockJWTService.On("GenerateTokenPair", mock.Anything).Return(&model.TokenPair{AccessToken: "a", RefreshToken: "r", ExpiresIn: 3600}, nil)
	mockUserRepo.On("UpdateRefreshToken", ctx, mock.Anything, "r").Return(nil)
	mockUserRepo.On("UpdateLastLogin", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("GetByIDNoTenant", mock.Anything, mock.Anything).Return(&user_model.User{Email: req.Email, Name: req.Name}, nil)
	mockEmailVerifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockEmailService.On("SendEmailVerification", mock.Anything, mock.Anything).Return(nil)

	// Assert professional inherits exact user data
	var capturedProf *professional_model.Professional
	mockProfRepo.On("Create", ctx, mock.MatchedBy(func(p *professional_model.Professional) bool {
		capturedProf = p
		return true
	})).Return(nil)
	mockProfRepo.On("GetByUserID", ctx, mock.Anything, mock.Anything).Return(nil, errors.ErrProfessionalNotFoundInstance).Maybe()

	resp, err := svc.Register(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "Maria Silva", capturedProf.Name)
	assert.Equal(t, "maria@example.com", capturedProf.Email)
	assert.Equal(t, "11888888888", capturedProf.Phone)
	assert.NotNil(t, capturedProf.UserID)
	// Working hours default should be set
	assert.NotEmpty(t, capturedProf.WorkingHours.Monday)
	assert.NotEmpty(t, capturedProf.WorkingHours.Friday)
	assert.Empty(t, capturedProf.WorkingHours.Saturday)
	assert.Empty(t, capturedProf.WorkingHours.Sunday)
}

func TestAuthService_Register_SucceedsEvenIfProfessionalFails(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAccountRepo := new(MockAccountRepository)
	mockJWTService := new(MockJWTService)
	mockEmailService := new(MockEmailService)
	mockEmailVerifRepo := new(MockEmailVerificationRepository)
	mockPassResetRepo := new(MockPasswordResetRepository)
	mockPlanRepo := new(MockPlanRepository)
	mockProfRepo := new(MockProfessionalRepository)

	svc := NewAuthService(
		mockUserRepo,
		mockClinicRepo,
		mockAccountRepo,
		mockPassResetRepo,
		mockEmailVerifRepo,
		mockPlanRepo,
		mockProfRepo,
		nil,
		mockJWTService,
		mockEmailService,
		"http://localhost:3000",
	).(*authService)

	ctx := context.Background()

	req := &model.RegisterRequest{
		Email:      "test@example.com",
		ClinicName: "Test Clinic",
		Name:       "Test User",
		Phone:      "11999999999",
		Password:   "password123",
		Plan:       "trial",
	}

	mockPlanRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(&plan_model.Plan{ID: uuid.New(), Slug: "professional", Name: "Professional"}, nil)
	mockUserRepo.On("ExistsByEmail", ctx, req.Email).Return(false, nil)
	mockClinicRepo.On("ExistsBySlug", ctx, mock.Anything).Return(false, nil)
	mockAccountRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockClinicRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockJWTService.On("GenerateTokenPair", mock.Anything).Return(&model.TokenPair{AccessToken: "a", RefreshToken: "r", ExpiresIn: 3600}, nil)
	mockUserRepo.On("UpdateRefreshToken", ctx, mock.Anything, "r").Return(nil)
	mockUserRepo.On("UpdateLastLogin", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("GetByIDNoTenant", mock.Anything, mock.Anything).Return(&user_model.User{Email: req.Email, Name: req.Name}, nil)
	mockEmailVerifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockEmailService.On("SendEmailVerification", mock.Anything, mock.Anything).Return(nil)

	// Professional creation FAILS — but registration should still succeed
	mockProfRepo.On("Create", ctx, mock.Anything).Return(errors.ErrInternalInstance.WithMessage("db error"))
	mockProfRepo.On("GetByUserID", ctx, mock.Anything, mock.Anything).Return(nil, errors.ErrProfessionalNotFoundInstance).Maybe()

	resp, err := svc.Register(ctx, req)

	// Registration succeeds despite professional creation failure
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	time.Sleep(100 * time.Millisecond)
}

func TestAuthService_GoogleLogin_CreatesProfessionalLinkedToUser(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAccountRepo := new(MockAccountRepository)
	mockJWTService := new(MockJWTService)
	mockEmailService := new(MockEmailService)
	mockEmailVerifRepo := new(MockEmailVerificationRepository)
	mockPassResetRepo := new(MockPasswordResetRepository)
	mockGoogleAuth := new(MockGoogleAuthClient)
	mockPlanRepo := new(MockPlanRepository)
	mockProfRepo := new(MockProfessionalRepository)

	svc := NewAuthServiceWithGoogle(
		mockUserRepo,
		mockClinicRepo,
		mockAccountRepo,
		mockPassResetRepo,
		mockEmailVerifRepo,
		mockPlanRepo,
		mockProfRepo,
		nil, // clinicMemberRepo
		mockJWTService,
		mockGoogleAuth,
		mockEmailService,
		"http://localhost:3000",
	).(*authService)

	ctx := context.Background()

	req := &model.GoogleLoginRequest{IDToken: "valid_token"}

	googleUser := &GoogleUserInfo{
		ID:            "g123",
		Email:         "google@example.com",
		Name:          "Google User",
		VerifiedEmail: true,
		Picture:       "https://pic.example.com/photo.jpg",
	}

	mockPlanRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(&plan_model.Plan{ID: uuid.New(), Slug: "basic", Name: "Basic"}, nil)
	mockGoogleAuth.On("VerifyIDToken", ctx, req.IDToken).Return(googleUser, nil)
	mockUserRepo.On("GetByEmail", ctx, googleUser.Email).Return(nil, errors.ErrNotFoundInstance)
	mockClinicRepo.On("ExistsBySlug", ctx, mock.Anything).Return(false, nil)
	mockAccountRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockClinicRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockJWTService.On("GenerateTokenPair", mock.Anything).Return(&model.TokenPair{AccessToken: "a", RefreshToken: "r", ExpiresIn: 3600}, nil)
	mockUserRepo.On("UpdateRefreshToken", ctx, mock.Anything, "r").Return(nil)
	mockUserRepo.On("UpdateLastLogin", ctx, mock.Anything).Return(nil)
	mockClinicRepo.On("GetByID", ctx, mock.Anything).Return(&clinic_model.Clinic{Name: "Test Clinic"}, nil)

	// KEY ASSERTION: Professional must be created linked to user
	mockProfRepo.On("Create", ctx, mock.MatchedBy(func(p *professional_model.Professional) bool {
		return p.UserID != nil &&
			p.Name == googleUser.Name &&
			p.Email == googleUser.Email &&
			p.IsActive == true
	})).Return(nil)
	mockProfRepo.On("GetByUserID", ctx, mock.Anything, mock.Anything).Return(nil, errors.ErrProfessionalNotFoundInstance).Maybe()

	resp, err := svc.GoogleLogin(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	mockProfRepo.AssertExpectations(t)
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
		nil, // professionalRepo
		nil, // clinicMemberRepo
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
	mockPlanRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(&plan_model.Plan{ID: uuid.New(), Slug: "basic", Name: "Basic"}, nil)
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
	mockUserRepo.On("GetByIDNoTenant", mock.Anything, mock.Anything).Return(&user_model.User{Email: req.Email, Name: req.Name}, nil)

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
		nil, // professionalRepo
		nil, // clinicMemberRepo
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
	mockPlanRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(&plan_model.Plan{ID: uuid.New(), Slug: "basic", Name: "Basic"}, nil)
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

// TestAuthService_Register_DefaultsWhatsAppRemindersEnabled garante que toda
// clinica nova ja vem com o lembrete WhatsApp habilitado por padrao. Se essa
// flag for false, o reminder_worker filtra o appointment fora da query e
// nenhum lembrete eh enviado — quebrando o core do produto.
func TestAuthService_Register_DefaultsWhatsAppRemindersEnabled(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAccountRepo := new(MockAccountRepository)
	mockJWTService := new(MockJWTService)
	mockEmailService := new(MockEmailService)
	mockEmailVerifRepo := new(MockEmailVerificationRepository)
	mockPassResetRepo := new(MockPasswordResetRepository)
	mockPlanRepo := new(MockPlanRepository)
	mockProfRepo := new(MockProfessionalRepository)

	svc := NewAuthService(
		mockUserRepo,
		mockClinicRepo,
		mockAccountRepo,
		mockPassResetRepo,
		mockEmailVerifRepo,
		mockPlanRepo,
		mockProfRepo,
		nil,
		mockJWTService,
		mockEmailService,
		"http://localhost:3000",
	).(*authService)

	ctx := context.Background()
	req := &model.RegisterRequest{
		Email:      "newclinic@example.com",
		ClinicName: "New Clinic",
		Name:       "Owner",
		Phone:      "11999999999",
		Password:   "password123",
		Plan:       "trial",
	}

	planID := uuid.New()
	mockPlanRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(&plan_model.Plan{ID: planID, Slug: "professional", Name: "Professional"}, nil)
	mockUserRepo.On("ExistsByEmail", ctx, req.Email).Return(false, nil)
	mockClinicRepo.On("ExistsBySlug", ctx, mock.Anything).Return(false, nil)
	mockAccountRepo.On("Create", ctx, mock.Anything).Return(nil)

	var createdClinic *clinic_model.Clinic
	mockClinicRepo.On("Create", ctx, mock.MatchedBy(func(c *clinic_model.Clinic) bool {
		createdClinic = c
		return true
	})).Return(nil)

	mockUserRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockJWTService.On("GenerateTokenPair", mock.Anything).Return(&model.TokenPair{AccessToken: "a", RefreshToken: "r", ExpiresIn: 3600}, nil)
	mockUserRepo.On("UpdateRefreshToken", ctx, mock.Anything, "r").Return(nil)
	mockUserRepo.On("UpdateLastLogin", ctx, mock.Anything).Return(nil)
	mockUserRepo.On("GetByIDNoTenant", mock.Anything, mock.Anything).Return(&user_model.User{Email: req.Email, Name: req.Name}, nil)
	mockEmailVerifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockEmailService.On("SendEmailVerification", mock.Anything, mock.Anything).Return(nil)
	mockProfRepo.On("Create", ctx, mock.Anything).Return(nil)
	mockProfRepo.On("GetByUserID", ctx, mock.Anything, mock.Anything).Return(nil, errors.ErrProfessionalNotFoundInstance).Maybe()

	_, err := svc.Register(ctx, req)
	assert.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	assert.NotNil(t, createdClinic)
	assert.True(t, createdClinic.Settings.WhatsAppRemindersEnabled,
		"clinica nova deve vir com WhatsAppRemindersEnabled=true por padrao — sem isso o reminder_worker filtra o appointment fora")
}
