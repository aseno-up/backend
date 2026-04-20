package handler_test

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

	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/clinic/handler"
	"github.com/aseno/backend/pkg/clinic/model"
)

// MockClinicService implements service.ClinicService.
type MockClinicService struct {
	mock.Mock
}

func (m *MockClinicService) GetByID(ctx context.Context, id uuid.UUID) (*model.ClinicResponse, error) {
	args := m.Called(ctx, id)
	if r := args.Get(0); r != nil {
		return r.(*model.ClinicResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockClinicService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateClinicRequest) (*model.ClinicResponse, error) {
	args := m.Called(ctx, id, req)
	if r := args.Get(0); r != nil {
		return r.(*model.ClinicResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockAuditService tracks LogFromGin calls for assertions.
type MockAuditService struct {
	mock.Mock
}

func (m *MockAuditService) Log(ctx context.Context, entry audit_service.AuditEntry) error {
	return nil
}

func (m *MockAuditService) LogFromGin(c *gin.Context, action audit_model.AuditAction, entityType audit_model.AuditEntityType, entityID string, oldData, newData interface{}) error {
	args := m.Called(action, entityType, entityID, oldData, newData)
	return args.Error(0)
}

func (m *MockAuditService) List(ctx context.Context, filter *audit_model.AuditLogFilter) (*audit_model.AuditLogResponse, error) {
	return nil, nil
}

// setupRouter wires the clinic handler with stubs and a middleware that
// injects auth context equivalent to what JWT middleware produces in prod.
func setupRouter(t *testing.T, clinicSvc *MockClinicService, auditSvc *MockAuditService, clinicID, userID uuid.UUID, role string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := handler.NewClinicHandler(clinicSvc, auditSvc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("clinic_id", clinicID)
		c.Set("user_id", userID)
		c.Set("email", "admin@example.com")
		c.Set("user_role", role)
		c.Next()
	})
	h.RegisterRoutes(router)
	return router
}

func TestUpdateClinic_EmitsSettingsAuditLog_WhenSettingsChange(t *testing.T) {
	clinicID := uuid.New()
	userID := uuid.New()
	old := &model.ClinicResponse{ID: clinicID, Name: "Old"}
	updated := &model.ClinicResponse{ID: clinicID, Name: "Old"}

	clinicSvc := new(MockClinicService)
	clinicSvc.On("GetByID", mock.Anything, clinicID).Return(old, nil).Once()
	clinicSvc.On("Update", mock.Anything, clinicID, mock.Anything).Return(updated, nil).Once()

	auditSvc := new(MockAuditService)
	auditSvc.On("LogFromGin",
		audit_model.AuditActionUpdate,
		audit_model.AuditEntitySettings,
		clinicID.String(),
		old,
		updated,
	).Return(nil).Once()

	router := setupRouter(t, clinicSvc, auditSvc, clinicID, userID, "admin")

	disable := false
	reqBody := model.UpdateClinicRequest{
		Settings: &model.UpdateClinicSettingsRequest{
			AIToolReschedule: &disable,
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/clinic", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	clinicSvc.AssertExpectations(t)
	auditSvc.AssertExpectations(t)
}

func TestUpdateClinic_EmitsClinicAuditLog_WhenOnlyClinicFieldsChange(t *testing.T) {
	clinicID := uuid.New()
	userID := uuid.New()
	old := &model.ClinicResponse{ID: clinicID, Name: "Old"}
	updated := &model.ClinicResponse{ID: clinicID, Name: "New"}

	clinicSvc := new(MockClinicService)
	clinicSvc.On("GetByID", mock.Anything, clinicID).Return(old, nil).Once()
	clinicSvc.On("Update", mock.Anything, clinicID, mock.Anything).Return(updated, nil).Once()

	auditSvc := new(MockAuditService)
	auditSvc.On("LogFromGin",
		audit_model.AuditActionUpdate,
		audit_model.AuditEntityClinic,
		clinicID.String(),
		old,
		updated,
	).Return(nil).Once()

	router := setupRouter(t, clinicSvc, auditSvc, clinicID, userID, "admin")

	reqBody := model.UpdateClinicRequest{Name: "New"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/clinic", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	auditSvc.AssertExpectations(t)
}

func TestUpdateClinic_Forbidden_ForNonAdminRole(t *testing.T) {
	clinicID := uuid.New()
	userID := uuid.New()
	clinicSvc := new(MockClinicService)
	auditSvc := new(MockAuditService)

	router := setupRouter(t, clinicSvc, auditSvc, clinicID, userID, "professional")

	reqBody := model.UpdateClinicRequest{Name: "attempt"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/clinic", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	// Neither service nor audit should have been touched.
	clinicSvc.AssertNotCalled(t, "Update")
	auditSvc.AssertNotCalled(t, "LogFromGin")
}
