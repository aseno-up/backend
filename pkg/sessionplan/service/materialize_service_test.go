package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/sessionplan/model"
)

// --- Mock Repositories ---

type mockMaterializePlanRepo struct {
	plan *model.SessionPlan
}

func (m *mockMaterializePlanRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*model.SessionPlan, error) {
	return m.plan, nil
}

// Stub remaining interface methods
func (m *mockMaterializePlanRepo) Create(_ context.Context, _ *model.SessionPlan) error { return nil }
func (m *mockMaterializePlanRepo) GetByClinic(_ context.Context, _ uuid.UUID, _ pagination.Filter) ([]*model.SessionPlan, int64, error) {
	return nil, 0, nil
}
func (m *mockMaterializePlanRepo) GetByPatient(_ context.Context, _, _ uuid.UUID) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockMaterializePlanRepo) GetByProfessional(_ context.Context, _, _ uuid.UUID) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockMaterializePlanRepo) GetActivePlansForGeneration(_ context.Context, _ time.Time) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockMaterializePlanRepo) GetAllActive(_ context.Context, _ uuid.UUID) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockMaterializePlanRepo) Update(_ context.Context, _ *model.SessionPlan) error { return nil }
func (m *mockMaterializePlanRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ model.PlanStatus) error {
	return nil
}
func (m *mockMaterializePlanRepo) IncrementSessionsDone(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *mockMaterializePlanRepo) UpdateLastGenerated(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockMaterializePlanRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockMaterializePlanRepo) BulkCancel(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}

type mockMaterializeAppointmentRepo struct {
	appointments []*appointment_model.Appointment
	created      *appointment_model.Appointment
}

func (m *mockMaterializeAppointmentRepo) Create(_ context.Context, apt *appointment_model.Appointment) error {
	apt.ID = uuid.New()
	m.created = apt
	return nil
}

func (m *mockMaterializeAppointmentRepo) GetByID(_ context.Context, id, _ uuid.UUID) (*appointment_model.Appointment, error) {
	if m.created != nil && m.created.ID == id {
		return m.created, nil
	}
	return &appointment_model.Appointment{ID: id}, nil
}

func (m *mockMaterializeAppointmentRepo) GetByDateRange(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return m.appointments, nil
}

// Stub remaining interface methods
func (m *mockMaterializeAppointmentRepo) List(_ context.Context, _ uuid.UUID, _ *appointment_model.AppointmentFilter) ([]*appointment_model.Appointment, int64, error) {
	return nil, 0, nil
}
func (m *mockMaterializeAppointmentRepo) GetByDay(_ context.Context, _ uuid.UUID, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockMaterializeAppointmentRepo) Update(_ context.Context, _ *appointment_model.Appointment) error {
	return nil
}
func (m *mockMaterializeAppointmentRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockMaterializeAppointmentRepo) CheckConflict(_ context.Context, _, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockMaterializeAppointmentRepo) GetByPatient(_ context.Context, _, _ uuid.UUID, _ int) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockMaterializeAppointmentRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ appointment_model.AppointmentStatus) error {
	return nil
}
func (m *mockMaterializeAppointmentRepo) GetPendingReminders(_ context.Context, _ string, _, _ time.Time) ([]*appointment_model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockMaterializeAppointmentRepo) MarkReminderSent(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockMaterializeAppointmentRepo) GetPendingByPhone(_ context.Context, _ uuid.UUID, _ string) (*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockMaterializeAppointmentRepo) GetExpiredConfirmations(_ context.Context, _ int) ([]*appointment_model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockMaterializeAppointmentRepo) MarkConfirmationExpired(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *mockMaterializeAppointmentRepo) SetConfirmationExpiresAt(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockMaterializeAppointmentRepo) GetSlotPerformanceStats(_ context.Context, _, _ uuid.UUID) ([]*appointment_model.SlotPerformanceStats, error) {
	return nil, nil
}
func (m *mockMaterializeAppointmentRepo) CountByProfessionalID(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockMaterializeAppointmentRepo) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, _ appointment_model.PaymentStatus) error {
	return nil
}
func (m *mockMaterializeAppointmentRepo) GetByDateRangeWithExceptions(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return m.appointments, nil
}

// --- Tests ---

func TestMaterialize_RejectsWhenSessionLimitReached(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	now := time.Now()

	plan := &model.SessionPlan{
		ID:            uuid.New(),
		ClinicID:      clinicID,
		Status:        model.PlanStatusActive,
		TotalSessions: 10,
		SessionsDone:  10, // Already at limit
		ProfessionalID: &profID,
		RRule:         "FREQ=WEEKLY;BYDAY=MO",
		DtStart:       &now,
		DurationMins:  50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	_, err := svc.Materialize(context.Background(), clinicID, plan.ID, now.Format("2006-01-02"))
	require.Error(t, err, "Should reject materialization when session limit reached")
	assert.Contains(t, err.Error(), "maximum sessions")
}

func TestMaterialize_RejectsWhenPlanCancelled(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	now := time.Now()

	plan := &model.SessionPlan{
		ID:            uuid.New(),
		ClinicID:      clinicID,
		Status:        model.PlanStatusCancelled,
		TotalSessions: 10,
		SessionsDone:  3,
		ProfessionalID: &profID,
		RRule:         "FREQ=WEEKLY;BYDAY=MO",
		DtStart:       &now,
		DurationMins:  50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	_, err := svc.Materialize(context.Background(), clinicID, plan.ID, now.Format("2006-01-02"))
	require.Error(t, err, "Should reject materialization for cancelled plan")
	assert.Contains(t, err.Error(), "not active")
}

func TestMaterialize_AllowsWhenUnderLimit(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()

	// Use a Monday so RRULE matches
	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC) // Monday
	dtstart := monday

	plan := &model.SessionPlan{
		ID:            uuid.New(),
		ClinicID:      clinicID,
		Status:        model.PlanStatusActive,
		TotalSessions: 10,
		SessionsDone:  3,
		ProfessionalID: &profID,
		RRule:         "FREQ=WEEKLY;BYDAY=MO",
		DtStart:       &dtstart,
		DurationMins:  50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	apt, err := svc.Materialize(context.Background(), clinicID, plan.ID, monday.Format("2006-01-02"))
	require.NoError(t, err, "Should allow materialization when under limit")
	require.NotNil(t, apt)
}

func TestMaterialize_AllowsIndeterminatePlan(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()

	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	dtstart := monday

	plan := &model.SessionPlan{
		ID:            uuid.New(),
		ClinicID:      clinicID,
		Status:        model.PlanStatusActive,
		TotalSessions: -1, // Indeterminate
		SessionsDone:  100,
		ProfessionalID: &profID,
		RRule:         "FREQ=WEEKLY;BYDAY=MO",
		DtStart:       &dtstart,
		DurationMins:  50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	apt, err := svc.Materialize(context.Background(), clinicID, plan.ID, monday.Format("2006-01-02"))
	require.NoError(t, err, "Indeterminate plan should always allow materialization")
	require.NotNil(t, apt)
}

func TestMaterialize_RejectsGroupPlan(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()

	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	dtstart := monday

	plan := &model.SessionPlan{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		Type:           model.PlanTypeGroup,
		TotalSessions:  -1,
		SessionsDone:   0,
		ProfessionalID: &profID,
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &dtstart,
		DurationMins:   50,
		Title:          "Grupo Pilates",
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	_, err := svc.Materialize(context.Background(), clinicID, plan.ID, monday.Format("2006-01-02"))
	require.Error(t, err, "Should reject materialization for group plans")
	assert.Contains(t, err.Error(), "group")
	assert.Nil(t, aptRepo.created, "No appointment should be created for group plans")
}

func TestMaterialize_UsesDayTimesForAppointmentTime(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()

	// Wednesday 2026-04-08, dtstart at 09:00
	wednesday := time.Date(2026, 4, 8, 9, 0, 0, 0, time.UTC)
	dtstart := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC) // Monday start

	plan := &model.SessionPlan{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		TotalSessions:  10,
		SessionsDone:   0,
		ProfessionalID: &profID,
		RRule:          "FREQ=WEEKLY;BYDAY=MO,WE",
		DtStart:        &dtstart,
		DurationMins:   50,
		DayTimes:       model.DayTimesMap{"MO": "08:00", "WE": "14:00"},
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	apt, err := svc.Materialize(context.Background(), clinicID, plan.ID, wednesday.Format("2006-01-02"))
	require.NoError(t, err)
	require.NotNil(t, apt)
	assert.Equal(t, 14, apt.StartTime.Hour(), "Wednesday session should start at 14:00 as defined in day_times, not 09:00")
	assert.Equal(t, 0, apt.StartTime.Minute())
}

func TestMaterialize_IncludesProcedureFromSessionPlan(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	procedureID := uuid.New()

	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	dtstart := monday

	plan := &model.SessionPlan{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		TotalSessions:  10,
		SessionsDone:   0,
		ProfessionalID: &profID,
		ProcedureID:    &procedureID,
		SessionPrice:   150.0,
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &dtstart,
		DurationMins:   50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	_, err := svc.Materialize(context.Background(), clinicID, plan.ID, monday.Format("2006-01-02"))
	require.NoError(t, err)
	require.NotNil(t, aptRepo.created)

	// The materialized appointment must include the procedure from the session plan
	require.Len(t, aptRepo.created.Procedures, 1, "Materialized appointment must include the session plan procedure")
	assert.Equal(t, procedureID, aptRepo.created.Procedures[0].ProcedureID)
	assert.Equal(t, 150.0, aptRepo.created.Procedures[0].Price)
}

func TestMaterialize_NoProcedureWhenPlanHasNone(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()

	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	dtstart := monday

	plan := &model.SessionPlan{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		TotalSessions:  10,
		SessionsDone:   0,
		ProfessionalID: &profID,
		ProcedureID:    nil, // No procedure
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &dtstart,
		DurationMins:   50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	_, err := svc.Materialize(context.Background(), clinicID, plan.ID, monday.Format("2006-01-02"))
	require.NoError(t, err)
	require.NotNil(t, aptRepo.created)
	assert.Empty(t, aptRepo.created.Procedures, "Appointment without procedure should have empty procedures")
}

func TestRescheduleOccurrence_IncludesProcedureFromSessionPlan(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	procedureID := uuid.New()

	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	wednesday := time.Date(2026, 4, 8, 9, 0, 0, 0, time.UTC)
	dtstart := monday

	plan := &model.SessionPlan{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		TotalSessions:  10,
		SessionsDone:   0,
		ProfessionalID: &profID,
		ProcedureID:    &procedureID,
		SessionPrice:   200.0,
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &dtstart,
		DurationMins:   50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	_, err := svc.RescheduleOccurrence(context.Background(), clinicID, plan.ID, monday.Format("2006-01-02"), wednesday)
	require.NoError(t, err)
	require.NotNil(t, aptRepo.created)

	require.Len(t, aptRepo.created.Procedures, 1, "Rescheduled appointment must include the session plan procedure")
	assert.Equal(t, procedureID, aptRepo.created.Procedures[0].ProcedureID)
	assert.Equal(t, 200.0, aptRepo.created.Procedures[0].Price)
}

// --- Tests: Exception flow (cancel/reschedule single occurrence) ---

func TestCancelOccurrence_SetsIsExceptionAndCanceledStatus(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()

	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	dtstart := monday

	plan := &model.SessionPlan{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		TotalSessions:  10,
		SessionsDone:   3,
		ProfessionalID: &profID,
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &dtstart,
		DurationMins:   50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	apt, err := svc.CancelOccurrence(context.Background(), clinicID, plan.ID, monday.Format("2006-01-02"))
	require.NoError(t, err, "Should cancel a single occurrence")
	require.NotNil(t, apt)
	assert.True(t, apt.IsException, "Cancelled occurrence must be marked as exception")
	assert.NotNil(t, apt.OriginalDate, "Cancelled occurrence must have original date")
	assert.Equal(t, appointment_model.AppointmentStatusCanceled, apt.Status, "Cancelled occurrence must have canceled status")
}

func TestRescheduleOccurrence_SetsIsExceptionAndOriginalDate(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()

	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	wednesday := time.Date(2026, 4, 8, 9, 0, 0, 0, time.UTC)
	dtstart := monday

	plan := &model.SessionPlan{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		TotalSessions:  10,
		SessionsDone:   3,
		ProfessionalID: &profID,
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &dtstart,
		DurationMins:   50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo)

	apt, err := svc.RescheduleOccurrence(context.Background(), clinicID, plan.ID, monday.Format("2006-01-02"), wednesday)
	require.NoError(t, err, "Should reschedule a single occurrence")
	require.NotNil(t, apt)
	assert.True(t, apt.IsException, "Rescheduled occurrence must be marked as exception")
	assert.NotNil(t, apt.OriginalDate, "Rescheduled occurrence must have original date")
	assert.Equal(t, monday.Format("2006-01-02"), apt.OriginalDate.Format("2006-01-02"), "OriginalDate must match the original virtual date")
	assert.Equal(t, wednesday.Format("2006-01-02"), apt.StartTime.Format("2006-01-02"), "StartTime must be the new date")
}
