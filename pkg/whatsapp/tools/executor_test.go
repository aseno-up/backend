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
		specialty   string
		procedureID *uuid.UUID
	}
}

func (s *stubProfessionalLister) ListAvailable(_ context.Context, _ uuid.UUID, specialty string, procedureID *uuid.UUID) ([]*ProfessionalInfo, error) {
	s.called.specialty = specialty
	s.called.procedureID = procedureID
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

// Real-world bug: AI listed 14 slots in one message, overwhelming the
// patient. The cap is enforced server-side (executor) — prompt rules alone
// were unreliable. Per-professional path caps at 4 distributed picks.

func TestGetAvailableSlots_PerProfessional_CapsAtFour(t *testing.T) {
	profID := uuid.New()

	// 15 slots in one day — the kind of agenda that triggered the bug.
	manySlots := []*AvailableSlot{
		slotAt(profID, 2026, 4, 28, 8),
		slotAt(profID, 2026, 4, 28, 9),
		slotAt(profID, 2026, 4, 28, 10),
		slotAt(profID, 2026, 4, 28, 11),
		slotAt(profID, 2026, 4, 28, 12),
		slotAt(profID, 2026, 4, 28, 13),
		slotAt(profID, 2026, 4, 28, 14),
		slotAt(profID, 2026, 4, 28, 15),
		slotAt(profID, 2026, 4, 28, 16),
		slotAt(profID, 2026, 4, 28, 17),
		slotAt(profID, 2026, 4, 29, 9),
		slotAt(profID, 2026, 4, 29, 11),
		slotAt(profID, 2026, 4, 29, 14),
		slotAt(profID, 2026, 4, 29, 16),
		slotAt(profID, 2026, 4, 30, 10),
	}

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: &stubProfessionalLister{result: []*ProfessionalInfo{{ID: profID, Name: "Hamilton"}}},
		SlotFinder:         &stubSlotFinder{slots: map[uuid.UUID][]*AvailableSlot{profID: manySlots}},
	}

	out, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_slots", map[string]interface{}{
		"professional_id": profID.String(),
	})
	require.NoError(t, err)
	slots := out.([]*AvailableSlot)
	assert.LessOrEqual(t, len(slots), 4, "tool MUST cap output at 4 slots regardless of how many the agenda has — prompt rules alone are unreliable")
}

// time_filter — surgical check for a specific HH:MM the patient asked
// about. Solves the "11h não aparece na amostra de 4" bug.

func TestGetAvailableSlots_TimeFilter_FindsExactMatch(t *testing.T) {
	profID := uuid.New()
	// Hamilton tem slots de 30min: 10:00, 11:00, 12:00, 15:00, 16:00 livres.
	// Sample distribuído pegou 10:00, 13:00*, 17:00 (mas 13:00 não existe).
	// Patient asks "tem 11h?" — tool must surface the actual 11:00 slot,
	// even though it was cut from the sample.
	slots := []*AvailableSlot{
		slotInBRT(profID, 2026, 4, 28, 10, 0),
		slotInBRT(profID, 2026, 4, 28, 11, 0),
		slotInBRT(profID, 2026, 4, 28, 12, 0),
		slotInBRT(profID, 2026, 4, 28, 15, 0),
		slotInBRT(profID, 2026, 4, 28, 16, 0),
	}

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: &stubProfessionalLister{result: []*ProfessionalInfo{{ID: profID, Name: "Hamilton"}}},
		SlotFinder:         &stubSlotFinder{slots: map[uuid.UUID][]*AvailableSlot{profID: slots}},
	}

	out, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_slots", map[string]interface{}{
		"professional_id": profID.String(),
		"date":            "2026-04-28",
		"time_filter":     "11:00",
	})
	require.NoError(t, err)
	got := out.([]*AvailableSlot)
	require.Len(t, got, 1, "time_filter must return the exact slot when free, ignoring the distributed-sample cap")
	assert.Equal(t, 11, got[0].StartTime.In(brtLoc()).Hour())
}

func TestGetAvailableSlots_TimeFilter_EmptyWhenOccupied(t *testing.T) {
	// 11:00 was occupied → no slot in backend → tool returns empty.
	profID := uuid.New()
	slots := []*AvailableSlot{
		slotInBRT(profID, 2026, 4, 28, 10, 0),
		slotInBRT(profID, 2026, 4, 28, 12, 0), // 11:00 missing = ocupado
		slotInBRT(profID, 2026, 4, 28, 15, 0),
	}
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: &stubProfessionalLister{result: []*ProfessionalInfo{{ID: profID, Name: "Hamilton"}}},
		SlotFinder:         &stubSlotFinder{slots: map[uuid.UUID][]*AvailableSlot{profID: slots}},
	}
	out, _ := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_slots", map[string]interface{}{
		"professional_id": profID.String(),
		"date":            "2026-04-28",
		"time_filter":     "11:00",
	})
	got := out.([]*AvailableSlot)
	assert.Empty(t, got, "time_filter must return empty when the requested HH:MM is not in the backend's free slots")
}

func TestGetAvailableSlots_TimeFilter_InvalidFormatErrors(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: &stubProfessionalLister{result: []*ProfessionalInfo{{ID: uuid.New(), Name: "x"}}},
		SlotFinder:         &stubSlotFinder{},
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_slots", map[string]interface{}{
		"professional_id": uuid.New().String(),
		"time_filter":     "abc",
	})
	assert.Error(t, err, "invalid time_filter must surface as error so the LLM corrects it")
}

func brtLoc() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.Local
	}
	return loc
}

func slotInBRT(profID uuid.UUID, y, m, d, h, min int) *AvailableSlot {
	loc := brtLoc()
	start := time.Date(y, time.Month(m), d, h, min, 0, 0, loc)
	return &AvailableSlot{
		ProfessionalID:   profID,
		ProfessionalName: "Test",
		StartTime:        start,
		EndTime:          start.Add(30 * time.Minute),
	}
}

// procedure_id filter on get_available_slots — prevents the AI from
// surfacing slots of professionals who don't perform the procedure.
// Real-world bug: AI listed Mariana for "RPG — Avaliação" but only Hamilton
// has that association in professional_procedures.

func TestGetAvailableSlots_ProcedureFilter_ExcludesUnqualifiedProfessional(t *testing.T) {
	hamilton := uuid.New() // qualified
	mariana := uuid.New()  // NOT qualified
	procID := uuid.New()   // RPG — Avaliação

	// ProfessionalLister returns ONLY hamilton when filtering by procedure_id.
	lister := &stubProfessionalLister{
		result: []*ProfessionalInfo{
			{ID: hamilton, Name: "Hamilton"},
		},
	}

	// SlotFinder returns slots for both professionals (without filter).
	hamiltonSlot := slotInBRT(hamilton, 2026, 4, 28, 11, 0)
	marianaSlot := slotInBRT(mariana, 2026, 4, 28, 11, 0)

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
		SlotFinder: &stubSlotFinder{
			slots: map[uuid.UUID][]*AvailableSlot{
				hamilton: {hamiltonSlot},
				mariana:  {marianaSlot},
			},
		},
	}

	// Patient asks for 11h amanhã, AI passes procedure_id of RPG — Avaliação.
	out, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_slots", map[string]interface{}{
		"professional_id": hamilton.String(),
		"date":            "2026-04-28",
		"time_filter":     "11:00",
		"procedure_id":    procID.String(),
	})
	require.NoError(t, err)
	got := out.([]*AvailableSlot)
	require.Len(t, got, 1, "must return only the qualified professional's slot")
	assert.Equal(t, hamilton, got[0].ProfessionalID)
}

func TestGetAvailableSlots_ProcedureFilter_UnqualifiedProfessionalReturnsEmpty(t *testing.T) {
	mariana := uuid.New()
	procID := uuid.New()
	// ProfessionalLister returns NOBODY for this procedure (because Mariana
	// isn't on the list of qualified professionals for the procedure).
	lister := &stubProfessionalLister{result: []*ProfessionalInfo{}}

	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
		SlotFinder: &stubSlotFinder{
			slots: map[uuid.UUID][]*AvailableSlot{
				mariana: {slotInBRT(mariana, 2026, 4, 28, 11, 0)},
			},
		},
	}

	out, _ := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_slots", map[string]interface{}{
		"professional_id": mariana.String(),
		"procedure_id":    procID.String(),
	})
	got := out.([]*AvailableSlot)
	assert.Empty(t, got, "unqualified professional must produce empty result regardless of slots")
}

func TestGetAvailableSlots_ProcedureFilter_InvalidUUIDErrors(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: &stubProfessionalLister{},
		SlotFinder:         &stubSlotFinder{},
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_slots", map[string]interface{}{
		"procedure_id": "not-a-uuid",
	})
	assert.Error(t, err, "invalid procedure_id must surface as error so the LLM corrects")
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

// ---------------------------------------------------------------------------
// complete_handoff_to_human — sole tool exposed in handoff mode. Marks the
// existing per-patient human takeover so the takeover gate at the WhatsApp
// service layer silences subsequent patient messages without further LLM
// calls. Returns the clinic-configured canned reply as the patient-facing
// message.
// ---------------------------------------------------------------------------

type stubHumanTakeoverMarker struct {
	calls    int
	clinicID uuid.UUID
	phone    string
	at       time.Time
	err      error
}

func (s *stubHumanTakeoverMarker) Mark(_ context.Context, clinicID uuid.UUID, phone string, at time.Time) error {
	s.calls++
	s.clinicID = clinicID
	s.phone = phone
	s.at = at
	return s.err
}

func TestCompleteHandoff_Registered(t *testing.T) {
	registry := NewRegistry()
	tool, exists := registry.GetTool("complete_handoff_to_human")
	require.True(t, exists, "complete_handoff_to_human must be registered so the LLM can see it in handoff mode")
	assert.NotEmpty(t, tool.Description)
}

func TestCompleteHandoff_MarksTakeoverWithExecutionContextValues(t *testing.T) {
	marker := &stubHumanTakeoverMarker{}
	clinicID := uuid.New()
	phone := "5551999990001"

	execCtx := &ExecutionContext{
		Context:             context.Background(),
		ClinicID:            clinicID,
		PatientPhone:        phone,
		HumanTakeoverMarker: marker,
		BlockedIntentReply:  "Pode aguardar um minutinho? Alguém do nosso time vai te atender em breve.",
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "complete_handoff_to_human", map[string]interface{}{})
	require.NoError(t, err)

	assert.Equal(t, 1, marker.calls, "must call Mark exactly once per invocation")
	assert.Equal(t, clinicID, marker.clinicID, "Mark must receive the clinic ID from ExecutionContext (multi-tenant)")
	assert.Equal(t, phone, marker.phone, "Mark must receive the patient phone from ExecutionContext")
	assert.WithinDuration(t, time.Now(), marker.at, 2*time.Second, "Mark must receive a timestamp around now()")
}

// TestCompleteHandoff_ReturnsNoPatientFacingMessage pins the user-facing
// contract: handoff mode is silent — no canned reply is sent to the patient.
// The result carries only the status; the agent service uses that to
// suppress the AI's text reply. (Until 2026-05 the result also carried a
// "message" field with a clinic-configured canned line; we removed it
// because dropping a robotic "aguarde, alguém te atende" mid-conversation
// looked tacky and broke flow.)
func TestCompleteHandoff_ReturnsNoPatientFacingMessage(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:             context.Background(),
		ClinicID:            uuid.New(),
		PatientPhone:        "5551999990001",
		HumanTakeoverMarker: &stubHumanTakeoverMarker{},
		BlockedIntentReply:  "this should NOT appear in the result",
	}

	result, err := NewExecutor(NewRegistry()).Execute(execCtx, "complete_handoff_to_human", map[string]interface{}{})
	require.NoError(t, err)

	m, ok := result.(map[string]interface{})
	require.True(t, ok, "result must be a structured map so the LLM can act on it deterministically")
	assert.Equal(t, "handoff_completed", m["status"])
	assert.NotContains(t, m, "message", "handoff result must NOT carry a patient-facing message — the AI's reply is suppressed entirely")
}

func TestCompleteHandoff_FailsLoudlyWhenMarkerNotConfigured(t *testing.T) {
	// Defensive: if main.go forgets to wire the marker, the tool must not
	// silently succeed (which would leave the team blind to subsequent
	// patient messages because the takeover gate would never fire).
	execCtx := &ExecutionContext{
		Context:             context.Background(),
		ClinicID:            uuid.New(),
		PatientPhone:        "5551999990001",
		HumanTakeoverMarker: nil,
		BlockedIntentReply:  "aguarde",
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "complete_handoff_to_human", map[string]interface{}{})
	assert.Error(t, err, "missing marker must surface as an error so wiring bugs are caught early")
}

// TestCompleteHandoff_SilentMode_DoesNotMarkTakeover covers the "AI decides
// not to respond" use case: when the LLM judges that replying would confuse
// the patient (e.g. patient sends "sim" with no AI question to answer), it
// invokes the tool with mode="silent". This must NOT mark the takeover
// (otherwise we'd silence the patient for hours over a one-off message) —
// just signal that this single turn produces no patient-facing reply.
func TestCompleteHandoff_SilentMode_DoesNotMarkTakeover(t *testing.T) {
	marker := &stubHumanTakeoverMarker{}
	execCtx := &ExecutionContext{
		Context:             context.Background(),
		ClinicID:            uuid.New(),
		PatientPhone:        "5551999990001",
		HumanTakeoverMarker: marker,
		BlockedIntentReply:  "aguarde",
	}

	result, err := NewExecutor(NewRegistry()).Execute(execCtx, "complete_handoff_to_human", map[string]interface{}{
		"mode": "silent",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, marker.calls, "silent mode must not mark the human takeover — that would pause the patient for hours")

	m, ok := result.(map[string]interface{})
	require.True(t, ok, "result must be a structured map so the agent service can detect silent mode")
	assert.Equal(t, "silenced", m["status"], "silent mode must return status=silenced so downstream skips the patient-facing send")
	assert.NotContains(t, m, "message", "silent mode has no patient-facing message — there's nothing to send")
}

// TestCompleteHandoff_DefaultMode_StillMarksTakeover guards the existing
// behavior: the tool predates the mode parameter, so calls without a mode
// argument must keep working as before (mark takeover, return canned reply).
// Without this regression test, every existing eval that exercises handoff
// would silently flip semantics.
func TestCompleteHandoff_DefaultMode_StillMarksTakeover(t *testing.T) {
	marker := &stubHumanTakeoverMarker{}
	execCtx := &ExecutionContext{
		Context:             context.Background(),
		ClinicID:            uuid.New(),
		PatientPhone:        "5551999990001",
		HumanTakeoverMarker: marker,
		BlockedIntentReply:  "ignored",
	}

	result, err := NewExecutor(NewRegistry()).Execute(execCtx, "complete_handoff_to_human", map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, 1, marker.calls, "default mode must keep marking takeover")

	m, _ := result.(map[string]interface{})
	assert.Equal(t, "handoff_completed", m["status"])
	assert.NotContains(t, m, "message", "handoff result must NOT carry a patient-facing message — reply is suppressed")
}

// TestCompleteHandoff_ExplicitHandoffMode_MarksTakeover verifies the
// explicit "handoff" mode value behaves identically to the legacy
// no-argument call — important because the LLM may start passing
// mode="handoff" once it's aware of the parameter.
func TestCompleteHandoff_ExplicitHandoffMode_MarksTakeover(t *testing.T) {
	marker := &stubHumanTakeoverMarker{}
	execCtx := &ExecutionContext{
		Context:             context.Background(),
		ClinicID:            uuid.New(),
		PatientPhone:        "5551999990001",
		HumanTakeoverMarker: marker,
		BlockedIntentReply:  "aguarde",
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "complete_handoff_to_human", map[string]interface{}{
		"mode": "handoff",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, marker.calls, "explicit mode=handoff must keep marking takeover")
}

// TestCompleteHandoff_SilentMode_TolerantWhenMarkerNotConfigured: silent does
// not need the marker (it never calls Mark), so a nil marker must NOT be a
// hard failure here. This decouples the silent path from the wiring check
// that protects the handoff path.
func TestCompleteHandoff_SilentMode_TolerantWhenMarkerNotConfigured(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:             context.Background(),
		ClinicID:            uuid.New(),
		PatientPhone:        "5551999990001",
		HumanTakeoverMarker: nil,
		BlockedIntentReply:  "",
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "complete_handoff_to_human", map[string]interface{}{
		"mode": "silent",
	})
	assert.NoError(t, err, "silent mode should not require the marker — it never marks anything")
}

// list_procedures with name_contains filter — robust path for "vocês têm X?"
// questions where X could be a procedure name in any specialty. Solves the
// real-world bug where the LLM filtered by specialty="RPG" and missed
// "RPG — Avaliação" because that procedure was tagged under Fisioterapia.

func TestListProcedures_NameContainsFilter_CaseInsensitiveSubstring(t *testing.T) {
	clinicID := uuid.New()
	rpgID := uuid.New()
	rpgAvalID := uuid.New()
	pilatesID := uuid.New()

	lister := &stubProcedureLister{
		result: []*ProcedureInfo{
			{ID: rpgID, Name: "RPG", Price: 200, DurationMinutes: 60, SpecialtyName: "RPG"},
			{ID: rpgAvalID, Name: "RPG — Avaliação", Price: 80, DurationMinutes: 40, SpecialtyName: "Fisioterapia"},
			{ID: pilatesID, Name: "Pilates Solo", Price: 130, DurationMinutes: 50, SpecialtyName: "Pilates"},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        clinicID,
		ProcedureLister: lister,
	}

	result, err := NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{
		"name_contains": "rpg",
	})
	require.NoError(t, err)

	procs, ok := result.([]*ProcedureInfo)
	require.True(t, ok)
	assert.Len(t, procs, 2, "must return BOTH RPG procedures regardless of their specialty (RPG specialty AND Fisioterapia specialty)")

	names := []string{procs[0].Name, procs[1].Name}
	assert.Contains(t, names, "RPG")
	assert.Contains(t, names, "RPG — Avaliação", "RPG — Avaliação must be included even though its specialty is Fisioterapia, not RPG")
}

func TestListProcedures_NameContainsFilter_MatchesAvaliacaoAcrossSpecialties(t *testing.T) {
	lister := &stubProcedureLister{
		result: []*ProcedureInfo{
			{ID: uuid.New(), Name: "Avaliação Fisioterapia", Price: 180, DurationMinutes: 60, SpecialtyName: "Fisioterapia"},
			{ID: uuid.New(), Name: "Avaliação Pélvica", Price: 100, DurationMinutes: 50, SpecialtyName: "Fisioterapia"},
			{ID: uuid.New(), Name: "Avaliação Pilates", Price: 40, DurationMinutes: 30, SpecialtyName: "Pilates"},
			{ID: uuid.New(), Name: "RPG — Avaliação", Price: 80, DurationMinutes: 40, SpecialtyName: "Fisioterapia"},
			{ID: uuid.New(), Name: "RPG", Price: 200, DurationMinutes: 60, SpecialtyName: "RPG"},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		ProcedureLister: lister,
	}

	result, _ := NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{
		"name_contains": "Avaliação",
	})
	procs := result.([]*ProcedureInfo)
	assert.Len(t, procs, 4, "must return all 4 procedures whose name contains 'Avaliação' — including RPG — Avaliação which is in the Fisioterapia specialty")
}

func TestListProcedures_NameContainsFilter_AccentInsensitive(t *testing.T) {
	// Patient writes the term without accent (very common on phone keyboards).
	// Procedure name has accents in DB. Without normalization the filter
	// silently misses everything — bug latent waiting to happen.
	lister := &stubProcedureLister{
		result: []*ProcedureInfo{
			{ID: uuid.New(), Name: "Avaliação Fisioterapia", Price: 180, DurationMinutes: 60},
			{ID: uuid.New(), Name: "Reeducação Postural Global", Price: 200, DurationMinutes: 60},
			{ID: uuid.New(), Name: "Pilates", Price: 130, DurationMinutes: 50},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		ProcedureLister: lister,
	}

	// Patient typed "avaliacao" without ç/cedilla
	result, _ := NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{
		"name_contains": "avaliacao",
	})
	procs := result.([]*ProcedureInfo)
	require.Len(t, procs, 1, "must match 'Avaliação Fisioterapia' even without diacritic on the input term")
	assert.Equal(t, "Avaliação Fisioterapia", procs[0].Name)

	// Patient typed "REEDUCACAO" all caps no accent
	result, _ = NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{
		"name_contains": "REEDUCACAO",
	})
	procs = result.([]*ProcedureInfo)
	require.Len(t, procs, 1)
	assert.Equal(t, "Reeducação Postural Global", procs[0].Name)
}

func TestListProcedures_NameContainsFilter_AcceptsArrayOfTerms(t *testing.T) {
	// LLM passes both abbreviation and full name to cover patient phrasing
	// in one call: ["RPG", "Reeducação Postural Global"]. Each procedure
	// matches if ANY term is contained (OR semantics).
	lister := &stubProcedureLister{
		result: []*ProcedureInfo{
			{ID: uuid.New(), Name: "RPG", Price: 200, DurationMinutes: 60},
			{ID: uuid.New(), Name: "RPG — Avaliação", Price: 30, DurationMinutes: 30},
			{ID: uuid.New(), Name: "Reeducação Postural Global Infantil", Price: 220, DurationMinutes: 60},
			{ID: uuid.New(), Name: "Pilates", Price: 130, DurationMinutes: 50},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		ProcedureLister: lister,
	}

	result, err := NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{
		"name_contains": []interface{}{"RPG", "Reeducação Postural Global"},
	})
	require.NoError(t, err)
	procs := result.([]*ProcedureInfo)

	// 3 matches: 2 by "RPG" substring + 1 by "Reeducação Postural Global" substring
	require.Len(t, procs, 3, "OR semantics: a procedure matches if any term in the array is contained in its name")

	names := []string{procs[0].Name, procs[1].Name, procs[2].Name}
	assert.Contains(t, names, "RPG")
	assert.Contains(t, names, "RPG — Avaliação")
	assert.Contains(t, names, "Reeducação Postural Global Infantil")
}

func TestListProcedures_NameContainsFilter_AcceptsSingleStringForBackwardsCompat(t *testing.T) {
	// LLM might pass a string instead of array. Both are accepted.
	lister := &stubProcedureLister{
		result: []*ProcedureInfo{
			{ID: uuid.New(), Name: "RPG"},
			{ID: uuid.New(), Name: "Pilates"},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		ProcedureLister: lister,
	}
	result, _ := NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{
		"name_contains": "rpg", // string, not array
	})
	procs := result.([]*ProcedureInfo)
	require.Len(t, procs, 1)
	assert.Equal(t, "RPG", procs[0].Name)
}

func TestListProcedures_NameContainsFilter_EmptyTermsIgnored(t *testing.T) {
	// LLM hallucinating ["", "RPG"] or [""] should not collapse the filter
	// to "no filter" — empty/whitespace terms are dropped, not treated as
	// "match anything".
	lister := &stubProcedureLister{
		result: []*ProcedureInfo{
			{ID: uuid.New(), Name: "RPG"},
			{ID: uuid.New(), Name: "Pilates"},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		ProcedureLister: lister,
	}
	result, _ := NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{
		"name_contains": []interface{}{"", "  ", "RPG"},
	})
	procs := result.([]*ProcedureInfo)
	require.Len(t, procs, 1, "empty/whitespace terms must be dropped, not treated as match-everything")
	assert.Equal(t, "RPG", procs[0].Name)
}

// Real-world bug: AI listed Hamilton + Thaiane as professionals for
// "RPG — Avaliação", but neither has an active association with that
// procedure. Without a procedure_id filter the tool returns all clinic
// professionals and the LLM invents qualifications.

func TestGetAvailableProfessionals_PassesProcedureIDToLister(t *testing.T) {
	procID := uuid.New()
	lister := &stubProfessionalLister{
		result: []*ProfessionalInfo{
			{ID: uuid.New(), Name: "Dr. Carlos"},
		},
	}
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_professionals", map[string]interface{}{
		"procedure_id": procID.String(),
	})
	require.NoError(t, err)
	require.NotNil(t, lister.called.procedureID, "procedure_id from params must reach the lister so it can apply the join")
	assert.Equal(t, procID, *lister.called.procedureID)
}

func TestGetAvailableProfessionals_NoProcedureIDPassesNil(t *testing.T) {
	lister := &stubProfessionalLister{result: []*ProfessionalInfo{}}
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_professionals", map[string]interface{}{})
	require.NoError(t, err)
	assert.Nil(t, lister.called.procedureID, "no param = no filter (full clinic professional list)")
}

func TestGetAvailableProfessionals_InvalidProcedureID_ReturnsError(t *testing.T) {
	// Hallucinated procedure_id ("abc") must surface as an error so the LLM
	// retries with the correct UUID instead of silently getting the full
	// clinic list and listing unqualified professionals.
	lister := &stubProfessionalLister{}
	execCtx := &ExecutionContext{
		Context:            context.Background(),
		ClinicID:           uuid.New(),
		ProfessionalLister: lister,
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_available_professionals", map[string]interface{}{
		"procedure_id": "not-a-uuid",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid procedure_id")
}

type stubMetadataWriter struct {
	calls []struct {
		convID uuid.UUID
		key    string
		value  interface{}
	}
}

func (s *stubMetadataWriter) SetMetadata(_ context.Context, conversationID uuid.UUID, key string, value interface{}) error {
	s.calls = append(s.calls, struct {
		convID uuid.UUID
		key    string
		value  interface{}
	}{conversationID, key, value})
	return nil
}

func TestListProcedures_PersistsFilterSnapshotToConversationMetadata(t *testing.T) {
	// After list_procedures runs with a filter, the next turn's prompt must
	// be able to recall the active topic. This test pins down the contract:
	// metadata is written under LastProcedureFilterMetadataKey with both the
	// filter terms and the matched procedure names.
	convID := uuid.New()
	writer := &stubMetadataWriter{}
	lister := &stubProcedureLister{
		result: []*ProcedureInfo{
			{ID: uuid.New(), Name: "RPG"},
			{ID: uuid.New(), Name: "RPG — Avaliação"},
			{ID: uuid.New(), Name: "Pilates"},
		},
	}

	execCtx := &ExecutionContext{
		Context:              context.Background(),
		ClinicID:             uuid.New(),
		ConversationID:       convID,
		ProcedureLister:      lister,
		ConversationMetadata: writer,
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{
		"name_contains": []interface{}{"RPG"},
	})
	require.NoError(t, err)

	require.Len(t, writer.calls, 1, "exactly one metadata write per filtered list_procedures call")
	call := writer.calls[0]
	assert.Equal(t, convID, call.convID, "metadata must be scoped to the current conversation")
	assert.Equal(t, LastProcedureFilterMetadataKey, call.key)

	snapshot, ok := call.value.(LastProcedureFilterSnapshot)
	require.True(t, ok, "snapshot must use the typed shape so the prompt builder can decode it reliably")
	assert.Equal(t, []string{"RPG"}, snapshot.Filter)
	assert.ElementsMatch(t, []string{"RPG", "RPG — Avaliação"}, snapshot.Names)
}

func TestListProcedures_NoFilter_DoesNotPersistMetadata(t *testing.T) {
	// list_procedures without a filter is "show me everything" — not a
	// focused topic worth carrying forward. Persisting it would mislead the
	// next turn's hint into thinking the patient is interested in 100
	// random procedures.
	writer := &stubMetadataWriter{}
	lister := &stubProcedureLister{
		result: []*ProcedureInfo{{ID: uuid.New(), Name: "RPG"}},
	}
	execCtx := &ExecutionContext{
		Context:              context.Background(),
		ClinicID:             uuid.New(),
		ConversationID:       uuid.New(),
		ProcedureLister:      lister,
		ConversationMetadata: writer,
	}

	_, _ = NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{})

	assert.Empty(t, writer.calls, "no filter = no topic snapshot")
}

func TestListProcedures_NoFilter_ReturnsAll(t *testing.T) {
	// Regression: empty params still returns full catalog.
	lister := &stubProcedureLister{
		result: []*ProcedureInfo{
			{ID: uuid.New(), Name: "RPG"},
			{ID: uuid.New(), Name: "Pilates"},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		ProcedureLister: lister,
	}

	result, _ := NewExecutor(NewRegistry()).Execute(execCtx, "list_procedures", map[string]interface{}{})
	assert.Len(t, result.([]*ProcedureInfo), 2)
}

func TestCompleteHandoff_PropagatesMarkerError(t *testing.T) {
	marker := &stubHumanTakeoverMarker{err: assert.AnError}
	execCtx := &ExecutionContext{
		Context:             context.Background(),
		ClinicID:            uuid.New(),
		PatientPhone:        "5551999990001",
		HumanTakeoverMarker: marker,
		BlockedIntentReply:  "aguarde",
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "complete_handoff_to_human", map[string]interface{}{})
	assert.Error(t, err, "DB failures during Mark must propagate so the LLM can retry rather than silently 'succeed'")
}
