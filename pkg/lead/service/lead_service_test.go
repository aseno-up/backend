package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aseno/backend/pkg/lead/model"
)

// --- Mocks ---

type mockLeadRepository struct {
	mock.Mock
}

func (m *mockLeadRepository) Create(ctx context.Context, lead *model.Lead) error {
	args := m.Called(ctx, lead)
	return args.Error(0)
}

func (m *mockLeadRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Lead, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Lead), args.Error(1)
}

func (m *mockLeadRepository) List(ctx context.Context, filter model.LeadFilter) ([]*model.Lead, int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*model.Lead), args.Get(1).(int64), args.Error(2)
}

func (m *mockLeadRepository) Update(ctx context.Context, lead *model.Lead) error {
	args := m.Called(ctx, lead)
	return args.Error(0)
}

func (m *mockLeadRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockLeadRepository) GetByExternalID(ctx context.Context, externalID string) (*model.Lead, error) {
	args := m.Called(ctx, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Lead), args.Error(1)
}

type mockCRMProvider struct {
	mock.Mock
}

func (m *mockCRMProvider) CreatePage(ctx context.Context, lead *model.Lead) (string, error) {
	args := m.Called(ctx, lead)
	return args.String(0), args.Error(1)
}

func (m *mockCRMProvider) UpdatePage(ctx context.Context, externalID string, lead *model.Lead) error {
	args := m.Called(ctx, externalID, lead)
	return args.Error(0)
}

// --- Tests ---

func TestCreate_DefaultStatus(t *testing.T) {
	repo := new(mockLeadRepository)
	svc := NewLeadService(repo, nil)

	repo.On("Create", mock.Anything, mock.MatchedBy(func(l *model.Lead) bool {
		return l.Status == model.LeadStatusIdentificado && l.ClinicName == "Clínica Teste"
	})).Return(nil)

	req := &model.CreateLeadRequest{
		ClinicName:  "Clínica Teste",
		ContactName: "João",
	}

	resp, err := svc.Create(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, model.LeadStatusIdentificado, resp.Status)
	repo.AssertExpectations(t)
}

func TestCreate_WithExplicitStatus(t *testing.T) {
	repo := new(mockLeadRepository)
	svc := NewLeadService(repo, nil)

	repo.On("Create", mock.Anything, mock.MatchedBy(func(l *model.Lead) bool {
		return l.Status == model.LeadStatusContactado
	})).Return(nil)

	req := &model.CreateLeadRequest{
		ClinicName:  "Clínica ABC",
		ContactName: "Maria",
		Status:      model.LeadStatusContactado,
	}

	resp, err := svc.Create(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, model.LeadStatusContactado, resp.Status)
	repo.AssertExpectations(t)
}

func TestCreate_WithCRMSync(t *testing.T) {
	repo := new(mockLeadRepository)
	crmProvider := new(mockCRMProvider)
	svc := NewLeadService(repo, crmProvider)

	repo.On("Create", mock.Anything, mock.Anything).Return(nil)
	crmProvider.On("CreatePage", mock.Anything, mock.Anything).Return("notion-123", nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(l *model.Lead) bool {
		return l.ExternalID == "notion-123"
	})).Return(nil)

	req := &model.CreateLeadRequest{
		ClinicName:  "Clínica CRM",
		ContactName: "Test",
	}

	resp, err := svc.Create(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	crmProvider.AssertExpectations(t)
}

func TestUpdate_PartialFields(t *testing.T) {
	repo := new(mockLeadRepository)
	svc := NewLeadService(repo, nil)

	leadID := uuid.New()

	existing := &model.Lead{
		ID:          leadID,
		ClinicName:  "Original",
		ContactName: "Original Contact",
		Phone:       "11999999999",
		Status:      model.LeadStatusIdentificado,
	}

	repo.On("GetByID", mock.Anything, leadID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(l *model.Lead) bool {
		return l.ClinicName == "Updated" && l.ContactName == "Original Contact"
	})).Return(nil)

	newName := "Updated"
	req := &model.UpdateLeadRequest{
		ClinicName: &newName,
	}

	resp, err := svc.Update(context.Background(), leadID, req)

	assert.NoError(t, err)
	assert.Equal(t, "Updated", resp.ClinicName)
	assert.Equal(t, "Original Contact", resp.ContactName)
	repo.AssertExpectations(t)
}

func TestUpdate_LinkAccountAutoConverts(t *testing.T) {
	repo := new(mockLeadRepository)
	svc := NewLeadService(repo, nil)

	leadID := uuid.New()
	accountID := uuid.New()

	existing := &model.Lead{
		ID:     leadID,
		Status: model.LeadStatusNegociando,
	}

	repo.On("GetByID", mock.Anything, leadID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(l *model.Lead) bool {
		return l.Status == model.LeadStatusConvertido && l.AccountID != nil && l.ConvertedAt != nil
	})).Return(nil)

	req := &model.UpdateLeadRequest{
		AccountID: &accountID,
	}

	resp, err := svc.Update(context.Background(), leadID, req)

	assert.NoError(t, err)
	assert.Equal(t, model.LeadStatusConvertido, resp.Status)
	assert.NotNil(t, resp.AccountID)
	assert.NotNil(t, resp.ConvertedAt)
	repo.AssertExpectations(t)
}

func TestList_Pagination(t *testing.T) {
	repo := new(mockLeadRepository)
	svc := NewLeadService(repo, nil)

	filter := model.LeadFilter{Page: 1, Limit: 10}

	leads := []*model.Lead{
		{ID: uuid.New(), ClinicName: "Lead 1", Status: model.LeadStatusIdentificado},
		{ID: uuid.New(), ClinicName: "Lead 2", Status: model.LeadStatusContactado},
	}

	repo.On("List", mock.Anything, filter).Return(leads, int64(2), nil)

	resp, err := svc.List(context.Background(), filter)

	assert.NoError(t, err)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, int64(2), resp.Meta.Total)
	assert.Equal(t, 1, resp.Meta.Page)
	repo.AssertExpectations(t)
}
