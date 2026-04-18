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

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }

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

func TestUpdate_BusinessInfoFAQInstagramDocument(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)

	info := "Atendemos só particular. Aceitamos Pix."
	faq := "Q: Quanto custa? A: Consulte na recepção."
	insta := "@clinica_teste"
	doc := "12.345.678/0001-99"

	result, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
		Document:     &doc,
		Instagram:    &insta,
		BusinessInfo: &info,
		FAQ:          &faq,
	})

	require.NoError(t, err)
	assert.Equal(t, doc, result.Document)
	assert.Equal(t, insta, result.Instagram)
	assert.Equal(t, info, result.BusinessInfo)
	assert.Equal(t, faq, result.FAQ)

	stored := repo.clinics[clinic.ID]
	assert.Equal(t, doc, stored.Document)
	assert.Equal(t, insta, stored.Instagram)
	assert.Equal(t, info, stored.BusinessInfo)
	assert.Equal(t, faq, stored.FAQ)
}

func TestUpdate_BusinessInfoFAQ_NilDoesNotChange(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)
	clinic.BusinessInfo = "existing info"
	clinic.FAQ = "existing faq"

	result, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
		Name: "Other Name",
	})

	require.NoError(t, err)
	assert.Equal(t, "existing info", result.BusinessInfo)
	assert.Equal(t, "existing faq", result.FAQ)
}

func TestUpdate_BusinessInfoFAQ_EmptyStringClears(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)
	clinic.BusinessInfo = "existing info"
	clinic.FAQ = "existing faq"

	empty := ""
	result, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
		BusinessInfo: &empty,
		FAQ:          &empty,
	})

	require.NoError(t, err)
	assert.Equal(t, "", result.BusinessInfo)
	assert.Equal(t, "", result.FAQ)
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

func iPtr(v int) *int { return &v }

func TestUpdate_PaymentReminderDaysBefore_OutOfRange(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)

	for _, bad := range []int{0, 8, -1, 100} {
		_, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
			Settings: &model.UpdateClinicSettingsRequest{
				PaymentReminderDaysBefore: iPtr(bad),
			},
		})
		require.Error(t, err, "expected error for days=%d", bad)
		assert.Contains(t, err.Error(), "between 1 and 7")
	}
}

func TestUpdate_PaymentReminderDaysBefore_Valid(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)

	for _, good := range []int{1, 3, 7} {
		result, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
			Settings: &model.UpdateClinicSettingsRequest{
				PaymentReminderDaysBefore: iPtr(good),
			},
		})
		require.NoError(t, err, "expected no error for days=%d", good)
		require.NotNil(t, result.Settings.PaymentReminderDaysBefore)
		assert.Equal(t, good, *result.Settings.PaymentReminderDaysBefore)
	}
}

func TestUpdate_PaymentReminderChannelToggles(t *testing.T) {
	repo := newMockClinicRepo()
	svc := NewClinicService(repo, "http://localhost:3000")
	clinic := createTestClinic(repo)

	result, err := svc.Update(context.Background(), clinic.ID, &model.UpdateClinicRequest{
		Settings: &model.UpdateClinicSettingsRequest{
			PaymentReminderWhatsAppEnabled: boolPtr(true),
			PaymentReminderEmailEnabled:    boolPtr(false),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Settings.PaymentReminderWhatsAppEnabled)
	require.NotNil(t, result.Settings.PaymentReminderEmailEnabled)
	assert.True(t, *result.Settings.PaymentReminderWhatsAppEnabled)
	assert.False(t, *result.Settings.PaymentReminderEmailEnabled)
}

func TestClinicSettings_GetPaymentReminderDaysBefore_Default(t *testing.T) {
	var s model.ClinicSettings
	assert.Equal(t, model.DefaultPaymentReminderDaysBefore, s.GetPaymentReminderDaysBefore())

	bad := 10
	s.PaymentReminderDaysBefore = &bad
	assert.Equal(t, model.DefaultPaymentReminderDaysBefore, s.GetPaymentReminderDaysBefore())

	good := 5
	s.PaymentReminderDaysBefore = &good
	assert.Equal(t, 5, s.GetPaymentReminderDaysBefore())
}
