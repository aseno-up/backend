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
	specialty_model "github.com/aseno/backend/pkg/specialty/model"
	specialty_repository "github.com/aseno/backend/pkg/specialty/repository"
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

func (m *MockProfessionalRepository) CountActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (int64, error) {
	args := m.Called(ctx, clinicID)
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

// MockSpecialtyRepo
type MockSpecialtyRepo struct {
	specialty_repository.SpecialtyRepository
	mock.Mock
}

func (m *MockSpecialtyRepo) GetByID(id, clinicID uuid.UUID) (*specialty_model.Specialty, error) {
	args := m.Called(id, clinicID)
	var sp *specialty_model.Specialty
	if args.Get(0) != nil {
		sp = args.Get(0).(*specialty_model.Specialty)
	}
	return sp, args.Error(1)
}

func (m *MockSpecialtyRepo) SetProfessionalSpecialties(professionalID, clinicID uuid.UUID, specialtyIDs []uuid.UUID) error {
	return m.Called(professionalID, clinicID, specialtyIDs).Error(0)
}

func (m *MockSpecialtyRepo) GetByProfessionalID(professionalID uuid.UUID) ([]*specialty_model.Specialty, error) {
	args := m.Called(professionalID)
	var specs []*specialty_model.Specialty
	if args.Get(0) != nil {
		specs = args.Get(0).([]*specialty_model.Specialty)
	}
	return specs, args.Error(1)
}

func (m *MockSpecialtyRepo) GetByProfessionalIDs(professionalIDs []uuid.UUID) (map[uuid.UUID][]*specialty_model.Specialty, error) {
	args := m.Called(professionalIDs)
	var m2 map[uuid.UUID][]*specialty_model.Specialty
	if args.Get(0) != nil {
		m2 = args.Get(0).(map[uuid.UUID][]*specialty_model.Specialty)
	}
	return m2, args.Error(1)
}

// --- Tests ---

func TestCreateForExistingUser_Success(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)

	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

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

	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

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

	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

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

	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

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

	// Plan limit is 2 per clinic, already 2 professionals in this clinic
	mockClinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		AccountID: accountID,
		Account:   &account_model.Account{MaxProfessionals: 2},
	}, nil)
	mockRepo.On("CountActiveByClinicID", ctx, clinicID).Return(int64(2), nil)

	resp, err := svc.CreateForExistingUser(ctx, clinicID, userID)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Limite")
}

// TestCreate_LimitIsPerClinic_NotAcrossAccount ensures that the plan limit is
// evaluated per clinic (unidade), not aggregated across all clinics of the account.
// An account with 2 clinics should allow each clinic to have up to plan.MaxProfessionals
// independently.
func TestCreate_LimitIsPerClinic_NotAcrossAccount(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)

	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

	ctx := context.Background()
	clinicBID := uuid.New()
	accountID := uuid.New()

	// Plan limit = 7 per clinic. Clinic B currently has 0 professionals.
	// (If the limit were aggregated per account, another clinic with 7 would block this.)
	mockClinicRepo.On("GetByID", ctx, clinicBID).Return(&clinic_model.Clinic{
		AccountID: accountID,
		Account:   &account_model.Account{MaxProfessionals: 7},
	}, nil)
	mockRepo.On("CountActiveByClinicID", ctx, clinicBID).Return(int64(0), nil)
	mockRepo.On("Create", ctx, mock.AnythingOfType("*model.Professional")).Return(nil)
	// Create always goes through buildResponse, which queries specialties of the new pro.
	mockSpecialtyRepo.On("GetByProfessionalID", mock.AnythingOfType("uuid.UUID")).Return([]*specialty_model.Specialty{}, nil)

	resp, err := svc.Create(ctx, clinicBID, &model.CreateProfessionalRequest{
		Name:  "Prof B1",
		Email: "b1@example.com",
		Phone: "11988887777",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Confirms the service queried by clinic, not by account
	mockRepo.AssertCalled(t, "CountActiveByClinicID", ctx, clinicBID)
}

// --- Specialty sync on Update ---

func TestUpdate_SpecialtyIDs_Nil_DoesNotTouchSpecialties(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)
	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

	ctx := context.Background()
	clinicID := uuid.New()
	profID := uuid.New()

	mockRepo.On("GetByID", ctx, profID, clinicID).Return(&model.Professional{ID: profID, ClinicID: clinicID, Name: "Old"}, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*model.Professional")).Return(nil)
	// buildResponse call
	mockSpecialtyRepo.On("GetByProfessionalID", profID).Return([]*specialty_model.Specialty{}, nil)

	_, err := svc.Update(ctx, profID, clinicID, &model.UpdateProfessionalRequest{Name: "New"})

	assert.NoError(t, err)
	// Specialty sync functions must NOT have been called when SpecialtyIDs is nil
	mockSpecialtyRepo.AssertNotCalled(t, "SetProfessionalSpecialties", mock.Anything, mock.Anything, mock.Anything)
	mockSpecialtyRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestUpdate_SpecialtyIDs_Empty_RemovesAll(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)
	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

	ctx := context.Background()
	clinicID := uuid.New()
	profID := uuid.New()

	mockRepo.On("GetByID", ctx, profID, clinicID).Return(&model.Professional{ID: profID, ClinicID: clinicID}, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*model.Professional")).Return(nil)
	mockSpecialtyRepo.On("SetProfessionalSpecialties", profID, clinicID, []uuid.UUID{}).Return(nil)
	mockSpecialtyRepo.On("GetByProfessionalID", profID).Return([]*specialty_model.Specialty{}, nil)

	emptyIDs := []uuid.UUID{}
	_, err := svc.Update(ctx, profID, clinicID, &model.UpdateProfessionalRequest{SpecialtyIDs: &emptyIDs})

	assert.NoError(t, err)
	mockSpecialtyRepo.AssertCalled(t, "SetProfessionalSpecialties", profID, clinicID, []uuid.UUID{})
}

func TestUpdate_SpecialtyIDs_Replaces(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)
	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

	ctx := context.Background()
	clinicID := uuid.New()
	profID := uuid.New()
	specA := uuid.New()
	specB := uuid.New()

	mockRepo.On("GetByID", ctx, profID, clinicID).Return(&model.Professional{ID: profID, ClinicID: clinicID}, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*model.Professional")).Return(nil)
	mockSpecialtyRepo.On("GetByID", specA, clinicID).Return(&specialty_model.Specialty{ID: specA, ClinicID: clinicID, Name: "Fisioterapia"}, nil)
	mockSpecialtyRepo.On("GetByID", specB, clinicID).Return(&specialty_model.Specialty{ID: specB, ClinicID: clinicID, Name: "Pilates"}, nil)
	mockSpecialtyRepo.On("SetProfessionalSpecialties", profID, clinicID, []uuid.UUID{specA, specB}).Return(nil)
	mockSpecialtyRepo.On("GetByProfessionalID", profID).Return([]*specialty_model.Specialty{
		{ID: specA, Name: "Fisioterapia"},
		{ID: specB, Name: "Pilates"},
	}, nil)

	ids := []uuid.UUID{specA, specB}
	resp, err := svc.Update(ctx, profID, clinicID, &model.UpdateProfessionalRequest{SpecialtyIDs: &ids})

	assert.NoError(t, err)
	assert.Len(t, resp.Specialties, 2)
	assert.Equal(t, "Fisioterapia", resp.Specialties[0].Name)
	mockSpecialtyRepo.AssertCalled(t, "SetProfessionalSpecialties", profID, clinicID, []uuid.UUID{specA, specB})
}

func TestUpdate_SpecialtyIDs_FromAnotherClinic_Rejects(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)
	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

	ctx := context.Background()
	clinicID := uuid.New()
	profID := uuid.New()
	foreignSpec := uuid.New()

	mockRepo.On("GetByID", ctx, profID, clinicID).Return(&model.Professional{ID: profID, ClinicID: clinicID}, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*model.Professional")).Return(nil)
	// Specialty doesn't belong to this clinic
	mockSpecialtyRepo.On("GetByID", foreignSpec, clinicID).Return(nil, assert.AnError)

	ids := []uuid.UUID{foreignSpec}
	resp, err := svc.Update(ctx, profID, clinicID, &model.UpdateProfessionalRequest{SpecialtyIDs: &ids})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "does not belong")
	mockSpecialtyRepo.AssertNotCalled(t, "SetProfessionalSpecialties", mock.Anything, mock.Anything, mock.Anything)
}

func TestCreate_WithSpecialtyIDs_Persists(t *testing.T) {
	mockRepo := new(MockProfessionalRepository)
	mockClinicRepo := new(MockClinicRepository)
	mockAppointmentRepo := new(MockAppointmentRepository)
	mockUserRepo := new(MockUserRepository)
	mockSpecialtyRepo := new(MockSpecialtyRepo)
	svc := NewProfessionalService(mockRepo, mockClinicRepo, mockAppointmentRepo, mockUserRepo, mockSpecialtyRepo)

	ctx := context.Background()
	clinicID := uuid.New()
	accountID := uuid.New()
	specA := uuid.New()

	mockClinicRepo.On("GetByID", ctx, clinicID).Return(&clinic_model.Clinic{
		AccountID: accountID,
		Account:   &account_model.Account{MaxProfessionals: -1},
	}, nil)
	mockRepo.On("Create", ctx, mock.MatchedBy(func(p *model.Professional) bool {
		if p.ID == uuid.Nil {
			// Simulate GORM assigning an ID after Create
			p.ID = uuid.New()
		}
		return p.Name == "Dr. Thaiane"
	})).Return(nil)
	mockSpecialtyRepo.On("GetByID", specA, clinicID).Return(&specialty_model.Specialty{ID: specA, ClinicID: clinicID, Name: "Fisioterapia"}, nil)
	mockSpecialtyRepo.On("SetProfessionalSpecialties", mock.AnythingOfType("uuid.UUID"), clinicID, []uuid.UUID{specA}).Return(nil)
	mockSpecialtyRepo.On("GetByProfessionalID", mock.AnythingOfType("uuid.UUID")).Return([]*specialty_model.Specialty{
		{ID: specA, Name: "Fisioterapia"},
	}, nil)

	resp, err := svc.Create(ctx, clinicID, &model.CreateProfessionalRequest{
		Name:         "Dr. Thaiane",
		SpecialtyIDs: []uuid.UUID{specA},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Specialties, 1)
	assert.Equal(t, "Fisioterapia", resp.Specialties[0].Name)
}
