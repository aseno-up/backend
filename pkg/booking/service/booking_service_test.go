package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	account_model "github.com/aseno/backend/pkg/account/model"
	appointmentModel "github.com/aseno/backend/pkg/appointment/model"
	bookingModel "github.com/aseno/backend/pkg/booking/model"
	clinicModel "github.com/aseno/backend/pkg/clinic/model"
	emailSvc "github.com/aseno/backend/pkg/email/service"
	notificationModel "github.com/aseno/backend/pkg/notification/model"
	"github.com/aseno/backend/pkg/pagination"
	patientModel "github.com/aseno/backend/pkg/patient/model"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	portalModel "github.com/aseno/backend/pkg/portalaccount/model"
	procedureModel "github.com/aseno/backend/pkg/procedure/model"
	professionalModel "github.com/aseno/backend/pkg/professional/model"
	specialtyModel "github.com/aseno/backend/pkg/specialty/model"
)

// Mock implementations

type mockClinicRepo struct{ mock.Mock }

func (m *mockClinicRepo) GetBySlug(ctx context.Context, slug string) (*clinicModel.Clinic, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clinicModel.Clinic), args.Error(1)
}

type mockProfessionalRepo struct{ mock.Mock }

func (m *mockProfessionalRepo) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*professionalModel.Professional, int64, error) {
	args := m.Called(ctx, clinicID, filter)
	return args.Get(0).([]*professionalModel.Professional), args.Get(1).(int64), args.Error(2)
}

func (m *mockProfessionalRepo) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*professionalModel.Professional, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*professionalModel.Professional), args.Error(1)
}

type mockAppointmentRepo struct{ mock.Mock }

func (m *mockAppointmentRepo) GetByDateRange(ctx context.Context, clinicID uuid.UUID, start, end time.Time, professionalID *uuid.UUID) ([]*appointmentModel.Appointment, error) {
	args := m.Called(ctx, clinicID, start, end, professionalID)
	return args.Get(0).([]*appointmentModel.Appointment), args.Error(1)
}

func (m *mockAppointmentRepo) CheckConflict(ctx context.Context, clinicID, professionalID uuid.UUID, start, end time.Time, excludeID *uuid.UUID) (bool, error) {
	args := m.Called(ctx, clinicID, professionalID, start, end, excludeID)
	return args.Bool(0), args.Error(1)
}

func (m *mockAppointmentRepo) Create(ctx context.Context, appointment *appointmentModel.Appointment) error {
	return m.Called(ctx, appointment).Error(0)
}

type mockPatientRepo struct{ mock.Mock }

func (m *mockPatientRepo) GetByPhone(ctx context.Context, phone string, clinicID uuid.UUID) (*patientModel.Patient, error) {
	args := m.Called(ctx, phone, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patientModel.Patient), args.Error(1)
}

func (m *mockPatientRepo) Create(ctx context.Context, patient *patientModel.Patient) error {
	return m.Called(ctx, patient).Error(0)
}

type mockNotificationSvc struct{ mock.Mock }

func (m *mockNotificationSvc) Create(ctx context.Context, notification *notificationModel.Notification) error {
	return m.Called(ctx, notification).Error(0)
}

type mockSpecialtyRepo struct{ mock.Mock }

func (m *mockSpecialtyRepo) ListActiveByClinic(clinicID uuid.UUID) ([]*specialtyModel.Specialty, error) {
	args := m.Called(clinicID)
	return args.Get(0).([]*specialtyModel.Specialty), args.Error(1)
}

func (m *mockSpecialtyRepo) CountByClinic(clinicID uuid.UUID) (int64, error) {
	args := m.Called(clinicID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockSpecialtyRepo) GetByProfessionalIDs(professionalIDs []uuid.UUID) (map[uuid.UUID][]*specialtyModel.Specialty, error) {
	args := m.Called(professionalIDs)
	return args.Get(0).(map[uuid.UUID][]*specialtyModel.Specialty), args.Error(1)
}

type mockEmailSvc struct{ mock.Mock }

func (m *mockEmailSvc) SendInvite(ctx context.Context, data *emailSvc.InviteEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailSvc) SendWelcome(ctx context.Context, data *emailSvc.WelcomeEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailSvc) SendPasswordReset(ctx context.Context, data *emailSvc.ResetPasswordEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailSvc) SendEmailVerification(ctx context.Context, data *emailSvc.EmailVerificationData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailSvc) SendContactForm(ctx context.Context, data *emailSvc.ContactFormData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailSvc) SendBookingConfirmation(ctx context.Context, data *emailSvc.BookingConfirmationEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailSvc) SendPortalEmailVerification(ctx context.Context, data *emailSvc.PortalEmailVerificationData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailSvc) SendLeadIsca(ctx context.Context, data *emailSvc.LeadIscaEmailData) error {
	return m.Called(ctx, data).Error(0)
}

type mockProcedureRepo struct{ mock.Mock }

func (m *mockProcedureRepo) ListByClinicAndSpecialty(ctx context.Context, clinicID uuid.UUID, specialtyID *uuid.UUID) ([]*procedureModel.Procedure, error) {
	args := m.Called(ctx, clinicID, specialtyID)
	return args.Get(0).([]*procedureModel.Procedure), args.Error(1)
}

func (m *mockProcedureRepo) GetByID(id, clinicID uuid.UUID) (*procedureModel.Procedure, error) {
	args := m.Called(id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*procedureModel.Procedure), args.Error(1)
}

type mockPortalRepo struct{ mock.Mock }

func (m *mockPortalRepo) Create(ctx context.Context, account *portalModel.PortalAccount) error {
	return m.Called(ctx, account).Error(0)
}

// Helpers

func newTestClinic() *clinicModel.Clinic {
	return &clinicModel.Clinic{
		ID:   uuid.New(),
		Name: "Clinica Teste",
		Slug: "clinica-teste",
		Settings: clinicModel.ClinicSettings{
			AllowOnlineBooking:        true,
			DefaultAppointmentMinutes: 50,
		},
		Account: &account_model.Account{
			SubscriptionStatus: account_model.SubscriptionStatusActive,
			Plan: &plan_model.Plan{
				Slug: "professional",
			},
		},
	}
}

func newTestService() (*bookingService, *mockClinicRepo, *mockProfessionalRepo, *mockAppointmentRepo, *mockPatientRepo, *mockNotificationSvc, *mockSpecialtyRepo, *mockProcedureRepo, *mockEmailSvc) {
	clinicRepo := new(mockClinicRepo)
	profRepo := new(mockProfessionalRepo)
	aptRepo := new(mockAppointmentRepo)
	patientRepo := new(mockPatientRepo)
	notifSvc := new(mockNotificationSvc)
	specRepo := new(mockSpecialtyRepo)
	procRepo := new(mockProcedureRepo)
	portalRepo := new(mockPortalRepo)
	portalRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	emailMock := new(mockEmailSvc)

	svc := NewBookingService(clinicRepo, profRepo, aptRepo, patientRepo, notifSvc, specRepo, procRepo, nil, portalRepo, emailMock, "http://localhost:3001").(*bookingService)
	return svc, clinicRepo, profRepo, aptRepo, patientRepo, notifSvc, specRepo, procRepo, emailMock
}

// Tests

func TestCreateBooking_LGPD_AlwaysCreatesNewPatient(t *testing.T) {
	svc, clinicRepo, profRepo, aptRepo, patientRepo, notifSvc, _, procRepo, emailMock := newTestService()
	ctx := context.Background()

	clinic := newTestClinic()
	profID := uuid.New()
	professional := &professionalModel.Professional{
		ID:       profID,
		ClinicID: clinic.ID,
		Name:     "Dr. Ana",
	}

	futureTime := time.Now().Add(24 * time.Hour)

	clinicRepo.On("GetBySlug", ctx, "clinica-teste").Return(clinic, nil)
	profRepo.On("GetByID", ctx, profID, clinic.ID).Return(professional, nil)
	aptRepo.On("CheckConflict", ctx, clinic.ID, profID, futureTime, mock.Anything, mock.Anything).Return(false, nil)

	// LGPD: patient should ALWAYS be created, never looked up by phone
	patientRepo.On("Create", ctx, mock.MatchedBy(func(p *patientModel.Patient) bool {
		return p.Source == patientModel.PatientSourceSelfRegistered &&
			p.Name == "Maria Silva" &&
			p.Phone == "11999998888" &&
			p.Email == "maria@email.com" &&
			p.ClinicID == clinic.ID
	})).Return(nil)

	aptRepo.On("Create", ctx, mock.AnythingOfType("*model.Appointment")).Return(nil)
	notifSvc.On("Create", ctx, mock.AnythingOfType("*model.Notification")).Return(nil)
	emailMock.On("SendBookingConfirmation", ctx, mock.AnythingOfType("*service.BookingConfirmationEmailData")).Return(nil)

	_ = procRepo

	result, err := svc.CreateBooking(ctx, "clinica-teste", &bookingModel.CreateBookingRequest{
		ProfessionalID: profID,
		StartTime:      futureTime,
		PatientName:    "Maria Silva",
		PatientPhone:   "11999998888",
		PatientEmail:   "maria@email.com",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Dr. Ana", result.Professional)

	// LGPD: Verify GetByPhone was NEVER called
	patientRepo.AssertNotCalled(t, "GetByPhone")
	// Verify patient was created with SELF_REGISTERED source
	patientRepo.AssertExpectations(t)
}

func TestCreateBooking_WithProcedure_UsesProcedureDuration(t *testing.T) {
	svc, clinicRepo, profRepo, aptRepo, patientRepo, notifSvc, _, procRepo, _ := newTestService()
	ctx := context.Background()

	clinic := newTestClinic()
	clinic.Settings.DefaultAppointmentMinutes = 50

	profID := uuid.New()
	procID := uuid.New()
	professional := &professionalModel.Professional{ID: profID, ClinicID: clinic.ID, Name: "Dr. Ana"}
	procedure := &procedureModel.Procedure{ID: procID, ClinicID: clinic.ID, DurationMinutes: 30}

	futureTime := time.Now().Add(24 * time.Hour)
	expectedEnd := futureTime.Add(30 * time.Minute) // 30 min from procedure, not 50

	clinicRepo.On("GetBySlug", ctx, "clinica-teste").Return(clinic, nil)
	profRepo.On("GetByID", ctx, profID, clinic.ID).Return(professional, nil)
	procRepo.On("GetByID", procID, clinic.ID).Return(procedure, nil)
	aptRepo.On("CheckConflict", ctx, clinic.ID, profID, futureTime, mock.Anything, mock.Anything).Return(false, nil)
	patientRepo.On("Create", ctx, mock.AnythingOfType("*model.Patient")).Return(nil)
	aptRepo.On("Create", ctx, mock.MatchedBy(func(a *appointmentModel.Appointment) bool {
		return a.EndTime.Equal(expectedEnd)
	})).Return(nil)
	notifSvc.On("Create", ctx, mock.AnythingOfType("*model.Notification")).Return(nil)

	result, err := svc.CreateBooking(ctx, "clinica-teste", &bookingModel.CreateBookingRequest{
		ProfessionalID: profID,
		StartTime:      futureTime,
		ProcedureID:    &procID,
		PatientName:    "João",
		PatientPhone:   "11888887777",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Verify procedure duration was used (30 min)
	assert.Equal(t, expectedEnd, result.EndTime)
}

func TestGetSpecialties_ReturnsActiveOnly(t *testing.T) {
	svc, clinicRepo, _, _, _, _, specRepo, _, _ := newTestService()
	ctx := context.Background()

	clinic := newTestClinic()
	specialties := []*specialtyModel.Specialty{
		{ID: uuid.New(), ClinicID: clinic.ID, Name: "Psicologia", Active: true},
		{ID: uuid.New(), ClinicID: clinic.ID, Name: "Fisioterapia", Active: true},
	}

	clinicRepo.On("GetBySlug", ctx, "clinica-teste").Return(clinic, nil)
	specRepo.On("ListActiveByClinic", clinic.ID).Return(specialties, nil)

	result, err := svc.GetSpecialties(ctx, "clinica-teste")

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Psicologia", result[0].Name)
}

func TestCreateBooking_RequirePayment_WithPricedProcedure_SetsAwaitingPayment(t *testing.T) {
	svc, clinicRepo, profRepo, aptRepo, patientRepo, notifSvc, _, procRepo, _ := newTestService()
	ctx := context.Background()

	clinic := newTestClinic()
	clinic.Settings.RequirePaymentBeforeBooking = true

	profID := uuid.New()
	procID := uuid.New()
	professional := &professionalModel.Professional{ID: profID, ClinicID: clinic.ID, Name: "Dr. Ana"}
	procedure := &procedureModel.Procedure{ID: procID, ClinicID: clinic.ID, DurationMinutes: 30, Price: 150.0}

	futureTime := time.Now().Add(24 * time.Hour)

	clinicRepo.On("GetBySlug", ctx, "clinica-teste").Return(clinic, nil)
	profRepo.On("GetByID", ctx, profID, clinic.ID).Return(professional, nil)
	procRepo.On("GetByID", procID, clinic.ID).Return(procedure, nil)
	aptRepo.On("CheckConflict", ctx, clinic.ID, profID, futureTime, mock.Anything, mock.Anything).Return(false, nil)
	patientRepo.On("Create", ctx, mock.AnythingOfType("*model.Patient")).Return(nil)
	aptRepo.On("Create", ctx, mock.MatchedBy(func(a *appointmentModel.Appointment) bool {
		return a.Status == appointmentModel.AppointmentStatusAwaitingPayment && a.Price == 150.0
	})).Return(nil)
	notifSvc.On("Create", ctx, mock.AnythingOfType("*model.Notification")).Return(nil)

	result, err := svc.CreateBooking(ctx, "clinica-teste", &bookingModel.CreateBookingRequest{
		ProfessionalID: profID,
		StartTime:      futureTime,
		ProcedureID:    &procID,
		PatientName:    "João",
		PatientPhone:   "11888887777",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "awaiting_payment", result.Status)
	assert.Equal(t, 150.0, result.Price)
}

func TestCreateBooking_RequirePayment_NoProcedure_ReturnsError(t *testing.T) {
	svc, clinicRepo, profRepo, aptRepo, patientRepo, _, _, _, _ := newTestService()
	ctx := context.Background()

	clinic := newTestClinic()
	clinic.Settings.RequirePaymentBeforeBooking = true

	profID := uuid.New()
	professional := &professionalModel.Professional{ID: profID, ClinicID: clinic.ID, Name: "Dr. Ana"}

	futureTime := time.Now().Add(24 * time.Hour)

	clinicRepo.On("GetBySlug", ctx, "clinica-teste").Return(clinic, nil)
	profRepo.On("GetByID", ctx, profID, clinic.ID).Return(professional, nil)
	aptRepo.On("CheckConflict", ctx, clinic.ID, profID, futureTime, mock.Anything, mock.Anything).Return(false, nil)
	patientRepo.On("Create", ctx, mock.AnythingOfType("*model.Patient")).Return(nil)

	_, err := svc.CreateBooking(ctx, "clinica-teste", &bookingModel.CreateBookingRequest{
		ProfessionalID: profID,
		StartTime:      futureTime,
		PatientName:    "João",
		PatientPhone:   "11888887777",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pagamento antecipado")
}

func TestCreateBooking_RequirePayment_ProcedureWithNoPrice_ReturnsError(t *testing.T) {
	svc, clinicRepo, profRepo, aptRepo, patientRepo, _, _, procRepo, _ := newTestService()
	ctx := context.Background()

	clinic := newTestClinic()
	clinic.Settings.RequirePaymentBeforeBooking = true

	profID := uuid.New()
	procID := uuid.New()
	professional := &professionalModel.Professional{ID: profID, ClinicID: clinic.ID, Name: "Dr. Ana"}
	procedure := &procedureModel.Procedure{ID: procID, ClinicID: clinic.ID, DurationMinutes: 30, Price: 0}

	futureTime := time.Now().Add(24 * time.Hour)

	clinicRepo.On("GetBySlug", ctx, "clinica-teste").Return(clinic, nil)
	profRepo.On("GetByID", ctx, profID, clinic.ID).Return(professional, nil)
	procRepo.On("GetByID", procID, clinic.ID).Return(procedure, nil)
	aptRepo.On("CheckConflict", ctx, clinic.ID, profID, futureTime, mock.Anything, mock.Anything).Return(false, nil)
	patientRepo.On("Create", ctx, mock.AnythingOfType("*model.Patient")).Return(nil)

	_, err := svc.CreateBooking(ctx, "clinica-teste", &bookingModel.CreateBookingRequest{
		ProfessionalID: profID,
		StartTime:      futureTime,
		ProcedureID:    &procID,
		PatientName:    "João",
		PatientPhone:   "11888887777",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pagamento antecipado")
}

func TestCreateBooking_NoPaymentRequired_NoProcedure_Works(t *testing.T) {
	svc, clinicRepo, profRepo, aptRepo, patientRepo, notifSvc, _, _, _ := newTestService()
	ctx := context.Background()

	clinic := newTestClinic()
	clinic.Settings.RequirePaymentBeforeBooking = false

	profID := uuid.New()
	professional := &professionalModel.Professional{ID: profID, ClinicID: clinic.ID, Name: "Dr. Ana"}

	futureTime := time.Now().Add(24 * time.Hour)

	clinicRepo.On("GetBySlug", ctx, "clinica-teste").Return(clinic, nil)
	profRepo.On("GetByID", ctx, profID, clinic.ID).Return(professional, nil)
	aptRepo.On("CheckConflict", ctx, clinic.ID, profID, futureTime, mock.Anything, mock.Anything).Return(false, nil)
	patientRepo.On("Create", ctx, mock.AnythingOfType("*model.Patient")).Return(nil)
	aptRepo.On("Create", ctx, mock.MatchedBy(func(a *appointmentModel.Appointment) bool {
		return a.Status == appointmentModel.AppointmentStatusPending && a.Price == 0
	})).Return(nil)
	notifSvc.On("Create", ctx, mock.AnythingOfType("*model.Notification")).Return(nil)

	result, err := svc.CreateBooking(ctx, "clinica-teste", &bookingModel.CreateBookingRequest{
		ProfessionalID: profID,
		StartTime:      futureTime,
		PatientName:    "João",
		PatientPhone:   "11888887777",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "pending", result.Status)
	assert.Equal(t, 0.0, result.Price)
}

func TestGetProcedures_FilteredBySpecialty(t *testing.T) {
	svc, clinicRepo, _, _, _, _, _, procRepo, _ := newTestService()
	ctx := context.Background()

	clinic := newTestClinic()
	specID := uuid.New()
	procedures := []*procedureModel.Procedure{
		{ID: uuid.New(), ClinicID: clinic.ID, Name: "Consulta", SpecialtyID: &specID, Price: 200, DurationMinutes: 50},
	}

	clinicRepo.On("GetBySlug", ctx, "clinica-teste").Return(clinic, nil)
	procRepo.On("ListByClinicAndSpecialty", ctx, clinic.ID, &specID).Return(procedures, nil)

	result, err := svc.GetProcedures(ctx, "clinica-teste", &specID, nil)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Consulta", result[0].Name)
	assert.Equal(t, 200.0, result[0].Price)
}
