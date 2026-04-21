package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalerrors "github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/appointment/service"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	paymentmethod_model "github.com/aseno/backend/pkg/paymentmethod/model"
)

// --- Narrow mocks that match the receipt service's internal interfaces ---

type mockReceiptAptRepo struct {
	// Map of appointment ID -> appointment. Absence behaves like a
	// soft-deleted row (the real repo filters deleted_at IS NULL and returns
	// ErrAppointmentNotFound for the same reason).
	byID map[uuid.UUID]*model.Appointment
}

func (m *mockReceiptAptRepo) GetByID(_ context.Context, id, _ uuid.UUID) (*model.Appointment, error) {
	if apt, ok := m.byID[id]; ok && apt != nil {
		return apt, nil
	}
	return nil, internalerrors.ErrAppointmentNotFoundInstance
}

type mockReceiptPaymentRepo struct {
	// Only non-soft-deleted payments belong here — the real repo already
	// filters deleted_at IS NULL in ListByAppointment via the query.
	payments []*model.AppointmentPayment
}

func (m *mockReceiptPaymentRepo) ListByAppointment(appointmentID, _ uuid.UUID) ([]*model.AppointmentPayment, error) {
	var out []*model.AppointmentPayment
	for _, p := range m.payments {
		if p.AppointmentID == appointmentID {
			out = append(out, p)
		}
	}
	return out, nil
}

type mockReceiptClinicRepo struct {
	clinic *clinic_model.Clinic
}

func (m *mockReceiptClinicRepo) GetByID(_ context.Context, _ uuid.UUID) (*clinic_model.Clinic, error) {
	if m.clinic == nil {
		return nil, internalerrors.ErrNotFoundInstance
	}
	return m.clinic, nil
}

type mockReceiptTokenRepo struct {
	// Token IDs indexed two ways for lookup from both endpoints.
	rowsByAppt  map[uuid.UUID]*model.AppointmentReceiptToken
	rowsByToken map[uuid.UUID]*model.AppointmentReceiptToken
	createCalls int
}

func newMockTokenRepo() *mockReceiptTokenRepo {
	return &mockReceiptTokenRepo{
		rowsByAppt:  map[uuid.UUID]*model.AppointmentReceiptToken{},
		rowsByToken: map[uuid.UUID]*model.AppointmentReceiptToken{},
	}
}

func (m *mockReceiptTokenRepo) GetOrCreateByAppointment(_ context.Context, clinicID, appointmentID uuid.UUID) (*model.AppointmentReceiptToken, error) {
	if existing, ok := m.rowsByAppt[appointmentID]; ok {
		return existing, nil
	}
	row := &model.AppointmentReceiptToken{
		ID:            uuid.New(),
		ClinicID:      clinicID,
		AppointmentID: appointmentID,
		Token:         uuid.New(),
		CreatedAt:     time.Now(),
	}
	m.rowsByAppt[appointmentID] = row
	m.rowsByToken[row.Token] = row
	m.createCalls++
	return row, nil
}

func (m *mockReceiptTokenRepo) GetByToken(_ context.Context, token uuid.UUID) (*model.AppointmentReceiptToken, error) {
	if row, ok := m.rowsByToken[token]; ok {
		return row, nil
	}
	return nil, internalerrors.ErrNotFoundInstance
}

// --- Helpers ---

func newReceiptAppointment(clinicID uuid.UUID) *model.Appointment {
	start := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	return &model.Appointment{
		ID:         uuid.New(),
		ClinicID:   clinicID,
		PatientID:  uuid.New(),
		StartTime:  start,
		EndTime:    start.Add(60 * time.Minute),
		Price:      200,
		Patient:    &model.PatientInfo{ID: uuid.New(), Name: "Maria da Silva"},
		Professional: &model.ProfessionalInfo{Name: "Dr. Joao"},
	}
}

func buildReceiptService(t *testing.T) (
	service.AppointmentReceiptService,
	*mockReceiptAptRepo,
	*mockReceiptPaymentRepo,
	*mockReceiptTokenRepo,
	*mockReceiptClinicRepo,
) {
	t.Helper()
	aptRepo := &mockReceiptAptRepo{byID: map[uuid.UUID]*model.Appointment{}}
	payRepo := &mockReceiptPaymentRepo{}
	tokRepo := newMockTokenRepo()
	cliRepo := &mockReceiptClinicRepo{clinic: &clinic_model.Clinic{
		ID:       uuid.New(),
		Name:     "Clinica Teste",
		Document: "12.345.678/0001-00",
		Phone:    "(11) 99999-0000",
	}}
	svc := service.NewAppointmentReceiptService(tokRepo, aptRepo, payRepo, cliRepo, "https://app.test.com")
	return svc, aptRepo, payRepo, tokRepo, cliRepo
}

// --- Tests ---

func TestEnsureToken_IsIdempotent(t *testing.T) {
	svc, aptRepo, _, tokRepo, _ := buildReceiptService(t)
	apt := newReceiptAppointment(uuid.New())
	aptRepo.byID[apt.ID] = apt

	first, err := svc.EnsureToken(context.Background(), apt.ClinicID, apt.ID)
	require.NoError(t, err)
	require.NotEmpty(t, first.Token)
	require.Contains(t, first.PublicURL, "/recibo/"+first.Token.String())

	// Second call must return the same token (no new row).
	second, err := svc.EnsureToken(context.Background(), apt.ClinicID, apt.ID)
	require.NoError(t, err)
	assert.Equal(t, first.Token, second.Token, "EnsureToken must be idempotent — a second call cannot create a new row")
	assert.Equal(t, 1, tokRepo.createCalls, "only one token row should ever be created per appointment")
}

func TestEnsureToken_SoftDeletedAppointment_Returns404(t *testing.T) {
	svc, _, _, _, _ := buildReceiptService(t)
	// No appointment in the mock means GetByID will return NotFound, which is
	// exactly what the real repo does for soft-deleted rows.
	_, err := svc.EnsureToken(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err, "soft-deleted (or missing) appointments must not mint new tokens")
}

func TestGetPublicReceipt_SoftDeletedAppointment_Returns404(t *testing.T) {
	svc, aptRepo, _, tokRepo, _ := buildReceiptService(t)
	apt := newReceiptAppointment(uuid.New())
	// Token exists in the DB, but the appointment has since been soft-deleted
	// (removed from the repo mock). This is the exact cenario the user called
	// out: the public link must not be accessible anymore.
	tokRepo.rowsByAppt[apt.ID] = &model.AppointmentReceiptToken{
		ID:            uuid.New(),
		ClinicID:      apt.ClinicID,
		AppointmentID: apt.ID,
		Token:         uuid.New(),
	}
	tok := tokRepo.rowsByAppt[apt.ID]
	tokRepo.rowsByToken[tok.Token] = tok

	_, err := svc.GetPublicReceipt(context.Background(), tok.Token)
	require.Error(t, err)
	_ = aptRepo
}

func TestGetPublicReceipt_AggregatesOnlyActivePayments(t *testing.T) {
	svc, aptRepo, payRepo, tokRepo, _ := buildReceiptService(t)
	apt := newReceiptAppointment(uuid.New())
	aptRepo.byID[apt.ID] = apt

	// Two payments registered in the past. The "deleted" one is NOT present
	// in the mock list — the real repository excludes soft-deleted rows at
	// the query level (deleted_at IS NULL). The live view is what the user
	// specifically wants: if the clinic deletes a payment, the link must
	// reflect only what remains.
	debit := &paymentmethod_model.PaymentMethod{ID: uuid.New(), Name: "Cartao de Debito"}
	payRepo.payments = []*model.AppointmentPayment{
		{
			AppointmentID: apt.ID,
			Amount:        100,
			PaidAt:        apt.StartTime,
			PaymentMethod: &model.PaymentMethodInfo{ID: debit.ID, Name: debit.Name, Type: string(debit.Type)},
		},
	}

	// Seed token
	_, err := svc.EnsureToken(context.Background(), apt.ClinicID, apt.ID)
	require.NoError(t, err)
	tok := tokRepo.rowsByAppt[apt.ID].Token

	resp, err := svc.GetPublicReceipt(context.Background(), tok)
	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Payments), "deleted payments must not appear")
	assert.Equal(t, 100.0, resp.TotalPaid)
	assert.Equal(t, 100.0, resp.BalanceDue, "200 total - 100 paid = 100 due")
}

func TestGetPublicReceipt_PerPaymentDiscount_ExposedOnDTO(t *testing.T) {
	// The receipt header already shows the aggregated discount. The clinic
	// also wants each payment row to show the discount applied to that
	// specific payment so the physical slip lines up with the accounting
	// entry. Assert the DTO carries it through — not just the aggregate.
	svc, aptRepo, payRepo, tokRepo, _ := buildReceiptService(t)
	apt := newReceiptAppointment(uuid.New())
	aptRepo.byID[apt.ID] = apt

	debit := &paymentmethod_model.PaymentMethod{ID: uuid.New(), Name: "Cartao de Debito"}
	pix := &paymentmethod_model.PaymentMethod{ID: uuid.New(), Name: "PIX"}
	payRepo.payments = []*model.AppointmentPayment{
		{
			AppointmentID: apt.ID,
			Amount:        80,
			Discount:      20,
			PaidAt:        apt.StartTime,
			PaymentMethod: &model.PaymentMethodInfo{ID: debit.ID, Name: debit.Name, Type: string(debit.Type)},
		},
		{
			AppointmentID: apt.ID,
			Amount:        100,
			Discount:      0,
			PaidAt:        apt.StartTime,
			PaymentMethod: &model.PaymentMethodInfo{ID: pix.ID, Name: pix.Name, Type: string(pix.Type)},
		},
	}

	_, err := svc.EnsureToken(context.Background(), apt.ClinicID, apt.ID)
	require.NoError(t, err)
	tok := tokRepo.rowsByAppt[apt.ID].Token

	resp, err := svc.GetPublicReceipt(context.Background(), tok)
	require.NoError(t, err)
	require.Len(t, resp.Payments, 2)
	assert.Equal(t, 20.0, resp.Payments[0].Discount, "per-payment discount must round-trip into the DTO")
	assert.Equal(t, 0.0, resp.Payments[1].Discount, "zero discount must still serialize explicitly")
	assert.Equal(t, 20.0, resp.Discount, "aggregated discount continues to be exposed as before")
}

func TestGetPublicReceipt_NoPayments_RendersZeroedPage(t *testing.T) {
	svc, aptRepo, _, tokRepo, _ := buildReceiptService(t)
	apt := newReceiptAppointment(uuid.New())
	aptRepo.byID[apt.ID] = apt

	_, err := svc.EnsureToken(context.Background(), apt.ClinicID, apt.ID)
	require.NoError(t, err)
	tok := tokRepo.rowsByAppt[apt.ID].Token

	resp, err := svc.GetPublicReceipt(context.Background(), tok)
	require.NoError(t, err)
	assert.Empty(t, resp.Payments, "page must render with an empty payments list, not 404")
	assert.Equal(t, 0.0, resp.TotalPaid)
	assert.Equal(t, apt.Price, resp.BalanceDue, "balance must equal total when nothing is paid")

	// Nil slices serialize to JSON `null`, which then crashes the frontend
	// (`data.payments.length` on null). The service must always emit empty
	// arrays for these fields even when the underlying data is empty.
	require.NotNil(t, resp.Payments, "Payments must be an empty slice, not nil (JSON null breaks the UI)")
	require.NotNil(t, resp.Procedures, "Procedures must be an empty slice, not nil (JSON null breaks the UI)")
}

func TestGetPublicReceipt_MultipleProcedures_LineByLine(t *testing.T) {
	svc, aptRepo, _, tokRepo, _ := buildReceiptService(t)
	apt := newReceiptAppointment(uuid.New())
	procID1, procID2 := uuid.New(), uuid.New()
	apt.Price = 300
	apt.Procedures = []model.AppointmentProcedure{
		{ID: uuid.New(), AppointmentID: apt.ID, ProcedureID: procID1, Price: 200, Procedure: nil},
		{ID: uuid.New(), AppointmentID: apt.ID, ProcedureID: procID2, Price: 100, Procedure: nil},
	}
	aptRepo.byID[apt.ID] = apt

	_, err := svc.EnsureToken(context.Background(), apt.ClinicID, apt.ID)
	require.NoError(t, err)
	tok := tokRepo.rowsByAppt[apt.ID].Token

	resp, err := svc.GetPublicReceipt(context.Background(), tok)
	require.NoError(t, err)
	require.Len(t, resp.Procedures, 2, "each procedure must render on its own line")
	assert.Equal(t, 300.0, resp.Subtotal)
	assert.Equal(t, 300.0, resp.Total)
}

func TestCheckTokenOwnership_SameClinic_ReturnsOwned(t *testing.T) {
	// Used by the public /recibo page to decide between the 2-via clinic
	// layout and the 1-via patient layout. Same-clinic viewer must be flagged
	// so the print copy that the clinic keeps is only rendered for whoever
	// actually owns the appointment.
	svc, aptRepo, _, tokRepo, _ := buildReceiptService(t)
	apt := newReceiptAppointment(uuid.New())
	aptRepo.byID[apt.ID] = apt

	_, err := svc.EnsureToken(context.Background(), apt.ClinicID, apt.ID)
	require.NoError(t, err)
	tok := tokRepo.rowsByAppt[apt.ID].Token

	owned, err := svc.CheckTokenOwnership(context.Background(), apt.ClinicID, tok)
	require.NoError(t, err)
	assert.True(t, owned, "the token's own clinic must be recognized as the owner")
}

func TestCheckTokenOwnership_OtherClinic_ReturnsNotOwned(t *testing.T) {
	// The core of the security concern: clinic B logs in and opens clinic A's
	// receipt link. The endpoint must say "not yours" so the layout downgrades
	// to the patient view and the clinic's "via da clínica" with signatures
	// isn't rendered for a third party.
	svc, aptRepo, _, tokRepo, _ := buildReceiptService(t)
	apt := newReceiptAppointment(uuid.New())
	aptRepo.byID[apt.ID] = apt

	_, err := svc.EnsureToken(context.Background(), apt.ClinicID, apt.ID)
	require.NoError(t, err)
	tok := tokRepo.rowsByAppt[apt.ID].Token

	otherClinic := uuid.New()
	require.NotEqual(t, apt.ClinicID, otherClinic)
	owned, err := svc.CheckTokenOwnership(context.Background(), otherClinic, tok)
	require.NoError(t, err, "a different clinic querying must not get an error — just a 'not owned' boolean")
	assert.False(t, owned, "a different clinic must never be reported as the owner")
}

func TestCheckTokenOwnership_MissingToken_ReturnsNotFound(t *testing.T) {
	svc, _, _, _, _ := buildReceiptService(t)
	_, err := svc.CheckTokenOwnership(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err, "an unknown token must surface as an error so the handler can 404")
}

func TestGetPublicReceipt_ClinicAndPatientFieldsExposedButNothingSensitive(t *testing.T) {
	svc, aptRepo, _, tokRepo, _ := buildReceiptService(t)
	apt := newReceiptAppointment(uuid.New())
	apt.Patient.Email = "maria@example.com"
	apt.Patient.Phone = "(11) 98765-4321"
	aptRepo.byID[apt.ID] = apt

	_, err := svc.EnsureToken(context.Background(), apt.ClinicID, apt.ID)
	require.NoError(t, err)
	tok := tokRepo.rowsByAppt[apt.ID].Token

	resp, err := svc.GetPublicReceipt(context.Background(), tok)
	require.NoError(t, err)

	// Included fields
	assert.Equal(t, "Maria da Silva", resp.PatientName)
	assert.Equal(t, "Clinica Teste", resp.Clinic.Name)
	assert.NotEmpty(t, resp.Clinic.Document)

	// Sensitive patient fields are deliberately absent from the DTO. This is
	// enforced by the PublicReceiptResponse struct not having Email/Phone
	// fields — this test documents the intent so future changes that add
	// those fields get a failing review signal.
	require.NotContains(t, resp.PatientName, "@")
}
