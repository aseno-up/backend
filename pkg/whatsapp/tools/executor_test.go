package tools

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPatientNameUpdater struct {
	calledWith struct {
		clinicID  uuid.UUID
		patientID uuid.UUID
		name      string
	}
	err error
}

func (m *mockPatientNameUpdater) UpdateName(ctx context.Context, clinicID, patientID uuid.UUID, newName string) error {
	m.calledWith.clinicID = clinicID
	m.calledWith.patientID = patientID
	m.calledWith.name = newName
	return m.err
}

func TestUpdatePatientName_Registered(t *testing.T) {
	registry := NewRegistry()
	tool, exists := registry.GetTool("update_patient_name")
	require.True(t, exists, "update_patient_name tool should be registered")
	assert.NotEmpty(t, tool.Description)
}

func TestUpdatePatientName_RequiresNewName(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	patientID := uuid.New()
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		PatientID:          &patientID,
		PatientNameUpdater: &mockPatientNameUpdater{},
	}

	// Missing new_name should fail
	_, err := executor.Execute(execCtx, "update_patient_name", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "new_name is required")
}

func TestUpdatePatientName_RequiresPatientID(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		PatientID:          nil, // no patient ID
		PatientNameUpdater: &mockPatientNameUpdater{},
	}

	_, err := executor.Execute(execCtx, "update_patient_name", map[string]interface{}{
		"new_name": "Renato",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "patient not identified")
}

func TestUpdatePatientName_Success(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	mock := &mockPatientNameUpdater{}
	patientID := uuid.New()
	clinicID := uuid.New()

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           clinicID,
		PatientID:          &patientID,
		PatientName:        "Hamilton",
		PatientNameUpdater: mock,
	}

	result, err := executor.Execute(execCtx, "update_patient_name", map[string]interface{}{
		"new_name": "Renato",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, resultMap["success"])
	assert.Equal(t, "Hamilton", resultMap["old_name"])
	assert.Equal(t, "Renato", resultMap["new_name"])

	// Verify the mock was called correctly
	assert.Equal(t, clinicID, mock.calledWith.clinicID)
	assert.Equal(t, patientID, mock.calledWith.patientID)
	assert.Equal(t, "Renato", mock.calledWith.name)
}
