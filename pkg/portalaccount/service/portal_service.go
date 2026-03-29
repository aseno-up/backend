package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	appointmentModel "github.com/aseno/backend/pkg/appointment/model"
	clinicModel "github.com/aseno/backend/pkg/clinic/model"
	emailService "github.com/aseno/backend/pkg/email/service"
	notificationModel "github.com/aseno/backend/pkg/notification/model"
	patientModel "github.com/aseno/backend/pkg/patient/model"
	procedureModel "github.com/aseno/backend/pkg/procedure/model"
	professionalModel "github.com/aseno/backend/pkg/professional/model"
	"github.com/aseno/backend/pkg/portalaccount/model"
	"github.com/aseno/backend/pkg/portalaccount/repository"
)

var (
	ErrInvalidToken       = errors.New("token inválido ou expirado")
	ErrAccountNotFound    = errors.New("conta não encontrada")
	ErrEmailNotVerified   = errors.New("email ainda não foi verificado")
	ErrInvalidCredentials = errors.New("email ou senha inválidos")
	ErrAlreadyRegistered  = errors.New("conta já possui senha cadastrada")
)

// PortalJWTClaims contains the claims for portal JWT tokens
type PortalJWTClaims struct {
	PatientID uuid.UUID `json:"patient_id"`
	ClinicID  uuid.UUID `json:"clinic_id"`
	Email     string    `json:"email"`
	Type      string    `json:"type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// PortalTokenPair contains access and refresh tokens
type PortalTokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Providers for external dependencies
type ClinicProvider interface {
	GetByID(ctx context.Context, id uuid.UUID) (*clinicModel.Clinic, error)
}

type PatientProvider interface {
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*patientModel.Patient, error)
}

type AppointmentProvider interface {
	GetByPatient(ctx context.Context, patientID, clinicID uuid.UUID, limit int) ([]*appointmentModel.Appointment, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*appointmentModel.Appointment, error)
	Create(ctx context.Context, appointment *appointmentModel.Appointment) error
	Update(ctx context.Context, appointment *appointmentModel.Appointment) error
	CheckConflict(ctx context.Context, clinicID, professionalID uuid.UUID, start, end time.Time, excludeID *uuid.UUID) (bool, error)
}

type ProfessionalProvider interface {
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*professionalModel.Professional, error)
}

type ProcedureProvider interface {
	GetByID(id, clinicID uuid.UUID) (*procedureModel.Procedure, error)
}

type NotificationCreator interface {
	Create(ctx context.Context, notification *notificationModel.Notification) error
}

// PortalService defines the interface for portal operations
type PortalService interface {
	// Auth
	ValidateRegistrationToken(ctx context.Context, token string) (*model.ValidateTokenResponse, error)
	Register(ctx context.Context, token, password string) error
	VerifyEmail(ctx context.Context, token string) error
	Login(ctx context.Context, email, password string) (*PortalTokenPair, *model.PortalMeResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*PortalTokenPair, error)
	ValidateAccessToken(tokenStr string) (*PortalJWTClaims, error)

	// Data
	GetMe(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PortalMeResponse, error)
	GetAppointments(ctx context.Context, patientID, clinicID uuid.UUID) ([]*appointmentModel.Appointment, error)
	GetStats(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PortalStatsResponse, error)
	CancelAppointment(ctx context.Context, patientID, clinicID, appointmentID uuid.UUID) error
	CreateAppointment(ctx context.Context, patientID, clinicID uuid.UUID, req *model.PortalCreateAppointmentRequest) (*model.PortalAppointmentResponse, error)
}

type portalService struct {
	repo             repository.PortalAccountRepository
	clinicRepo       ClinicProvider
	patientRepo      PatientProvider
	appointmentRepo  AppointmentProvider
	professionalRepo ProfessionalProvider
	procedureRepo    ProcedureProvider
	notificationSvc  NotificationCreator
	emailSvc         emailService.EmailService
	jwtSecret        string
	frontendURL      string
}

func NewPortalService(
	repo repository.PortalAccountRepository,
	clinicRepo ClinicProvider,
	patientRepo PatientProvider,
	appointmentRepo AppointmentProvider,
	professionalRepo ProfessionalProvider,
	procedureRepo ProcedureProvider,
	notificationSvc NotificationCreator,
	emailSvc emailService.EmailService,
	jwtSecret string,
	frontendURL string,
) PortalService {
	return &portalService{
		repo:             repo,
		clinicRepo:       clinicRepo,
		patientRepo:      patientRepo,
		appointmentRepo:  appointmentRepo,
		professionalRepo: professionalRepo,
		procedureRepo:    procedureRepo,
		notificationSvc:  notificationSvc,
		emailSvc:         emailSvc,
		jwtSecret:        jwtSecret,
		frontendURL:      frontendURL,
	}
}

// --- JWT ---

func (s *portalService) generateTokenPair(patientID, clinicID uuid.UUID, email string) (*PortalTokenPair, error) {
	accessToken, err := s.generateToken(patientID, clinicID, email, "access", 30*time.Minute)
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.generateToken(patientID, clinicID, email, "refresh", 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	return &PortalTokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *portalService) generateToken(patientID, clinicID uuid.UUID, email, tokenType string, ttl time.Duration) (string, error) {
	claims := &PortalJWTClaims{
		PatientID: patientID,
		ClinicID:  clinicID,
		Email:     email,
		Type:      tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   patientID.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *portalService) ValidateAccessToken(tokenStr string) (*PortalJWTClaims, error) {
	return s.validateToken(tokenStr, "access")
}

func (s *portalService) validateToken(tokenStr, expectedType string) (*PortalJWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &PortalJWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*PortalJWTClaims)
	if !ok || !token.Valid || claims.Type != expectedType {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// --- Auth ---

func (s *portalService) ValidateRegistrationToken(ctx context.Context, token string) (*model.ValidateTokenResponse, error) {
	account, err := s.repo.GetByRegistrationToken(ctx, token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !account.IsRegistrationTokenValid() {
		return nil, ErrInvalidToken
	}

	patient, err := s.patientRepo.GetByID(ctx, account.PatientID, account.ClinicID)
	if err != nil {
		return nil, ErrAccountNotFound
	}

	clinic, err := s.clinicRepo.GetByID(ctx, account.ClinicID)
	if err != nil {
		return nil, ErrAccountNotFound
	}

	return &model.ValidateTokenResponse{
		Email:       account.Email,
		PatientName: patient.Name,
		ClinicName:  clinic.Name,
	}, nil
}

func (s *portalService) Register(ctx context.Context, token, password string) error {
	account, err := s.repo.GetByRegistrationToken(ctx, token)
	if err != nil {
		return ErrInvalidToken
	}

	if !account.IsRegistrationTokenValid() {
		return ErrInvalidToken
	}

	if account.HasPassword() {
		return ErrAlreadyRegistered
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to save password: %w", err)
	}
	hashStr := string(hash)
	account.PasswordHash = &hashStr

	// Generate email verification token
	verifyToken, err := model.GenerateToken()
	if err != nil {
		return fmt.Errorf("failed to generate verification token: %w", err)
	}
	account.EmailVerificationToken = &verifyToken

	if err := s.repo.Update(ctx, account); err != nil {
		return err
	}

	// Send verification email
	if s.emailSvc != nil {
		clinic, _ := s.clinicRepo.GetByID(ctx, account.ClinicID)
		clinicName := ""
		if clinic != nil {
			clinicName = clinic.Name
		}

		verifyLink := fmt.Sprintf("%s/portal/verificar-email?token=%s", s.frontendURL, verifyToken)
		if err := s.emailSvc.SendPortalEmailVerification(ctx, &emailService.PortalEmailVerificationData{
			RecipientEmail: account.Email,
			RecipientName:  account.Email,
			ClinicName:     clinicName,
			VerifyLink:     verifyLink,
		}); err != nil {
			fmt.Printf("[PORTAL] Failed to send verification email to %s: %v\n", account.Email, err)
		}
	}

	return nil
}

func (s *portalService) VerifyEmail(ctx context.Context, token string) error {
	account, err := s.repo.GetByEmailVerificationToken(ctx, token)
	if err != nil {
		return ErrInvalidToken
	}

	now := time.Now()
	account.EmailVerifiedAt = &now
	account.EmailVerificationToken = nil

	return s.repo.Update(ctx, account)
}

func (s *portalService) Login(ctx context.Context, email, password string) (*PortalTokenPair, *model.PortalMeResponse, error) {
	account, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}

	if !account.HasPassword() {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*account.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if !account.IsEmailVerified() {
		return nil, nil, ErrEmailNotVerified
	}

	// Update last login
	now := time.Now()
	account.LastLoginAt = &now
	_ = s.repo.Update(ctx, account)

	tokens, err := s.generateTokenPair(account.PatientID, account.ClinicID, account.Email)
	if err != nil {
		return nil, nil, err
	}

	me, err := s.GetMe(ctx, account.PatientID, account.ClinicID)
	if err != nil {
		return nil, nil, err
	}

	return tokens, me, nil
}

func (s *portalService) RefreshToken(ctx context.Context, refreshToken string) (*PortalTokenPair, error) {
	claims, err := s.validateToken(refreshToken, "refresh")
	if err != nil {
		return nil, err
	}

	return s.generateTokenPair(claims.PatientID, claims.ClinicID, claims.Email)
}

// --- Data ---

func (s *portalService) GetMe(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PortalMeResponse, error) {
	patient, err := s.patientRepo.GetByID(ctx, patientID, clinicID)
	if err != nil {
		return nil, ErrAccountNotFound
	}

	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, ErrAccountNotFound
	}

	return &model.PortalMeResponse{
		PatientID:  patient.ID,
		ClinicID:   clinic.ID,
		ClinicName: clinic.Name,
		ClinicSlug: clinic.Slug,
		Name:       patient.Name,
		Email:      patient.Email,
		Phone:      patient.Phone,
	}, nil
}

func (s *portalService) GetAppointments(ctx context.Context, patientID, clinicID uuid.UUID) ([]*appointmentModel.Appointment, error) {
	return s.appointmentRepo.GetByPatient(ctx, patientID, clinicID, 500)
}

func (s *portalService) GetStats(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PortalStatsResponse, error) {
	appointments, err := s.appointmentRepo.GetByPatient(ctx, patientID, clinicID, 500)
	if err != nil {
		return nil, err
	}

	stats := &model.PortalStatsResponse{}
	now := time.Now()
	oneMonthAgo := now.AddDate(0, -1, 0)

	for _, apt := range appointments {
		stats.TotalAppointments++

		if apt.Status == appointmentModel.AppointmentStatusNoShow {
			stats.NoShowCount++
		}

		if apt.StartTime.After(oneMonthAgo) && apt.StartTime.Before(now) {
			stats.AppointmentsLastMonth++
		}

		// Find next future appointment
		if apt.StartTime.After(now) &&
			(apt.Status == appointmentModel.AppointmentStatusPending || apt.Status == appointmentModel.AppointmentStatusConfirmed) {
			if stats.NextAppointment == nil || apt.StartTime.Before(*stats.NextAppointment) {
				t := apt.StartTime
				stats.NextAppointment = &t
				stats.NextProfessionalName = apt.Professional.Name
			}
		}
	}

	return stats, nil
}

func (s *portalService) CancelAppointment(ctx context.Context, patientID, clinicID, appointmentID uuid.UUID) error {
	apt, err := s.appointmentRepo.GetByID(ctx, appointmentID, clinicID)
	if err != nil {
		return errors.New("agendamento não encontrado")
	}

	// Verify this appointment belongs to the patient
	if apt.PatientID != patientID {
		return errors.New("agendamento não encontrado")
	}

	// Only allow canceling future pending/confirmed appointments
	if apt.StartTime.Before(time.Now()) {
		return errors.New("não é possível cancelar agendamentos passados")
	}

	if apt.Status != appointmentModel.AppointmentStatusPending && apt.Status != appointmentModel.AppointmentStatusConfirmed {
		return errors.New("este agendamento não pode ser cancelado")
	}

	now := time.Now()
	apt.Status = appointmentModel.AppointmentStatusCanceled
	apt.CanceledAt = &now
	apt.CancelReason = "Cancelado pelo paciente via portal"

	return s.appointmentRepo.Update(ctx, apt)
}

// CreateAppointment creates an appointment for a logged-in portal patient (no new patient created)
func (s *portalService) CreateAppointment(ctx context.Context, patientID, clinicID uuid.UUID, req *model.PortalCreateAppointmentRequest) (*model.PortalAppointmentResponse, error) {
	// Verify patient exists
	patient, err := s.patientRepo.GetByID(ctx, patientID, clinicID)
	if err != nil {
		return nil, errors.New("paciente não encontrado")
	}

	// Verify professional exists
	professional, err := s.professionalRepo.GetByID(ctx, req.ProfessionalID, clinicID)
	if err != nil {
		return nil, errors.New("profissional não encontrado")
	}

	// Validate start time
	if req.StartTime.Before(time.Now()) {
		return nil, errors.New("a data e hora do agendamento devem ser no futuro")
	}

	// Calculate duration
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, errors.New("clínica não encontrada")
	}

	durationMinutes := clinic.Settings.DefaultAppointmentMinutes
	if durationMinutes == 0 {
		durationMinutes = 50
	}
	if req.ProcedureID != nil {
		if proc, err := s.procedureRepo.GetByID(*req.ProcedureID, clinicID); err == nil && proc.DurationMinutes > 0 {
			durationMinutes = proc.DurationMinutes
		}
	}
	endTime := req.StartTime.Add(time.Duration(durationMinutes) * time.Minute)

	// Check conflicts
	hasConflict, err := s.appointmentRepo.CheckConflict(ctx, clinicID, req.ProfessionalID, req.StartTime, endTime, nil)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, errors.New("este horário não está mais disponível")
	}

	// Create appointment using existing patient_id
	appointment := &appointmentModel.Appointment{
		ClinicID:       clinicID,
		PatientID:      patientID,
		ProfessionalID: req.ProfessionalID,
		StartTime:      req.StartTime,
		EndTime:        endTime,
		Status:         appointmentModel.AppointmentStatusPending,
		Type:           appointmentModel.AppointmentTypeRegular,
		Title:          "Agendamento via Portal",
		PatientNotes:   req.Notes,
	}

	if err := s.appointmentRepo.Create(ctx, appointment); err != nil {
		return nil, err
	}

	// Notification for clinic
	if s.notificationSvc != nil {
		notification := &notificationModel.Notification{
			ClinicID: clinicID,
			Type:     notificationModel.NotificationTypeNewBooking,
			Title:    "Novo Agendamento via Portal",
			Message:  fmt.Sprintf("%s agendou via portal com %s para %s", patient.Name, professional.Name, req.StartTime.Format("02/01 às 15:04")),
			Data: notificationModel.JSON{
				"appointment_id":    appointment.ID.String(),
				"patient_name":      patient.Name,
				"professional_name": professional.Name,
				"start_time":        req.StartTime.Format(time.RFC3339),
			},
		}
		_ = s.notificationSvc.Create(ctx, notification)
	}

	return &model.PortalAppointmentResponse{
		ID:           appointment.ID,
		Professional: professional.Name,
		StartTime:    appointment.StartTime,
		EndTime:      appointment.EndTime,
		Status:       string(appointment.Status),
	}, nil
}
