package tools

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Group session plan slots — Pilates / aulas em grupo
//
// Distinct from get_available_slots (1-on-1 appointments derived from
// professional working_hours minus existing appointments). Group slots are
// fixed (weekday, start_time, capacity) tuples on a session_plan_slots row;
// the AI must show real slots, not granular times computed from working_hours.
// ---------------------------------------------------------------------------

type stubGroupSlotLister struct {
	result   []*GroupPlanInfo
	err      error
	received GroupSlotFilter
	clinicID uuid.UUID
	calls    int
}

func (s *stubGroupSlotLister) ListGroupSlots(_ context.Context, clinicID uuid.UUID, filter GroupSlotFilter) ([]*GroupPlanInfo, error) {
	s.calls++
	s.clinicID = clinicID
	s.received = filter
	return s.result, s.err
}

func strPtr(s string) *string { return &s }

func TestGetGroupClassSlots_Registered(t *testing.T) {
	registry := NewRegistry()
	tool, exists := registry.GetTool("get_group_class_slots")
	require.True(t, exists, "get_group_class_slots must be registered so the LLM can use it for Pilates/group classes instead of hallucinating slots from working_hours")
	assert.NotEmpty(t, tool.Description)
}

func TestGetGroupClassSlots_ReturnsPlansWithSlots_NullableProfessional(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	slot1 := uuid.New()
	slot2 := uuid.New()

	lister := &stubGroupSlotLister{
		result: []*GroupPlanInfo{
			{
				PlanID:    planID,
				PlanTitle: "Pilates",
				Slots: []GroupSlotInfo{
					// First slot has no professional defined — ProfessionalName must be nil.
					{SlotID: slot1, Weekday: 1, WeekdayPT: "Segunda", StartTime: "08:00", Capacity: 6, Enrolled: 2, Available: 4, ProfessionalName: nil},
					{SlotID: slot2, Weekday: 1, WeekdayPT: "Segunda", StartTime: "09:00", Capacity: 6, Enrolled: 6, Available: 0, ProfessionalName: strPtr("Thaiane Simões")},
				},
			},
		},
	}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        clinicID,
		GroupSlotLister: lister,
	}

	res, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_group_class_slots", map[string]interface{}{
		"weekday": float64(1),
	})
	require.NoError(t, err)

	plans, ok := res.([]*GroupPlanInfo)
	require.True(t, ok, "result must be a slice of GroupPlanInfo so the LLM sees structured plan→slot data")
	require.Len(t, plans, 1)
	require.Len(t, plans[0].Slots, 2)
	assert.Nil(t, plans[0].Slots[0].ProfessionalName, "slot without professional_id must surface as nil — never as empty string or 'Profissional'")
	require.NotNil(t, plans[0].Slots[1].ProfessionalName)
	assert.Equal(t, "Thaiane Simões", *plans[0].Slots[1].ProfessionalName)
	assert.Equal(t, 0, plans[0].Slots[1].Available, "available must already account for active enrollments (capacity - enrolled)")
	assert.Equal(t, clinicID, lister.clinicID, "INV-001: every group-slot lookup must filter by clinic_id")
}

func TestGetGroupClassSlots_FilterByProcedureID(t *testing.T) {
	procedureID := uuid.New()
	lister := &stubGroupSlotLister{result: []*GroupPlanInfo{}}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		GroupSlotLister: lister,
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_group_class_slots", map[string]interface{}{
		"procedure_id": procedureID.String(),
		"weekday":      float64(1),
	})
	require.NoError(t, err)
	require.NotNil(t, lister.received.ProcedureID)
	assert.Equal(t, procedureID, *lister.received.ProcedureID, "procedure_id param must reach the lister so we restrict to plans linked to that procedure")
}

func TestGetGroupClassSlots_RequiresWeekday(t *testing.T) {
	// Returning the full weekly grid (50+ slots for typical group plans)
	// overwhelms the patient on mobile and the LLM tends to fabricate
	// slot picks from the response. weekday must be required at the tool
	// boundary, independent of any prompt instruction.
	lister := &stubGroupSlotLister{result: []*GroupPlanInfo{}}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		GroupSlotLister: lister,
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_group_class_slots", map[string]interface{}{
		"procedure_id": uuid.New().String(),
	})
	require.Error(t, err, "calling without weekday must fail — the LLM must be forced to scope its query to a single day")
	assert.Contains(t, err.Error(), "weekday is required")
	assert.Equal(t, 0, lister.calls, "lister must not be called when validation fails")
}

func TestGetGroupClassSlots_RegisteredAsRequired(t *testing.T) {
	// Schema-level enforcement: weekday must appear in the Required list
	// so the LLM provider rejects the call before it hits our executor.
	registry := NewRegistry()
	tool, ok := registry.GetTool("get_group_class_slots")
	require.True(t, ok)
	require.NotNil(t, tool.Parameters)
	assert.Contains(t, tool.Parameters.Required, "weekday",
		"weekday must be in the Required schema field so providers like OpenAI/Gemini reject calls missing it")
}

func TestGetGroupClassSlots_FilterByWeekday(t *testing.T) {
	lister := &stubGroupSlotLister{result: []*GroupPlanInfo{}}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		GroupSlotLister: lister,
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_group_class_slots", map[string]interface{}{
		"weekday": float64(3), // Wednesday
	})
	require.NoError(t, err)
	require.NotNil(t, lister.received.Weekday)
	assert.Equal(t, 3, *lister.received.Weekday)
}

func TestGetGroupClassSlots_FilterByTime(t *testing.T) {
	lister := &stubGroupSlotLister{result: []*GroupPlanInfo{}}
	execCtx := &ExecutionContext{
		Context:         context.Background(),
		ClinicID:        uuid.New(),
		GroupSlotLister: lister,
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_group_class_slots", map[string]interface{}{
		"weekday":     float64(2),
		"time_filter": "18:10",
	})
	require.NoError(t, err)
	assert.Equal(t, "18:10", lister.received.TimeFilter, "time_filter must reach the lister so it can narrow to a specific slot")
}

func TestGetGroupClassSlots_NotConfigured_Errors(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:  context.Background(),
		ClinicID: uuid.New(),
		// GroupSlotLister intentionally nil — wiring bug should fail loudly,
		// not silently return empty (which the LLM would interpret as "sem
		// vagas" and tell the patient there are no classes).
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "get_group_class_slots", map[string]interface{}{
		"weekday": float64(1),
	})
	assert.Error(t, err, "missing GroupSlotLister must surface as an error so wiring bugs are caught")
}

// ---------------------------------------------------------------------------
// list_partner_plans — read-only helper so the LLM can resolve "Gympass Silver"
// to a partner_plan_id before calling enroll_patient_in_class.
// ---------------------------------------------------------------------------

type stubPartnerPlanLister struct {
	result   []*PartnerPlanInfo
	err      error
	clinicID uuid.UUID
	calls    int
}

func (s *stubPartnerPlanLister) ListPartnerPlans(_ context.Context, clinicID uuid.UUID) ([]*PartnerPlanInfo, error) {
	s.calls++
	s.clinicID = clinicID
	return s.result, s.err
}

func TestListPartnerPlans_Registered(t *testing.T) {
	registry := NewRegistry()
	tool, exists := registry.GetTool("list_partner_plans")
	require.True(t, exists, "list_partner_plans must be registered so the LLM can resolve convênio names to UUIDs")
	assert.NotEmpty(t, tool.Description)
}

func TestListPartnerPlans_ReturnsActivePlansWithNetwork(t *testing.T) {
	clinicID := uuid.New()
	silver := uuid.New()
	tp2 := uuid.New()
	max3 := 3
	lister := &stubPartnerPlanLister{
		result: []*PartnerPlanInfo{
			{ID: silver, NetworkName: "Gympass", Name: "Silver", MaxSessionsPerWeek: &max3},
			{ID: tp2, NetworkName: "Total Pass", Name: "Tp2", MaxSessionsPerWeek: &max3},
		},
	}
	execCtx := &ExecutionContext{
		Context:           context.Background(),
		ClinicID:          clinicID,
		PartnerPlanLister: lister,
	}

	res, err := NewExecutor(NewRegistry()).Execute(execCtx, "list_partner_plans", map[string]interface{}{})
	require.NoError(t, err)
	plans, ok := res.([]*PartnerPlanInfo)
	require.True(t, ok)
	require.Len(t, plans, 2)
	assert.Equal(t, "Gympass", plans[0].NetworkName, "network_name must accompany the plan so the LLM can disambiguate (Silver appears in multiple networks)")
	assert.Equal(t, "Silver", plans[0].Name)
	require.NotNil(t, plans[0].MaxSessionsPerWeek)
	assert.Equal(t, 3, *plans[0].MaxSessionsPerWeek)
	assert.Equal(t, clinicID, lister.clinicID, "INV-001: clinic_id scoping required")
}

func TestListPartnerPlans_NotConfigured_Errors(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:  context.Background(),
		ClinicID: uuid.New(),
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "list_partner_plans", map[string]interface{}{})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// enroll_patient_in_class — write tool, gated by AIToolEnrollClass.
// ---------------------------------------------------------------------------

type stubClassEnroller struct {
	result   *ClassEnrollmentInfo
	err      error
	received struct {
		clinicID      uuid.UUID
		patientID     uuid.UUID
		planID        uuid.UUID
		slotIDs       []uuid.UUID
		partnerPlanID *uuid.UUID
		monthlyAmount *float64
		billingDay    *int
	}
	calls int
}

func (s *stubClassEnroller) Enroll(
	_ context.Context,
	clinicID, patientID, planID uuid.UUID,
	slotIDs []uuid.UUID,
	partnerPlanID *uuid.UUID,
	monthlyAmount *float64,
	billingDay *int,
) (*ClassEnrollmentInfo, error) {
	s.calls++
	s.received.clinicID = clinicID
	s.received.patientID = patientID
	s.received.planID = planID
	s.received.slotIDs = slotIDs
	s.received.partnerPlanID = partnerPlanID
	s.received.monthlyAmount = monthlyAmount
	s.received.billingDay = billingDay
	return s.result, s.err
}

func TestEnrollPatientInClass_Registered(t *testing.T) {
	registry := NewRegistry()
	tool, exists := registry.GetTool("enroll_patient_in_class")
	require.True(t, exists, "enroll_patient_in_class must be registered (gating happens via DisallowedTools)")
	assert.NotEmpty(t, tool.Description)
}

func TestEnrollPatientInClass_RequiresPatientID(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		ClinicID:      uuid.New(),
		PatientID:     nil, // patient not identified yet
		ClassEnroller: &stubClassEnroller{},
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "enroll_patient_in_class", map[string]interface{}{
		"plan_id":  uuid.New().String(),
		"slot_ids": []interface{}{uuid.New().String()},
	})
	assert.Error(t, err, "enrollment without an identified patient must fail — never silently enroll the wrong person")
}

func TestEnrollPatientInClass_RequiresPlanID(t *testing.T) {
	patientID := uuid.New()
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		ClinicID:      uuid.New(),
		PatientID:     &patientID,
		ClassEnroller: &stubClassEnroller{},
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "enroll_patient_in_class", map[string]interface{}{
		"slot_ids": []interface{}{uuid.New().String()},
	})
	assert.Error(t, err)
}

func TestEnrollPatientInClass_RequiresSlotIDs(t *testing.T) {
	patientID := uuid.New()
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		ClinicID:      uuid.New(),
		PatientID:     &patientID,
		ClassEnroller: &stubClassEnroller{},
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "enroll_patient_in_class", map[string]interface{}{
		"plan_id": uuid.New().String(),
		// slot_ids missing
	})
	assert.Error(t, err)
}

func TestEnrollPatientInClass_DelegatesAllParamsToEnroller(t *testing.T) {
	clinicID := uuid.New()
	patientID := uuid.New()
	planID := uuid.New()
	slot1 := uuid.New()
	slot2 := uuid.New()
	partnerPlanID := uuid.New()
	enrollmentID := uuid.New()
	monthly := 195.0

	enroller := &stubClassEnroller{
		result: &ClassEnrollmentInfo{
			EnrollmentID:  enrollmentID,
			PlanTitle:     "Pilates",
			Weekdays:      []int{1, 3, 5},
			MonthlyAmount: &monthly,
			PartnerPlan:   "Gympass Silver",
			Status:        "active",
		},
	}
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		ClinicID:      clinicID,
		PatientID:     &patientID,
		ClassEnroller: enroller,
	}

	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "enroll_patient_in_class", map[string]interface{}{
		"plan_id":         planID.String(),
		"slot_ids":        []interface{}{slot1.String(), slot2.String()},
		"partner_plan_id": partnerPlanID.String(),
		"monthly_amount":  195.0,
		"billing_day":     float64(10),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, enroller.calls)
	assert.Equal(t, clinicID, enroller.received.clinicID, "INV-001: clinic_id from execCtx")
	assert.Equal(t, patientID, enroller.received.patientID, "patient_id always from execCtx — never accepted as param (would let LLM enroll the wrong patient)")
	assert.Equal(t, planID, enroller.received.planID)
	assert.ElementsMatch(t, []uuid.UUID{slot1, slot2}, enroller.received.slotIDs)
	require.NotNil(t, enroller.received.partnerPlanID)
	assert.Equal(t, partnerPlanID, *enroller.received.partnerPlanID)
	require.NotNil(t, enroller.received.monthlyAmount)
	assert.Equal(t, 195.0, *enroller.received.monthlyAmount)
	require.NotNil(t, enroller.received.billingDay)
	assert.Equal(t, 10, *enroller.received.billingDay)
}

func TestEnrollPatientInClass_OmitsOptionalParamsAsNilPointers(t *testing.T) {
	patientID := uuid.New()
	planID := uuid.New()
	slot1 := uuid.New()

	enroller := &stubClassEnroller{
		result: &ClassEnrollmentInfo{Status: "active"},
	}
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		ClinicID:      uuid.New(),
		PatientID:     &patientID,
		ClassEnroller: enroller,
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "enroll_patient_in_class", map[string]interface{}{
		"plan_id":  planID.String(),
		"slot_ids": []interface{}{slot1.String()},
	})
	require.NoError(t, err)
	assert.Nil(t, enroller.received.partnerPlanID, "absent partner_plan_id must be nil pointer (not zero-uuid) so the service-side validation knows it was not provided")
	assert.Nil(t, enroller.received.monthlyAmount)
	assert.Nil(t, enroller.received.billingDay)
}

func TestEnrollPatientInClass_NotConfigured_Errors(t *testing.T) {
	patientID := uuid.New()
	execCtx := &ExecutionContext{
		Context:   context.Background(),
		ClinicID:  uuid.New(),
		PatientID: &patientID,
		// ClassEnroller intentionally nil
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "enroll_patient_in_class", map[string]interface{}{
		"plan_id":  uuid.New().String(),
		"slot_ids": []interface{}{uuid.New().String()},
	})
	assert.Error(t, err, "missing ClassEnroller must surface as an error so wiring bugs are caught early")
}

func TestEnrollPatientInClass_PropagatesEnrollerError(t *testing.T) {
	patientID := uuid.New()
	enroller := &stubClassEnroller{err: assertSomeServiceError{}}
	execCtx := &ExecutionContext{
		Context:       context.Background(),
		ClinicID:      uuid.New(),
		PatientID:     &patientID,
		ClassEnroller: enroller,
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "enroll_patient_in_class", map[string]interface{}{
		"plan_id":  uuid.New().String(),
		"slot_ids": []interface{}{uuid.New().String()},
	})
	assert.Error(t, err, "service-layer validation errors (capacity, partner-plan freq, etc.) must surface to the LLM so it can show a helpful message to the patient")
}

type assertSomeServiceError struct{}

func (assertSomeServiceError) Error() string { return "slot has reached its capacity" }

// ---------------------------------------------------------------------------
// confirm_class_attendance — patient confirms participation in next group
// session. Gated by AIToolConfirmClass (separate from AIToolConfirm which
// gates 1-on-1 appointment confirmations).
// ---------------------------------------------------------------------------

type stubClassConfirmer struct {
	result   *ClassConfirmationInfo
	err      error
	received struct {
		clinicID  uuid.UUID
		patientID uuid.UUID
	}
	calls int
}

func (s *stubClassConfirmer) Confirm(_ context.Context, clinicID, patientID uuid.UUID) (*ClassConfirmationInfo, error) {
	s.calls++
	s.received.clinicID = clinicID
	s.received.patientID = patientID
	return s.result, s.err
}

func TestConfirmClassAttendance_Registered(t *testing.T) {
	registry := NewRegistry()
	tool, exists := registry.GetTool("confirm_class_attendance")
	require.True(t, exists, "confirm_class_attendance must be registered (gating happens via DisallowedTools)")
	assert.NotEmpty(t, tool.Description)
}

func TestConfirmClassAttendance_RequiresPatientID(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:        context.Background(),
		ClinicID:       uuid.New(),
		PatientID:      nil,
		ClassConfirmer: &stubClassConfirmer{},
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "confirm_class_attendance", map[string]interface{}{})
	assert.Error(t, err, "confirmation without an identified patient must fail — never silently confirm the wrong person's class")
}

func TestConfirmClassAttendance_DelegatesToConfirmer(t *testing.T) {
	clinicID := uuid.New()
	patientID := uuid.New()
	classDate := "2026-04-30"
	confirmer := &stubClassConfirmer{
		result: &ClassConfirmationInfo{
			AttendanceID: uuid.New(),
			PlanTitle:    "Pilates",
			SessionDate:  classDate,
			StartTime:    "18:10",
			Status:       "confirmed",
		},
	}
	execCtx := &ExecutionContext{
		Context:        context.Background(),
		ClinicID:       clinicID,
		PatientID:      &patientID,
		ClassConfirmer: confirmer,
	}

	res, err := NewExecutor(NewRegistry()).Execute(execCtx, "confirm_class_attendance", map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, 1, confirmer.calls)
	assert.Equal(t, clinicID, confirmer.received.clinicID, "INV-001: clinic_id from execCtx")
	assert.Equal(t, patientID, confirmer.received.patientID, "patient_id from execCtx — never accepted as param")

	info, ok := res.(*ClassConfirmationInfo)
	require.True(t, ok)
	assert.Equal(t, classDate, info.SessionDate, "the class date must come back so the LLM can echo it in the patient-facing reply")
	assert.Equal(t, "18:10", info.StartTime)
	assert.Equal(t, "Pilates", info.PlanTitle)
}

func TestConfirmClassAttendance_NotConfigured_Errors(t *testing.T) {
	patientID := uuid.New()
	execCtx := &ExecutionContext{
		Context:   context.Background(),
		ClinicID:  uuid.New(),
		PatientID: &patientID,
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "confirm_class_attendance", map[string]interface{}{})
	assert.Error(t, err, "missing ClassConfirmer must surface as an error so wiring bugs are caught early")
}

// ---------------------------------------------------------------------------
// present_enrollment_summary — server-built confirmation message that
// validates slot_ids actually exist and belong to the plan. Closes the
// hallucination vector where the LLM would invent slot picks in free-text
// markdown.
// ---------------------------------------------------------------------------

type stubEnrollmentSummaryPresenter struct {
	result   *EnrollmentSummaryInfo
	err      error
	received struct {
		clinicID      uuid.UUID
		planID        uuid.UUID
		slotIDs       []uuid.UUID
		partnerPlanID *uuid.UUID
		monthlyAmount *float64
		billingDay    *int
	}
	calls int
}

func (s *stubEnrollmentSummaryPresenter) PresentSummary(
	_ context.Context,
	clinicID, planID uuid.UUID,
	slotIDs []uuid.UUID,
	partnerPlanID *uuid.UUID,
	monthlyAmount *float64,
	billingDay *int,
) (*EnrollmentSummaryInfo, error) {
	s.calls++
	s.received.clinicID = clinicID
	s.received.planID = planID
	s.received.slotIDs = slotIDs
	s.received.partnerPlanID = partnerPlanID
	s.received.monthlyAmount = monthlyAmount
	s.received.billingDay = billingDay
	return s.result, s.err
}

func TestPresentEnrollmentSummary_Registered(t *testing.T) {
	registry := NewRegistry()
	tool, ok := registry.GetTool("present_enrollment_summary")
	require.True(t, ok, "present_enrollment_summary must be registered — without it, the LLM falls back to free-text markdown which it can fabricate")
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, tool.Parameters)
	assert.Contains(t, tool.Parameters.Required, "plan_id")
	assert.Contains(t, tool.Parameters.Required, "slot_ids")
}

func TestPresentEnrollmentSummary_RequiresPlanID(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:                     context.Background(),
		ClinicID:                    uuid.New(),
		EnrollmentSummaryPresenter: &stubEnrollmentSummaryPresenter{},
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "present_enrollment_summary", map[string]interface{}{
		"slot_ids": []interface{}{uuid.New().String()},
	})
	assert.Error(t, err)
}

func TestPresentEnrollmentSummary_RequiresSlotIDs(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:                     context.Background(),
		ClinicID:                    uuid.New(),
		EnrollmentSummaryPresenter: &stubEnrollmentSummaryPresenter{},
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "present_enrollment_summary", map[string]interface{}{
		"plan_id": uuid.New().String(),
	})
	assert.Error(t, err)
}

func TestPresentEnrollmentSummary_DelegatesAllParamsToPresenter(t *testing.T) {
	clinicID := uuid.New()
	planID := uuid.New()
	slot1 := uuid.New()
	slot2 := uuid.New()
	partnerPlanID := uuid.New()

	presenter := &stubEnrollmentSummaryPresenter{
		result: &EnrollmentSummaryInfo{
			SummaryText: "Antes de finalizar, confirma a matrícula?\n\n*Modalidade:* Pilates\n*Frequência:* 2x/semana\n*Horários:* Segunda 09:00, Quarta 09:00\n*Convênio:* Particular\n*Mensalidade:* R$ 175,00\n\nPosso confirmar?",
			PlanID:      planID,
			PlanTitle:   "Pilates",
			SlotIDs:     []uuid.UUID{slot1, slot2},
		},
	}
	execCtx := &ExecutionContext{
		Context:                     context.Background(),
		ClinicID:                    clinicID,
		EnrollmentSummaryPresenter: presenter,
	}

	res, err := NewExecutor(NewRegistry()).Execute(execCtx, "present_enrollment_summary", map[string]interface{}{
		"plan_id":         planID.String(),
		"slot_ids":        []interface{}{slot1.String(), slot2.String()},
		"partner_plan_id": partnerPlanID.String(),
		"monthly_amount":  175.0,
		"billing_day":     float64(5),
	})
	require.NoError(t, err)

	info, ok := res.(*EnrollmentSummaryInfo)
	require.True(t, ok)
	assert.Contains(t, info.SummaryText, "*Modalidade:*", "summary must contain the bold-formatted fields server-built — the AI cannot fabricate this")
	assert.Equal(t, clinicID, presenter.received.clinicID, "INV-001: clinic_id from execCtx")
	assert.Equal(t, planID, presenter.received.planID)
	assert.ElementsMatch(t, []uuid.UUID{slot1, slot2}, presenter.received.slotIDs)
	require.NotNil(t, presenter.received.partnerPlanID)
	assert.Equal(t, partnerPlanID, *presenter.received.partnerPlanID)
	require.NotNil(t, presenter.received.monthlyAmount)
	assert.Equal(t, 175.0, *presenter.received.monthlyAmount)
}

func TestPresentEnrollmentSummary_NotConfigured_Errors(t *testing.T) {
	execCtx := &ExecutionContext{
		Context:  context.Background(),
		ClinicID: uuid.New(),
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "present_enrollment_summary", map[string]interface{}{
		"plan_id":  uuid.New().String(),
		"slot_ids": []interface{}{uuid.New().String()},
	})
	assert.Error(t, err)
}

func TestPresentEnrollmentSummary_PropagatesValidatorError(t *testing.T) {
	// When a slot_id is hallucinated, the presenter must return an error
	// the LLM can see and react to ("Slot X not found in plan Y") so it
	// goes back and re-asks the patient.
	presenter := &stubEnrollmentSummaryPresenter{err: assertSomeServiceError{}}
	execCtx := &ExecutionContext{
		Context:                     context.Background(),
		ClinicID:                    uuid.New(),
		EnrollmentSummaryPresenter: presenter,
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "present_enrollment_summary", map[string]interface{}{
		"plan_id":  uuid.New().String(),
		"slot_ids": []interface{}{uuid.New().String()},
	})
	assert.Error(t, err)
}

func TestConfirmClassAttendance_PropagatesError(t *testing.T) {
	patientID := uuid.New()
	confirmer := &stubClassConfirmer{err: assertSomeServiceError{}}
	execCtx := &ExecutionContext{
		Context:        context.Background(),
		ClinicID:       uuid.New(),
		PatientID:      &patientID,
		ClassConfirmer: confirmer,
	}
	_, err := NewExecutor(NewRegistry()).Execute(execCtx, "confirm_class_attendance", map[string]interface{}{})
	assert.Error(t, err, "service-layer errors (no active enrollment, no upcoming class, etc.) must reach the LLM so it can explain to the patient")
}
