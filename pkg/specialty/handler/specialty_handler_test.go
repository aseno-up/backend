package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/specialty/handler"
	"github.com/aseno/backend/pkg/specialty/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSpecialtyService is a mock implementation of SpecialtyService
type MockSpecialtyService struct {
	mock.Mock
}

func (m *MockSpecialtyService) Create(clinicID uuid.UUID, req *model.CreateSpecialtyRequest) (*model.SpecialtyResponse, error) {
	args := m.Called(clinicID, req)
	return args.Get(0).(*model.SpecialtyResponse), args.Error(1)
}

func (m *MockSpecialtyService) GetByID(clinicID, id uuid.UUID) (*model.SpecialtyResponse, error) {
	args := m.Called(clinicID, id)
	return args.Get(0).(*model.SpecialtyResponse), args.Error(1)
}

func (m *MockSpecialtyService) Update(clinicID, id uuid.UUID, req *model.UpdateSpecialtyRequest) (*model.SpecialtyResponse, error) {
	args := m.Called(clinicID, id, req)
	return args.Get(0).(*model.SpecialtyResponse), args.Error(1)
}

func (m *MockSpecialtyService) Delete(clinicID, id uuid.UUID) error {
	args := m.Called(clinicID, id)
	return args.Error(0)
}

func (m *MockSpecialtyService) List(clinicID uuid.UUID) ([]*model.SpecialtyResponse, error) {
	args := m.Called(clinicID)
	return args.Get(0).([]*model.SpecialtyResponse), args.Error(1)
}

func (m *MockSpecialtyService) ListActive(clinicID uuid.UUID) ([]*model.SpecialtyResponse, error) {
	args := m.Called(clinicID)
	return args.Get(0).([]*model.SpecialtyResponse), args.Error(1)
}

// MockAuditService is a mock implementation of AuditService
type MockAuditService struct {
	mock.Mock
}

func (m *MockAuditService) Log(ctx context.Context, entry audit_service.AuditEntry) error {
	return nil
}

func (m *MockAuditService) LogFromGin(c *gin.Context, action audit_model.AuditAction, entityType audit_model.AuditEntityType, entityID string, oldData, newData interface{}) error {
	return nil
}

func (m *MockAuditService) List(ctx context.Context, clinicID uuid.UUID, filter *audit_model.AuditLogFilter) (*audit_model.AuditLogResponse, error) {
	return nil, nil
}

func TestListActive_ResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockSpecialtyService)
	mockAudit := new(MockAuditService)
	h := handler.NewSpecialtyHandler(mockSvc, mockAudit)

	router := gin.New()
	// Middleware to set clinic_id
	clinicID := uuid.New()
	router.Use(func(c *gin.Context) {
		c.Set("clinic_id", clinicID)
		c.Next()
	})
	router.GET("/specialties/active", h.ListActive)

	mockSpecialties := []*model.SpecialtyResponse{
		{
			ID:   uuid.New(),
			Name: "Cardiologia",
		},
	}

	mockSvc.On("ListActive", clinicID).Return(mockSpecialties, nil)

	req, _ := http.NewRequest(http.MethodGet, "/specialties/active", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// This is what the frontend expects: an object with a "data" key containing the array
	assert.Contains(t, response, "data")
	data, ok := response["data"].([]interface{})
	assert.True(t, ok, "response['data'] should be an array")
	assert.Len(t, data, 1)

	first := data[0].(map[string]interface{})
	assert.Equal(t, "Cardiologia", first["name"])

	mockSvc.AssertExpectations(t)
}
