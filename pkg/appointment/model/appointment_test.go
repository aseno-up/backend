package model

import (
	"testing"
	"time"

	procedureModel "github.com/aseno/backend/pkg/procedure/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAppointment_NewFields_ToResponse(t *testing.T) {
	now := time.Now()
	clinicID := uuid.New()
	patientID := uuid.New()
	profID := uuid.New()
	procID := uuid.New()

	apt := &Appointment{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		PatientID:      patientID,
		ProfessionalID: profID,
		StartTime:      now,
		EndTime:        now.Add(50 * time.Minute),
		Status:         AppointmentStatusPending,
		Type:           AppointmentTypeRegular,
		Procedures: []AppointmentProcedure{
			{
				ID:          uuid.New(),
				ProcedureID: procID,
				Price:       150.00,
				Procedure: &procedureModel.Procedure{
					ID:   procID,
					Name: "Limpeza de pele",
				},
			},
		},
		Notes: "Pele sensível",
		Price: 150.00,
	}

	resp := apt.ToResponse()

	assert.Equal(t, 1, len(resp.Procedures), "Procedures deve propagar N:N")
	assert.Equal(t, procID, resp.Procedures[0].ProcedureID)
	assert.Equal(t, 150.00, resp.Procedures[0].Price)
	assert.NotNil(t, resp.Procedures[0].Procedure)
	assert.Equal(t, "Limpeza de pele", resp.Procedures[0].Procedure.Name)
	assert.Equal(t, "Pele sensível", resp.Notes)
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

	assert.Empty(t, resp.Procedures, "Procedures deve ser vazio por padrão")
	assert.Equal(t, float64(0), resp.Price)
}
