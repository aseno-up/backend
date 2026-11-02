package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/pkg/attendance/model"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
)

// --- Mock GroupSessionProfessional Repo ---

type mockGSPRepo struct {
	records map[string]*model.GroupSessionProfessional // key: planID+date
}

func newMockGSPRepo() *mockGSPRepo {
	return &mockGSPRepo{records: make(map[string]*model.GroupSessionProfessional)}
}

func (m *mockGSPRepo) key(planID uuid.UUID, date string) string {
	return planID.String() + "|" + date
}

func (m *mockGSPRepo) Upsert(_ context.Context, record *model.GroupSessionProfessional) error {
	record.ID = uuid.New()
	m.records[m.key(record.SessionPlanID, record.SessionDate)] = record
	return nil
}

func (m *mockGSPRepo) GetByPlanAndDate(_ context.Context, _, sessionPlanID uuid.UUID, date string) (*model.GroupSessionProfessional, error) {
	r, ok := m.records[m.key(sessionPlanID, date)]
	if !ok {
		return nil, nil
	}
	return r, nil
}

// --- Tests ---

func TestSetProfessional_FlexiblePlan_Success(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	profID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Type:           sessionplan_model.PlanTypeGroup,
		ProfessionalID: nil, // flexible
	}

	repo := newMockGSPRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	svc := NewGroupSessionProfessionalService(repo, planRepo)

	result, err := svc.SetProfessional(context.Background(), clinicID, planID, "2026-04-09", profID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, profID, result.ProfessionalID)
	assert.Equal(t, "2026-04-09", result.SessionDate)
}

func TestSetProfessional_FixedPlan_MatchingProfessional_Success(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	profID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Type:           sessionplan_model.PlanTypeGroup,
		ProfessionalID: &profID, // fixed
	}

	repo := newMockGSPRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	svc := NewGroupSessionProfessionalService(repo, planRepo)

	result, err := svc.SetProfessional(context.Background(), clinicID, planID, "2026-04-09", profID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, profID, result.ProfessionalID)
}

func TestSetProfessional_FixedPlan_DifferentProfessional_Error(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	fixedProfID := uuid.New()
	otherProfID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Type:           sessionplan_model.PlanTypeGroup,
		ProfessionalID: &fixedProfID,
	}

	repo := newMockGSPRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	svc := NewGroupSessionProfessionalService(repo, planRepo)

	_, err := svc.SetProfessional(context.Background(), clinicID, planID, "2026-04-09", otherProfID)

	require.Error(t, err, "Should reject different professional for fixed plan")
}

func TestSetProfessional_IndividualPlan_Error(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	profID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:       planID,
		ClinicID: clinicID,
		Type:     sessionplan_model.PlanTypeIndividual,
	}

	repo := newMockGSPRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	svc := NewGroupSessionProfessionalService(repo, planRepo)

	_, err := svc.SetProfessional(context.Background(), clinicID, planID, "2026-04-09", profID)

	require.Error(t, err, "Should reject individual plans")
}

func TestSetProfessional_Upsert_OverwritesPrevious(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	prof1 := uuid.New()
	prof2 := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Type:           sessionplan_model.PlanTypeGroup,
		ProfessionalID: nil, // flexible
	}

	repo := newMockGSPRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	svc := NewGroupSessionProfessionalService(repo, planRepo)

	_, err := svc.SetProfessional(context.Background(), clinicID, planID, "2026-04-09", prof1)
	require.NoError(t, err)

	result, err := svc.SetProfessional(context.Background(), clinicID, planID, "2026-04-09", prof2)
	require.NoError(t, err)
	assert.Equal(t, prof2, result.ProfessionalID, "Should overwrite previous professional")
}

func TestGetProfessional_NotSet_ReturnsNil(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()

	repo := newMockGSPRepo()
	planRepo := &mockGroupPlanRepo{plan: &sessionplan_model.SessionPlan{}}
	svc := NewGroupSessionProfessionalService(repo, planRepo)

	result, err := svc.GetProfessional(context.Background(), clinicID, planID, "2026-04-09")

	require.NoError(t, err)
	assert.Nil(t, result, "Should return nil when no professional assigned")
}

func TestGetProfessional_Set_ReturnsProfessional(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	profID := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Type:           sessionplan_model.PlanTypeGroup,
		ProfessionalID: nil,
	}

	repo := newMockGSPRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	svc := NewGroupSessionProfessionalService(repo, planRepo)

	_, err := svc.SetProfessional(context.Background(), clinicID, planID, "2026-04-09", profID)
	require.NoError(t, err)

	result, err := svc.GetProfessional(context.Background(), clinicID, planID, "2026-04-09")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, profID, result.ProfessionalID)
}

func TestSetProfessional_DifferentDates_IndependentAssignments(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	prof1 := uuid.New()
	prof2 := uuid.New()

	plan := &sessionplan_model.SessionPlan{
		ID:             planID,
		ClinicID:       clinicID,
		Type:           sessionplan_model.PlanTypeGroup,
		ProfessionalID: nil,
	}

	repo := newMockGSPRepo()
	planRepo := &mockGroupPlanRepo{plan: plan}
	svc := NewGroupSessionProfessionalService(repo, planRepo)

	_, err := svc.SetProfessional(context.Background(), clinicID, planID, "2026-04-09", prof1)
	require.NoError(t, err)

	_, err = svc.SetProfessional(context.Background(), clinicID, planID, "2026-04-10", prof2)
	require.NoError(t, err)

	r1, _ := svc.GetProfessional(context.Background(), clinicID, planID, "2026-04-09")
	r2, _ := svc.GetProfessional(context.Background(), clinicID, planID, "2026-04-10")

	require.NotNil(t, r1)
	require.NotNil(t, r2)
	assert.Equal(t, prof1, r1.ProfessionalID, "Date 1 should have prof1")
	assert.Equal(t, prof2, r2.ProfessionalID, "Date 2 should have prof2")
}
