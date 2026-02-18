package service

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/clinic/model"
	"github.com/aseno/backend/pkg/clinic/repository"
)

// OnboardingService defines the interface for onboarding operations
type OnboardingService interface {
	GetStatus(ctx context.Context, clinicID uuid.UUID) (*model.OnboardingStatus, error)
}

// onboardingService implements OnboardingService
type onboardingService struct {
	db         *gorm.DB
	clinicRepo repository.ClinicRepository
}

// NewOnboardingService creates a new onboarding service
func NewOnboardingService(db *gorm.DB, clinicRepo repository.ClinicRepository) OnboardingService {
	return &onboardingService{
		db:         db,
		clinicRepo: clinicRepo,
	}
}

// GetStatus returns the current onboarding status for a clinic
func (s *onboardingService) GetStatus(ctx context.Context, clinicID uuid.UUID) (*model.OnboardingStatus, error) {
	// Get clinic info
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	// Count professionals
	var profCount int64
	if err := s.db.WithContext(ctx).
		Table("professionals").
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID).
		Count(&profCount).Error; err != nil {
		return nil, err
	}

	// Check if any professional has schedule configured
	var hasSchedule bool
	if profCount > 0 {
		var scheduleCount int64
		// Check if working_hours JSON is not empty/null
		if err := s.db.WithContext(ctx).
			Table("professionals").
			Where("clinic_id = ? AND deleted_at IS NULL", clinicID).
			Where("working_hours IS NOT NULL AND working_hours != '{}'::jsonb AND working_hours != 'null'").
			Count(&scheduleCount).Error; err != nil {
			return nil, err
		}
		hasSchedule = scheduleCount > 0
	}

	// Count appointments
	var aptCount int64
	if err := s.db.WithContext(ctx).
		Table("appointments").
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID).
		Count(&aptCount).Error; err != nil {
		return nil, err
	}

	// Check clinic info completeness
	clinicInfoComplete := clinic.Name != "" && clinic.Phone != "" && clinic.Email != ""

	// Calculate progress (5 steps, each worth 20%)
	steps := []bool{
		clinicInfoComplete,       // Step 1: Clinic info
		profCount > 0,            // Step 2: At least one professional
		hasSchedule,              // Step 3: Schedule configured
		clinic.WhatsAppConnected, // Step 4: WhatsApp connected
		aptCount > 0,             // Step 5: At least one appointment
	}

	completed := 0
	for _, step := range steps {
		if step {
			completed++
		}
	}
	progress := (completed * 100) / len(steps)

	// Determine next step
	nextStep := model.StepComplete
	if !clinicInfoComplete {
		nextStep = model.StepClinicInfo
	} else if profCount == 0 {
		nextStep = model.StepProfessionals
	} else if !hasSchedule {
		nextStep = model.StepSchedule
	} else if !clinic.WhatsAppConnected {
		nextStep = model.StepWhatsApp
	} else if aptCount == 0 {
		nextStep = model.StepFirstAppt
	}

	return &model.OnboardingStatus{
		ClinicInfoComplete:    clinicInfoComplete,
		ProfessionalsCount:    int(profCount),
		HasScheduleConfigured: hasSchedule,
		WhatsAppConnected:     clinic.WhatsAppConnected,
		GoogleConnected:       clinic.GoogleConnected,
		AppointmentsCount:     int(aptCount),
		Progress:              progress,
		NextStep:              string(nextStep),
		IsComplete:            progress == 100,
	}, nil
}
