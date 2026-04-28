package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aseno/backend/pkg/appointment/handler"
	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/appointment/service"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	calendar_service "github.com/aseno/backend/pkg/calendar/service"
	profModel "github.com/aseno/backend/pkg/professional/model"
)

type mockAppointmentService struct {
	mock.Mock
}

func (m *mockAppointmentService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateAppointmentRequest) (*model.AppointmentResponse, error) {
	args := m.Called(ctx, clinicID, req)
	if r := args.Get(0); r != nil {
		return r.(*model.AppointmentResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockAppointmentService) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error) {
	return nil, nil
}
func (m *mockAppointmentService) List(ctx context.Context, clinicID uuid.UUID, filter *model.AppointmentFilter) (*model.AppointmentListResponse, error) {
	return nil, nil
}
func (m *mockAppointmentService) GetByDay(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error) {
	return nil, nil
}
func (m *mockAppointmentService) GetByWeek(ctx context.Context, clinicID uuid.UUID, date time.Time, professionalID *uuid.UUID) ([]*model.AppointmentResponse, error) {
	return nil, nil
}
func (m *mockAppointmentService) Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateAppointmentRequest) (*model.AppointmentResponse, error) {
	return nil, nil
}
func (m *mockAppointmentService) Delete(ctx context.Context, id, clinicID uuid.UUID, deletePayments bool) error {
	return nil
}
func (m *mockAppointmentService) GetDeletePreview(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentDeletePreview, error) {
	return nil, nil
}
func (m *mockAppointmentService) Confirm(ctx context.Context, id, clinicID uuid.UUID, req *model.ConfirmAppointmentRequest) (*model.AppointmentResponse, error) {
	return nil, nil
}
func (m *mockAppointmentService) Cancel(ctx context.Context, id, clinicID uuid.UUID, req *model.CancelAppointmentRequest) (*model.AppointmentResponse, error) {
	return nil, nil
}
func (m *mockAppointmentService) Complete(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error) {
	return nil, nil
}
func (m *mockAppointmentService) MarkNoShow(ctx context.Context, id, clinicID uuid.UUID) (*model.AppointmentResponse, error) {
	return nil, nil
}
func (m *mockAppointmentService) CreateFromWaitlist(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (m *mockAppointmentService) Reschedule(ctx context.Context, appointmentID, clinicID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) error {
	return nil
}
func (m *mockAppointmentService) SetWaitlistProcessor(_ service.WaitlistService)               {}
func (m *mockAppointmentService) SetPatientRiskCalculator(_ service.PatientRiskCalculator)     {}
func (m *mockAppointmentService) SetCommissionCalculator(_ service.CommissionCalculator)       {}
func (m *mockAppointmentService) SetCalendarService(_ calendar_service.CalendarService)        {}
func (m *mockAppointmentService) SetSessionPlanHandler(_ service.SessionPlanCompletionHandler) {}
func (m *mockAppointmentService) SetPaymentLister(_ service.PaymentLister)                     {}
func (m *mockAppointmentService) SetFinanceUpserter(_ service.FinanceUpserter)                 {}
func (m *mockAppointmentService) SuggestSmartSlots(ctx context.Context, clinicID, professionalID uuid.UUID, date time.Time, days int, durationMinutes int) ([]*model.SmartSlotSuggestion, error) {
	return nil, nil
}

var _ service.AppointmentService = (*mockAppointmentService)(nil)

// Ensure the professional service dep compiles without being used by Create.
var _ = profModel.Professional{}

type mockAuditService struct {
	mock.Mock
}

func (m *mockAuditService) Log(ctx context.Context, entry audit_service.AuditEntry) error {
	return nil
}

func (m *mockAuditService) LogFromGin(c *gin.Context, action audit_model.AuditAction, entityType audit_model.AuditEntityType, entityID string, oldData, newData interface{}) error {
	args := m.Called(action, entityType, entityID, oldData, newData)
	return args.Error(0)
}

func (m *mockAuditService) List(ctx context.Context, filter *audit_model.AuditLogFilter) (*audit_model.AuditLogResponse, error) {
	return nil, nil
}

func setupHandler(t *testing.T, svc *mockAppointmentService, auditSvc *mockAuditService, clinicID, userID uuid.UUID) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := handler.NewAppointmentHandler(svc, handler.WithAuditService(auditSvc))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("clinic_id", clinicID)
		c.Set("user_id", userID)
		c.Set("email", "admin@example.com")
		c.Set("user_role", "admin")
		c.Next()
	})
	h.RegisterRoutes(router)
	return router
}

func TestCreate_EmitsCreateAuditLog(t *testing.T) {
	clinicID := uuid.New()
	userID := uuid.New()
	appointmentID := uuid.New()

	created := &model.AppointmentResponse{
		ID:             appointmentID,
		ClinicID:       clinicID,
		PatientID:      uuid.New(),
		ProfessionalID: uuid.New(),
		StartTime:      time.Now(),
		EndTime:        time.Now().Add(30 * time.Minute),
	}

	svc := new(mockAppointmentService)
	svc.On("Create", mock.Anything, clinicID, mock.Anything).Return(created, nil).Once()

	auditSvc := new(mockAuditService)
	auditSvc.On("LogFromGin",
		audit_model.AuditActionCreate,
		audit_model.AuditEntityAppointment,
		appointmentID.String(),
		nil,
		created,
	).Return(nil).Once()

	router := setupHandler(t, svc, auditSvc, clinicID, userID)

	reqBody := model.CreateAppointmentRequest{
		PatientID:       uuid.New(),
		ProfessionalID:  uuid.New(),
		StartTime:       time.Now(),
		DurationMinutes: 30,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/appointments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	svc.AssertExpectations(t)
	auditSvc.AssertExpectations(t)
}
