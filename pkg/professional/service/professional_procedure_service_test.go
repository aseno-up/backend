package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aseno/backend/internal/errors"
	procedure_model "github.com/aseno/backend/pkg/procedure/model"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/professional/model"
)

// --- Mock: ProfessionalProcedureRepository ---

type mockPPRepo struct {
	mock.Mock
}

func (m *mockPPRepo) BulkUpsert(ctx context.Context, clinicID, professionalID uuid.UUID, items []model.ProfessionalProcedure) error {
	return m.Called(ctx, clinicID, professionalID, items).Error(0)
}

func (m *mockPPRepo) ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID) ([]model.ProfessionalProcedure, error) {
	args := m.Called(ctx, professionalID, clinicID)
	return args.Get(0).([]model.ProfessionalProcedure), args.Error(1)
}

func (m *mockPPRepo) ListByProcedure(ctx context.Context, procedureID, clinicID uuid.UUID) ([]model.ProfessionalProcedure, error) {
	args := m.Called(ctx, procedureID, clinicID)
	return args.Get(0).([]model.ProfessionalProcedure), args.Error(1)
}

func (m *mockPPRepo) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return m.Called(ctx, id, clinicID).Error(0)
}

func (m *mockPPRepo) DeleteByProfessionalAndProcedure(ctx context.Context, professionalID, procedureID, clinicID uuid.UUID) error {
	return m.Called(ctx, professionalID, procedureID, clinicID).Error(0)
}

// --- Mock: ProfessionalRepository ---

type mockProfRepo struct {
	mock.Mock
}

func (m *mockProfRepo) Create(ctx context.Context, professional *model.Professional) error {
	return m.Called(ctx, professional).Error(0)
}

func (m *mockProfRepo) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Professional, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Professional), args.Error(1)
}

func (m *mockProfRepo) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Professional, int64, error) {
	args := m.Called(ctx, clinicID, filter)
	return args.Get(0).([]*model.Professional), args.Get(1).(int64), args.Error(2)
}

func (m *mockProfRepo) Update(ctx context.Context, professional *model.Professional) error {
	return m.Called(ctx, professional).Error(0)
}

func (m *mockProfRepo) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return m.Called(ctx, id, clinicID).Error(0)
}

func (m *mockProfRepo) BulkDelete(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error {
	return m.Called(ctx, ids, clinicID).Error(0)
}

func (m *mockProfRepo) GetByUserID(ctx context.Context, userID, clinicID uuid.UUID) (*model.Professional, error) {
	args := m.Called(ctx, userID, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Professional), args.Error(1)
}

func (m *mockProfRepo) CountActiveByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

// --- Mock: ProcedureRepository ---

type mockProcedureRepo struct {
	mock.Mock
}

func (m *mockProcedureRepo) Create(procedure *procedure_model.Procedure) error {
	return m.Called(procedure).Error(0)
}

func (m *mockProcedureRepo) GetByID(id uuid.UUID, clinicID uuid.UUID) (*procedure_model.Procedure, error) {
	args := m.Called(id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*procedure_model.Procedure), args.Error(1)
}

func (m *mockProcedureRepo) GetByIDs(ids []uuid.UUID, clinicID uuid.UUID) ([]*procedure_model.Procedure, error) {
	args := m.Called(ids, clinicID)
	return args.Get(0).([]*procedure_model.Procedure), args.Error(1)
}

func (m *mockProcedureRepo) Update(procedure *procedure_model.Procedure) error {
	return m.Called(procedure).Error(0)
}

func (m *mockProcedureRepo) Delete(id uuid.UUID, clinicID uuid.UUID) error {
	return m.Called(id, clinicID).Error(0)
}

func (m *mockProcedureRepo) BulkDelete(ids []uuid.UUID, clinicID uuid.UUID) error {
	return m.Called(ids, clinicID).Error(0)
}

func (m *mockProcedureRepo) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*procedure_model.Procedure, int64, error) {
	args := m.Called(ctx, clinicID, filter)
	return args.Get(0).([]*procedure_model.Procedure), args.Get(1).(int64), args.Error(2)
}

func (m *mockProcedureRepo) ListByClinicAndSpecialty(ctx context.Context, clinicID uuid.UUID, specialtyID *uuid.UUID) ([]*procedure_model.Procedure, error) {
	args := m.Called(ctx, clinicID, specialtyID)
	return args.Get(0).([]*procedure_model.Procedure), args.Error(1)
}

// --- Helpers ---

func newTestService() (ProfessionalProcedureService, *mockPPRepo, *mockProfRepo, *mockProcedureRepo) {
	ppRepo := new(mockPPRepo)
	profRepo := new(mockProfRepo)
	procRepo := new(mockProcedureRepo)
	svc := NewProfessionalProcedureService(ppRepo, profRepo, procRepo)
	return svc, ppRepo, profRepo, procRepo
}

var (
	testClinicID       = uuid.New()
	testProfessionalID = uuid.New()
	testProcedureID1   = uuid.New()
	testProcedureID2   = uuid.New()
)

// --- Tests ---

func TestBulkUpsert_ValidPercentage(t *testing.T) {
	svc, ppRepo, profRepo, procRepo := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	procRepo.On("GetByIDs", []uuid.UUID{testProcedureID1}, testClinicID).
		Return([]*procedure_model.Procedure{{ID: testProcedureID1, ClinicID: testClinicID, Name: "Consulta", Price: 150}}, nil)

	ppRepo.On("BulkUpsert", ctx, testClinicID, testProfessionalID, mock.AnythingOfType("[]model.ProfessionalProcedure")).Return(nil)

	// For the ListByProfessional call after upsert
	ppRepo.On("ListByProfessional", ctx, testProfessionalID, testClinicID).
		Return([]model.ProfessionalProcedure{
			{ID: uuid.New(), ProfessionalID: testProfessionalID, ProcedureID: testProcedureID1, CommissionType: "percentage", CommissionValue: 30, IsActive: true},
		}, nil)
	procRepo.On("GetByIDs", mock.AnythingOfType("[]uuid.UUID"), testClinicID).
		Return([]*procedure_model.Procedure{{ID: testProcedureID1, ClinicID: testClinicID, Name: "Consulta", Price: 150}}, nil).Maybe()

	req := &model.BulkUpsertProfessionalProcedureRequest{
		Items: []model.BulkUpsertItem{
			{ProcedureID: testProcedureID1, CommissionType: "percentage", CommissionValue: 30},
		},
	}

	result, err := svc.BulkUpsert(ctx, testClinicID, testProfessionalID, req)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "percentage", result[0].CommissionType)
	assert.Equal(t, float64(30), result[0].CommissionValue)
	ppRepo.AssertExpectations(t)
}

func TestBulkUpsert_ValidFixed(t *testing.T) {
	svc, ppRepo, profRepo, procRepo := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	procRepo.On("GetByIDs", []uuid.UUID{testProcedureID1}, testClinicID).
		Return([]*procedure_model.Procedure{{ID: testProcedureID1, ClinicID: testClinicID, Name: "Consulta", Price: 150}}, nil)

	ppRepo.On("BulkUpsert", ctx, testClinicID, testProfessionalID, mock.AnythingOfType("[]model.ProfessionalProcedure")).Return(nil)

	ppRepo.On("ListByProfessional", ctx, testProfessionalID, testClinicID).
		Return([]model.ProfessionalProcedure{
			{ID: uuid.New(), ProfessionalID: testProfessionalID, ProcedureID: testProcedureID1, CommissionType: "fixed", CommissionValue: 40, IsActive: true},
		}, nil)
	procRepo.On("GetByIDs", mock.AnythingOfType("[]uuid.UUID"), testClinicID).
		Return([]*procedure_model.Procedure{{ID: testProcedureID1, ClinicID: testClinicID, Name: "Consulta", Price: 150}}, nil).Maybe()

	req := &model.BulkUpsertProfessionalProcedureRequest{
		Items: []model.BulkUpsertItem{
			{ProcedureID: testProcedureID1, CommissionType: "fixed", CommissionValue: 40},
		},
	}

	result, err := svc.BulkUpsert(ctx, testClinicID, testProfessionalID, req)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "fixed", result[0].CommissionType)
	assert.Equal(t, float64(40), result[0].CommissionValue)
}

func TestBulkUpsert_InvalidType(t *testing.T) {
	svc, _, profRepo, _ := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	req := &model.BulkUpsertProfessionalProcedureRequest{
		Items: []model.BulkUpsertItem{
			{ProcedureID: testProcedureID1, CommissionType: "invalid", CommissionValue: 30},
		},
	}

	result, err := svc.BulkUpsert(ctx, testClinicID, testProfessionalID, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "Tipo de repasse invalido")
}

func TestBulkUpsert_PercentageAbove100(t *testing.T) {
	svc, _, profRepo, _ := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	req := &model.BulkUpsertProfessionalProcedureRequest{
		Items: []model.BulkUpsertItem{
			{ProcedureID: testProcedureID1, CommissionType: "percentage", CommissionValue: 150},
		},
	}

	result, err := svc.BulkUpsert(ctx, testClinicID, testProfessionalID, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "entre 0 e 100")
}

func TestBulkUpsert_PercentageZero(t *testing.T) {
	svc, _, profRepo, _ := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	req := &model.BulkUpsertProfessionalProcedureRequest{
		Items: []model.BulkUpsertItem{
			{ProcedureID: testProcedureID1, CommissionType: "percentage", CommissionValue: 0},
		},
	}

	result, err := svc.BulkUpsert(ctx, testClinicID, testProfessionalID, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "entre 0 e 100")
}

func TestBulkUpsert_NegativeFixed(t *testing.T) {
	svc, _, profRepo, _ := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	req := &model.BulkUpsertProfessionalProcedureRequest{
		Items: []model.BulkUpsertItem{
			{ProcedureID: testProcedureID1, CommissionType: "fixed", CommissionValue: -10},
		},
	}

	result, err := svc.BulkUpsert(ctx, testClinicID, testProfessionalID, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nao pode ser negativo")
}

func TestBulkUpsert_FixedExceedsPrice(t *testing.T) {
	svc, _, profRepo, procRepo := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	// Procedure costs R$60, but commission is R$100
	procRepo.On("GetByIDs", mock.AnythingOfType("[]uuid.UUID"), testClinicID).
		Return([]*procedure_model.Procedure{{ID: testProcedureID1, ClinicID: testClinicID, Name: "Consulta", Price: 60}}, nil)

	req := &model.BulkUpsertProfessionalProcedureRequest{
		Items: []model.BulkUpsertItem{
			{ProcedureID: testProcedureID1, CommissionType: "fixed", CommissionValue: 100},
		},
	}

	result, err := svc.BulkUpsert(ctx, testClinicID, testProfessionalID, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nao pode ser maior que o preco")
}

func TestBulkUpsert_ProcedureNotFound(t *testing.T) {
	svc, _, profRepo, procRepo := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	// Return only 1 procedure when 2 were requested
	procRepo.On("GetByIDs", mock.AnythingOfType("[]uuid.UUID"), testClinicID).
		Return([]*procedure_model.Procedure{{ID: testProcedureID1, ClinicID: testClinicID, Name: "Consulta", Price: 150}}, nil)

	req := &model.BulkUpsertProfessionalProcedureRequest{
		Items: []model.BulkUpsertItem{
			{ProcedureID: testProcedureID1, CommissionType: "percentage", CommissionValue: 30},
			{ProcedureID: testProcedureID2, CommissionType: "fixed", CommissionValue: 40},
		},
	}

	result, err := svc.BulkUpsert(ctx, testClinicID, testProfessionalID, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nao foram encontrados")
}

func TestBulkUpsert_ProfessionalNotFound(t *testing.T) {
	svc, _, profRepo, _ := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(nil, errors.ErrProfessionalNotFoundInstance)

	req := &model.BulkUpsertProfessionalProcedureRequest{
		Items: []model.BulkUpsertItem{
			{ProcedureID: testProcedureID1, CommissionType: "percentage", CommissionValue: 30},
		},
	}

	result, err := svc.BulkUpsert(ctx, testClinicID, testProfessionalID, req)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestListByProfessional_Success(t *testing.T) {
	svc, ppRepo, profRepo, procRepo := newTestService()
	ctx := context.Background()

	ppID := uuid.New()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	ppRepo.On("ListByProfessional", ctx, testProfessionalID, testClinicID).
		Return([]model.ProfessionalProcedure{
			{ID: ppID, ProfessionalID: testProfessionalID, ProcedureID: testProcedureID1, CommissionType: "percentage", CommissionValue: 30, IsActive: true},
		}, nil)

	procRepo.On("GetByIDs", []uuid.UUID{testProcedureID1}, testClinicID).
		Return([]*procedure_model.Procedure{{ID: testProcedureID1, ClinicID: testClinicID, Name: "Consulta", Price: 150}}, nil)

	result, err := svc.ListByProfessional(ctx, testProfessionalID, testClinicID)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Consulta", result[0].ProcedureName)
	assert.Equal(t, float64(150), result[0].ProcedurePrice)
	assert.Equal(t, "percentage", result[0].CommissionType)
	assert.Equal(t, float64(30), result[0].CommissionValue)
}

func TestListByProfessional_Empty(t *testing.T) {
	svc, ppRepo, profRepo, _ := newTestService()
	ctx := context.Background()

	profRepo.On("GetByID", ctx, testProfessionalID, testClinicID).
		Return(&model.Professional{ID: testProfessionalID, ClinicID: testClinicID}, nil)

	ppRepo.On("ListByProfessional", ctx, testProfessionalID, testClinicID).
		Return([]model.ProfessionalProcedure{}, nil)

	result, err := svc.ListByProfessional(ctx, testProfessionalID, testClinicID)

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestDelete_Success(t *testing.T) {
	svc, ppRepo, _, _ := newTestService()
	ctx := context.Background()

	ppID := uuid.New()
	ppRepo.On("Delete", ctx, ppID, testClinicID).Return(nil)

	err := svc.Delete(ctx, ppID, testClinicID)

	assert.NoError(t, err)
	ppRepo.AssertExpectations(t)
}

func TestDelete_NotFound(t *testing.T) {
	svc, ppRepo, _, _ := newTestService()
	ctx := context.Background()

	ppID := uuid.New()
	ppRepo.On("Delete", ctx, ppID, testClinicID).Return(errors.ErrNotFoundInstance)

	err := svc.Delete(ctx, ppID, testClinicID)

	assert.Error(t, err)
}
