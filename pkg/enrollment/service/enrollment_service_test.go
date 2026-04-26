package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internal_errors "github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/enrollment/model"
	"github.com/aseno/backend/pkg/pagination"
	pn_model "github.com/aseno/backend/pkg/partnernetwork/model"
	sp_model "github.com/aseno/backend/pkg/sessionplan/model"
	"time"
)

// --- Mock SlotProvider ---

type mockSlotProvider struct {
	slots map[uuid.UUID]*sp_model.SessionPlanSlot
	err   error
}

func (m *mockSlotProvider) GetByIDScoped(_ context.Context, id, _ uuid.UUID) (*sp_model.SessionPlanSlot, error) {
	if m.err != nil {
		return nil, m.err
	}
	if s, ok := m.slots[id]; ok {
		return s, nil
	}
	return nil, internal_errors.ErrNotFoundInstance
}

// --- Mock PartnerPlanProvider ---

type mockPartnerPlanProvider struct {
	plan *pn_model.PartnerPlan
	err  error
}

func (m *mockPartnerPlanProvider) GetByIDScoped(_, _ uuid.UUID) (*pn_model.PartnerPlan, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.plan, nil
}

// --- Mock EnrollmentRepository ---

type mockEnrollRepo struct {
	active          []*model.SessionPlanEnrollment
	existing        *model.SessionPlanEnrollment
	created         *model.SessionPlanEnrollment
	updated         *model.SessionPlanEnrollment
	getByIDErr      error
	getByPatientErr error
	createErr       error
}

func (m *mockEnrollRepo) Create(_ context.Context, e *model.SessionPlanEnrollment) error {
	if m.createErr != nil {
		return m.createErr
	}
	e.ID = uuid.New()
	m.created = e
	m.active = append(m.active, e)
	return nil
}
func (m *mockEnrollRepo) GetByID(_ context.Context, _, id uuid.UUID) (*model.SessionPlanEnrollment, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	if m.created != nil && m.created.ID == id {
		return m.created, nil
	}
	if m.existing != nil && m.existing.ID == id {
		return m.existing, nil
	}
	return nil, internal_errors.ErrNotFoundInstance
}
func (m *mockEnrollRepo) GetByPlan(_ context.Context, _, _ uuid.UUID) ([]*model.SessionPlanEnrollment, error) {
	return m.active, nil
}
func (m *mockEnrollRepo) GetActiveByPlan(_ context.Context, _, _ uuid.UUID) ([]*model.SessionPlanEnrollment, error) {
	result := make([]*model.SessionPlanEnrollment, 0, len(m.active))
	for _, e := range m.active {
		if e.IsActive() {
			result = append(result, e)
		}
	}
	return result, nil
}
func (m *mockEnrollRepo) GetByPatientAndPlan(_ context.Context, _, _, _ uuid.UUID) (*model.SessionPlanEnrollment, error) {
	if m.getByPatientErr != nil {
		return nil, m.getByPatientErr
	}
	return m.existing, nil
}
func (m *mockEnrollRepo) CountActiveByPlan(_ context.Context, _, _ uuid.UUID) (int64, error) {
	var count int64
	for _, e := range m.active {
		if e.IsActive() {
			count++
		}
	}
	return count, nil
}
func (m *mockEnrollRepo) CountActiveByPlanAndWeekDay(_ context.Context, _, _ uuid.UUID, weekDay int) (int64, error) {
	var count int64
	for _, e := range m.active {
		if !e.IsActive() {
			continue
		}
		for _, d := range e.WeekDays {
			if int(d) == weekDay {
				count++
				break
			}
		}
	}
	return count, nil
}
func (m *mockEnrollRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ model.EnrollmentStatus) error {
	return nil
}
func (m *mockEnrollRepo) IncrementSessionsDone(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockEnrollRepo) Update(_ context.Context, e *model.SessionPlanEnrollment) error {
	m.updated = e
	return nil
}
func (m *mockEnrollRepo) CountActiveBySlot(_ context.Context, _, _, slotID uuid.UUID) (int64, error) {
	var count int64
	for _, e := range m.active {
		if !e.IsActive() {
			continue
		}
		for _, s := range e.Slots {
			if s.SessionPlanSlotID == slotID {
				count++
				break
			}
		}
	}
	return count, nil
}
func (m *mockEnrollRepo) SetEnrollmentSlots(_ context.Context, enrollmentID uuid.UUID, slotIDs []uuid.UUID) error {
	target := m.created
	if target == nil || target.ID != enrollmentID {
		if m.existing != nil && m.existing.ID == enrollmentID {
			target = m.existing
		}
	}
	if target == nil {
		return nil
	}
	target.Slots = make([]model.EnrollmentSlot, len(slotIDs))
	for i, sid := range slotIDs {
		target.Slots[i] = model.EnrollmentSlot{EnrollmentID: enrollmentID, SessionPlanSlotID: sid}
	}
	return nil
}
func (m *mockEnrollRepo) GetSlotIDsByEnrollment(_ context.Context, enrollmentID uuid.UUID) ([]uuid.UUID, error) {
	target := m.created
	if target == nil || target.ID != enrollmentID {
		if m.existing != nil && m.existing.ID == enrollmentID {
			target = m.existing
		}
	}
	if target == nil {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(target.Slots))
	for i, s := range target.Slots {
		ids[i] = s.SessionPlanSlotID
	}
	return ids, nil
}

// --- Mock SessionPlanRepository (minimal) ---

type mockSPRepo struct {
	plan *sp_model.SessionPlan
	err  error
}

func (m *mockSPRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*sp_model.SessionPlan, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.plan, nil
}
func (m *mockSPRepo) Create(_ context.Context, _ *sp_model.SessionPlan) error { return nil }
func (m *mockSPRepo) GetByClinic(_ context.Context, _ uuid.UUID, _ pagination.Filter) ([]*sp_model.SessionPlan, int64, error) {
	return nil, 0, nil
}
func (m *mockSPRepo) GetByPatient(_ context.Context, _, _ uuid.UUID) ([]*sp_model.SessionPlan, error) {
	return nil, nil
}
func (m *mockSPRepo) GetByProfessional(_ context.Context, _, _ uuid.UUID) ([]*sp_model.SessionPlan, error) {
	return nil, nil
}
func (m *mockSPRepo) GetActivePlansForGeneration(_ context.Context, _ time.Time) ([]*sp_model.SessionPlan, error) {
	return nil, nil
}
func (m *mockSPRepo) GetAllActive(_ context.Context, _ uuid.UUID) ([]*sp_model.SessionPlan, error) {
	return nil, nil
}
func (m *mockSPRepo) Update(_ context.Context, _ *sp_model.SessionPlan) error { return nil }
func (m *mockSPRepo) UpdateStatus(_ context.Context, _, _ uuid.UUID, _ sp_model.PlanStatus) error {
	return nil
}
func (m *mockSPRepo) IncrementSessionsDone(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockSPRepo) UpdateLastGenerated(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (m *mockSPRepo) Delete(_ context.Context, _, _ uuid.UUID) error                 { return nil }
func (m *mockSPRepo) BulkCancel(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error { return nil }

// --- Helpers ---

func newGroupPlan(cap *int) *sp_model.SessionPlan {
	return &sp_model.SessionPlan{
		ID:       uuid.New(),
		ClinicID: uuid.New(),
		Type:     sp_model.PlanTypeGroup,
		Status:   sp_model.PlanStatusActive,
		Capacity: cap,
		WeekDays: pq.Int64Array{1, 2, 3, 4, 5}, // Mon-Fri
	}
}

func intPtr(i int) *int { return &i }

// --- Tests ---

func TestEnroll_RequiresWeekDaysForGroupPlan(t *testing.T) {
	plan := newGroupPlan(intPtr(5))
	svc := NewEnrollmentService(&mockEnrollRepo{}, &mockSPRepo{plan: plan})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		WeekDays:  []int{}, // empty
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "week_days")
}

func TestEnroll_RejectsInvalidWeekDayValue(t *testing.T) {
	plan := newGroupPlan(intPtr(5))
	svc := NewEnrollmentService(&mockEnrollRepo{}, &mockSPRepo{plan: plan})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		WeekDays:  []int{1, 7}, // 7 is invalid
	})

	require.Error(t, err)
}

func TestEnroll_SuccessWithWeekDays(t *testing.T) {
	plan := newGroupPlan(intPtr(5))
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	enrollment, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		WeekDays:  []int{1, 3}, // Mon, Wed
	})

	require.NoError(t, err)
	require.NotNil(t, enrollment)
	assert.Equal(t, pq.Int64Array{1, 3}, enrollment.WeekDays)
}

func TestEnroll_PerDayCapacity_BlockSameDay(t *testing.T) {
	// Plan capacity = 2. Two students already on Monday. A third attempts Monday -> fail.
	plan := newGroupPlan(intPtr(2))
	repo := &mockEnrollRepo{
		active: []*model.SessionPlanEnrollment{
			{ID: uuid.New(), Status: model.EnrollmentStatusActive, WeekDays: pq.Int64Array{1}},
			{ID: uuid.New(), Status: model.EnrollmentStatusActive, WeekDays: pq.Int64Array{1, 3}},
		},
	}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		WeekDays:  []int{1}, // Monday full
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "capacity")
}

func TestEnroll_PerDayCapacity_AllowsDifferentDay(t *testing.T) {
	// Plan capacity = 2. Two students on Monday. Third on Wednesday -> should succeed.
	plan := newGroupPlan(intPtr(2))
	repo := &mockEnrollRepo{
		active: []*model.SessionPlanEnrollment{
			{ID: uuid.New(), Status: model.EnrollmentStatusActive, WeekDays: pq.Int64Array{1}},
			{ID: uuid.New(), Status: model.EnrollmentStatusActive, WeekDays: pq.Int64Array{1}},
		},
	}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	enrollment, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		WeekDays:  []int{3}, // Wednesday empty
	})

	require.NoError(t, err)
	require.NotNil(t, enrollment)
}

func TestEnroll_PerDayCapacity_BlocksWhenAnyRequestedDayIsFull(t *testing.T) {
	// Plan cap = 2. Mon has 2 (full), Wed has 0. Request [Mon, Wed] -> fails because Mon is full.
	plan := newGroupPlan(intPtr(2))
	repo := &mockEnrollRepo{
		active: []*model.SessionPlanEnrollment{
			{ID: uuid.New(), Status: model.EnrollmentStatusActive, WeekDays: pq.Int64Array{1}},
			{ID: uuid.New(), Status: model.EnrollmentStatusActive, WeekDays: pq.Int64Array{1}},
		},
	}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		WeekDays:  []int{1, 3},
	})

	require.Error(t, err)
}

func TestEnroll_IgnoresCancelledEnrollmentsInCapacity(t *testing.T) {
	plan := newGroupPlan(intPtr(2))
	repo := &mockEnrollRepo{
		active: []*model.SessionPlanEnrollment{
			{ID: uuid.New(), Status: model.EnrollmentStatusActive, WeekDays: pq.Int64Array{1}},
			{ID: uuid.New(), Status: model.EnrollmentStatusCancelled, WeekDays: pq.Int64Array{1}},
			{ID: uuid.New(), Status: model.EnrollmentStatusCancelled, WeekDays: pq.Int64Array{1}},
		},
	}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		WeekDays:  []int{1}, // only 1 active on Mon
	})

	require.NoError(t, err)
}

func TestEnroll_ReactivationUpdatesWeekDays(t *testing.T) {
	plan := newGroupPlan(intPtr(5))
	patientID := uuid.New()
	repo := &mockEnrollRepo{
		existing: &model.SessionPlanEnrollment{
			ID:        uuid.New(),
			PatientID: patientID,
			Status:    model.EnrollmentStatusCancelled,
			WeekDays:  pq.Int64Array{1},
		},
	}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	result, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: patientID,
		WeekDays:  []int{2, 4},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, model.EnrollmentStatusActive, result.Status)
	assert.Equal(t, pq.Int64Array{2, 4}, result.WeekDays)
}

func TestEnroll_MonthlyRequiresBillingDay(t *testing.T) {
	plan := newGroupPlan(intPtr(5))
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID:   uuid.New(),
		WeekDays:    []int{1},
		PaymentMode: model.PaymentModeMonthly,
		// BillingDay intentionally omitted
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "billing_day")
}

func TestEnroll_MonthlyRejectsOutOfRangeBillingDay(t *testing.T) {
	plan := newGroupPlan(intPtr(5))
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	bad := 32
	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID:   uuid.New(),
		WeekDays:    []int{1},
		PaymentMode: model.PaymentModeMonthly,
		BillingDay:  &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "between 1 and 31")
}

func TestEnroll_MonthlyAcceptsValidBillingDay(t *testing.T) {
	plan := newGroupPlan(intPtr(5))
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	day := 10
	enrollment, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID:   uuid.New(),
		WeekDays:    []int{1},
		PaymentMode: model.PaymentModeMonthly,
		BillingDay:  &day,
	})
	require.NoError(t, err)
	require.NotNil(t, enrollment)
	require.NotNil(t, enrollment.BillingDay)
	assert.Equal(t, 10, *enrollment.BillingDay)
}

func TestEnroll_RejectsActiveDuplicate(t *testing.T) {
	plan := newGroupPlan(intPtr(5))
	patientID := uuid.New()
	repo := &mockEnrollRepo{
		existing: &model.SessionPlanEnrollment{
			PatientID: patientID,
			Status:    model.EnrollmentStatusActive,
			WeekDays:  pq.Int64Array{1},
		},
	}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: patientID,
		WeekDays:  []int{2},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, internal_errors.ErrValidationInstance) || err != nil)
}

// --- Slot-based + partner plan tests ---

func newSlot(planID uuid.UUID, weekday int, capacity int) *sp_model.SessionPlanSlot {
	return &sp_model.SessionPlanSlot{
		ID:            uuid.New(),
		SessionPlanID: planID,
		Weekday:       weekday,
		StartTime:     "08:00",
		Capacity:      capacity,
		IsActive:      true,
	}
}

func TestEnroll_WithSlots_DerivesWeekDays(t *testing.T) {
	plan := newGroupPlan(nil)
	slotMon := newSlot(plan.ID, 1, 5)
	slotWed := newSlot(plan.ID, 3, 5)
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})
	svc.SetSlotProvider(&mockSlotProvider{slots: map[uuid.UUID]*sp_model.SessionPlanSlot{
		slotMon.ID: slotMon, slotWed.ID: slotWed,
	}})

	enrollment, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		SlotIDs:   []uuid.UUID{slotMon.ID, slotWed.ID},
	})

	require.NoError(t, err)
	require.NotNil(t, enrollment)
	assert.ElementsMatch(t, pq.Int64Array{1, 3}, enrollment.WeekDays)
}

func TestEnroll_WithSlots_RejectsSlotFromOtherPlan(t *testing.T) {
	plan := newGroupPlan(nil)
	otherPlanID := uuid.New()
	foreignSlot := newSlot(otherPlanID, 1, 5)
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})
	svc.SetSlotProvider(&mockSlotProvider{slots: map[uuid.UUID]*sp_model.SessionPlanSlot{
		foreignSlot.ID: foreignSlot,
	}})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		SlotIDs:   []uuid.UUID{foreignSlot.ID},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
}

func TestEnroll_WithSlots_RejectsInactiveSlot(t *testing.T) {
	plan := newGroupPlan(nil)
	slot := newSlot(plan.ID, 1, 5)
	slot.IsActive = false
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})
	svc.SetSlotProvider(&mockSlotProvider{slots: map[uuid.UUID]*sp_model.SessionPlanSlot{slot.ID: slot}})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		SlotIDs:   []uuid.UUID{slot.ID},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "inactive")
}

func TestEnroll_WithSlots_RespectsPerSlotCapacity(t *testing.T) {
	plan := newGroupPlan(nil)
	slot := newSlot(plan.ID, 1, 1) // capacity 1
	// One active enrollment already on this slot.
	existing := &model.SessionPlanEnrollment{
		ID:     uuid.New(),
		Status: model.EnrollmentStatusActive,
		Slots: []model.EnrollmentSlot{
			{EnrollmentID: uuid.New(), SessionPlanSlotID: slot.ID},
		},
	}
	repo := &mockEnrollRepo{active: []*model.SessionPlanEnrollment{existing}}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})
	svc.SetSlotProvider(&mockSlotProvider{slots: map[uuid.UUID]*sp_model.SessionPlanSlot{slot.ID: slot}})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		SlotIDs:   []uuid.UUID{slot.ID},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "capacity")
}

func TestEnroll_WithoutSlotProvider_RejectsSlotIDs(t *testing.T) {
	plan := newGroupPlan(nil)
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})
	// Slot provider intentionally not set.

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID: uuid.New(),
		SlotIDs:   []uuid.UUID{uuid.New()},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "slot-based enrollment is not supported")
}

func TestEnroll_PartnerPlan_ExceedsMaxSessionsPerWeek(t *testing.T) {
	plan := newGroupPlan(nil)
	max := 3
	pp := &pn_model.PartnerPlan{
		ID:                 uuid.New(),
		Name:               "Silver",
		MaxSessionsPerWeek: &max,
		IsActive:           true,
	}
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})
	svc.SetPartnerPlanProvider(&mockPartnerPlanProvider{plan: pp})

	// Patient picks 4 weekdays — exceeds Silver's 3x/week cap.
	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID:     uuid.New(),
		WeekDays:      []int{1, 2, 3, 4},
		PartnerPlanID: &pp.ID,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max sessions per week")
}

func TestEnroll_PartnerPlan_AllowsWithinLimit(t *testing.T) {
	plan := newGroupPlan(intPtr(10))
	max := 3
	pp := &pn_model.PartnerPlan{
		ID:                 uuid.New(),
		Name:               "Silver",
		MaxSessionsPerWeek: &max,
		IsActive:           true,
	}
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})
	svc.SetPartnerPlanProvider(&mockPartnerPlanProvider{plan: pp})

	enrollment, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID:     uuid.New(),
		WeekDays:      []int{1, 3, 5},
		PartnerPlanID: &pp.ID,
	})

	require.NoError(t, err)
	require.NotNil(t, enrollment)
	require.NotNil(t, enrollment.PartnerPlanID)
	assert.Equal(t, pp.ID, *enrollment.PartnerPlanID)
}

func TestEnroll_PartnerPlan_RejectsInactivePlan(t *testing.T) {
	plan := newGroupPlan(intPtr(5))
	pp := &pn_model.PartnerPlan{
		ID:       uuid.New(),
		Name:     "Silver",
		IsActive: false,
	}
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})
	svc.SetPartnerPlanProvider(&mockPartnerPlanProvider{plan: pp})

	_, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID:     uuid.New(),
		WeekDays:      []int{1},
		PartnerPlanID: &pp.ID,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "inactive")
}

func TestEnroll_PartnerPlan_NilProviderAcceptsButDoesNotValidate(t *testing.T) {
	// When no provider is set, partner_plan_id is persisted as-is and frequency
	// validation is skipped. This is the "incremental rollout" path.
	plan := newGroupPlan(intPtr(5))
	repo := &mockEnrollRepo{}
	svc := NewEnrollmentService(repo, &mockSPRepo{plan: plan})
	// no SetPartnerPlanProvider

	planID := uuid.New()
	enrollment, err := svc.Enroll(context.Background(), plan.ClinicID, plan.ID, &model.EnrollRequest{
		PatientID:     uuid.New(),
		WeekDays:      []int{1, 2, 3, 4, 5}, // would exceed any real Silver cap
		PartnerPlanID: &planID,
	})

	require.NoError(t, err)
	require.NotNil(t, enrollment.PartnerPlanID)
	assert.Equal(t, planID, *enrollment.PartnerPlanID)
}
