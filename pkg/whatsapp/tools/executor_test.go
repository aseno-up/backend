package tools

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mocks for the new tools ---

type stubProfessionalLister struct {
	result []*ProfessionalInfo
	err    error
	called struct {
		specialty string
	}
}

func (s *stubProfessionalLister) ListAvailable(_ context.Context, _ uuid.UUID, specialty string) ([]*ProfessionalInfo, error) {
	s.called.specialty = specialty
	return s.result, s.err
}

type stubProcedureLister struct {
	result []*ProcedureInfo
	err    error
	called struct {
		specialty string
	}
}

func (s *stubProcedureLister) ListProcedures(_ context.Context, _ uuid.UUID, specialty string) ([]*ProcedureInfo, error) {
	s.called.specialty = specialty
	return s.result, s.err
}

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

// --- get_available_professionals with enriched payload ---

func TestGetAvailableProfessionals_ReturnsSpecialtiesArrayAndWorkingHours(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	thaianeID := uuid.New()
	fisio := uuid.New()
	pilates := uuid.New()

	lister := &stubProfessionalLister{
		result: []*ProfessionalInfo{
			{
				ID:   thaianeID,
				Name: "Thaiane Simões",
				Specialties: []SpecialtyRef{
					{ID: fisio, Name: "Fisioterapia"},
					{ID: pilates, Name: "Pilates"},
				},
				WorkingHours: &WorkingHours{
					Monday: []TimeSlot{{Start: "08:00", End: "14:10"}},
					Tuesday: []TimeSlot{{Start: "09:00", End: "18:00"}},
				},
				IsAvailable: true,
			},
		},
	}

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
	}

	result, err := executor.Execute(execCtx, "get_available_professionals", map[string]interface{}{})
	require.NoError(t, err)

	list, ok := result.([]*ProfessionalInfo)
	require.True(t, ok, "expected []*ProfessionalInfo, got %T", result)
	require.Len(t, list, 1)

	p := list[0]
	assert.Equal(t, "Thaiane Simões", p.Name)
	assert.Len(t, p.Specialties, 2)
	assert.Equal(t, "Fisioterapia", p.Specialties[0].Name)
	require.NotNil(t, p.WorkingHours)
	assert.Equal(t, "08:00", p.WorkingHours.Monday[0].Start)
	assert.Equal(t, "14:10", p.WorkingHours.Monday[0].End)
}

func TestGetAvailableProfessionals_PassesSpecialtyFilter(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	lister := &stubProfessionalLister{result: []*ProfessionalInfo{}}
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
	}

	_, err := executor.Execute(execCtx, "get_available_professionals", map[string]interface{}{
		"specialty": "fisioterapia",
	})
	require.NoError(t, err)
	assert.Equal(t, "fisioterapia", lister.called.specialty)
}

// --- list_procedures tool ---

func TestListProcedures_Registered(t *testing.T) {
	registry := NewRegistry()
	tool, exists := registry.GetTool("list_procedures")
	require.True(t, exists, "list_procedures tool should be registered")
	assert.NotEmpty(t, tool.Description)
}

func TestListProcedures_ReturnsNamePriceDurationSpecialty(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	lister := &stubProcedureLister{
		result: []*ProcedureInfo{
			{ID: uuid.New(), Name: "Liberação Miofascial", Price: 150.00, DurationMinutes: 60, SpecialtyName: "Fisioterapia"},
			{ID: uuid.New(), Name: "Massoterapia", Price: 190.00, DurationMinutes: 50, SpecialtyName: "Fisioterapia"},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		ProcedureLister: lister,
	}

	result, err := executor.Execute(execCtx, "list_procedures", map[string]interface{}{})
	require.NoError(t, err)

	list, ok := result.([]*ProcedureInfo)
	require.True(t, ok)
	require.Len(t, list, 2)
	assert.Equal(t, "Liberação Miofascial", list[0].Name)
	assert.Equal(t, 150.00, list[0].Price)
	assert.Equal(t, 60, list[0].DurationMinutes)
	assert.Equal(t, "Fisioterapia", list[0].SpecialtyName)
}

func TestListProcedures_PassesSpecialtyFilter(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	lister := &stubProcedureLister{result: []*ProcedureInfo{}}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		ProcedureLister: lister,
	}

	_, err := executor.Execute(execCtx, "list_procedures", map[string]interface{}{
		"specialty": "Pilates",
	})
	require.NoError(t, err)
	assert.Equal(t, "Pilates", lister.called.specialty)
}

func TestListProcedures_NotConfigured_ReturnsError(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		ProcedureLister: nil,
	}

	_, err := executor.Execute(execCtx, "list_procedures", map[string]interface{}{})
	assert.Error(t, err)
}
