package tools

import (
	"context"
	"testing"
	"time"

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

type stubSpecialtyLister struct {
	result []*SpecialtyRef
	err    error
}

func (s *stubSpecialtyLister) ListSpecialties(_ context.Context, _ uuid.UUID) ([]*SpecialtyRef, error) {
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
					Monday:  []TimeSlot{{Start: "08:00", End: "14:10"}},
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

// --- list_specialties tool ---

func TestListSpecialties_Registered(t *testing.T) {
	registry := NewRegistry()
	tool, exists := registry.GetTool("list_specialties")
	require.True(t, exists, "list_specialties tool should be registered")
	assert.NotEmpty(t, tool.Description)
}

func TestListSpecialties_ReturnsIDAndName(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	fisioID := uuid.New()
	nutriID := uuid.New()
	lister := &stubSpecialtyLister{
		result: []*SpecialtyRef{
			{ID: fisioID, Name: "Fisioterapeuta"},
			{ID: nutriID, Name: "Nutricionista"},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		SpecialtyLister: lister,
	}

	result, err := executor.Execute(execCtx, "list_specialties", map[string]interface{}{})
	require.NoError(t, err)

	list, ok := result.([]*SpecialtyRef)
	require.True(t, ok)
	require.Len(t, list, 2)
	assert.Equal(t, "Fisioterapeuta", list[0].Name)
	assert.Equal(t, fisioID, list[0].ID)
}

func TestListSpecialties_NotConfigured_ReturnsError(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)

	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		SpecialtyLister: nil,
	}

	_, err := executor.Execute(execCtx, "list_specialties", map[string]interface{}{})
	assert.Error(t, err)
}

// --- Regression: specialty filter must be exact case-insensitive only (no substring) ---

func TestGetAvailableProfessionals_FilterIsCaseInsensitiveExact(t *testing.T) {
	// The adapter (main.go) is what filters; here we verify the executor passes
	// the raw specialty through so the adapter can apply exact matching.
	registry := NewRegistry()
	executor := NewExecutor(registry)

	lister := &stubProfessionalLister{result: []*ProfessionalInfo{}}
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
	}

	_, err := executor.Execute(execCtx, "get_available_professionals", map[string]interface{}{
		"specialty": "Fisioterapeuta",
	})
	require.NoError(t, err)
	assert.Equal(t, "Fisioterapeuta", lister.called.specialty)
}

// slot builds an AvailableSlot at the given hour on a fixed reference day.
func slot(hour int) *AvailableSlot {
	return &AvailableSlot{
		StartTime: time.Date(2026, 4, 20, hour, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 4, 20, hour+1, 0, 0, 0, time.UTC),
	}
}

func TestSampleSlotsAcrossDay_ReturnsAllWhenUnderLimit(t *testing.T) {
	in := []*AvailableSlot{slot(9), slot(10), slot(11)}
	out := sampleSlotsAcrossDay(in, 5)
	assert.Equal(t, in, out, "when slots <= max, returns all")
}

func TestSampleSlotsAcrossDay_SpansFullRange(t *testing.T) {
	// Full-day schedule 9-17 with 1h slots = 9 slots. Regression test: the
	// previous implementations either cut at slot 13:00 (early afternoon)
	// or interleaved but stopped before 16h, making that slot invisible.
	// The new algorithm MUST include the first and last slot.
	in := []*AvailableSlot{slot(9), slot(10), slot(11), slot(12), slot(13), slot(14), slot(15), slot(16), slot(17)}
	out := sampleSlotsAcrossDay(in, 5)

	require.Len(t, out, 5)
	assert.Equal(t, 9, out[0].StartTime.Hour(), "first slot of the day must be included")
	assert.Equal(t, 17, out[len(out)-1].StartTime.Hour(), "last slot of the day must be included")
}

func TestSampleSlotsAcrossDay_IncludesLateAfternoon(t *testing.T) {
	in := []*AvailableSlot{slot(9), slot(10), slot(11), slot(12), slot(13), slot(14), slot(15), slot(16), slot(17)}
	out := sampleSlotsAcrossDay(in, 5)

	// Find any slot at or after 15h. This fails on the old interleave-
	// truncated-at-5 behavior.
	hasLateAfternoon := false
	for _, s := range out {
		if s.StartTime.Hour() >= 15 {
			hasLateAfternoon = true
			break
		}
	}
	assert.True(t, hasLateAfternoon, "sample must include at least one late-afternoon slot (>=15h)")
}

func TestSampleSlotsAcrossDay_MorningOnlyStillWorks(t *testing.T) {
	in := []*AvailableSlot{slot(8), slot(9), slot(10), slot(11)}
	out := sampleSlotsAcrossDay(in, 3)
	require.Len(t, out, 3)
	for _, s := range out {
		assert.Less(t, s.StartTime.Hour(), 12, "all slots in this test are morning")
	}
}

func TestSampleSlotsAcrossDay_EmptyInput(t *testing.T) {
	out := sampleSlotsAcrossDay(nil, 5)
	assert.Empty(t, out)
}

// ---------------------------------------------------------------------------
// quick_schedule tests
// ---------------------------------------------------------------------------

type stubSlotFinder struct {
	slots map[uuid.UUID][]*AvailableSlot
}

func (s *stubSlotFinder) GetAvailableSlots(_ context.Context, _ uuid.UUID, profID uuid.UUID, _ time.Time, _ int) ([]*AvailableSlot, error) {
	return s.slots[profID], nil
}

func (s *stubSlotFinder) GetAvailableSlotsMultipleProfessionals(_ context.Context, _ uuid.UUID, _ []uuid.UUID, _ time.Time, _ int, _ int, _ int) ([]*AvailableSlot, error) {
	return nil, nil
}

type stubAppointmentCreator struct {
	called bool
	result *AppointmentInfo
}

func (s *stubAppointmentCreator) CreateFromSchedulingIntent(_ context.Context, _, _, _ uuid.UUID, _ time.Time, _ int) (*AppointmentInfo, error) {
	s.called = true
	return s.result, nil
}

type stubPatientFinder struct{}

func (s *stubPatientFinder) FindOrCreateByPhone(_ context.Context, _ uuid.UUID, _, _ string) (*PatientInfo, error) {
	return &PatientInfo{ID: uuid.New()}, nil
}

func slotAt(profID uuid.UUID, y, m, d, h int) *AvailableSlot {
	return &AvailableSlot{
		ProfessionalID:   profID,
		ProfessionalName: "Test",
		StartTime:        time.Date(y, time.Month(m), d, h, 0, 0, 0, time.UTC),
		EndTime:          time.Date(y, time.Month(m), d, h+1, 0, 0, 0, time.UTC),
	}
}

func TestQuickSchedule_ExactMatch_CreatesAppointment(t *testing.T) {
	profID := uuid.New()
	clinicID := uuid.New()
	patientID := uuid.New()

	lister := &stubProfessionalLister{result: []*ProfessionalInfo{
		{ID: profID, Name: "José Hamilton Santos Júnior", Specialty: "Geral"},
	}}
	slotFinder := &stubSlotFinder{slots: map[uuid.UUID][]*AvailableSlot{
		profID: {
			slotAt(profID, 2026, 4, 20, 15),
			slotAt(profID, 2026, 4, 20, 16),
			slotAt(profID, 2026, 4, 20, 17),
		},
	}}
	creator := &stubAppointmentCreator{result: &AppointmentInfo{ID: uuid.New()}}

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           clinicID,
		PatientID:          &patientID,
		ProfessionalLister: lister,
		SlotFinder:         slotFinder,
		AppointmentCreator: creator,
	}

	out, err := NewExecutor(NewRegistry()).Execute(execCtx, "quick_schedule", map[string]interface{}{
		"professional_name": "hamilton",
		"date":              "2026-04-20",
		"start_time":        "16:00",
	})
	require.NoError(t, err)
	m, _ := out.(map[string]interface{})
	assert.Equal(t, "scheduled", m["status"], "single name match + slot available must auto-create")
	assert.True(t, creator.called, "AppointmentCreator must be invoked")
}

func TestQuickSchedule_AmbiguousName_AsksToChoose(t *testing.T) {
	p1, p2 := uuid.New(), uuid.New()
	lister := &stubProfessionalLister{result: []*ProfessionalInfo{
		{ID: p1, Name: "Hamilton Silva"},
		{ID: p2, Name: "Hamilton Souza"},
	}}
	creator := &stubAppointmentCreator{}

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
		SlotFinder:         &stubSlotFinder{},
		AppointmentCreator: creator,
	}

	out, _ := NewExecutor(NewRegistry()).Execute(execCtx, "quick_schedule", map[string]interface{}{
		"professional_name": "hamilton",
		"date":              "2026-04-20",
		"start_time":        "16:00",
	})
	m, _ := out.(map[string]interface{})
	assert.Equal(t, "ambiguous_professional", m["status"])
	assert.False(t, creator.called, "must NOT create when multiple matches exist")
	cands := m["candidates"].([]map[string]string)
	assert.Len(t, cands, 2)
}

func TestQuickSchedule_NotFound_ReturnsAvailableList(t *testing.T) {
	lister := &stubProfessionalLister{result: []*ProfessionalInfo{
		{ID: uuid.New(), Name: "Carla Laine"},
	}}
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
		SlotFinder:         &stubSlotFinder{},
		AppointmentCreator: &stubAppointmentCreator{},
	}
	out, _ := NewExecutor(NewRegistry()).Execute(execCtx, "quick_schedule", map[string]interface{}{
		"professional_name": "hamilton",
		"date":              "2026-04-20",
		"start_time":        "16:00",
	})
	m, _ := out.(map[string]interface{})
	assert.Equal(t, "professional_not_found", m["status"])
}

func TestQuickSchedule_SlotTaken_ReturnsAlternatives(t *testing.T) {
	profID := uuid.New()
	patientID := uuid.New()
	lister := &stubProfessionalLister{result: []*ProfessionalInfo{
		{ID: profID, Name: "Dr. Hamilton"},
	}}
	slotFinder := &stubSlotFinder{slots: map[uuid.UUID][]*AvailableSlot{
		profID: {
			slotAt(profID, 2026, 4, 20, 9),
			slotAt(profID, 2026, 4, 20, 10),
		},
	}}
	creator := &stubAppointmentCreator{}

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		PatientID:          &patientID,
		ProfessionalLister: lister,
		SlotFinder:         slotFinder,
		AppointmentCreator: creator,
	}

	out, _ := NewExecutor(NewRegistry()).Execute(execCtx, "quick_schedule", map[string]interface{}{
		"professional_name": "hamilton",
		"date":              "2026-04-20",
		"start_time":        "16:00", // not in the list
	})
	m, _ := out.(map[string]interface{})
	assert.Equal(t, "not_available", m["status"])
	assert.False(t, creator.called)
	alt := m["available_same_day"].([]map[string]string)
	assert.Len(t, alt, 2, "must surface same-day alternatives so LLM can offer them")
}
