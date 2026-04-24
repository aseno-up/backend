package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/sessionplan/model"
)

// --- Mock Repositories ---

type mockPlanRepo struct {
	plan    *model.SessionPlan
	updated *model.SessionPlan
}

func (m *mockPlanRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*model.SessionPlan, error) {
	return m.plan, nil
}
func (m *mockPlanRepo) Create(_ context.Context, _ *model.SessionPlan) error { return nil }
func (m *mockPlanRepo) GetByClinic(_ context.Context, _ uuid.UUID, _ pagination.Filter) ([]*model.SessionPlan, int64, error) {
	return nil, 0, nil
}
func (m *mockPlanRepo) GetByPatient(_ context.Context, _, _ uuid.UUID) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) GetByProfessional(_ context.Context, _, _ uuid.UUID) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) GetActivePlansForGeneration(_ context.Context, _ time.Time) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) GetAllActive(_ context.Context, _ uuid.UUID) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) Update(_ context.Context, plan *model.SessionPlan) error {
	m.updated = plan
	return nil
}
func (m *mockPlanRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ model.PlanStatus) error {
	return nil
}
func (m *mockPlanRepo) IncrementSessionsDone(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockPlanRepo) UpdateLastGenerated(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockPlanRepo) Delete(_ context.Context, _, _ uuid.UUID) error                 { return nil }
func (m *mockPlanRepo) BulkCancel(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error { return nil }

type mockAptRepo struct{}

func (m *mockAptRepo) Create(_ context.Context, _ *appointment_model.Appointment) error { return nil }
func (m *mockAptRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepo) GetByDateRange(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepo) GetByDateRangeWithExceptions(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepo) FindPlanAppointmentOnDate(_ context.Context, _, _ uuid.UUID, _ time.Time) (*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepo) GetByDay(_ context.Context, _ uuid.UUID, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepo) Update(_ context.Context, _ *appointment_model.Appointment) error { return nil }
func (m *mockAptRepo) CheckConflict(_ context.Context, _, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockAptRepo) List(_ context.Context, _ uuid.UUID, _ *appointment_model.AppointmentFilter) ([]*appointment_model.Appointment, int64, error) {
	return nil, 0, nil
}
func (m *mockAptRepo) GetByPatient(_ context.Context, _, _ uuid.UUID, _ int) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ appointment_model.AppointmentStatus) error {
	return nil
}
func (m *mockAptRepo) GetPendingReminders(_ context.Context, _ string, _, _ time.Time) ([]*appointment_model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockAptRepo) MarkReminderSent(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockAptRepo) GetPendingByPhone(_ context.Context, _ uuid.UUID, _ string) (*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAptRepo) GetExpiredConfirmations(_ context.Context, _ int) ([]*appointment_model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockAptRepo) MarkConfirmationExpired(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockAptRepo) SetConfirmationExpiresAt(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockAptRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockAptRepo) DeleteCascade(_ context.Context, _, _ uuid.UUID, _ bool) error {
	return nil
}
func (m *mockAptRepo) SoftDeleteAsCanceledException(_ context.Context, _, _ uuid.UUID, _ bool) error {
	return nil
}
func (m *mockAptRepo) GetDeletePreview(_ context.Context, _, _ uuid.UUID) (*appointment_model.AppointmentDeletePreview, error) {
	return &appointment_model.AppointmentDeletePreview{}, nil
}
func (m *mockAptRepo) GetSlotPerformanceStats(_ context.Context, _, _ uuid.UUID) ([]*appointment_model.SlotPerformanceStats, error) {
	return nil, nil
}
func (m *mockAptRepo) CountByProfessionalID(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockAptRepo) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, _ appointment_model.PaymentStatus) error {
	return nil
}

// --- Mock PlanPaymentLister + FinancePlanUpserter ---

type mockPlanPaymentLister struct {
	payments []*model.SessionPlanPayment
}

func (m *mockPlanPaymentLister) GetByPlan(_ context.Context, _, _ uuid.UUID) ([]*model.SessionPlanPayment, error) {
	return m.payments, nil
}

type mockPlanFinanceUpserter struct {
	deletedIDs []uuid.UUID
}

func (m *mockPlanFinanceUpserter) UpsertFromSessionPlanPayment(_ context.Context, _ *model.SessionPlanPayment, _ uuid.UUID, _, _ string) error {
	return nil
}

func (m *mockPlanFinanceUpserter) DeleteFromSessionPlanPayment(_ context.Context, sessionPlanPaymentID, _ uuid.UUID) error {
	m.deletedIDs = append(m.deletedIDs, sessionPlanPaymentID)
	return nil
}

// --- Tests ---

func basePlan() *model.SessionPlan {
	profID := uuid.New()
	return &model.SessionPlan{
		ID:             uuid.New(),
		ClinicID:       uuid.New(),
		PatientID:      uuid.New(),
		ProfessionalID: &profID,
		Title:          "Fisioterapia",
		Status:         model.PlanStatusActive,
		Type:           model.PlanTypeIndividual,
		Frequency:      model.FrequencyWeekly,
		WeekDays:       pq.Int64Array{1, 3}, // Mon, Wed
		RRule:          "FREQ=WEEKLY;BYDAY=MO,WE",
		PreferredTime:  "09:00",
		DurationMins:   50,
		TotalSessions:  10,
		SessionsDone:   0,
		SessionPrice:   150.0,
	}
}

func TestUpdate_LegacyFields(t *testing.T) {
	plan := basePlan()
	repo := &mockPlanRepo{plan: plan}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	title := "Novo Título"
	notes := "Notas atualizadas"
	req := &model.UpdateSessionPlanRequest{
		Title: &title,
		Notes: &notes,
	}

	result, err := svc.Update(context.Background(), plan.ClinicID, plan.ID, req)
	require.NoError(t, err)
	assert.Equal(t, "Novo Título", result.Title)
	assert.Equal(t, "Notas atualizadas", result.Notes)
	// RRULE should not change
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO,WE", result.RRule)
}

func TestUpdate_Frequency_RecalculatesRRule(t *testing.T) {
	plan := basePlan()
	repo := &mockPlanRepo{plan: plan}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	newFreq := model.FrequencyBiweekly
	newWeekDays := []int{1, 3, 5} // Mon, Wed, Fri
	req := &model.UpdateSessionPlanRequest{
		Frequency: &newFreq,
		WeekDays:  &newWeekDays,
	}

	_, err := svc.Update(context.Background(), plan.ClinicID, plan.ID, req)
	require.NoError(t, err)

	updated := repo.updated
	require.NotNil(t, updated)
	assert.Equal(t, model.FrequencyBiweekly, updated.Frequency)
	assert.Contains(t, updated.RRule, "FREQ=WEEKLY")
	assert.Contains(t, updated.RRule, "MO")
	assert.Contains(t, updated.RRule, "WE")
	assert.Contains(t, updated.RRule, "FR")
}

func TestUpdate_WeekDays_RecalculatesRRule(t *testing.T) {
	plan := basePlan()
	repo := &mockPlanRepo{plan: plan}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	newWeekDays := []int{2, 4} // Tue, Thu
	req := &model.UpdateSessionPlanRequest{
		WeekDays: &newWeekDays,
	}

	_, err := svc.Update(context.Background(), plan.ClinicID, plan.ID, req)
	require.NoError(t, err)

	updated := repo.updated
	require.NotNil(t, updated)
	assert.Contains(t, updated.RRule, "TU")
	assert.Contains(t, updated.RRule, "TH")
	assert.NotContains(t, updated.RRule, "MO")
}

func TestUpdate_DayTimes(t *testing.T) {
	plan := basePlan()
	repo := &mockPlanRepo{plan: plan}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	dayTimes := model.DayTimesMap{"MO": "08:00", "WE": "14:00"}
	req := &model.UpdateSessionPlanRequest{
		DayTimes: &dayTimes,
	}

	_, err := svc.Update(context.Background(), plan.ClinicID, plan.ID, req)
	require.NoError(t, err)

	updated := repo.updated
	require.NotNil(t, updated)
	assert.Equal(t, "08:00", updated.DayTimes["MO"])
	assert.Equal(t, "14:00", updated.DayTimes["WE"])
}

func TestUpdate_ProfessionalID(t *testing.T) {
	plan := basePlan()
	repo := &mockPlanRepo{plan: plan}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	newProfID := uuid.New()
	req := &model.UpdateSessionPlanRequest{
		ProfessionalID: &newProfID,
	}

	_, err := svc.Update(context.Background(), plan.ClinicID, plan.ID, req)
	require.NoError(t, err)

	updated := repo.updated
	require.NotNil(t, updated)
	assert.Equal(t, newProfID, *updated.ProfessionalID)
}

func TestUpdate_Capacity(t *testing.T) {
	plan := basePlan()
	plan.Type = model.PlanTypeGroup
	plan.Capacity = intPtr(10)
	repo := &mockPlanRepo{plan: plan}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	newCap := 15
	req := &model.UpdateSessionPlanRequest{
		Capacity: &newCap,
	}

	_, err := svc.Update(context.Background(), plan.ClinicID, plan.ID, req)
	require.NoError(t, err)

	updated := repo.updated
	require.NotNil(t, updated)
	assert.Equal(t, 15, *updated.Capacity)
}

func TestUpdate_DurationMins(t *testing.T) {
	plan := basePlan()
	repo := &mockPlanRepo{plan: plan}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	newDuration := 60
	req := &model.UpdateSessionPlanRequest{
		DurationMins: &newDuration,
	}

	_, err := svc.Update(context.Background(), plan.ClinicID, plan.ID, req)
	require.NoError(t, err)

	updated := repo.updated
	require.NotNil(t, updated)
	assert.Equal(t, 60, updated.DurationMins)
}

func intPtr(v int) *int {
	return &v
}

func TestCreate_IndividualMonthly_RequiresBillingDay(t *testing.T) {
	repo := &mockPlanRepo{plan: basePlan()}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	procID := uuid.New()
	req := &model.CreateSessionPlanRequest{
		PatientID:     uuid.New(),
		Title:         "Plano Mensal",
		Type:          model.PlanTypeIndividual,
		PaymentMode:   model.PaymentModeMonthly,
		Frequency:     model.FrequencyWeekly,
		PreferredTime: "09:00",
		DurationMins:  50,
		TotalSessions: 10,
		StartDate:     "2026-05-15",
		ProcedureID:   &procID,
		// BillingDay intentionally omitted
	}

	_, err := svc.Create(context.Background(), uuid.New(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "billing_day")
}

func TestCreate_IndividualMonthly_BillingDayOutOfRange(t *testing.T) {
	repo := &mockPlanRepo{plan: basePlan()}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	procID := uuid.New()
	bad := 32
	req := &model.CreateSessionPlanRequest{
		PatientID:     uuid.New(),
		Title:         "Plano Mensal",
		Type:          model.PlanTypeIndividual,
		PaymentMode:   model.PaymentModeMonthly,
		Frequency:     model.FrequencyWeekly,
		PreferredTime: "09:00",
		DurationMins:  50,
		TotalSessions: 10,
		StartDate:     "2026-05-15",
		ProcedureID:   &procID,
		BillingDay:    &bad,
	}

	_, err := svc.Create(context.Background(), uuid.New(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "between 1 and 31")
}

func TestCreate_IndividualMonthly_ValidBillingDay(t *testing.T) {
	repo := &mockPlanRepo{plan: basePlan()}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	procID := uuid.New()
	day := 15
	req := &model.CreateSessionPlanRequest{
		PatientID:     uuid.New(),
		Title:         "Plano Mensal",
		Type:          model.PlanTypeIndividual,
		PaymentMode:   model.PaymentModeMonthly,
		Frequency:     model.FrequencyWeekly,
		PreferredTime: "09:00",
		DurationMins:  50,
		TotalSessions: 10,
		StartDate:     "2026-05-15",
		ProcedureID:   &procID,
		BillingDay:    &day,
	}

	_, err := svc.Create(context.Background(), uuid.New(), req)
	require.NoError(t, err)
}

func TestCreate_IndividualPerSession_BillingDayNotRequired(t *testing.T) {
	repo := &mockPlanRepo{plan: basePlan()}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	procID := uuid.New()
	req := &model.CreateSessionPlanRequest{
		PatientID:     uuid.New(),
		Title:         "Plano Per Session",
		Type:          model.PlanTypeIndividual,
		PaymentMode:   model.PaymentModePerSession,
		Frequency:     model.FrequencyWeekly,
		PreferredTime: "09:00",
		DurationMins:  50,
		TotalSessions: 10,
		StartDate:     "2026-05-15",
		ProcedureID:   &procID,
	}

	_, err := svc.Create(context.Background(), uuid.New(), req)
	require.NoError(t, err)
}

func TestUpdate_BillingDay(t *testing.T) {
	plan := basePlan()
	repo := &mockPlanRepo{plan: plan}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	day := 20
	req := &model.UpdateSessionPlanRequest{
		BillingDay: &day,
	}

	_, err := svc.Update(context.Background(), plan.ClinicID, plan.ID, req)
	require.NoError(t, err)

	updated := repo.updated
	require.NotNil(t, updated)
	require.NotNil(t, updated.BillingDay)
	assert.Equal(t, 20, *updated.BillingDay)
}

// Regression: deleting a session plan that has payments must also drop the
// corresponding rows from financial_transactions. Same bug as appointments —
// the plan delete path only touches the plan row; orphan finance rows keep
// showing up on the Lançamentos tab.
func TestDelete_CleansFinanceForEachPlanPayment(t *testing.T) {
	plan := basePlan()
	repo := &mockPlanRepo{plan: plan}

	pay1 := &model.SessionPlanPayment{ID: uuid.New(), ClinicID: plan.ClinicID, SessionPlanID: plan.ID}
	pay2 := &model.SessionPlanPayment{ID: uuid.New(), ClinicID: plan.ClinicID, SessionPlanID: plan.ID}
	lister := &mockPlanPaymentLister{payments: []*model.SessionPlanPayment{pay1, pay2}}
	finance := &mockPlanFinanceUpserter{}

	svc := NewSessionPlanService(repo, &mockAptRepo{})
	svc.SetPaymentLister(lister)
	svc.SetFinanceUpserter(finance)

	err := svc.Delete(context.Background(), plan.ClinicID, plan.ID)
	require.NoError(t, err)

	assert.ElementsMatch(t,
		[]uuid.UUID{pay1.ID, pay2.ID},
		finance.deletedIDs,
		"every plan payment must have its finance row cleaned up",
	)
}

// Defensive: without the hooks wired (legacy call sites, older tests) Delete
// must still succeed — the cleanup is best-effort, not a blocking step.
func TestDelete_WithoutHooks_Succeeds(t *testing.T) {
	plan := basePlan()
	repo := &mockPlanRepo{plan: plan}
	svc := NewSessionPlanService(repo, &mockAptRepo{})

	err := svc.Delete(context.Background(), plan.ClinicID, plan.ID)
	require.NoError(t, err)
}
