package repository

import (
	"context"
	"testing"

	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAppointmentRepository_List(t *testing.T) {
	// Setup a clean DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// In SQLite we can't use full AutoMigrate due to Postgres specific syntax in some models,
	// so we manually create the tables needed for this repository's List method.
	err = db.Exec(`CREATE TABLE appointments (id text PRIMARY KEY, clinic_id text, patient_id text, professional_id text, start_time datetime, status text, deleted_at datetime)`).Error
	require.NoError(t, err)
	err = db.Exec(`CREATE TABLE patients (id text PRIMARY KEY, deleted_at datetime)`).Error
	require.NoError(t, err)
	err = db.Exec(`CREATE TABLE professionals (id text PRIMARY KEY, deleted_at datetime)`).Error
	require.NoError(t, err)
	err = db.Exec(`CREATE TABLE appointment_procedures (id text PRIMARY KEY, appointment_id text, procedure_id text)`).Error
	require.NoError(t, err)
	err = db.Exec(`CREATE TABLE procedures (id text PRIMARY KEY, deleted_at datetime)`).Error
	require.NoError(t, err)
	err = db.Exec(`CREATE TABLE appointment_specialties (id text PRIMARY KEY, clinic_id text, appointment_id text, specialty_id text, created_at datetime, updated_at datetime, deleted_at datetime)`).Error
	require.NoError(t, err)

	clinicID := uuid.New()
	patientID := uuid.New()
	profID := uuid.New()

	err = db.Exec(`INSERT INTO patients (id) VALUES (?)`, patientID.String()).Error
	require.NoError(t, err)
	err = db.Exec(`INSERT INTO professionals (id) VALUES (?)`, profID.String()).Error
	require.NoError(t, err)

	// Create a dummy appointment
	aptID := uuid.New()
	err = db.Exec(`INSERT INTO appointments (id, clinic_id, patient_id, professional_id, start_time, status)
		VALUES (?, ?, ?, ?, datetime('now'), 'pending')`,
		aptID.String(), clinicID.String(), patientID.String(), profID.String()).Error
	require.NoError(t, err)

	repo := NewAppointmentRepository(db)
	ctx := context.Background()

	t.Run("Successfully list appointments with all preloads", func(t *testing.T) {
		appointments, total, err := repo.List(ctx, clinicID, &model.AppointmentFilter{Page: 1, PerPage: 10})

		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, appointments, 1)
		assert.Equal(t, aptID, appointments[0].ID)
		assert.NotNil(t, appointments[0].Patient)
		assert.NotNil(t, appointments[0].Professional)
	})
}

func setupCascadeDB(t *testing.T) (*gorm.DB, uuid.UUID, uuid.UUID) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`CREATE TABLE appointments (id text PRIMARY KEY, clinic_id text, patient_id text, professional_id text, start_time datetime, status text, deleted_at datetime)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE appointment_procedures (id text PRIMARY KEY, appointment_id text, procedure_id text, price real, created_at datetime, updated_at datetime)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE appointment_payments (id text PRIMARY KEY, clinic_id text, appointment_id text, payment_method_id text, amount real, discount real, installments integer, notes text, paid_at datetime, created_at datetime, updated_at datetime, deleted_at datetime)`).Error)

	clinicID := uuid.New()
	aptID := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO appointments (id, clinic_id, start_time, status) VALUES (?, ?, datetime('now'), 'pending')`,
		aptID.String(), clinicID.String()).Error)

	require.NoError(t, db.Exec(`INSERT INTO appointment_procedures (id, appointment_id, procedure_id, price) VALUES (?, ?, ?, 100)`,
		uuid.New().String(), aptID.String(), uuid.New().String()).Error)
	require.NoError(t, db.Exec(`INSERT INTO appointment_procedures (id, appointment_id, procedure_id, price) VALUES (?, ?, ?, 50)`,
		uuid.New().String(), aptID.String(), uuid.New().String()).Error)

	require.NoError(t, db.Exec(`INSERT INTO appointment_payments (id, clinic_id, appointment_id, payment_method_id, amount, discount, paid_at) VALUES (?, ?, ?, ?, 80, 0, datetime('now'))`,
		uuid.New().String(), clinicID.String(), aptID.String(), uuid.New().String()).Error)
	require.NoError(t, db.Exec(`INSERT INTO appointment_payments (id, clinic_id, appointment_id, payment_method_id, amount, discount, paid_at) VALUES (?, ?, ?, ?, 30, 5, datetime('now'))`,
		uuid.New().String(), clinicID.String(), aptID.String(), uuid.New().String()).Error)

	return db, clinicID, aptID
}

func TestAppointmentRepository_GetDeletePreview_CountsProceduresAndPayments(t *testing.T) {
	db, clinicID, aptID := setupCascadeDB(t)
	repo := NewAppointmentRepository(db)

	preview, err := repo.GetDeletePreview(context.Background(), aptID, clinicID)

	require.NoError(t, err)
	assert.Equal(t, 2, preview.ProceduresCount)
	assert.Equal(t, 2, preview.PaymentsCount)
	assert.InDelta(t, 105.0, preview.PaymentsTotal, 0.01, "total should sum amount-discount across payments")
}

func TestAppointmentRepository_DeleteCascade_KeepsPaymentsByDefault(t *testing.T) {
	db, clinicID, aptID := setupCascadeDB(t)
	repo := NewAppointmentRepository(db)

	err := repo.DeleteCascade(context.Background(), aptID, clinicID, false)
	require.NoError(t, err)

	// Appointment is removed.
	var aptCount int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM appointments WHERE id = ?`, aptID.String()).Scan(&aptCount).Error)
	assert.Equal(t, int64(0), aptCount)

	// Procedures are removed in cascade.
	var procCount int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM appointment_procedures WHERE appointment_id = ?`, aptID.String()).Scan(&procCount).Error)
	assert.Equal(t, int64(0), procCount)

	// Payments are preserved — they are kept for financial history.
	var payCount int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM appointment_payments WHERE appointment_id = ?`, aptID.String()).Scan(&payCount).Error)
	assert.Equal(t, int64(2), payCount, "payments must not be touched when deletePayments=false")
}

func TestAppointmentRepository_DeleteCascade_DeletesPaymentsWhenRequested(t *testing.T) {
	db, clinicID, aptID := setupCascadeDB(t)
	repo := NewAppointmentRepository(db)

	err := repo.DeleteCascade(context.Background(), aptID, clinicID, true)
	require.NoError(t, err)

	// Payments are removed together with the appointment.
	var payCount int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM appointment_payments WHERE appointment_id = ?`, aptID.String()).Scan(&payCount).Error)
	assert.Equal(t, int64(0), payCount, "payments should be removed when deletePayments=true")
}

func TestAppointmentRepository_DeleteCascade_RejectsAppointmentFromAnotherClinic(t *testing.T) {
	db, _, aptID := setupCascadeDB(t)
	repo := NewAppointmentRepository(db)

	otherClinic := uuid.New()
	err := repo.DeleteCascade(context.Background(), aptID, otherClinic, true)

	require.Error(t, err, "must not allow cross-clinic deletion")
}
