package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
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
}

// NewPatientWhatsAppConnectionService creates a new service
func NewPatientWhatsAppConnectionService(
	repo repository.PatientWhatsAppConnectionRepository,
	patientRepo repository.PatientRepository,
	whatsappRepo whatsapp_repository.WhatsAppConnectionRepository,
) PatientWhatsAppConnectionService {
	return &patientWhatsAppConnectionService{
		repo:         repo,
		patientRepo:  patientRepo,
		whatsappRepo: whatsappRepo,
	}
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

	return conn.ToResponse(), nil
}

// Update creates or updates the connection config for a patient
func (s *patientWhatsAppConnectionService) Update(ctx context.Context, patientID, clinicID uuid.UUID, req *model.UpdatePatientWhatsAppConnectionRequest) (*model.PatientWhatsAppConnectionResponse, error) {
	// Verify patient belongs to clinic
	_, err := s.patientRepo.GetByID(ctx, patientID, clinicID)
	if err != nil {
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

	return conn.ToResponse(), nil
}
