package service

import (
	"testing"

	"github.com/aseno/backend/pkg/partnernetwork/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type mockPartnerPlanRepo struct {
	mock.Mock
}

func (m *mockPartnerPlanRepo) Create(p *model.PartnerPlan) error {
	return m.Called(p).Error(0)
}

func (m *mockPartnerPlanRepo) GetByID(id uuid.UUID) (*model.PartnerPlan, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PartnerPlan), args.Error(1)
}

func (m *mockPartnerPlanRepo) GetByIDScoped(id, clinicID uuid.UUID) (*model.PartnerPlan, error) {
	args := m.Called(id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PartnerPlan), args.Error(1)
}

func (m *mockPartnerPlanRepo) GetByNetworkAndName(networkID uuid.UUID, name string) (*model.PartnerPlan, error) {
	args := m.Called(networkID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PartnerPlan), args.Error(1)
}

func (m *mockPartnerPlanRepo) Update(p *model.PartnerPlan) error {
	return m.Called(p).Error(0)
}

func (m *mockPartnerPlanRepo) Delete(id, clinicID uuid.UUID) error {
	return m.Called(id, clinicID).Error(0)
}

func (m *mockPartnerPlanRepo) ListByNetwork(networkID uuid.UUID) ([]*model.PartnerPlan, error) {
	args := m.Called(networkID)
	return args.Get(0).([]*model.PartnerPlan), args.Error(1)
}

func intPtr(i int) *int { return &i }

func TestPartnerPlanService_Create(t *testing.T) {
	planRepo := new(mockPartnerPlanRepo)
	netRepo := new(mockPartnerNetworkRepo)
	svc := NewPartnerPlanService(planRepo, netRepo)

	clinicID := uuid.New()
	networkID := uuid.New()
	network := &model.PartnerNetwork{ID: networkID, ClinicID: clinicID, Name: "Gympass"}

	netRepo.On("GetByID", networkID, clinicID).Return(network, nil)
	planRepo.On("GetByNetworkAndName", networkID, "Silver").Return(nil, gorm.ErrRecordNotFound)
	planRepo.On("Create", mock.AnythingOfType("*model.PartnerPlan")).Return(nil)

	max := 3
	resp, err := svc.Create(clinicID, &model.CreatePartnerPlanRequest{
		PartnerNetworkID:   networkID,
		Name:               "Silver",
		MaxSessionsPerWeek: &max,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Silver", resp.Name)
	assert.Equal(t, networkID, resp.PartnerNetworkID)
	assert.Equal(t, intPtr(3), resp.MaxSessionsPerWeek)
	assert.True(t, resp.IsActive)
	netRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
}

func TestPartnerPlanService_Create_NetworkMissing(t *testing.T) {
	planRepo := new(mockPartnerPlanRepo)
	netRepo := new(mockPartnerNetworkRepo)
	svc := NewPartnerPlanService(planRepo, netRepo)

	clinicID := uuid.New()
	networkID := uuid.New()

	netRepo.On("GetByID", networkID, clinicID).Return(nil, gorm.ErrRecordNotFound)

	resp, err := svc.Create(clinicID, &model.CreatePartnerPlanRequest{
		PartnerNetworkID: networkID,
		Name:             "Silver",
	})

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrPartnerNetworkNotFound)
}

func TestPartnerPlanService_Create_DuplicateNameInNetwork(t *testing.T) {
	planRepo := new(mockPartnerPlanRepo)
	netRepo := new(mockPartnerNetworkRepo)
	svc := NewPartnerPlanService(planRepo, netRepo)

	clinicID := uuid.New()
	networkID := uuid.New()
	network := &model.PartnerNetwork{ID: networkID, ClinicID: clinicID, Name: "Gympass"}
	existing := &model.PartnerPlan{ID: uuid.New(), PartnerNetworkID: networkID, Name: "Silver"}

	netRepo.On("GetByID", networkID, clinicID).Return(network, nil)
	planRepo.On("GetByNetworkAndName", networkID, "Silver").Return(existing, nil)

	resp, err := svc.Create(clinicID, &model.CreatePartnerPlanRequest{
		PartnerNetworkID: networkID,
		Name:             "Silver",
	})

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrPartnerPlanNameExists)
}

func TestPartnerPlanService_Update_MaxSessionsPerWeek(t *testing.T) {
	planRepo := new(mockPartnerPlanRepo)
	netRepo := new(mockPartnerNetworkRepo)
	svc := NewPartnerPlanService(planRepo, netRepo)

	clinicID := uuid.New()
	networkID := uuid.New()
	id := uuid.New()
	current := &model.PartnerPlan{
		ID:                 id,
		PartnerNetworkID:   networkID,
		Name:               "Silver",
		MaxSessionsPerWeek: intPtr(3),
		IsActive:           true,
	}

	planRepo.On("GetByIDScoped", id, clinicID).Return(current, nil)
	planRepo.On("Update", mock.AnythingOfType("*model.PartnerPlan")).Return(nil)

	newMax := 6
	resp, err := svc.Update(clinicID, id, &model.UpdatePartnerPlanRequest{
		MaxSessionsPerWeek: &newMax,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, intPtr(6), resp.MaxSessionsPerWeek)
}

func TestPartnerPlanService_GetByID_NotFound(t *testing.T) {
	planRepo := new(mockPartnerPlanRepo)
	netRepo := new(mockPartnerNetworkRepo)
	svc := NewPartnerPlanService(planRepo, netRepo)

	clinicID := uuid.New()
	id := uuid.New()

	planRepo.On("GetByIDScoped", id, clinicID).Return(nil, gorm.ErrRecordNotFound)

	resp, err := svc.GetByID(clinicID, id)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrPartnerPlanNotFound)
}

func TestPartnerPlanService_ListByNetwork(t *testing.T) {
	planRepo := new(mockPartnerPlanRepo)
	netRepo := new(mockPartnerNetworkRepo)
	svc := NewPartnerPlanService(planRepo, netRepo)

	clinicID := uuid.New()
	networkID := uuid.New()
	network := &model.PartnerNetwork{ID: networkID, ClinicID: clinicID, Name: "Gympass"}

	netRepo.On("GetByID", networkID, clinicID).Return(network, nil)
	planRepo.On("ListByNetwork", networkID).Return([]*model.PartnerPlan{
		{ID: uuid.New(), PartnerNetworkID: networkID, Name: "Gold"},
		{ID: uuid.New(), PartnerNetworkID: networkID, Name: "Silver"},
	}, nil)

	resp, err := svc.ListByNetwork(clinicID, networkID)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "Gold", resp[0].Name)
}
