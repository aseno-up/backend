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
func (m *mockMaterializeAppointmentRepo) DeleteCascade(_ context.Context, _, _ uuid.UUID, _ bool) error {
	return nil
}
func (m *mockMaterializeAppointmentRepo) SoftDeleteAsCanceledException(_ context.Context, _, _ uuid.UUID, _ bool) error {
	return nil
}
func (m *mockMaterializeAppointmentRepo) GetDeletePreview(_ context.Context, _, _ uuid.UUID) (*appointment_model.AppointmentDeletePreview, error) {
	return &appointment_model.AppointmentDeletePreview{}, nil
}
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
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		TotalSessions:  10,
		SessionsDone:   10, // Already at limit
		ProfessionalID: &profID,
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &now,
		DurationMins:   50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo, nil)

	_, err := svc.Materialize(context.Background(), clinicID, plan.ID, now.Format("2006-01-02"))
	require.Error(t, err, "Should reject materialization when session limit reached")
	assert.Contains(t, err.Error(), "maximum sessions")
}

func TestMaterialize_RejectsWhenPlanCancelled(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	now := time.Now()

	plan := &model.SessionPlan{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusCancelled,
		TotalSessions:  10,
		SessionsDone:   3,
		ProfessionalID: &profID,
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &now,
		DurationMins:   50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo, nil)

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

	svc := NewMaterializeService(planRepo, aptRepo, nil)

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
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		TotalSessions:  -1, // Indeterminate
		SessionsDone:   100,
		ProfessionalID: &profID,
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &dtstart,
		DurationMins:   50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{}

	svc := NewMaterializeService(planRepo, aptRepo, nil)

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

	svc := NewMaterializeService(planRepo, aptRepo, nil)

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

	svc := NewMaterializeService(planRepo, aptRepo, nil)

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

	svc := NewMaterializeService(planRepo, aptRepo, nil)

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

	svc := NewMaterializeService(planRepo, aptRepo, nil)

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

	svc := NewMaterializeService(planRepo, aptRepo, nil)

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

	svc := NewMaterializeService(planRepo, aptRepo, nil)

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

	svc := NewMaterializeService(planRepo, aptRepo, nil)

	apt, err := svc.RescheduleOccurrence(context.Background(), clinicID, plan.ID, monday.Format("2006-01-02"), wednesday)
	require.NoError(t, err, "Should reschedule a single occurrence")
	require.NotNil(t, apt)
	assert.True(t, apt.IsException, "Rescheduled occurrence must be marked as exception")
	assert.NotNil(t, apt.OriginalDate, "Rescheduled occurrence must have original date")
	assert.Equal(t, monday.Format("2006-01-02"), apt.OriginalDate.Format("2006-01-02"), "OriginalDate must match the original virtual date")
	assert.Equal(t, wednesday.Format("2006-01-02"), apt.StartTime.Format("2006-01-02"), "StartTime must be the new date")
}

// Regression: when an appointment was already rescheduled once, its current
// start_time may fall on a day that is NOT a valid RRULE occurrence (e.g., the
// plan repeats on Thursdays but the appointment was moved to a Friday).
// Dragging it again previously returned 422 "No session scheduled for the
// original date" because the RRULE check ran before we looked up the existing
// appointment. The fix reorders the logic: an existing materialized
// appointment for the plan is updated in place without requiring the date to
// match the RRULE.
func TestRescheduleOccurrence_UpdatesAlreadyRescheduledAppointment(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	planID := uuid.New()

	thursday := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	// Plan repeats on Thursdays.
	dtstart := thursday

	// Simulate an appointment that was already rescheduled from Thursday Apr 23
	// to Friday Apr 24. It now lives on Apr 24, a day the RRULE does not cover.
	friday := time.Date(2026, 4, 24, 13, 0, 0, 0, time.UTC)
	existing := &appointment_model.Appointment{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		ProfessionalID: profID,
		SessionPlanID:  &planID,
		StartTime:      friday,
		EndTime:        friday.Add(50 * time.Minute),
		IsException:    true,
		OriginalDate:   &thursday,
		Status:         appointment_model.AppointmentStatusPending,
	}

	plan := &model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		TotalSessions:  -1,
		SessionsDone:   0,
		ProfessionalID: &profID,
		RRule:          "FREQ=WEEKLY;BYDAY=TH",
		DtStart:        &dtstart,
		DurationMins:   50,
	}

	planRepo := &mockMaterializePlanRepo{plan: plan}
	aptRepo := &mockMaterializeAppointmentRepo{appointments: []*appointment_model.Appointment{existing}}

	svc := NewMaterializeService(planRepo, aptRepo, nil)

	// Reschedule within the same Friday — this is the scenario that was failing.
	newTime := time.Date(2026, 4, 24, 16, 0, 0, 0, time.UTC)
	apt, err := svc.RescheduleOccurrence(context.Background(), clinicID, plan.ID, "2026-04-24", newTime)
	require.NoError(t, err, "Rescheduling an already-rescheduled appointment must not require the current date to match the RRULE")
	require.NotNil(t, apt)
	assert.Equal(t, newTime, apt.StartTime, "StartTime must be updated to the new time")
	assert.True(t, apt.IsException, "Appointment must remain an exception")
	assert.NotNil(t, apt.OriginalDate, "OriginalDate must be preserved across reschedules")
	assert.Equal(t, thursday, *apt.OriginalDate, "OriginalDate must keep the first-reschedule source date, not be overwritten")
}
