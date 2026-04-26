package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	enrollment_model "github.com/aseno/backend/pkg/enrollment/model"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/sessionplan/model"
)

// --- Mock Repos for CalendarFacade ---

type mockCalendarPlanRepo struct {
	plans []*model.SessionPlan
}

func (m *mockCalendarPlanRepo) GetByProfessional(_ context.Context, _, _ uuid.UUID) ([]*model.SessionPlan, error) {
	return m.plans, nil
}

func (m *mockCalendarPlanRepo) GetAllActive(_ context.Context, _ uuid.UUID) ([]*model.SessionPlan, error) {
	return m.plans, nil
}

// Stubs
func (m *mockCalendarPlanRepo) Create(_ context.Context, _ *model.SessionPlan) error { return nil }
func (m *mockCalendarPlanRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockCalendarPlanRepo) GetByClinic(_ context.Context, _ uuid.UUID, _ pagination.Filter) ([]*model.SessionPlan, int64, error) {
	return nil, 0, nil
}
func (m *mockCalendarPlanRepo) GetByPatient(_ context.Context, _, _ uuid.UUID) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockCalendarPlanRepo) GetActivePlansForGeneration(_ context.Context, _ time.Time) ([]*model.SessionPlan, error) {
	return nil, nil
}
func (m *mockCalendarPlanRepo) Update(_ context.Context, _ *model.SessionPlan) error { return nil }
func (m *mockCalendarPlanRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ model.PlanStatus) error {
	return nil
}
func (m *mockCalendarPlanRepo) IncrementSessionsDone(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *mockCalendarPlanRepo) UpdateLastGenerated(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockCalendarPlanRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockCalendarPlanRepo) BulkCancel(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}

type mockCalendarAppointmentRepo struct {
	appointments []*appointment_model.Appointment
}

func (m *mockCalendarAppointmentRepo) GetByDateRange(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return m.appointments, nil
}

// Stubs
func (m *mockCalendarAppointmentRepo) Create(_ context.Context, _ *appointment_model.Appointment) error {
	return nil
}
func (m *mockCalendarAppointmentRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockCalendarAppointmentRepo) List(_ context.Context, _ uuid.UUID, _ *appointment_model.AppointmentFilter) ([]*appointment_model.Appointment, int64, error) {
	return nil, 0, nil
}
func (m *mockCalendarAppointmentRepo) GetByDay(_ context.Context, _ uuid.UUID, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockCalendarAppointmentRepo) Update(_ context.Context, _ *appointment_model.Appointment) error {
	return nil
}
func (m *mockCalendarAppointmentRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockCalendarAppointmentRepo) DeleteCascade(_ context.Context, _, _ uuid.UUID, _ bool) error {
	return nil
}
func (m *mockCalendarAppointmentRepo) SoftDeleteAsCanceledException(_ context.Context, _, _ uuid.UUID, _ bool) error {
	return nil
}
func (m *mockCalendarAppointmentRepo) GetDeletePreview(_ context.Context, _, _ uuid.UUID) (*appointment_model.AppointmentDeletePreview, error) {
	return &appointment_model.AppointmentDeletePreview{}, nil
}
func (m *mockCalendarAppointmentRepo) CheckConflict(_ context.Context, _, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockCalendarAppointmentRepo) GetByPatient(_ context.Context, _, _ uuid.UUID, _ int) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockCalendarAppointmentRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ appointment_model.AppointmentStatus) error {
	return nil
}
func (m *mockCalendarAppointmentRepo) GetPendingReminders(_ context.Context, _ string, _, _ time.Time) ([]*appointment_model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockCalendarAppointmentRepo) MarkReminderSent(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockCalendarAppointmentRepo) GetPendingByPhone(_ context.Context, _ uuid.UUID, _ string) (*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockCalendarAppointmentRepo) GetExpiredConfirmations(_ context.Context, _ int) ([]*appointment_model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockCalendarAppointmentRepo) MarkConfirmationExpired(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *mockCalendarAppointmentRepo) SetConfirmationExpiresAt(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockCalendarAppointmentRepo) GetSlotPerformanceStats(_ context.Context, _, _ uuid.UUID) ([]*appointment_model.SlotPerformanceStats, error) {
	return nil, nil
}
func (m *mockCalendarAppointmentRepo) CountByProfessionalID(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockCalendarAppointmentRepo) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, _ appointment_model.PaymentStatus) error {
	return nil
}
func (m *mockCalendarAppointmentRepo) GetByDateRangeWithExceptions(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return m.appointments, nil
}
func (m *mockCalendarAppointmentRepo) FindPlanAppointmentOnDate(_ context.Context, _, _ uuid.UUID, _ time.Time) (*appointment_model.Appointment, error) {
	return nil, nil
}

type mockCalendarEnrollmentRepo struct {
	active []*enrollment_model.SessionPlanEnrollment
}

func (m *mockCalendarEnrollmentRepo) CountActiveByPlan(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return int64(len(m.active)), nil
}

func (m *mockCalendarEnrollmentRepo) CountActiveByPlanAndWeekDay(_ context.Context, _, _ uuid.UUID, weekDay int) (int64, error) {
	var count int64
	for _, e := range m.active {
		if e.HasWeekDay(weekDay) {
			count++
		}
	}
	return count, nil
}

// Stubs
func (m *mockCalendarEnrollmentRepo) Create(_ context.Context, _ *enrollment_model.SessionPlanEnrollment) error {
	return nil
}
func (m *mockCalendarEnrollmentRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*enrollment_model.SessionPlanEnrollment, error) {
	return nil, nil
}
func (m *mockCalendarEnrollmentRepo) GetByPlan(_ context.Context, _, _ uuid.UUID) ([]*enrollment_model.SessionPlanEnrollment, error) {
	return nil, nil
}
func (m *mockCalendarEnrollmentRepo) GetActiveByPlan(_ context.Context, _, _ uuid.UUID) ([]*enrollment_model.SessionPlanEnrollment, error) {
	return m.active, nil
}
func (m *mockCalendarEnrollmentRepo) GetByPatientAndPlan(_ context.Context, _, _, _ uuid.UUID) (*enrollment_model.SessionPlanEnrollment, error) {
	return nil, nil
}
func (m *mockCalendarEnrollmentRepo) IncrementSessionsDone(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *mockCalendarEnrollmentRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ enrollment_model.EnrollmentStatus) error {
	return nil
}
func (m *mockCalendarEnrollmentRepo) Update(_ context.Context, _ *enrollment_model.SessionPlanEnrollment) error {
	return nil
}
func (m *mockCalendarEnrollmentRepo) CountActiveBySlot(_ context.Context, _, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockCalendarEnrollmentRepo) SetEnrollmentSlots(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}
func (m *mockCalendarEnrollmentRepo) GetSlotIDsByEnrollment(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

// --- Tests ---

func TestCalendarFacade_ExcludesVirtualForCancelledExceptionAppointment(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	planID := uuid.New()

	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	dtstart := monday

	plan := &model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		Type:           model.PlanTypeIndividual,
		TotalSessions:  10,
		SessionsDone:   3,
		ProfessionalID: &profID,
		PatientID:      uuid.New(),
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &dtstart,
		DurationMins:   50,
		Title:          "Fisio",
	}

	// A cancelled exception appointment exists for this Monday
	cancelledApt := &appointment_model.Appointment{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		SessionPlanID:  &planID,
		ProfessionalID: profID,
		StartTime:      monday,
		EndTime:        monday.Add(50 * time.Minute),
		Status:         appointment_model.AppointmentStatusCanceled,
		IsException:    true,
		OriginalDate:   &monday,
	}

	planRepo := &mockCalendarPlanRepo{plans: []*model.SessionPlan{plan}}
	aptRepo := &mockCalendarAppointmentRepo{appointments: []*appointment_model.Appointment{cancelledApt}}
	enrollRepo := &mockCalendarEnrollmentRepo{}

	facade := NewCalendarFacade(planRepo, aptRepo, enrollRepo)

	rangeStart := monday
	rangeEnd := monday.AddDate(0, 0, 7)

	events, err := facade.GetCalendarEvents(context.Background(), clinicID, &profID, rangeStart, rangeEnd)
	require.NoError(t, err)

	// The cancelled exception should appear as a real event (cancelled status)
	// But NO virtual event should appear for that same Monday
	virtualCountForMonday := 0
	realCountForMonday := 0
	for _, e := range events {
		if e.StartTime.Format("2006-01-02") == monday.Format("2006-01-02") {
			if e.IsVirtual {
				virtualCountForMonday++
			} else {
				realCountForMonday++
			}
		}
	}

	assert.Equal(t, 0, virtualCountForMonday, "Should NOT render virtual event for a cancelled exception date")
	assert.Equal(t, 1, realCountForMonday, "Should show the cancelled exception as a real event")
}

// Regression: a deleted session-plan appointment is stored as a cancelled
// exception with deleted_at set (a "tombstone"). Without the hide-and-suppress
// behavior, the tombstone either rendered on the calendar (looking like a
// ghost cancelled event) or — if it was hard-deleted instead — the virtual
// occurrence came back.
func TestCalendarFacade_HidesDeletedTombstoneButStillSuppressesVirtual(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	planID := uuid.New()

	monday := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	dtstart := monday

	plan := &model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		Type:           model.PlanTypeIndividual,
		TotalSessions:  10,
		SessionsDone:   3,
		ProfessionalID: &profID,
		PatientID:      uuid.New(),
		RRule:          "FREQ=WEEKLY;BYDAY=MO",
		DtStart:        &dtstart,
		DurationMins:   50,
		Title:          "Fisio",
	}

	// A deleted session-plan appointment: cancelled exception + deleted_at set.
	deletedAt := monday.Add(time.Hour)
	tombstone := &appointment_model.Appointment{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		SessionPlanID:  &planID,
		ProfessionalID: profID,
		StartTime:      monday,
		EndTime:        monday.Add(50 * time.Minute),
		Status:         appointment_model.AppointmentStatusCanceled,
		IsException:    true,
		OriginalDate:   &monday,
		DeletedAt:      &deletedAt,
	}

	planRepo := &mockCalendarPlanRepo{plans: []*model.SessionPlan{plan}}
	aptRepo := &mockCalendarAppointmentRepo{appointments: []*appointment_model.Appointment{tombstone}}
	enrollRepo := &mockCalendarEnrollmentRepo{}

	facade := NewCalendarFacade(planRepo, aptRepo, enrollRepo)

	events, err := facade.GetCalendarEvents(context.Background(), clinicID, &profID, monday, monday.AddDate(0, 0, 7))
	require.NoError(t, err)

	virtualCountForMonday := 0
	realCountForMonday := 0
	for _, e := range events {
		if e.StartTime.Format("2006-01-02") == monday.Format("2006-01-02") {
			if e.IsVirtual {
				virtualCountForMonday++
			} else {
				realCountForMonday++
			}
		}
	}

	assert.Equal(t, 0, virtualCountForMonday, "Virtual occurrence must be suppressed by the tombstone — otherwise deleted session-plan appointments 'come back'")
	assert.Equal(t, 0, realCountForMonday, "Tombstone must NOT render as a real event — the user deleted it and expects it gone")
}

func TestCalendarFacade_DayTimesProducesDifferentTimesPerDay(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	planID := uuid.New()

	// Monday Jan 5 2026 at 08:00
	monday := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)

	dayTimes := model.DayTimesMap{
		"MO": "08:00",
		"WE": "14:00",
	}

	plan := &model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		Type:           model.PlanTypeIndividual,
		TotalSessions:  -1,
		SessionsDone:   0,
		ProfessionalID: &profID,
		PatientID:      uuid.New(),
		RRule:          "FREQ=WEEKLY;BYDAY=MO,WE",
		DtStart:        &monday,
		DayTimes:       dayTimes,
		DurationMins:   50,
		Title:          "Pilates",
	}

	planRepo := &mockCalendarPlanRepo{plans: []*model.SessionPlan{plan}}
	aptRepo := &mockCalendarAppointmentRepo{appointments: nil}
	enrollRepo := &mockCalendarEnrollmentRepo{}

	facade := NewCalendarFacade(planRepo, aptRepo, enrollRepo)

	rangeStart := monday
	rangeEnd := monday.AddDate(0, 0, 6) // 6 days to avoid next Monday

	events, err := facade.GetCalendarEvents(context.Background(), clinicID, &profID, rangeStart, rangeEnd)
	require.NoError(t, err)

	// Should have 2 virtual events: MO at 08:00 and WE at 14:00
	virtualEvents := make([]*model.CalendarEvent, 0)
	for _, e := range events {
		if e.IsVirtual {
			virtualEvents = append(virtualEvents, e)
		}
	}
	require.Len(t, virtualEvents, 2)

	// Find MO and WE events
	var moEvent, weEvent *model.CalendarEvent
	for _, e := range virtualEvents {
		switch e.StartTime.Weekday() {
		case time.Monday:
			moEvent = e
		case time.Wednesday:
			weEvent = e
		}
	}

	require.NotNil(t, moEvent, "should have Monday event")
	require.NotNil(t, weEvent, "should have Wednesday event")

	assert.Equal(t, 8, moEvent.StartTime.Hour(), "Monday should be at 08:00")
	assert.Equal(t, 14, weEvent.StartTime.Hour(), "Wednesday should be at 14:00")
}

func TestCalendarFacade_GroupPlanEnrolledCountIsPerDay(t *testing.T) {
	clinicID := uuid.New()
	profID := uuid.New()
	planID := uuid.New()

	// Monday 2026-04-06 — plan runs Mon/Wed/Fri
	monday := time.Date(2026, 4, 6, 18, 30, 0, 0, time.UTC)

	plan := &model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Status:         model.PlanStatusActive,
		Type:           model.PlanTypeGroup,
		TotalSessions:  -1,
		ProfessionalID: &profID,
		RRule:          "FREQ=WEEKLY;BYDAY=MO,WE,FR",
		DtStart:        &monday,
		DurationMins:   50,
		Title:          "Pilates Grupo",
	}

	// 3 enrollments:
	//  A: Mon only          → counts on Mon
	//  B: Mon + Wed         → counts on Mon and Wed
	//  C: Fri only          → counts on Fri
	// Expected per-day: Mon=2, Wed=1, Fri=1
	enrollments := []*enrollment_model.SessionPlanEnrollment{
		{ID: uuid.New(), ClinicID: clinicID, SessionPlanID: planID, Status: enrollment_model.EnrollmentStatusActive, WeekDays: []int64{1}},
		{ID: uuid.New(), ClinicID: clinicID, SessionPlanID: planID, Status: enrollment_model.EnrollmentStatusActive, WeekDays: []int64{1, 3}},
		{ID: uuid.New(), ClinicID: clinicID, SessionPlanID: planID, Status: enrollment_model.EnrollmentStatusActive, WeekDays: []int64{5}},
	}

	planRepo := &mockCalendarPlanRepo{plans: []*model.SessionPlan{plan}}
	aptRepo := &mockCalendarAppointmentRepo{appointments: nil}
	enrollRepo := &mockCalendarEnrollmentRepo{active: enrollments}

	facade := NewCalendarFacade(planRepo, aptRepo, enrollRepo)

	events, err := facade.GetCalendarEvents(context.Background(), clinicID, &profID, monday, monday.AddDate(0, 0, 6))
	require.NoError(t, err)

	countsByDow := map[time.Weekday]int{}
	for _, e := range events {
		if e.IsVirtual {
			countsByDow[e.StartTime.Weekday()] = e.EnrolledCount
		}
	}

	assert.Equal(t, 2, countsByDow[time.Monday], "Monday should count A+B")
	assert.Equal(t, 1, countsByDow[time.Wednesday], "Wednesday should count B only")
	assert.Equal(t, 1, countsByDow[time.Friday], "Friday should count C only")
}
