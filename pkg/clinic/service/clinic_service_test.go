package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/pkg/clinic/model"
)

// --- Mock Repository ---

type mockClinicRepo struct {
	clinics map[uuid.UUID]*model.Clinic
}

func newMockClinicRepo() *mockClinicRepo {
	return &mockClinicRepo{clinics: make(map[uuid.UUID]*model.Clinic)}
}

func (m *mockClinicRepo) Create(_ context.Context, clinic *model.Clinic) error {
	m.clinics[clinic.ID] = clinic
	return nil
}

func (m *mockClinicRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Clinic, error) {
	if c, ok := m.clinics[id]; ok {
		return c, nil
	}
	return nil, &notFoundErr{}
}

func (m *mockClinicRepo) GetBySlug(_ context.Context, _ string) (*model.Clinic, error) {
	return nil, &notFoundErr{}
}

func (m *mockClinicRepo) GetByEmail(_ context.Context, _ string) (*model.Clinic, error) {
	return nil, &notFoundErr{}
}

func (m *mockClinicRepo) Update(_ context.Context, clinic *model.Clinic) error {
	m.clinics[clinic.ID] = clinic
	return nil
}

func (m *mockClinicRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

func (m *mockClinicRepo) List(_ context.Context, _, _ int) ([]*model.Clinic, int64, error) {
	return nil, 0, nil
}

func (m *mockClinicRepo) ExistsBySlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockClinicRepo) ExistsByEmail(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockClinicRepo) CountByAccountID(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

type notFoundErr struct{}

func (e *notFoundErr) Error() string { return "not found" }

// --- Helper ---

func boolPtr(b bool) *bool { return &b }

func createTestClinic(repo *mockClinicRepo) *model.Clinic {
	clinic := &model.Clinic{
		ID:   uuid.New(),
		Name: "Test Clinic",
		Slug: "test-clinic",
		Settings: model.ClinicSettings{
			AllowOnlineBooking:          true,
			RequirePaymentBeforeBooking: false,
		},
	}
	repo.clinics[clinic.ID] = clinic
	return clinic
}

// --- Tests ---

func TestUpdate_RequirePaymentBeforeBooking_Enable(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)

	// Verify initial state
	assert.False(t, clinic.Settings.RequirePaymentBeforeBooking)

	// Enable require_payment_before_booking
	result, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
		Settings: &model.UpdateClinicSettingsRequest{
			RequirePaymentBeforeBooking: boolPtr(true),
		},
	})

	require.NoError(t, err)
	assert.True(t, result.Settings.RequirePaymentBeforeBooking)

	// Verify it persisted in repo
	stored := repo.clinics[clinic.ID]
	assert.True(t, stored.Settings.RequirePaymentBeforeBooking)
}

func TestUpdate_RequirePaymentBeforeBooking_Disable(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)
	clinic.Settings.RequirePaymentBeforeBooking = true

	// Disable
	result, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
		Settings: &model.UpdateClinicSettingsRequest{
			RequirePaymentBeforeBooking: boolPtr(false),
		},
	})

	require.NoError(t, err)
	assert.False(t, result.Settings.RequirePaymentBeforeBooking)
}

func TestUpdate_RequirePaymentBeforeBooking_NilDoesNotChange(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)
	clinic.Settings.RequirePaymentBeforeBooking = true

	// Update other settings without touching RequirePaymentBeforeBooking
	result, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
		Settings: &model.UpdateClinicSettingsRequest{
			AllowOnlineBooking: boolPtr(false),
		},
	})

	require.NoError(t, err)
	// Should remain true (unchanged)
	assert.True(t, result.Settings.RequirePaymentBeforeBooking)
	// AllowOnlineBooking should be updated
	assert.False(t, result.Settings.AllowOnlineBooking)
}

func TestUpdate_SettingsNil_DoesNotCrash(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)

	// Update without settings
	result, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
		Name: "New Name",
	})

	require.NoError(t, err)
	assert.Equal(t, "New Name", result.Name)
	assert.False(t, result.Settings.RequirePaymentBeforeBooking)
}
