package service

import (
	"testing"

	"github.com/aseno/backend/pkg/partnernetwork/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type mockPartnerNetworkRepo struct {
	mock.Mock
}

func (m *mockPartnerNetworkRepo) Create(n *model.PartnerNetwork) error {
	return m.Called(n).Error(0)
}

func (m *mockPartnerNetworkRepo) GetByID(id, clinicID uuid.UUID) (*model.PartnerNetwork, error) {
	args := m.Called(id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PartnerNetwork), args.Error(1)
}

func (m *mockPartnerNetworkRepo) GetByIDWithPlans(id, clinicID uuid.UUID) (*model.PartnerNetwork, error) {
	args := m.Called(id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PartnerNetwork), args.Error(1)
}

func (m *mockPartnerNetworkRepo) GetByName(name string, clinicID uuid.UUID) (*model.PartnerNetwork, error) {
	args := m.Called(name, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PartnerNetwork), args.Error(1)
}

func (m *mockPartnerNetworkRepo) Update(n *model.PartnerNetwork) error {
	return m.Called(n).Error(0)
}

func (m *mockPartnerNetworkRepo) Delete(id, clinicID uuid.UUID) error {
	return m.Called(id, clinicID).Error(0)
}

func (m *mockPartnerNetworkRepo) ListByClinic(clinicID uuid.UUID) ([]*model.PartnerNetwork, error) {
	args := m.Called(clinicID)
	return args.Get(0).([]*model.PartnerNetwork), args.Error(1)
}

func (m *mockPartnerNetworkRepo) ListActiveByClinic(clinicID uuid.UUID) ([]*model.PartnerNetwork, error) {
	args := m.Called(clinicID)
	return args.Get(0).([]*model.PartnerNetwork), args.Error(1)
}

func TestPartnerNetworkService_Create(t *testing.T) {
	repo := new(mockPartnerNetworkRepo)
	svc := NewPartnerNetworkService(repo)

	clinicID := uuid.New()
	req := &model.CreatePartnerNetworkRequest{Name: "Gympass"}

	repo.On("GetByName", "Gympass", clinicID).Return(nil, gorm.ErrRecordNotFound)
	repo.On("Create", mock.AnythingOfType("*model.PartnerNetwork")).Return(nil)

	resp, err := svc.Create(clinicID, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Gympass", resp.Name)
	assert.Equal(t, clinicID, resp.ClinicID)
	assert.True(t, resp.IsActive)
	repo.AssertExpectations(t)
}

func TestPartnerNetworkService_Create_DuplicateName(t *testing.T) {
	repo := new(mockPartnerNetworkRepo)
	svc := NewPartnerNetworkService(repo)

	clinicID := uuid.New()
	existing := &model.PartnerNetwork{ID: uuid.New(), ClinicID: clinicID, Name: "Gympass"}

	repo.On("GetByName", "Gympass", clinicID).Return(existing, nil)

	resp, err := svc.Create(clinicID, &model.CreatePartnerNetworkRequest{Name: "Gympass"})

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrPartnerNetworkNameExists)
	repo.AssertExpectations(t)
}

func TestPartnerNetworkService_GetByID_NotFound(t *testing.T) {
	repo := new(mockPartnerNetworkRepo)
	svc := NewPartnerNetworkService(repo)

	clinicID := uuid.New()
	id := uuid.New()

	repo.On("GetByIDWithPlans", id, clinicID).Return(nil, gorm.ErrRecordNotFound)

	resp, err := svc.GetByID(clinicID, id)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrPartnerNetworkNotFound)
}

func TestPartnerNetworkService_Update_RenameConflict(t *testing.T) {
	repo := new(mockPartnerNetworkRepo)
	svc := NewPartnerNetworkService(repo)

	clinicID := uuid.New()
	id := uuid.New()
	current := &model.PartnerNetwork{ID: id, ClinicID: clinicID, Name: "Gympass", IsActive: true}
	conflict := &model.PartnerNetwork{ID: uuid.New(), ClinicID: clinicID, Name: "Total Pass"}

	repo.On("GetByID", id, clinicID).Return(current, nil)
	repo.On("GetByName", "Total Pass", clinicID).Return(conflict, nil)

	newName := "Total Pass"
	resp, err := svc.Update(clinicID, id, &model.UpdatePartnerNetworkRequest{Name: &newName})

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrPartnerNetworkNameExists)
}

func TestPartnerNetworkService_Update_Deactivate(t *testing.T) {
	repo := new(mockPartnerNetworkRepo)
	svc := NewPartnerNetworkService(repo)

	clinicID := uuid.New()
	id := uuid.New()
	current := &model.PartnerNetwork{ID: id, ClinicID: clinicID, Name: "Gympass", IsActive: true}

	repo.On("GetByID", id, clinicID).Return(current, nil)
	repo.On("Update", mock.AnythingOfType("*model.PartnerNetwork")).Return(nil)

	inactive := false
	resp, err := svc.Update(clinicID, id, &model.UpdatePartnerNetworkRequest{IsActive: &inactive})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsActive)
	repo.AssertExpectations(t)
}

func TestPartnerNetworkService_Delete_NotFound(t *testing.T) {
	repo := new(mockPartnerNetworkRepo)
	svc := NewPartnerNetworkService(repo)

	clinicID := uuid.New()
	id := uuid.New()

	repo.On("GetByID", id, clinicID).Return(nil, gorm.ErrRecordNotFound)

	err := svc.Delete(clinicID, id)
	assert.ErrorIs(t, err, ErrPartnerNetworkNotFound)
}

func TestPartnerNetworkService_List(t *testing.T) {
	repo := new(mockPartnerNetworkRepo)
	svc := NewPartnerNetworkService(repo)

	clinicID := uuid.New()
	networks := []*model.PartnerNetwork{
		{ID: uuid.New(), ClinicID: clinicID, Name: "Gogood"},
		{ID: uuid.New(), ClinicID: clinicID, Name: "Gympass"},
	}

	repo.On("ListByClinic", clinicID).Return(networks, nil)

	resp, err := svc.List(clinicID)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "Gogood", resp[0].Name)
}
