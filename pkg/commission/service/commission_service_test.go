package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	appointmentModel "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/commission/model"
	professionalModel "github.com/aseno/backend/pkg/professional/model"
)

// --- Mock: CommissionRepository ---

type mockCommissionRepo struct{ mock.Mock }

func (m *mockCommissionRepo) Create(ctx context.Context, c *model.Commission) error {
	return m.Called(ctx, c).Error(0)
}

func (m *mockCommissionRepo) GetByAppointmentID(ctx context.Context, appointmentID, clinicID uuid.UUID) (*model.Commission, error) {
	args := m.Called(ctx, appointmentID, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Commission), args.Error(1)
}

func (m *mockCommissionRepo) ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID, from, to time.Time) ([]model.Commission, error) {
	args := m.Called(ctx, professionalID, clinicID, from, to)
	return args.Get(0).([]model.Commission), args.Error(1)
}

func (m *mockCommissionRepo) Summary(ctx context.Context, clinicID uuid.UUID, from, to time.Time) ([]model.ProfessionalCommissionSummary, error) {
	args := m.Called(ctx, clinicID, from, to)
	return args.Get(0).([]model.ProfessionalCommissionSummary), args.Error(1)
}

func (m *mockCommissionRepo) BulkMarkPaid(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error {
	return m.Called(ctx, ids, clinicID).Error(0)
}

func (m *mockCommissionRepo) DeleteByAppointmentID(ctx context.Context, appointmentID, clinicID uuid.UUID) error {
	return m.Called(ctx, appointmentID, clinicID).Error(0)
}

func (m *mockCommissionRepo) GetBySessionPlanAndDate(ctx context.Context, sessionPlanID, clinicID uuid.UUID, date string) ([]model.Commission, error) {
	args := m.Called(ctx, sessionPlanID, clinicID, date)
	return args.Get(0).([]model.Commission), args.Error(1)
}

func (m *mockCommissionRepo) DeleteBySessionPlanAndDate(ctx context.Context, sessionPlanID, clinicID uuid.UUID, date string) error {
	return m.Called(ctx, sessionPlanID, clinicID, date).Error(0)
}

func (m *mockCommissionRepo) GetByIDs(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) ([]model.Commission, error) {
	args := m.Called(ctx, ids, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Commission), args.Error(1)
}

// --- Mock: ProfessionalProcedureLookup ---

type mockPPLookup struct{ mock.Mock }

func (m *mockPPLookup) ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID) ([]professionalModel.ProfessionalProcedure, error) {
	args := m.Called(ctx, professionalID, clinicID)
	return args.Get(0).([]professionalModel.ProfessionalProcedure), args.Error(1)
}

// --- Mock: AppointmentLookup ---

type mockAptLookup struct{ mock.Mock }

func (m *mockAptLookup) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*appointmentModel.Appointment, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*appointmentModel.Appointment), args.Error(1)
}

// --- Helpers ---

var (
	testClinicID = uuid.New()
	testProfID   = uuid.New()
	testProcID   = uuid.New()
	testAptID    = uuid.New()
)

func newTestService() (CommissionService, *mockCommissionRepo, *mockPPLookup, *mockAptLookup) {
	repo := new(mockCommissionRepo)
	ppLookup := new(mockPPLookup)
	aptLookup := new(mockAptLookup)
	svc := NewCommissionService(repo, ppLookup, aptLookup)
	return svc, repo, ppLookup, aptLookup
}

// --- Tests ---

func TestCalculateAndStore_Percentage(t *testing.T) {
	svc, repo, ppLookup, _ := newTestService()
	ctx := context.Background()

	appointment := &appointmentModel.Appointment{
		ID:             testAptID,
		ProfessionalID: testProfID,
		Procedures: []appointmentModel.AppointmentProcedure{
			{ProcedureID: testProcID, Price: 150},
		},
	}

	ppLookup.On("ListByProfessional", ctx, testProfID, testClinicID).
		Return([]professionalModel.ProfessionalProcedure{
			{ProcedureID: testProcID, CommissionType: "percentage", CommissionValue: 40},
		}, nil)

	repo.On("Create", ctx, mock.MatchedBy(func(c *model.Commission) bool {
		return c.CommissionValue == 60 && // 150 * 40% = 60
			c.CommissionType == "percentage" &&
			c.CommissionRate == 40 &&
			c.SessionPrice == 150 &&
			c.Status == model.CommissionStatusPending
	})).Return(nil)

	err := svc.CalculateAndStore(ctx, appointment, testClinicID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestCalculateAndStore_Fixed(t *testing.T) {
	svc, repo, ppLookup, _ := newTestService()
	ctx := context.Background()

	appointment := &appointmentModel.Appointment{
		ID:             testAptID,
		ProfessionalID: testProfID,
		Procedures: []appointmentModel.AppointmentProcedure{
			{ProcedureID: testProcID, Price: 200},
		},
	}

	ppLookup.On("ListByProfessional", ctx, testProfID, testClinicID).
		Return([]professionalModel.ProfessionalProcedure{
			{ProcedureID: testProcID, CommissionType: "fixed", CommissionValue: 80},
		}, nil)

	repo.On("Create", ctx, mock.MatchedBy(func(c *model.Commission) bool {
		return c.CommissionValue == 80 &&
			c.CommissionType == "fixed" &&
			c.SessionPrice == 200
	})).Return(nil)

	err := svc.CalculateAndStore(ctx, appointment, testClinicID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestCalculateAndStore_NoProcedures(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	appointment := &appointmentModel.Appointment{
		ID:             testAptID,
		ProfessionalID: testProfID,
		Procedures:     []appointmentModel.AppointmentProcedure{},
	}

	err := svc.CalculateAndStore(ctx, appointment, testClinicID)

	assert.NoError(t, err)
}

func TestCalculateAndStore_NoCommissionConfig(t *testing.T) {
	svc, repo, ppLookup, _ := newTestService()
	ctx := context.Background()

	appointment := &appointmentModel.Appointment{
		ID:             testAptID,
		ProfessionalID: testProfID,
		Procedures: []appointmentModel.AppointmentProcedure{
			{ProcedureID: testProcID, Price: 150},
		},
	}

	// No commission configured for this procedure
	ppLookup.On("ListByProfessional", ctx, testProfID, testClinicID).
		Return([]professionalModel.ProfessionalProcedure{}, nil)

	err := svc.CalculateAndStore(ctx, appointment, testClinicID)

	assert.NoError(t, err)
	repo.AssertNotCalled(t, "Create")
}

func TestCalculateAndStore_FixedCappedAtPrice(t *testing.T) {
	svc, repo, ppLookup, _ := newTestService()
	ctx := context.Background()

	appointment := &appointmentModel.Appointment{
		ID:             testAptID,
		ProfessionalID: testProfID,
		Procedures: []appointmentModel.AppointmentProcedure{
			{ProcedureID: testProcID, Price: 50}, // Price is 50
		},
	}

	ppLookup.On("ListByProfessional", ctx, testProfID, testClinicID).
		Return([]professionalModel.ProfessionalProcedure{
			{ProcedureID: testProcID, CommissionType: "fixed", CommissionValue: 100}, // Commission is 100 > price 50
		}, nil)

	repo.On("Create", ctx, mock.MatchedBy(func(c *model.Commission) bool {
		return c.CommissionValue == 50 // Capped at session price
	})).Return(nil)

	err := svc.CalculateAndStore(ctx, appointment, testClinicID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestVoidByAppointment(t *testing.T) {
	svc, repo, _, _ := newTestService()
	ctx := context.Background()

	repo.On("DeleteByAppointmentID", ctx, testAptID, testClinicID).Return(nil)

	err := svc.VoidByAppointment(ctx, testAptID, testClinicID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

// --- Group Session Commission Tests ---

var testPlanID = uuid.New()

func TestCalculateAndStoreForGroupSession_Percentage(t *testing.T) {
	svc, repo, ppLookup, _ := newTestService()
	ctx := context.Background()

	input := &GroupSessionCommissionInput{
		SessionPlanID:  testPlanID,
		SessionDate:    "2026-04-09",
		ProfessionalID: testProfID,
		ProcedureID:    testProcID,
		SessionPrice:   200,
	}

	// No existing commission
	repo.On("GetBySessionPlanAndDate", ctx, testPlanID, testClinicID, "2026-04-09").
		Return([]model.Commission{}, nil)

	ppLookup.On("ListByProfessional", ctx, testProfID, testClinicID).
		Return([]professionalModel.ProfessionalProcedure{
			{ProcedureID: testProcID, CommissionType: "percentage", CommissionValue: 50},
		}, nil)

	repo.On("Create", ctx, mock.MatchedBy(func(c *model.Commission) bool {
		return c.CommissionValue == 100 && // 200 * 50% = 100
			c.SessionPlanID != nil && *c.SessionPlanID == testPlanID &&
			c.SessionDate == "2026-04-09" &&
			c.AppointmentID == nil &&
			c.Status == model.CommissionStatusPending
	})).Return(nil)

	err := svc.CalculateAndStoreForGroupSession(ctx, input, testClinicID)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestCalculateAndStoreForGroupSession_AlreadyExists_Idempotent(t *testing.T) {
	svc, repo, _, _ := newTestService()
	ctx := context.Background()

	input := &GroupSessionCommissionInput{
		SessionPlanID:  testPlanID,
		SessionDate:    "2026-04-09",
		ProfessionalID: testProfID,
		ProcedureID:    testProcID,
		SessionPrice:   200,
	}

	// Already exists
	repo.On("GetBySessionPlanAndDate", ctx, testPlanID, testClinicID, "2026-04-09").
		Return([]model.Commission{{ID: uuid.New()}}, nil)

	err := svc.CalculateAndStoreForGroupSession(ctx, input, testClinicID)
	assert.NoError(t, err)
	repo.AssertNotCalled(t, "Create")
}

func TestCalculateAndStoreForGroupSession_NoCommissionConfig(t *testing.T) {
	svc, repo, ppLookup, _ := newTestService()
	ctx := context.Background()

	input := &GroupSessionCommissionInput{
		SessionPlanID:  testPlanID,
		SessionDate:    "2026-04-09",
		ProfessionalID: testProfID,
		ProcedureID:    testProcID,
		SessionPrice:   200,
	}

	repo.On("GetBySessionPlanAndDate", ctx, testPlanID, testClinicID, "2026-04-09").
		Return([]model.Commission{}, nil)

	// No commission config for this professional
	ppLookup.On("ListByProfessional", ctx, testProfID, testClinicID).
		Return([]professionalModel.ProfessionalProcedure{}, nil)

	err := svc.CalculateAndStoreForGroupSession(ctx, input, testClinicID)
	assert.NoError(t, err)
	repo.AssertNotCalled(t, "Create")
}

func TestVoidByGroupSession(t *testing.T) {
	svc, repo, _, _ := newTestService()
	ctx := context.Background()

	repo.On("DeleteBySessionPlanAndDate", ctx, testPlanID, testClinicID, "2026-04-09").Return(nil)

	err := svc.VoidByGroupSession(ctx, testPlanID, testClinicID, "2026-04-09")
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestGetSummary(t *testing.T) {
	svc, repo, _, _ := newTestService()
	ctx := context.Background()

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	repo.On("Summary", ctx, testClinicID, from, to).
		Return([]model.ProfessionalCommissionSummary{
			{ProfessionalID: testProfID, ProfessionalName: "Dra. Ana", TotalSessions: 10, TotalCommission: 500, TotalPaid: 200, TotalPending: 300},
		}, nil)

	result, err := svc.GetSummary(ctx, testClinicID, from, to)

	assert.NoError(t, err)
	assert.Len(t, result.Professionals, 1)
	assert.Equal(t, 10, result.TotalSessions)
	assert.Equal(t, 500.0, result.TotalCommission)
	assert.Equal(t, 200.0, result.TotalPaid)
	assert.Equal(t, 300.0, result.TotalPending)
}
