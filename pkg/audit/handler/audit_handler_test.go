package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aseno/backend/pkg/audit/handler"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
)

type mockAuditService struct {
	mock.Mock
}

func (m *mockAuditService) Log(ctx context.Context, entry audit_service.AuditEntry) error {
	return nil
}
func (m *mockAuditService) LogFromGin(c *gin.Context, action audit_model.AuditAction, entityType audit_model.AuditEntityType, entityID string, oldData, newData interface{}) error {
	return nil
}
func (m *mockAuditService) List(ctx context.Context, filter *audit_model.AuditLogFilter) (*audit_model.AuditLogResponse, error) {
	args := m.Called(filter)
	if r := args.Get(0); r != nil {
		return r.(*audit_model.AuditLogResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func setupAuditRouter(t *testing.T, svc *mockAuditService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := handler.NewAuditHandler(svc)
	router := gin.New()
	h.RegisterRoutes(router)
	return router
}

// When no clinic_id filter is supplied, the handler must forward the filter
// as-is. The route is behind devOnlyMiddleware, so reaching this point implies
// the caller is a ClinicTypeDev tenant authorised to see every clinic.
func TestList_NoClinicFilter_QueriesAllTenants(t *testing.T) {
	svc := new(mockAuditService)
	svc.On("List", mock.MatchedBy(func(f *audit_model.AuditLogFilter) bool {
		return f.ClinicID == ""
	})).Return(&audit_model.AuditLogResponse{}, nil).Once()

	router := setupAuditRouter(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/audit-logs", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

// Dev users can still narrow to a specific tenant via ?clinic_id=<uuid>.
func TestList_WithClinicFilter_ScopesToThatTenant(t *testing.T) {
	target := "11111111-1111-1111-1111-111111111111"

	svc := new(mockAuditService)
	svc.On("List", mock.MatchedBy(func(f *audit_model.AuditLogFilter) bool {
		return f.ClinicID == target
	})).Return(&audit_model.AuditLogResponse{}, nil).Once()

	router := setupAuditRouter(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/audit-logs?clinic_id="+target, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}
