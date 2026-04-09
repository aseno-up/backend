package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/pkg/attendance/model"
	enrollment_model "github.com/aseno/backend/pkg/enrollment/model"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
)

// --- Mock Group Attendance Repo ---

type mockGroupAttendanceRepo struct {
	records  []*model.GroupSessionAttendance
	created  []*model.GroupSessionAttendance
	existing map[string]*model.GroupSessionAttendance // key: patientID
}

func newMockGroupAttendanceRepo() *mockGroupAttendanceRepo {
	return &mockGroupAttendanceRepo{
		existing: make(map[string]*model.GroupSessionAttendance),
	}
}

func (m *mockGroupAttendanceRepo) Create(_ context.Context, a *model.GroupSessionAttendance) error {
	a.ID = uuid.New()
	m.created = append(m.created, a)
	return nil
}

func (m *mockGroupAttendanceRepo) GetByPlanAndDate(_ context.Context, _, _ uuid.UUID, _ string) ([]*model.GroupSessionAttendance, error) {
	return m.records, nil
}

func (m *mockGroupAttendanceRepo) GetByPlanDateAndPatient(_ context.Context, _, _, _ uuid.UUID, _ string) (*model.GroupSessionAttendance, error) {
	// Return based on patientID key
	return nil, nil
}

func (m *mockGroupAttendanceRepo) Update(_ context.Context, a *model.GroupSessionAttendance) error {
	return nil
}

// --- Mock Plan Repo for group attendance ---

type mockGroupPlanRepo struct {
	plan *sessionplan_model.SessionPlan
}

func (m *mockGroupPlanRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*sessionplan_model.SessionPlan, error) {
	return m.plan, nil
}

// --- Mock Enrollment Repo for group attendance ---

type mockGroupEnrollmentRepo struct {
	enrollments      map[string]*enrollment_model.SessionPlanEnrollment // key: patientID
	incrementCalled  map[string]bool                                   // key: enrollmentID
	updateStatusArgs map[string]enrollment_model.EnrollmentStatus      // key: enrollmentID
}

func newMockGroupEnrollmentRepo() *mockGroupEnrollmentRepo {
	return &mockGroupEnrollmentRepo{
		enrollments:      make(map[string]*enrollment_model.SessionPlanEnrollment),
		incrementCalled:  make(map[string]bool),
		updateStatusArgs: make(map[string]enrollment_model.EnrollmentStatus),
	}
}

func (m *mockGroupEnrollmentRepo) Create(_ context.Context, _ *enrollment_model.SessionPlanEnrollment) error {
	return nil
}
func (m *mockGroupEnrollmentRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*enrollment_model.SessionPlanEnrollment, error) {
	return nil, nil
}
func (m *mockGroupEnrollmentRepo) GetByPlan(_ context.Context, _, _ uuid.UUID) ([]*enrollment_model.SessionPlanEnrollment, error) {
	return nil, nil
}
func (m *mockGroupEnrollmentRepo) GetActiveByPlan(_ context.Context, _, _ uuid.UUID) ([]*enrollment_model.SessionPlanEnrollment, error) {
	return nil, nil
}
func (m *mockGroupEnrollmentRepo) CountActiveByPlan(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (m *mockGroupEnrollmentRepo) Update(_ context.Context, _ *enrollment_model.SessionPlanEnrollment) error {
	return nil
}

func (m *mockGroupEnrollmentRepo) GetByPatientAndPlan(_ context.Context, _, patientID, _ uuid.UUID) (*enrollment_model.SessionPlanEnrollment, error) {
	if e, ok := m.enrollments[patientID.String()]; ok {
		return e, nil
	}
	return nil, nil
}

func (m *mockGroupEnrollmentRepo) IncrementSessionsDone(_ context.Context, _, enrollmentID uuid.UUID) error {
	m.incrementCalled[enrollmentID.String()] = true
	for _, e := range m.enrollments {
		if e.ID == enrollmentID {
			e.SessionsDone++
		}
	}
	return nil
}

func (m *mockGroupEnrollmentRepo) UpdateStatus(_ context.Context, _, enrollmentID uuid.UUID, status enrollment_model.EnrollmentStatus) error {
	m.updateStatusArgs[enrollmentID.String()] = status
	return nil
}

// --- Helper to build group attendance service ---

func buildGroupAttendanceService(
	gaRepo *mockGroupAttendanceRepo,
	planRepo *mockGroupPlanRepo,
	enrollRepo *mockGroupEnrollmentRepo,
) GroupAttendanceService {
	return NewGroupAttendanceService(gaRepo, planRepo, enrollRepo)
}

// --- Tests ---

func TestRecordGroupAttendance_PresentEnrolled_IncrementsSessionsDone(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	patientID := uuid.New()
	enrollmentID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:       planID,
		ClinicID: clinicID,
		Type:     sessionplan_model.PlanTypeGroup,
		Status:   sessionplan_model.PlanStatusActive,
	}

	totalSessions := 10
	enrollment := &enrollment_model.SessionPlanEnrollment{
		ID:            enrollmentID,
		ClinicID:      clinicID,
		SessionPlanID: planID,
		PatientID:     patientID,
		Status:        enrollment_model.EnrollmentStatusActive,
		TotalSessions: &totalSessions,
		SessionsDone:  3,
	}

	gaRepo := newMockGroupAttendanceRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	enrollRepo := newMockGroupEnrollmentRepo()
	enrollRepo.enrollments[patientID.String()] = enrollment

	svc := buildGroupAttendanceService(gaRepo, planRepo, enrollRepo)

	results, err := svc.RecordGroupAttendance(context.Background(), clinicID, planID, &model.RecordGroupAttendanceRequest{
		Date: "2026-04-09",
		Entries: []model.GroupAttendanceEntry{
			{PatientID: patientID, Status: model.AttendanceStatusPresent, IsGuest: false},
		},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, model.AttendanceStatusPresent, results[0].Status)
	assert.True(t, enrollRepo.incrementCalled[enrollmentID.String()], "Should increment enrollment sessions_done for present non-guest")
}

func TestRecordGroupAttendance_AbsentEnrolled_DoesNotIncrement(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	patientID := uuid.New()
	enrollmentID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:       planID,
		ClinicID: clinicID,
		Type:     sessionplan_model.PlanTypeGroup,
		Status:   sessionplan_model.PlanStatusActive,
	}

	totalSessions := 10
	enrollment := &enrollment_model.SessionPlanEnrollment{
		ID:            enrollmentID,
		ClinicID:      clinicID,
		SessionPlanID: planID,
		PatientID:     patientID,
		Status:        enrollment_model.EnrollmentStatusActive,
		TotalSessions: &totalSessions,
		SessionsDone:  3,
	}

	gaRepo := newMockGroupAttendanceRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	enrollRepo := newMockGroupEnrollmentRepo()
	enrollRepo.enrollments[patientID.String()] = enrollment

	svc := buildGroupAttendanceService(gaRepo, planRepo, enrollRepo)

	results, err := svc.RecordGroupAttendance(context.Background(), clinicID, planID, &model.RecordGroupAttendanceRequest{
		Date: "2026-04-09",
		Entries: []model.GroupAttendanceEntry{
			{PatientID: patientID, Status: model.AttendanceStatusAbsent, IsGuest: false},
		},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, model.AttendanceStatusAbsent, results[0].Status)
	assert.False(t, enrollRepo.incrementCalled[enrollmentID.String()], "Should NOT increment for absent")
}

func TestRecordGroupAttendance_GuestPresent_DoesNotIncrementEnrollment(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	guestID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:       planID,
		ClinicID: clinicID,
		Type:     sessionplan_model.PlanTypeGroup,
		Status:   sessionplan_model.PlanStatusActive,
	}

	gaRepo := newMockGroupAttendanceRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	enrollRepo := newMockGroupEnrollmentRepo()

	svc := buildGroupAttendanceService(gaRepo, planRepo, enrollRepo)

	results, err := svc.RecordGroupAttendance(context.Background(), clinicID, planID, &model.RecordGroupAttendanceRequest{
		Date: "2026-04-09",
		Entries: []model.GroupAttendanceEntry{
			{PatientID: guestID, Status: model.AttendanceStatusPresent, IsGuest: true},
		},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].IsGuest)
	assert.Empty(t, enrollRepo.incrementCalled, "Guest should NOT increment any enrollment")
}

func TestRecordGroupAttendance_LastSession_AutoCompletesEnrollment(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	patientID := uuid.New()
	enrollmentID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:       planID,
		ClinicID: clinicID,
		Type:     sessionplan_model.PlanTypeGroup,
		Status:   sessionplan_model.PlanStatusActive,
	}

	totalSessions := 10
	enrollment := &enrollment_model.SessionPlanEnrollment{
		ID:            enrollmentID,
		ClinicID:      clinicID,
		SessionPlanID: planID,
		PatientID:     patientID,
		Status:        enrollment_model.EnrollmentStatusActive,
		TotalSessions: &totalSessions,
		SessionsDone:  9, // last session
	}

	gaRepo := newMockGroupAttendanceRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	enrollRepo := newMockGroupEnrollmentRepo()
	enrollRepo.enrollments[patientID.String()] = enrollment

	svc := buildGroupAttendanceService(gaRepo, planRepo, enrollRepo)

	_, err := svc.RecordGroupAttendance(context.Background(), clinicID, planID, &model.RecordGroupAttendanceRequest{
		Date: "2026-04-09",
		Entries: []model.GroupAttendanceEntry{
			{PatientID: patientID, Status: model.AttendanceStatusPresent, IsGuest: false},
		},
	})

	require.NoError(t, err)
	assert.True(t, enrollRepo.incrementCalled[enrollmentID.String()])
	assert.Equal(t, enrollment_model.EnrollmentStatusCancelled, enrollRepo.updateStatusArgs[enrollmentID.String()],
		"Should auto-complete enrollment when last session reached")
}

func TestRecordGroupAttendance_IndeterminateEnrollment_NeverAutoCompletes(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	patientID := uuid.New()
	enrollmentID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:       planID,
		ClinicID: clinicID,
		Type:     sessionplan_model.PlanTypeGroup,
		Status:   sessionplan_model.PlanStatusActive,
	}

	enrollment := &enrollment_model.SessionPlanEnrollment{
		ID:            enrollmentID,
		ClinicID:      clinicID,
		SessionPlanID: planID,
		PatientID:     patientID,
		Status:        enrollment_model.EnrollmentStatusActive,
		TotalSessions: nil, // indeterminate
		SessionsDone:  50,
	}

	gaRepo := newMockGroupAttendanceRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	enrollRepo := newMockGroupEnrollmentRepo()
	enrollRepo.enrollments[patientID.String()] = enrollment

	svc := buildGroupAttendanceService(gaRepo, planRepo, enrollRepo)

	_, err := svc.RecordGroupAttendance(context.Background(), clinicID, planID, &model.RecordGroupAttendanceRequest{
		Date: "2026-04-09",
		Entries: []model.GroupAttendanceEntry{
			{PatientID: patientID, Status: model.AttendanceStatusPresent, IsGuest: false},
		},
	})

	require.NoError(t, err)
	assert.True(t, enrollRepo.incrementCalled[enrollmentID.String()])
	_, hasAutoComplete := enrollRepo.updateStatusArgs[enrollmentID.String()]
	assert.False(t, hasAutoComplete, "Indeterminate enrollment should NEVER auto-complete")
}

func TestRecordGroupAttendance_RejectsIndividualPlan(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:       planID,
		ClinicID: clinicID,
		Type:     sessionplan_model.PlanTypeIndividual,
		Status:   sessionplan_model.PlanStatusActive,
	}

	gaRepo := newMockGroupAttendanceRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	enrollRepo := newMockGroupEnrollmentRepo()

	svc := buildGroupAttendanceService(gaRepo, planRepo, enrollRepo)

	_, err := svc.RecordGroupAttendance(context.Background(), clinicID, planID, &model.RecordGroupAttendanceRequest{
		Date: "2026-04-09",
		Entries: []model.GroupAttendanceEntry{
			{PatientID: uuid.New(), Status: model.AttendanceStatusPresent},
		},
	})

	require.Error(t, err, "Should reject individual plans")
}

func TestRecordGroupAttendance_MultipleEntries(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	patient1 := uuid.New()
	patient2 := uuid.New()
	enrollment1ID := uuid.New()
	enrollment2ID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:       planID,
		ClinicID: clinicID,
		Type:     sessionplan_model.PlanTypeGroup,
		Status:   sessionplan_model.PlanStatusActive,
	}

	total := 20
	gaRepo := newMockGroupAttendanceRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	enrollRepo := newMockGroupEnrollmentRepo()
	enrollRepo.enrollments[patient1.String()] = &enrollment_model.SessionPlanEnrollment{
		ID: enrollment1ID, ClinicID: clinicID, SessionPlanID: planID,
		PatientID: patient1, Status: enrollment_model.EnrollmentStatusActive,
		TotalSessions: &total, SessionsDone: 5,
	}
	enrollRepo.enrollments[patient2.String()] = &enrollment_model.SessionPlanEnrollment{
		ID: enrollment2ID, ClinicID: clinicID, SessionPlanID: planID,
		PatientID: patient2, Status: enrollment_model.EnrollmentStatusActive,
		TotalSessions: &total, SessionsDone: 10,
	}

	svc := buildGroupAttendanceService(gaRepo, planRepo, enrollRepo)

	results, err := svc.RecordGroupAttendance(context.Background(), clinicID, planID, &model.RecordGroupAttendanceRequest{
		Date: "2026-04-09",
		Entries: []model.GroupAttendanceEntry{
			{PatientID: patient1, Status: model.AttendanceStatusPresent},
			{PatientID: patient2, Status: model.AttendanceStatusAbsent},
		},
	})

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.True(t, enrollRepo.incrementCalled[enrollment1ID.String()], "Patient1 present → should increment")
	assert.False(t, enrollRepo.incrementCalled[enrollment2ID.String()], "Patient2 absent → should NOT increment")
}
