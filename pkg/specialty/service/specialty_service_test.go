package service

import (
	"testing"

	"github.com/aseno/backend/pkg/specialty/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock repository
type mockSpecialtyRepo struct {
	mock.Mock
}

func (m *mockSpecialtyRepo) Create(specialty *model.Specialty) error {
	args := m.Called(specialty)
	return args.Error(0)
}

func (m *mockSpecialtyRepo) GetByID(id, clinicID uuid.UUID) (*model.Specialty, error) {
	args := m.Called(id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Specialty), args.Error(1)
}

func (m *mockSpecialtyRepo) GetByName(name string, clinicID uuid.UUID) (*model.Specialty, error) {
	args := m.Called(name, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Specialty), args.Error(1)
}

func (m *mockSpecialtyRepo) Update(specialty *model.Specialty) error {
	return m.Called(specialty).Error(0)
}

func (m *mockSpecialtyRepo) Delete(id, clinicID uuid.UUID) error {
	return m.Called(id, clinicID).Error(0)
}

func (m *mockSpecialtyRepo) ListByClinic(clinicID uuid.UUID) ([]*model.Specialty, error) {
	args := m.Called(clinicID)
	return args.Get(0).([]*model.Specialty), args.Error(1)
}

func (m *mockSpecialtyRepo) ListActiveByClinic(clinicID uuid.UUID) ([]*model.Specialty, error) {
	args := m.Called(clinicID)
	return args.Get(0).([]*model.Specialty), args.Error(1)
}

func (m *mockSpecialtyRepo) CountByClinic(clinicID uuid.UUID) (int64, error) {
	args := m.Called(clinicID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockSpecialtyRepo) GetByProfessionalID(professionalID uuid.UUID) ([]*model.Specialty, error) {
	args := m.Called(professionalID)
	return args.Get(0).([]*model.Specialty), args.Error(1)
}

func (m *mockSpecialtyRepo) GetByProfessionalIDs(professionalIDs []uuid.UUID) (map[uuid.UUID][]*model.Specialty, error) {
	args := m.Called(professionalIDs)
	return args.Get(0).(map[uuid.UUID][]*model.Specialty), args.Error(1)
}

func (m *mockSpecialtyRepo) SetProfessionalSpecialties(professionalID uuid.UUID, specialtyIDs []uuid.UUID) error {
	return m.Called(professionalID, specialtyIDs).Error(0)
}

func TestSpecialtyService_Create(t *testing.T) {
	repo := new(mockSpecialtyRepo)
	svc := NewSpecialtyService(repo)

	clinicID := uuid.New()
	req := &model.CreateSpecialtyRequest{Name: "Psicologia"}

	repo.On("Create", mock.AnythingOfType("*model.Specialty")).Return(nil)

	result, err := svc.Create(clinicID, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Psicologia", result.Name)
	assert.Equal(t, clinicID, result.ClinicID)
	assert.True(t, result.Active)
	repo.AssertExpectations(t)
}

func TestSpecialtyService_GetByID_NotFound(t *testing.T) {
	repo := new(mockSpecialtyRepo)
	svc := NewSpecialtyService(repo)

	clinicID := uuid.New()
	specID := uuid.New()

	repo.On("GetByID", specID, clinicID).Return(nil, ErrSpecialtyNotFound)

	result, err := svc.GetByID(clinicID, specID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrSpecialtyNotFound, err)
}

func TestSpecialtyService_Update(t *testing.T) {
	repo := new(mockSpecialtyRepo)
	svc := NewSpecialtyService(repo)

	clinicID := uuid.New()
	specID := uuid.New()
	existing := &model.Specialty{ID: specID, ClinicID: clinicID, Name: "Psicologia", Active: true}

	repo.On("GetByID", specID, clinicID).Return(existing, nil)
	repo.On("Update", mock.AnythingOfType("*model.Specialty")).Return(nil)

	newName := "Psiquiatria"
	result, err := svc.Update(clinicID, specID, &model.UpdateSpecialtyRequest{Name: &newName})

	assert.NoError(t, err)
	assert.Equal(t, "Psiquiatria", result.Name)
	repo.AssertExpectations(t)
}

func TestSpecialtyService_ListActive(t *testing.T) {
	repo := new(mockSpecialtyRepo)
	svc := NewSpecialtyService(repo)

	clinicID := uuid.New()
	specs := []*model.Specialty{
		{ID: uuid.New(), ClinicID: clinicID, Name: "Psicologia", Active: true},
		{ID: uuid.New(), ClinicID: clinicID, Name: "Fisioterapia", Active: true},
	}

	repo.On("ListActiveByClinic", clinicID).Return(specs, nil)

	result, err := svc.ListActive(clinicID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Psicologia", result[0].Name)
	assert.Equal(t, "Fisioterapia", result[1].Name)
	repo.AssertExpectations(t)
}

func TestSpecialtyService_Delete(t *testing.T) {
	repo := new(mockSpecialtyRepo)
	svc := NewSpecialtyService(repo)

	clinicID := uuid.New()
	specID := uuid.New()

	repo.On("Delete", specID, clinicID).Return(nil)

	err := svc.Delete(clinicID, specID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
