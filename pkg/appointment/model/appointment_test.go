package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAppointment_NewFields_ToResponse(t *testing.T) {
	now := time.Now()
	clinicID := uuid.New()
	patientID := uuid.New()
	profID := uuid.New()

	apt := &Appointment{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		PatientID:      patientID,
		ProfessionalID: profID,
		StartTime:      now,
		EndTime:        now.Add(50 * time.Minute),
		Status:         AppointmentStatusPending,
		Type:           AppointmentTypeRegular,
		Procedure:      "Limpeza de pele",
		TreatedRegion:  "Rosto",
		Notes:          "Pele sensível",
	}

	resp := apt.ToResponse()

	assert.Equal(t, "Limpeza de pele", resp.Procedure, "Procedure deve ser propagado para response")
	assert.Equal(t, "Rosto", resp.TreatedRegion, "TreatedRegion deve ser propagado para response")
	assert.Equal(t, "Pele sensível", resp.Notes, "Notes não deve ser afetado pelos novos campos")
}

func TestAppointment_NewFields_EmptyByDefault(t *testing.T) {
	now := time.Now()
	apt := &Appointment{
		ID:             uuid.New(),
		ClinicID:       uuid.New(),
		PatientID:      uuid.New(),
		ProfessionalID: uuid.New(),
		StartTime:      now,
		EndTime:        now.Add(50 * time.Minute),
		Status:         AppointmentStatusPending,
		Type:           AppointmentTypeRegular,
	}

	resp := apt.ToResponse()

	assert.Empty(t, resp.Procedure, "Procedure deve ser vazio por padrão")
	assert.Empty(t, resp.TreatedRegion, "TreatedRegion deve ser vazio por padrão")
}
