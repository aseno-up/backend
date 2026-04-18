package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/patient/model"
	"github.com/aseno/backend/pkg/patient/repository"
	whatsapp_repository "github.com/aseno/backend/pkg/whatsapp/repository"
)

// PatientWhatsAppConnectionService defines the interface for patient-whatsapp connection operations
type PatientWhatsAppConnectionService interface {
	GetByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientWhatsAppConnectionResponse, error)
	Update(ctx context.Context, patientID, clinicID uuid.UUID, req *model.UpdatePatientWhatsAppConnectionRequest) (*model.PatientWhatsAppConnectionResponse, error)
}

// patientWhatsAppConnectionService implements PatientWhatsAppConnectionService
type patientWhatsAppConnectionService struct {
	repo         repository.PatientWhatsAppConnectionRepository
	patientRepo  repository.PatientRepository
	whatsappRepo whatsapp_repository.WhatsAppConnectionRepository
	clinicRepo   clinic_repository.ClinicRepository
}

// NewPatientWhatsAppConnectionService creates a new service
func NewPatientWhatsAppConnectionService(
	repo repository.PatientWhatsAppConnectionRepository,
	patientRepo repository.PatientRepository,
	whatsappRepo whatsapp_repository.WhatsAppConnectionRepository,
	clinicRepo clinic_repository.ClinicRepository,
) PatientWhatsAppConnectionService {
	return &patientWhatsAppConnectionService{
		repo:         repo,
		patientRepo:  patientRepo,
		whatsappRepo: whatsappRepo,
		clinicRepo:   clinicRepo,
	}
}

// takeoverPauseDuration returns the clinic's configured AI pause window. Used
// to compute HumanTakeoverUntil on the response so the frontend can render the
// "IA pausada até HH:MM" badge without a second API call.
func (s *patientWhatsAppConnectionService) takeoverPauseDuration(ctx context.Context, clinicID uuid.UUID) time.Duration {
	if s.clinicRepo == nil {
		return 0
	}
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil || clinic == nil {
		return 0
	}
	return clinic.Settings.GetHumanTakeoverPauseDuration()
}

// GetByPatientID returns the connection config for a patient
func (s *patientWhatsAppConnectionService) GetByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientWhatsAppConnectionResponse, error) {
	// Verify patient belongs to clinic
	_, err := s.patientRepo.GetByID(ctx, patientID, clinicID)
	if err != nil {
		return nil, err
	}

	conn, err := s.repo.GetByPatientID(ctx, patientID, clinicID)
	if err != nil {
		// If not found, return defaults (everything enabled)
		if errors.IsNotFound(err) {
			return &model.PatientWhatsAppConnectionResponse{
				PatientID:           patientID,
				ClinicID:            clinicID,
				AISchedulingEnabled: true,
				RemindersEnabled:    true,
			}, nil
		}
		return nil, err
	}

	return conn.ToResponse(s.takeoverPauseDuration(ctx, clinicID)), nil
}

// Update creates or updates the connection config for a patient
func (s *patientWhatsAppConnectionService) Update(ctx context.Context, patientID, clinicID uuid.UUID, req *model.UpdatePatientWhatsAppConnectionRequest) (*model.PatientWhatsAppConnectionResponse, error) {
	// Verify patient belongs to clinic
	if _, err := s.patientRepo.GetByID(ctx, patientID, clinicID); err != nil {
		return nil, err
	}

	// Get active whatsapp connection for the clinic
	whatsappConn, err := s.whatsappRepo.GetActiveByClinicID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrValidationInstance.WithMessage("clinic has no active WhatsApp connection")
	}

	// Get existing or create new
	conn, err := s.repo.GetByPatientID(ctx, patientID, clinicID)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if conn == nil {
		conn = &model.PatientWhatsAppConnection{
			ClinicID:             clinicID,
			PatientID:            patientID,
			WhatsAppConnectionID: whatsappConn.ID,
			AISchedulingEnabled:  true,
			RemindersEnabled:     true,
		}
	}

	// Apply partial updates
	if req.AISchedulingEnabled != nil {
		conn.AISchedulingEnabled = *req.AISchedulingEnabled
	}
	if req.RemindersEnabled != nil {
		conn.RemindersEnabled = *req.RemindersEnabled
	}

	if err := s.repo.Upsert(ctx, conn); err != nil {
		return nil, err
	}

	// Explicit AI re-enable overrides any active human-takeover pause —
	// otherwise the gate would keep silencing the AI until the window
	// expired. Cleared via the same repo, same table, same row.
	if req.AISchedulingEnabled != nil && *req.AISchedulingEnabled {
		if err := s.repo.SetHumanTakeoverAt(ctx, patientID, clinicID, whatsappConn.ID, nil); err != nil {
			// Non-fatal: the re-enable already succeeded, takeover just
			// didn't clear. Log via caller through returned response.
			_ = err
		}
		conn.HumanTakeoverAt = nil
	}

	return conn.ToResponse(s.takeoverPauseDuration(ctx, clinicID)), nil
}
