package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aseno/backend/internal/errors"
	account_model "github.com/aseno/backend/pkg/account/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/professional/model"
	"github.com/aseno/backend/pkg/professional/repository"
	user_model "github.com/aseno/backend/pkg/user/model"
	user_repository "github.com/aseno/backend/pkg/user/repository"
)

// MockProfessionalRepository
type MockProfessionalRepository struct {
	repository.ProfessionalRepository
	mock.Mock
}

func (m *MockProfessionalRepository) Create(ctx context.Context, professional *model.Professional) error {
	args := m.Called(ctx, professional)
	return args.Error(0)
}

func (m *MockProfessionalRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Professional, error) {
	args := m.Called(ctx, id, clinicID)
	var prof *model.Professional
	if args.Get(0) != nil {
		prof = args.Get(0).(*model.Professional)
	}
	return prof, args.Error(1)
}

func (m *MockProfessionalRepository) GetByUserID(ctx context.Context, userID, clinicID uuid.UUID) (*model.Professional, error) {
	args := m.Called(ctx, userID, clinicID)
	var prof *model.Professional
	if args.Get(0) != nil {
		prof = args.Get(0).(*model.Professional)
	}
	return prof, args.Error(1)
}

func (m *MockProfessionalRepository) CountActiveByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockProfessionalRepository) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Professional, int64, error) {
	args := m.Called(ctx, clinicID, filter)
	return args.Get(0).([]*model.Professional), args.Get(1).(int64), args.Error(2)
}

func (m *MockProfessionalRepository) Update(ctx context.Context, professional *model.Professional) error {
	args := m.Called(ctx, professional)
	return args.Error(0)
}

// MockClinicRepository
type MockClinicRepository struct {
	clinic_repository.ClinicRepository
	mock.Mock
}

func (m *MockClinicRepository) GetByID(ctx context.Context, id uuid.UUID) (*clinic_model.Clinic, error) {
	args := m.Called(ctx, id)
	var clinic *clinic_model.Clinic
	if args.Get(0) != nil {
		clinic = args.Get(0).(*clinic_model.Clinic)
	}
	return clinic, args.Error(1)
}

// MockAppointmentRepository
type MockAppointmentRepository struct {
	appointment_repository.AppointmentRepository
	mock.Mock
}

func (m *MockAppointmentRepository) CountByProfessionalID(ctx context.Context, professionalID, clinicID uuid.UUID) (int64, error) {
	args := m.Called(ctx, professionalID, clinicID)
	return args.Get(0).(int64), args.Error(1)
}

// MockUserRepository
type MockUserRepository struct {
	user_repository.UserRepository
	mock.Mock
}

func (m *MockUserRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*user_model.User, error) {
	args := m.Called(ctx, id, clinicID)
	var user *user_model.User
	if args.Get(0) != nil {
		user = args.Get(0).(*user_model.User)
	}
	return user, args.Error(1)
}

// --- Tests ---

func TestCreateForExistingUser_Success(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)

	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo)

	ctx := context.Background()
	clinicID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()

	// User is owner, no professional linked
	mockUserRepo.On("GetByID", ctx, userID, clinicID).Return(&user_model.User{
		ID:       userID,
		ClinicID: clinicID,
		Name:     "Admin User",
		Email:    "admin@example.com",
		Phone:    "11999999999",
		Role:     user_model.UserRoleOwner,
		IsActive: true,
	}, nil)

	// No existing professional for this user
	mockRepo.On("GetByUserID", ctx, userID, clinicID).Return(nil, errors.ErrProfessionalNotFoundInstance)

	// Plan limit check
	mockClinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		AccountID: accountID,
		Account:   &account_model.Account{MaxProfessionals: -1},
	}, nil)

	// Create should be called with correct data
	mockRepo.On("Create", ctx, mock.MatchedBy(func(p *model.Professional) bool {
		return p.UserID != nil &&
			*p.UserID == userID &&
			p.Name == "Admin User" &&
			p.Email == "admin@example.com" &&
			p.Phone == "11999999999" &&
			p.ClinicID == clinicID &&
			p.IsActive == true
	})).Return(nil)

	resp, err := svc.CreateForExistingUser(ctx, clinicID, userID)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Admin User", resp.Name)
	mockRepo.AssertExpectations(t)
}

func TestCreateForExistingUser_UserAlreadyHasProfessional(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)

	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo)

	ctx := context.Background()
	clinicID := uuid.New()
	userID := uuid.New()

	mockUserRepo.On("GetByID", ctx, userID, clinicID).Return(&user_model.User{
		ID:       userID,
		ClinicID: clinicID,
		Name:     "Admin User",
		Role:     user_model.UserRoleOwner,
		IsActive: true,
	}, nil)

	// Professional already exists for this user
	mockRepo.On("GetByUserID", ctx, userID, clinicID).Return(&model.Professional{
		ID:     uuid.New(),
		UserID: &userID,
	}, nil)

	resp, err := svc.CreateForExistingUser(ctx, clinicID, userID)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "vinculado")
}

func TestCreateForExistingUser_InvalidRole(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)

	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo)

	ctx := context.Background()
	clinicID := uuid.New()
	userID := uuid.New()

	// User is receptionist — should not be allowed
	mockUserRepo.On("GetByID", ctx, userID, clinicID).Return(&user_model.User{
		ID:       userID,
		ClinicID: clinicID,
		Name:     "Receptionist",
		Role:     user_model.UserRoleReceptionist,
		IsActive: true,
	}, nil)

	resp, err := svc.CreateForExistingUser(ctx, clinicID, userID)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "owner ou admin")
}

func TestCreateForExistingUser_PlanLimit(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)

	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo)

	ctx := context.Background()
	clinicID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()

	mockUserRepo.On("GetByID", ctx, userID, clinicID).Return(&user_model.User{
		ID:       userID,
		ClinicID: clinicID,
		Name:     "Admin",
		Role:     user_model.UserRoleAdmin,
		IsActive: true,
	}, nil)

	mockRepo.On("GetByUserID", ctx, userID, clinicID).Return(nil, errors.ErrProfessionalNotFoundInstance)

	// Plan limit is 2, already 2 professionals
	mockClinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		AccountID: accountID,
		Account:   &account_model.Account{MaxProfessionals: 2},
	}, nil)
	mockRepo.On("CountActiveByAccountID", ctx, accountID).Return(int64(2), nil)

	resp, err := svc.CreateForExistingUser(ctx, clinicID, userID)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Limite")
}
