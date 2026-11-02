package service_test

import (
	"context"
	"testing"
	"time"

	account_model "github.com/aseno/backend/pkg/account/model"
	"github.com/aseno/backend/pkg/analytics/model"
	"github.com/aseno/backend/pkg/analytics/service"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock analytics repository ---

type mockAnalyticsRepo struct {
	financialSummary       *model.FinancialSummary
	dailyReceivables       []model.DailyReceivable
	paymentMethodBreakdown []model.PaymentMethodBreakdown
	unconciliated          []model.UnconciliatedAppointment
	monthlyMetrics         []model.MonthlyMetrics
	whatsappMetrics        *model.WhatsAppMetrics
	appointmentsSummary    *model.AppointmentsSummary
}

func (m *mockAnalyticsRepo) GetStatsByDayOfWeek(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) ([]model.TimeSlotStats, error) {
	return nil, nil
}
func (m *mockAnalyticsRepo) GetStatsByHour(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) ([]model.TimeSlotStats, error) {
	return nil, nil
}
func (m *mockAnalyticsRepo) GetStatsByDayAndHour(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) ([]model.TimeSlotStats, error) {
	return nil, nil
}
func (m *mockAnalyticsRepo) GetOverallStats(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) (*model.AnalyticsSummary, error) {
	return nil, nil
}
func (m *mockAnalyticsRepo) GetReportMetrics(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) ([]model.MonthlyMetrics, error) {
	return m.monthlyMetrics, nil
}
func (m *mockAnalyticsRepo) GetWhatsAppMetrics(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) (*model.WhatsAppMetrics, error) {
	return m.whatsappMetrics, nil
}
func (m *mockAnalyticsRepo) GetAppointmentsSummary(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) (*model.AppointmentsSummary, error) {
	return m.appointmentsSummary, nil
}
func (m *mockAnalyticsRepo) GetFinancialSummary(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) (*model.FinancialSummary, error) {
	return m.financialSummary, nil
}
func (m *mockAnalyticsRepo) GetDailyReceivables(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) ([]model.DailyReceivable, error) {
	return m.dailyReceivables, nil
}
func (m *mockAnalyticsRepo) GetPaymentMethodBreakdown(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]model.PaymentMethodBreakdown, error) {
	return m.paymentMethodBreakdown, nil
}
func (m *mockAnalyticsRepo) GetUnconciliatedAppointments(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _ time.Time) ([]model.UnconciliatedAppointment, error) {
	return m.unconciliated, nil
}

// --- Mock clinic repository ---

type mockClinicRepo struct{}

func (m *mockClinicRepo) GetByID(_ context.Context, _ uuid.UUID) (*clinic_model.Clinic, error) {
	return &clinic_model.Clinic{
		Account: &account_model.Account{
			Plan: &plan_model.Plan{Slug: "professional"},
		},
	}, nil
}
func (m *mockClinicRepo) GetBySlug(_ context.Context, _ string) (*clinic_model.Clinic, error) {
	return nil, nil
}
func (m *mockClinicRepo) GetByEmail(_ context.Context, _ string) (*clinic_model.Clinic, error) {
	return nil, nil
}
func (m *mockClinicRepo) Update(_ context.Context, _ *clinic_model.Clinic) error { return nil }
func (m *mockClinicRepo) Create(_ context.Context, _ *clinic_model.Clinic) error { return nil }
func (m *mockClinicRepo) Delete(_ context.Context, _ uuid.UUID) error            { return nil }
func (m *mockClinicRepo) List(_ context.Context, _, _ int) ([]*clinic_model.Clinic, int64, error) {
	return nil, 0, nil
}
func (m *mockClinicRepo) ExistsBySlug(_ context.Context, _ string) (bool, error)  { return false, nil }
func (m *mockClinicRepo) ExistsByEmail(_ context.Context, _ string) (bool, error) { return false, nil }
func (m *mockClinicRepo) CountByAccountID(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

// --- Tests ---

func TestGetFinancialReport_EmptyData(t *testing.T) {
	repo := &mockAnalyticsRepo{
		financialSummary:       &model.FinancialSummary{},
		dailyReceivables:       []model.DailyReceivable{},
		paymentMethodBreakdown: []model.PaymentMethodBreakdown{},
		unconciliated:          []model.UnconciliatedAppointment{},
	}

	svc := service.NewAnalyticsService(repo, &mockClinicRepo{})

	report, err := svc.GetFinancialReport(context.Background(), uuid.New(), &model.FinancialFilter{
		Month: "2026-04",
	})

	require.NoError(t, err)
	assert.Equal(t, float64(0), report.Summary.TotalExpected)
	assert.Equal(t, float64(0), report.Summary.TotalReceived)
	assert.Equal(t, float64(0), report.Summary.TotalPending)
	assert.Equal(t, 0, report.Summary.TotalSessions)
	assert.Empty(t, report.DailyBreakdown)
	assert.Empty(t, report.PaymentMethodBreakdown)
	assert.Empty(t, report.UnconciliatedAppointments)
}

func TestGetFinancialReport_WithMixedPayments(t *testing.T) {
	repo := &mockAnalyticsRepo{
		financialSummary: &model.FinancialSummary{
			TotalExpected:    5000,
			TotalReceived:    3500,
			TotalPending:     1500,
			TotalDiscount:    0,
			TotalSessions:    10,
			PaidSessions:     7,
			PendingSessions:  3,
			ConciliationRate: 70,
		},
		dailyReceivables: []model.DailyReceivable{
			{Date: "2026-04-07", Sessions: 5, Expected: 2500, Received: 2000, Pending: 500},
			{Date: "2026-04-06", Sessions: 5, Expected: 2500, Received: 1500, Pending: 1000},
		},
		paymentMethodBreakdown: []model.PaymentMethodBreakdown{
			{MethodName: "PIX", MethodType: "pix", Total: 2000, Count: 4, Percentage: 57.14},
			{MethodName: "Cartao", MethodType: "credit", Total: 1500, Count: 3, Percentage: 42.86},
		},
		unconciliated: []model.UnconciliatedAppointment{
			{AppointmentID: uuid.New().String(), Date: "2026-04-07", PatientName: "Joao", ProfessionalName: "Dra. Ana", Price: 500, PaymentStatus: "pending"},
		},
	}

	svc := service.NewAnalyticsService(repo, &mockClinicRepo{})

	report, err := svc.GetFinancialReport(context.Background(), uuid.New(), &model.FinancialFilter{
		Month: "2026-04",
	})

	require.NoError(t, err)
	assert.Equal(t, float64(5000), report.Summary.TotalExpected)
	assert.Equal(t, float64(3500), report.Summary.TotalReceived)
	assert.Equal(t, float64(1500), report.Summary.TotalPending)
	assert.Equal(t, 10, report.Summary.TotalSessions)
	assert.Equal(t, 7, report.Summary.PaidSessions)
	assert.Equal(t, float64(70), report.Summary.ConciliationRate)
	assert.Len(t, report.DailyBreakdown, 2)
	assert.Len(t, report.PaymentMethodBreakdown, 2)
	assert.Len(t, report.UnconciliatedAppointments, 1)
}

func TestGetFinancialReport_WithProfessionalFilter(t *testing.T) {
	repo := &mockAnalyticsRepo{
		financialSummary: &model.FinancialSummary{
			TotalExpected:    2000,
			TotalReceived:    2000,
			TotalPending:     0,
			TotalSessions:    4,
			PaidSessions:     4,
			PendingSessions:  0,
			ConciliationRate: 100,
		},
		dailyReceivables:       []model.DailyReceivable{},
		paymentMethodBreakdown: []model.PaymentMethodBreakdown{},
		unconciliated:          []model.UnconciliatedAppointment{},
	}

	svc := service.NewAnalyticsService(repo, &mockClinicRepo{})
	profID := uuid.New()

	report, err := svc.GetFinancialReport(context.Background(), uuid.New(), &model.FinancialFilter{
		Month:          "2026-04",
		ProfessionalID: &profID,
	})

	require.NoError(t, err)
	assert.Equal(t, float64(2000), report.Summary.TotalExpected)
	assert.Equal(t, float64(0), report.Summary.TotalPending)
	assert.Equal(t, float64(100), report.Summary.ConciliationRate)
	assert.Empty(t, report.UnconciliatedAppointments)
}

func TestGetFinancialReport_DefaultMonth(t *testing.T) {
	repo := &mockAnalyticsRepo{
		financialSummary:       &model.FinancialSummary{},
		dailyReceivables:       []model.DailyReceivable{},
		paymentMethodBreakdown: []model.PaymentMethodBreakdown{},
		unconciliated:          []model.UnconciliatedAppointment{},
	}

	svc := service.NewAnalyticsService(repo, &mockClinicRepo{})

	report, err := svc.GetFinancialReport(context.Background(), uuid.New(), &model.FinancialFilter{
		Month: "",
	})

	require.NoError(t, err)
	assert.NotNil(t, report)
}
