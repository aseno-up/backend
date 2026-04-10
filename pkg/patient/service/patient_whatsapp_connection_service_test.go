package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalErrors "github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/patient/model"
	whatsapp_model "github.com/aseno/backend/pkg/whatsapp/model"
)

// --- Mock Repositories ---

type mockPatientWhatsAppConnRepo struct {
	conns map[uuid.UUID]*model.PatientWhatsAppConnection // keyed by patientID
}

func newMockPatientWhatsAppConnRepo() *mockPatientWhatsAppConnRepo {
	return &mockPatientWhatsAppConnRepo{conns: make(map[uuid.UUID]*model.PatientWhatsAppConnection)}
}

func (m *mockPatientWhatsAppConnRepo) GetByPatientID(_ context.Context, patientID, _ uuid.UUID) (*model.PatientWhatsAppConnection, error) {
	if conn, ok := m.conns[patientID]; ok {
		return conn, nil
	}
	return nil, internalErrors.ErrNotFoundInstance
}

func (m *mockPatientWhatsAppConnRepo) Upsert(_ context.Context, conn *model.PatientWhatsAppConnection) error {
	if conn.ID == uuid.Nil {
		conn.ID = uuid.New()
	}
	m.conns[conn.PatientID] = conn
	return nil
}

func (m *mockPatientWhatsAppConnRepo) DeleteByPatientID(_ context.Context, patientID, _ uuid.UUID) error {
	delete(m.conns, patientID)
	return nil
}

type mockPatientRepoForConn struct {
	patients map[uuid.UUID]*model.Patient
}

func newMockPatientRepoForConn() *mockPatientRepoForConn {
	return &mockPatientRepoForConn{patients: make(map[uuid.UUID]*model.Patient)}
}

func (m *mockPatientRepoForConn) GetByID(_ context.Context, id, clinicID uuid.UUID) (*model.Patient, error) {
	p, ok := m.patients[id]
	if !ok || p.ClinicID != clinicID {
		return nil, internalErrors.ErrNotFoundInstance
	}
	return p, nil
}

// Unused methods to satisfy PatientRepository interface
func (m *mockPatientRepoForConn) Create(_ context.Context, _ *model.Patient) error { return nil }
func (m *mockPatientRepoForConn) List(_ context.Context, _ uuid.UUID, _ pagination.Filter) ([]*model.Patient, int64, error) { return nil, 0, nil }
func (m *mockPatientRepoForConn) Update(_ context.Context, _ *model.Patient) error { return nil }
func (m *mockPatientRepoForConn) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockPatientRepoForConn) BulkDelete(_ context.Context, _ []uuid.UUID, _ uuid.UUID) error { return nil }
func (m *mockPatientRepoForConn) GetByPhone(_ context.Context, _ string, _ uuid.UUID) (*model.Patient, error) { return nil, nil }
func (m *mockPatientRepoForConn) GetByDocument(_ context.Context, _ string, _ uuid.UUID) (*model.Patient, error) { return nil, nil }
func (m *mockPatientRepoForConn) GetAppointmentStats(_ context.Context, _, _ uuid.UUID) (*model.PatientAppointmentStats, error) { return nil, nil }
func (m *mockPatientRepoForConn) UpdateRiskScore(_ context.Context, _, _ uuid.UUID, _ model.NoShowRisk, _ *model.PatientAppointmentStats) error { return nil }
func (m *mockPatientRepoForConn) GetAllByClinicID(_ context.Context, _ uuid.UUID) ([]*model.Patient, error) { return nil, nil }
func (m *mockPatientRepoForConn) ListBySource(_ context.Context, _ uuid.UUID, _ model.PatientSource, _ pagination.Filter) ([]*model.Patient, int64, error) { return nil, 0, nil }
func (m *mockPatientRepoForConn) MergePatients(_ context.Context, _, _, _ uuid.UUID) error { return nil }
func (m *mockPatientRepoForConn) GetUnlinkedProvisionalPatients(_ context.Context, _ uuid.UUID) ([]*model.Patient, error) { return nil, nil }
func (m *mockPatientRepoForConn) SearchPatientsExcludingProvisional(_ context.Context, _ uuid.UUID, _ string, _ int) ([]*model.Patient, error) { return nil, nil }
func (m *mockPatientRepoForConn) MergePatientsBulk(_ context.Context, _, _ uuid.UUID, _ []uuid.UUID, _ uuid.UUID) (int64, error) { return 0, nil }

type mockWhatsAppConnRepoForPatient struct {
	conns map[uuid.UUID]*whatsapp_model.WhatsAppConnection
}

func newMockWhatsAppConnRepoForPatient() *mockWhatsAppConnRepoForPatient {
	return &mockWhatsAppConnRepoForPatient{conns: make(map[uuid.UUID]*whatsapp_model.WhatsAppConnection)}
}

func (m *mockWhatsAppConnRepoForPatient) GetActiveByClinicID(_ context.Context, clinicID uuid.UUID) (*whatsapp_model.WhatsAppConnection, error) {
	if conn, ok := m.conns[clinicID]; ok {
		return conn, nil
	}
	return nil, internalErrors.ErrNotFoundInstance
}

// Unused methods to satisfy interface
func (m *mockWhatsAppConnRepoForPatient) Create(_ context.Context, _ *whatsapp_model.WhatsAppConnection) error { return nil }
func (m *mockWhatsAppConnRepoForPatient) GetByInstance(_ context.Context, _ string) (*whatsapp_model.WhatsAppConnection, error) { return nil, nil }
func (m *mockWhatsAppConnRepoForPatient) Update(_ context.Context, _ *whatsapp_model.WhatsAppConnection) error { return nil }
func (m *mockWhatsAppConnRepoForPatient) UpdateConnected(_ context.Context, _ uuid.UUID, _ bool) error { return nil }
func (m *mockWhatsAppConnRepoForPatient) Deactivate(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockWhatsAppConnRepoForPatient) HasAnyByClinicID(_ context.Context, _ uuid.UUID) bool { return false }
func (m *mockWhatsAppConnRepoForPatient) UpsertInstance(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (m *mockWhatsAppConnRepoForPatient) CountActiveByAccountID(_ context.Context, _ uuid.UUID) (int64, error) { return 0, nil }

// --- Helpers ---

func boolPtrConn(b bool) *bool { return &b }

func setupConnTest() (
	*mockPatientWhatsAppConnRepo,
	*mockPatientRepoForConn,
	*mockWhatsAppConnRepoForPatient,
	PatientWhatsAppConnectionService,
	uuid.UUID, // clinicID
	uuid.UUID, // patientID
	uuid.UUID, // whatsappConnID
) {
	connRepo := newMockPatientWhatsAppConnRepo()
	patientRepo := newMockPatientRepoForConn()
	whatsappRepo := newMockWhatsAppConnRepoForPatient()

	clinicID := uuid.New()
	patientID := uuid.New()
	whatsappConnID := uuid.New()

	patientRepo.patients[patientID] = &model.Patient{
		ID:       patientID,
		ClinicID: clinicID,
		Name:     "Test Patient",
		Phone:    "11999999999",
	}

	whatsappRepo.conns[clinicID] = &whatsapp_model.WhatsAppConnection{
		ID:       whatsappConnID,
		ClinicID: clinicID,
		IsActive: true,
	}

	svc := NewPatientWhatsAppConnectionService(connRepo, patientRepo, whatsappRepo)

	return connRepo, patientRepo, whatsappRepo, svc, clinicID, patientID, whatsappConnID
}

// --- Tests ---

func TestGetByPatientID_NoRecord_ReturnsDefaults(t *testing.T) {
	_, _, _, svc, clinicID, patientID, _ := setupConnTest()

	result, err := svc.GetByPatientID(context.Background(), patientID, clinicID)

	require.NoError(t, err)
	assert.True(t, result.AISchedulingEnabled)
	assert.True(t, result.RemindersEnabled)
	assert.Equal(t, patientID, result.PatientID)
}

func TestGetByPatientID_WithRecord_ReturnsStored(t *testing.T) {
	connRepo, _, _, svc, clinicID, patientID, whatsappConnID := setupConnTest()

	connRepo.conns[patientID] = &model.PatientWhatsAppConnection{
		ID:                   uuid.New(),
		ClinicID:             clinicID,
		PatientID:            patientID,
		WhatsAppConnectionID: whatsappConnID,
		AISchedulingEnabled:  false,
		RemindersEnabled:     true,
	}

	result, err := svc.GetByPatientID(context.Background(), patientID, clinicID)

	require.NoError(t, err)
	assert.False(t, result.AISchedulingEnabled)
	assert.True(t, result.RemindersEnabled)
}

func TestGetByPatientID_WrongClinic_ReturnsError(t *testing.T) {
	_, _, _, svc, _, patientID, _ := setupConnTest()

	otherClinicID := uuid.New()
	_, err := svc.GetByPatientID(context.Background(), patientID, otherClinicID)

	assert.Error(t, err)
}

func TestUpdate_CreatesNewRecord(t *testing.T) {
	connRepo, _, _, svc, clinicID, patientID, whatsappConnID := setupConnTest()

	result, err := svc.Update(context.Background(), patientID, clinicID, &model.UpdatePatientWhatsAppConnectionRequest{
		AISchedulingEnabled: boolPtrConn(false),
	})

	require.NoError(t, err)
	assert.False(t, result.AISchedulingEnabled)
	assert.True(t, result.RemindersEnabled) // default

	// Verify persisted
	stored := connRepo.conns[patientID]
	assert.Equal(t, whatsappConnID, stored.WhatsAppConnectionID)
	assert.False(t, stored.AISchedulingEnabled)
}

func TestUpdate_UpdatesExistingRecord(t *testing.T) {
	connRepo, _, _, svc, clinicID, patientID, whatsappConnID := setupConnTest()

	// Create existing record
	connRepo.conns[patientID] = &model.PatientWhatsAppConnection{
		ID:                   uuid.New(),
		ClinicID:             clinicID,
		PatientID:            patientID,
		WhatsAppConnectionID: whatsappConnID,
		AISchedulingEnabled:  true,
		RemindersEnabled:     true,
	}

	// Update only reminders
	result, err := svc.Update(context.Background(), patientID, clinicID, &model.UpdatePatientWhatsAppConnectionRequest{
		RemindersEnabled: boolPtrConn(false),
	})

	require.NoError(t, err)
	assert.True(t, result.AISchedulingEnabled) // unchanged
	assert.False(t, result.RemindersEnabled)   // updated
}

func TestUpdate_NoWhatsAppConnection_ReturnsError(t *testing.T) {
	_, _, whatsappRepo, svc, clinicID, patientID, _ := setupConnTest()

	// Remove whatsapp connection
	delete(whatsappRepo.conns, clinicID)

	_, err := svc.Update(context.Background(), patientID, clinicID, &model.UpdatePatientWhatsAppConnectionRequest{
		AISchedulingEnabled: boolPtrConn(false),
	})

	assert.Error(t, err)
}

func TestUpdate_WrongClinic_ReturnsError(t *testing.T) {
	_, _, _, svc, _, patientID, _ := setupConnTest()

	otherClinicID := uuid.New()
	_, err := svc.Update(context.Background(), patientID, otherClinicID, &model.UpdatePatientWhatsAppConnectionRequest{
		AISchedulingEnabled: boolPtrConn(false),
	})

	assert.Error(t, err)
}
