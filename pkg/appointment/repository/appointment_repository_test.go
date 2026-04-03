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
