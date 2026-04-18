package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/attendance/model"
	enrollment_model "github.com/aseno/backend/pkg/enrollment/model"
	"github.com/aseno/backend/pkg/pagination"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
)

// --- Mock Repositories ---

type mockAttendanceRepo struct {
	created  *model.AppointmentAttendance
	existing *model.AppointmentAttendance
	records  []*model.AppointmentAttendance
}

func (m *mockAttendanceRepo) Create(_ context.Context, a *model.AppointmentAttendance) error {
	a.ID = uuid.New()
	m.created = a
	return nil
}

func (m *mockAttendanceRepo) GetByAppointmentAndPatient(_ context.Context, _, _, _ uuid.UUID) (*model.AppointmentAttendance, error) {
	return m.existing, nil
}

func (m *mockAttendanceRepo) GetByAppointmentID(_ context.Context, _, _ uuid.UUID) ([]*model.AppointmentAttendance, error) {
	return m.records, nil
}

func (m *mockAttendanceRepo) Update(_ context.Context, a *model.AppointmentAttendance) error {
	return nil
}

func (m *mockAttendanceRepo) Delete(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

type mockAppointmentRepo struct {
	appointment *appointment_model.Appointment
}

func (m *mockAppointmentRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*appointment_model.Appointment, error) {
	return m.appointment, nil
}

func (m *mockAppointmentRepo) Create(_ context.Context, _ *appointment_model.Appointment) error {
	return nil
}
func (m *mockAppointmentRepo) List(_ context.Context, _ uuid.UUID, _ *appointment_model.AppointmentFilter) ([]*appointment_model.Appointment, int64, error) {
	return nil, 0, nil
}
func (m *mockAppointmentRepo) GetByDateRange(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) GetByDay(_ context.Context, _ uuid.UUID, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) Update(_ context.Context, _ *appointment_model.Appointment) error {
	return nil
}
func (m *mockAppointmentRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockAppointmentRepo) DeleteCascade(_ context.Context, _, _ uuid.UUID, _ bool) error {
	return nil
}
func (m *mockAppointmentRepo) SoftDeleteAsCanceledException(_ context.Context, _, _ uuid.UUID, _ bool) error {
	return nil
}
func (m *mockAppointmentRepo) GetDeletePreview(_ context.Context, _, _ uuid.UUID) (*appointment_model.AppointmentDeletePreview, error) {
	return &appointment_model.AppointmentDeletePreview{}, nil
}
func (m *mockAppointmentRepo) CheckConflict(_ context.Context, _, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockAppointmentRepo) GetByPatient(_ context.Context, _, _ uuid.UUID, _ int) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ appointment_model.AppointmentStatus) error {
	return nil
}
func (m *mockAppointmentRepo) GetPendingReminders(_ context.Context, _ string, _, _ time.Time) ([]*appointment_model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) MarkReminderSent(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockAppointmentRepo) GetPendingByPhone(_ context.Context, _ uuid.UUID, _ string) (*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) GetExpiredConfirmations(_ context.Context, _ int) ([]*appointment_model.AppointmentWithClinic, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) MarkConfirmationExpired(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *mockAppointmentRepo) SetConfirmationExpiresAt(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockAppointmentRepo) GetSlotPerformanceStats(_ context.Context, _, _ uuid.UUID) ([]*appointment_model.SlotPerformanceStats, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) CountByProfessionalID(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockAppointmentRepo) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, _ appointment_model.PaymentStatus) error {
	return nil
}
func (m *mockAppointmentRepo) GetByDateRangeWithExceptions(_ context.Context, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) ([]*appointment_model.Appointment, error) {
	return nil, nil
}
func (m *mockAppointmentRepo) FindPlanAppointmentOnDate(_ context.Context, _, _ uuid.UUID, _ time.Time) (*appointment_model.Appointment, error) {
	return nil, nil
}

type mockPlanRepo struct {
	plan             *sessionplan_model.SessionPlan
	incrementCalled  bool
	updateStatusArgs *updateStatusCall
}

type updateStatusCall struct {
	clinicID uuid.UUID
	planID   uuid.UUID
	status   sessionplan_model.PlanStatus
}

func (m *mockPlanRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*sessionplan_model.SessionPlan, error) {
	return m.plan, nil
}

func (m *mockPlanRepo) IncrementSessionsDone(_ context.Context, _, _ uuid.UUID) error {
	m.incrementCalled = true
	if m.plan != nil {
		m.plan.SessionsDone++
	}
	return nil
}

func (m *mockPlanRepo) UpdateStatus(_ context.Context, clinicID, id uuid.UUID, status sessionplan_model.PlanStatus) error {
	m.updateStatusArgs = &updateStatusCall{clinicID: clinicID, planID: id, status: status}
	return nil
}

// Stub remaining interface methods
func (m *mockPlanRepo) Create(_ context.Context, _ *sessionplan_model.SessionPlan) error {
	return nil
}
func (m *mockPlanRepo) GetByClinic(_ context.Context, _ uuid.UUID, _ pagination.Filter) ([]*sessionplan_model.SessionPlan, int64, error) {
	return nil, 0, nil
}
func (m *mockPlanRepo) GetByPatient(_ context.Context, _, _ uuid.UUID) ([]*sessionplan_model.SessionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) GetByProfessional(_ context.Context, _, _ uuid.UUID) ([]*sessionplan_model.SessionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) GetActivePlansForGeneration(_ context.Context, _ time.Time) ([]*sessionplan_model.SessionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) GetAllActive(_ context.Context, _ uuid.UUID) ([]*sessionplan_model.SessionPlan, error) {
	return nil, nil
}
func (m *mockPlanRepo) Update(_ context.Context, _ *sessionplan_model.SessionPlan) error {
	return nil
}
func (m *mockPlanRepo) UpdateLastGenerated(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockPlanRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockPlanRepo) BulkCancel(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}

type mockEnrollmentRepo struct {
	enrollment       *enrollment_model.SessionPlanEnrollment
	incrementCalled  bool
	updateStatusArgs *enrollmentUpdateStatusCall
}

type enrollmentUpdateStatusCall struct {
	clinicID     uuid.UUID
	enrollmentID uuid.UUID
	status       enrollment_model.EnrollmentStatus
}

func (m *mockEnrollmentRepo) GetByPatientAndPlan(_ context.Context, _, _, _ uuid.UUID) (*enrollment_model.SessionPlanEnrollment, error) {
	return m.enrollment, nil
}

func (m *mockEnrollmentRepo) IncrementSessionsDone(_ context.Context, _, _ uuid.UUID) error {
	m.incrementCalled = true
	if m.enrollment != nil {
		m.enrollment.SessionsDone++
	}
	return nil
}

func (m *mockEnrollmentRepo) UpdateStatus(_ context.Context, clinicID, id uuid.UUID, status enrollment_model.EnrollmentStatus) error {
	m.updateStatusArgs = &enrollmentUpdateStatusCall{clinicID: clinicID, enrollmentID: id, status: status}
	return nil
}

// Stub remaining interface methods
func (m *mockEnrollmentRepo) Create(_ context.Context, _ *enrollment_model.SessionPlanEnrollment) error {
	return nil
}
func (m *mockEnrollmentRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*enrollment_model.SessionPlanEnrollment, error) {
	return m.enrollment, nil
}
func (m *mockEnrollmentRepo) GetByPlan(_ context.Context, _, _ uuid.UUID) ([]*enrollment_model.SessionPlanEnrollment, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) GetActiveByPlan(_ context.Context, _, _ uuid.UUID) ([]*enrollment_model.SessionPlanEnrollment, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) CountActiveByPlan(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockEnrollmentRepo) CountActiveByPlanAndWeekDay(_ context.Context, _, _ uuid.UUID, _ int) (int64, error) {
	return 0, nil
}
func (m *mockEnrollmentRepo) Update(_ context.Context, _ *enrollment_model.SessionPlanEnrollment) error {
	return nil
}

// --- Mock Payment Repo ---

type mockPaymentRepo struct {
	created *appointment_model.AppointmentPayment
}

func (m *mockPaymentRepo) Create(payment *appointment_model.AppointmentPayment) error {
	payment.ID = uuid.New()
	m.created = payment
	return nil
}
func (m *mockPaymentRepo) GetByID(_, _ uuid.UUID) (*appointment_model.AppointmentPayment, error) {
	return nil, nil
}
func (m *mockPaymentRepo) Update(_ *appointment_model.AppointmentPayment) error { return nil }
func (m *mockPaymentRepo) Delete(_, _ uuid.UUID) error                          { return nil }
func (m *mockPaymentRepo) ListByAppointment(_, _ uuid.UUID) ([]*appointment_model.AppointmentPayment, error) {
	return nil, nil
}
func (m *mockPaymentRepo) SumByAppointment(_ uuid.UUID) (float64, error) { return 0, nil }
func (m *mockPaymentRepo) ListBySessionPlan(_ context.Context, _, _ uuid.UUID) ([]*appointment_model.AppointmentPayment, error) {
	return nil, nil
}

// --- Helpers ---

func newTestAppointment(sessionPlanID *uuid.UUID) *appointment_model.Appointment {
	clinicID := uuid.New()
	return &appointment_model.Appointment{
		ID:            uuid.New(),
		ClinicID:      clinicID,
		PatientID:     uuid.New(),
		SessionPlanID: sessionPlanID,
		Status:        appointment_model.AppointmentStatusPending,
	}
}

func newTestPlan(planType sessionplan_model.PlanType, totalSessions, sessionsDone int) *sessionplan_model.SessionPlan {
	return &sessionplan_model.SessionPlan{
		ID:            uuid.New(),
		ClinicID:      uuid.New(),
		Type:          planType,
		Status:        sessionplan_model.PlanStatusActive,
		TotalSessions: totalSessions,
		SessionsDone:  sessionsDone,
	}
}

func intPtr(i int) *int {
	return &i
}

func buildTestService(attRepo *mockAttendanceRepo, aptRepo *mockAppointmentRepo, planRepo *mockPlanRepo, enrollRepo *mockEnrollmentRepo) AttendanceService {
	return NewAttendanceService(attRepo, aptRepo, planRepo, enrollRepo)
}

// --- Tests: Auto-completion on individual plans ---

func TestRecordAttendance_IndividualPlan_AutoCompletes_WhenLastSession(t *testing.T) {
	planID := uuid.New()
	clinicID := uuid.New()

	plan := newTestPlan(sessionplan_model.PlanTypeIndividual, 10, 9)
	plan.ID = planID
	plan.ClinicID = clinicID

	apt := newTestAppointment(&planID)
	apt.ClinicID = clinicID

	attRepo := &mockAttendanceRepo{}
	aptRepo := &mockAppointmentRepo{appointment: apt}
	planRepo := &mockPlanRepo{plan: plan}
	enrollRepo := &mockEnrollmentRepo{}

	svc := buildTestService(attRepo, aptRepo, planRepo, enrollRepo)

	_, err := svc.RecordAttendance(context.Background(), clinicID, apt.ID, &model.RecordAttendanceRequest{
		PatientID: apt.PatientID,
		Status:    model.AttendanceStatusPresent,
		IsGuest:   false,
	})

	require.NoError(t, err)
	assert.True(t, planRepo.incrementCalled, "Should have incremented sessions_done")
	require.NotNil(t, planRepo.updateStatusArgs, "Should have called UpdateStatus to complete the plan")
	assert.Equal(t, sessionplan_model.PlanStatusCompleted, planRepo.updateStatusArgs.status)
	assert.Equal(t, planID, planRepo.updateStatusArgs.planID)
}

func TestRecordAttendance_IndividualPlan_DoesNotComplete_WhenNotLastSession(t *testing.T) {
	planID := uuid.New()
	clinicID := uuid.New()

	plan := newTestPlan(sessionplan_model.PlanTypeIndividual, 10, 5)
	plan.ID = planID
	plan.ClinicID = clinicID

	apt := newTestAppointment(&planID)
	apt.ClinicID = clinicID

	attRepo := &mockAttendanceRepo{}
	aptRepo := &mockAppointmentRepo{appointment: apt}
	planRepo := &mockPlanRepo{plan: plan}
	enrollRepo := &mockEnrollmentRepo{}

	svc := buildTestService(attRepo, aptRepo, planRepo, enrollRepo)

	_, err := svc.RecordAttendance(context.Background(), clinicID, apt.ID, &model.RecordAttendanceRequest{
		PatientID: apt.PatientID,
		Status:    model.AttendanceStatusPresent,
		IsGuest:   false,
	})

	require.NoError(t, err)
	assert.True(t, planRepo.incrementCalled)
	assert.Nil(t, planRepo.updateStatusArgs, "Should NOT have called UpdateStatus — still has remaining sessions")
}

func TestRecordAttendance_IndeterminatePlan_NeverAutoCompletes(t *testing.T) {
	planID := uuid.New()
	clinicID := uuid.New()

	plan := newTestPlan(sessionplan_model.PlanTypeIndividual, -1, 100)
	plan.ID = planID
	plan.ClinicID = clinicID

	apt := newTestAppointment(&planID)
	apt.ClinicID = clinicID

	attRepo := &mockAttendanceRepo{}
	aptRepo := &mockAppointmentRepo{appointment: apt}
	planRepo := &mockPlanRepo{plan: plan}
	enrollRepo := &mockEnrollmentRepo{}

	svc := buildTestService(attRepo, aptRepo, planRepo, enrollRepo)

	_, err := svc.RecordAttendance(context.Background(), clinicID, apt.ID, &model.RecordAttendanceRequest{
		PatientID: apt.PatientID,
		Status:    model.AttendanceStatusPresent,
		IsGuest:   false,
	})

	require.NoError(t, err)
	assert.True(t, planRepo.incrementCalled)
	assert.Nil(t, planRepo.updateStatusArgs, "Indeterminate plan (total_sessions=-1) should NEVER auto-complete")
}

// --- Tests: Auto-completion on group enrollment ---

func TestRecordAttendance_GroupEnrollment_AutoCompletes_WhenLastSession(t *testing.T) {
	planID := uuid.New()
	clinicID := uuid.New()
	enrollmentID := uuid.New()
	patientID := uuid.New()

	plan := newTestPlan(sessionplan_model.PlanTypeGroup, -1, 0)
	plan.ID = planID
	plan.ClinicID = clinicID

	totalSessions := 10
	enrollment := &enrollment_model.SessionPlanEnrollment{
		ID:            enrollmentID,
		ClinicID:      clinicID,
		SessionPlanID: planID,
		PatientID:     patientID,
		Status:        enrollment_model.EnrollmentStatusActive,
		TotalSessions: &totalSessions,
		SessionsDone:  9,
	}

	apt := newTestAppointment(&planID)
	apt.ClinicID = clinicID
	apt.PatientID = patientID

	attRepo := &mockAttendanceRepo{}
	aptRepo := &mockAppointmentRepo{appointment: apt}
	planRepo := &mockPlanRepo{plan: plan}
	enrollRepo := &mockEnrollmentRepo{enrollment: enrollment}

	svc := buildTestService(attRepo, aptRepo, planRepo, enrollRepo)

	_, err := svc.RecordAttendance(context.Background(), clinicID, apt.ID, &model.RecordAttendanceRequest{
		PatientID: patientID,
		Status:    model.AttendanceStatusPresent,
		IsGuest:   false,
	})

	require.NoError(t, err)
	assert.True(t, enrollRepo.incrementCalled, "Should have incremented enrollment sessions_done")
	require.NotNil(t, enrollRepo.updateStatusArgs, "Should have called UpdateStatus to complete the enrollment")
	assert.Equal(t, enrollment_model.EnrollmentStatusCancelled, enrollRepo.updateStatusArgs.status)
	assert.Equal(t, enrollmentID, enrollRepo.updateStatusArgs.enrollmentID)
}

func TestRecordAttendance_GroupEnrollment_IndeterminateNeverAutoCompletes(t *testing.T) {
	planID := uuid.New()
	clinicID := uuid.New()
	enrollmentID := uuid.New()
	patientID := uuid.New()

	plan := newTestPlan(sessionplan_model.PlanTypeGroup, -1, 0)
	plan.ID = planID
	plan.ClinicID = clinicID

	enrollment := &enrollment_model.SessionPlanEnrollment{
		ID:            enrollmentID,
		ClinicID:      clinicID,
		SessionPlanID: planID,
		PatientID:     patientID,
		Status:        enrollment_model.EnrollmentStatusActive,
		TotalSessions: nil, // indeterminate
		SessionsDone:  50,
	}

	apt := newTestAppointment(&planID)
	apt.ClinicID = clinicID
	apt.PatientID = patientID

	attRepo := &mockAttendanceRepo{}
	aptRepo := &mockAppointmentRepo{appointment: apt}
	planRepo := &mockPlanRepo{plan: plan}
	enrollRepo := &mockEnrollmentRepo{enrollment: enrollment}

	svc := buildTestService(attRepo, aptRepo, planRepo, enrollRepo)

	_, err := svc.RecordAttendance(context.Background(), clinicID, apt.ID, &model.RecordAttendanceRequest{
		PatientID: patientID,
		Status:    model.AttendanceStatusPresent,
		IsGuest:   false,
	})

	require.NoError(t, err)
	assert.True(t, enrollRepo.incrementCalled)
	assert.Nil(t, enrollRepo.updateStatusArgs, "Indeterminate enrollment should NEVER auto-complete")
}

func TestRecordAttendance_Guest_WithPayment_CreatesPaymentAndLinksToAttendance(t *testing.T) {
	planID := uuid.New()
	clinicID := uuid.New()
	paymentMethodID := uuid.New()

	plan := newTestPlan(sessionplan_model.PlanTypeGroup, -1, 0)
	plan.ID = planID
	plan.ClinicID = clinicID

	apt := newTestAppointment(&planID)
	apt.ClinicID = clinicID

	attRepo := &mockAttendanceRepo{}
	aptRepo := &mockAppointmentRepo{appointment: apt}
	planRepo := &mockPlanRepo{plan: plan}
	enrollRepo := &mockEnrollmentRepo{}
	paymentRepo := &mockPaymentRepo{}

	svc := buildTestService(attRepo, aptRepo, planRepo, enrollRepo)
	svc.(*attendanceService).SetPaymentRepo(paymentRepo)

	amount := 150.0
	result, err := svc.RecordAttendance(context.Background(), clinicID, apt.ID, &model.RecordAttendanceRequest{
		PatientID:       uuid.New(),
		Status:          model.AttendanceStatusPresent,
		IsGuest:         true,
		PaymentMethodID: &paymentMethodID,
		PaymentAmount:   &amount,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsGuest, "Should be marked as guest")
	assert.NotNil(t, result.PaymentID, "Guest with payment info should have PaymentID set")
	require.NotNil(t, paymentRepo.created, "Payment should have been created")
	assert.Equal(t, amount, paymentRepo.created.Amount, "Payment amount should match")
	assert.Equal(t, paymentMethodID, paymentRepo.created.PaymentMethodID, "Payment method should match")
}

func TestRecordAttendance_Guest_WithoutPayment_NoPaymentID(t *testing.T) {
	planID := uuid.New()
	clinicID := uuid.New()

	plan := newTestPlan(sessionplan_model.PlanTypeGroup, -1, 0)
	plan.ID = planID
	plan.ClinicID = clinicID

	apt := newTestAppointment(&planID)
	apt.ClinicID = clinicID

	attRepo := &mockAttendanceRepo{}
	aptRepo := &mockAppointmentRepo{appointment: apt}
	planRepo := &mockPlanRepo{plan: plan}
	enrollRepo := &mockEnrollmentRepo{}

	svc := buildTestService(attRepo, aptRepo, planRepo, enrollRepo)

	result, err := svc.RecordAttendance(context.Background(), clinicID, apt.ID, &model.RecordAttendanceRequest{
		PatientID: uuid.New(),
		Status:    model.AttendanceStatusPresent,
		IsGuest:   true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsGuest)
	assert.Nil(t, result.PaymentID, "Guest without payment info should have nil PaymentID")
}

func TestRecordAttendance_Guest_DoesNotIncrementOrComplete(t *testing.T) {
	planID := uuid.New()
	clinicID := uuid.New()

	plan := newTestPlan(sessionplan_model.PlanTypeGroup, -1, 0)
	plan.ID = planID
	plan.ClinicID = clinicID

	apt := newTestAppointment(&planID)
	apt.ClinicID = clinicID

	attRepo := &mockAttendanceRepo{}
	aptRepo := &mockAppointmentRepo{appointment: apt}
	planRepo := &mockPlanRepo{plan: plan}
	enrollRepo := &mockEnrollmentRepo{}

	svc := buildTestService(attRepo, aptRepo, planRepo, enrollRepo)

	_, err := svc.RecordAttendance(context.Background(), clinicID, apt.ID, &model.RecordAttendanceRequest{
		PatientID: uuid.New(),
		Status:    model.AttendanceStatusPresent,
		IsGuest:   true,
	})

	require.NoError(t, err)
	assert.False(t, planRepo.incrementCalled, "Guest should NOT increment plan sessions")
	assert.False(t, enrollRepo.incrementCalled, "Guest should NOT increment enrollment sessions")
}
