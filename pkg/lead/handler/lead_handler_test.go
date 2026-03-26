package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aseno/backend/internal/errors"
	account_model "github.com/aseno/backend/pkg/account/model"
	"github.com/aseno/backend/pkg/lead/model"
	"github.com/aseno/backend/pkg/pagination"
)

type mockLeadService struct {
	mock.Mock
}

func (m *mockLeadService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateLeadRequest) (*model.LeadResponse, error) {
	args := m.Called(ctx, clinicID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.LeadResponse), args.Error(1)
}

func (m *mockLeadService) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.LeadResponse, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.LeadResponse), args.Error(1)
}

func (m *mockLeadService) List(ctx context.Context, clinicID uuid.UUID, filter model.LeadFilter) (pagination.Response[*model.LeadResponse], error) {
	args := m.Called(ctx, clinicID, filter)
	return args.Get(0).(pagination.Response[*model.LeadResponse]), args.Error(1)
}

func (m *mockLeadService) Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateLeadRequest) (*model.LeadResponse, error) {
	args := m.Called(ctx, id, clinicID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.LeadResponse), args.Error(1)
}

func (m *mockLeadService) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	args := m.Called(ctx, id, clinicID)
	return args.Error(0)
}

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
func (m *mockAccountRepo) ListAll(ctx context.Context) ([]account_model.Account, error) {
	args := m.Called(ctx)
	return args.Get(0).([]account_model.Account), args.Error(1)
}
func (m *mockAccountRepo) Update(ctx context.Context, account *account_model.Account) error {
	return m.Called(ctx, account).Error(0)
}
func (m *mockAccountRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func setupTestRouter(svc *mockLeadService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	clinicID := uuid.MustParse("d6b10acc-2307-413f-a4de-4c2edf3f9a70")
	userID := uuid.New()

	r.Use(func(c *gin.Context) {
		c.Set("clinic_id", clinicID)
		c.Set("user_id", userID)
		c.Next()
	})

	accRepo := new(mockAccountRepo)
	h := NewLeadHandler(svc, accRepo, nil)
	h.RegisterRoutes(r)

	return r
}

func TestHandler_Create_Success(t *testing.T) {
	svc := new(mockLeadService)
	router := setupTestRouter(svc)

	expected := &model.LeadResponse{
		ID:         uuid.New(),
		ClinicName: "Test Clinic",
		Status:     model.LeadStatusIdentificado,
	}

	svc.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(expected, nil)

	body, _ := json.Marshal(map[string]string{
		"clinic_name":  "Test Clinic",
		"contact_name": "John",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_Create_ValidationError(t *testing.T) {
	svc := new(mockLeadService)
	router := setupTestRouter(svc)

	body, _ := json.Marshal(map[string]string{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/leads", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetByID_Success(t *testing.T) {
	svc := new(mockLeadService)
	router := setupTestRouter(svc)

	leadID := uuid.New()
	expected := &model.LeadResponse{
		ID:         leadID,
		ClinicName: "Found",
	}

	svc.On("GetByID", mock.Anything, leadID, mock.Anything).Return(expected, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/leads/"+leadID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_GetByID_InvalidUUID(t *testing.T) {
	svc := new(mockLeadService)
	router := setupTestRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/leads/not-a-uuid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetByID_NotFound(t *testing.T) {
	svc := new(mockLeadService)
	router := setupTestRouter(svc)

	leadID := uuid.New()
	svc.On("GetByID", mock.Anything, leadID, mock.Anything).Return(nil, errors.ErrLeadNotFoundInstance)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/leads/"+leadID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_Delete_Success(t *testing.T) {
	svc := new(mockLeadService)
	router := setupTestRouter(svc)

	leadID := uuid.New()
	svc.On("Delete", mock.Anything, leadID, mock.Anything).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/leads/"+leadID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
