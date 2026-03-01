package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/professional/model"
	"github.com/aseno/backend/pkg/professional/repository"
	settings_service "github.com/aseno/backend/pkg/settings/service"
)

// ProfessionalService defines the interface for professional business logic
type ProfessionalService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateProfessionalRequest) (*model.ProfessionalResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.ProfessionalResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.ProfessionalResponse], error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateProfessionalRequest) (*model.ProfessionalResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
}

// professionalService implements ProfessionalService
type professionalService struct {
	repo       repository.ProfessionalRepository
	clinicRepo clinic_repository.ClinicRepository
	settings   settings_service.SettingsService
}

// NewProfessionalService creates a new professional service
func NewProfessionalService(repo repository.ProfessionalRepository, clinicRepo clinic_repository.ClinicRepository, settings settings_service.SettingsService) ProfessionalService {
	return &professionalService{
		repo:       repo,
		clinicRepo: clinicRepo,
		settings:   settings,
	}
}

// Create creates a new professional
func (s *professionalService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateProfessionalRequest) (*model.ProfessionalResponse, error) {
	// 1. Check plan limits
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	// Count active professionals (using default limit 1 to just get count)
	_, count, err := s.repo.List(ctx, clinicID, pagination.Filter{Limit: 1, Status: "active"})
	if err != nil {
		return nil, err
	}

	// Get limit from settings service
	limit := s.settings.GetPlanLimit(ctx, string(clinic.Plan))

	// Check if limit is exceeded (-1 means unlimited)
	if !s.settings.IsUnlimited(limit) && int(count) >= limit {
		// MVP: just block, no excess billing
		if !s.settings.IsExcessBillingEnabled(ctx) {
			return nil, errors.ErrValidationInstance.WithMessage(
				fmt.Sprintf("Limite de %d profissionais do plano %s atingido. Faça upgrade para adicionar mais.",
					limit, clinic.Plan))
		}
		// Future: allow excess with billing (not implemented yet)
	}

	// 2. Create professional
	professional := &model.Professional{
		ClinicID:        clinicID,
		Name:            req.Name,
		Email:           req.Email,
		Phone:           req.Phone,
		Specialty:       req.Specialty,
		RegistrationNum: req.RegistrationNum,
		Bio:             req.Bio,
		Color:           req.Color,
		IsActive:        true,
	}

	if req.WorkingHours != nil {
		professional.WorkingHours = *req.WorkingHours
	} else {
		// Default working hours: Mon-Fri 9:00-18:00
		defaultSlot := []model.TimeSlot{{Start: "09:00", End: "18:00"}}
		professional.WorkingHours = model.WorkingHours{
			Monday:    defaultSlot,
			Tuesday:   defaultSlot,
			Wednesday: defaultSlot,
			Thursday:  defaultSlot,
			Friday:    defaultSlot,
		}
	}

	if professional.Color == "" {
		professional.Color = "#7C3AED"
	}

	if err := s.repo.Create(ctx, professional); err != nil {
		return nil, err
	}

	return professional.ToResponse(), nil
}

// GetByID retrieves a professional by ID
func (s *professionalService) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.ProfessionalResponse, error) {
	professional, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}
	return professional.ToResponse(), nil
}

// List retrieves a paginated list of professionals
func (s *professionalService) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.ProfessionalResponse], error) {
	professionals, total, err := s.repo.List(ctx, clinicID, filter)
	if err != nil {
		return pagination.Response[*model.ProfessionalResponse]{}, err
	}

	data := make([]*model.ProfessionalResponse, len(professionals))
	for i, p := range professionals {
		data[i] = p.ToResponse()
	}

	return pagination.Response[*model.ProfessionalResponse]{
		Data: data,
		Meta: pagination.NewMeta(filter.GetPage(), filter.GetLimit(), total),
	}, nil
}

// Update updates a professional
func (s *professionalService) Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateProfessionalRequest) (*model.ProfessionalResponse, error) {
	professional, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Name != "" {
		professional.Name = req.Name
	}
	if req.Email != "" {
		professional.Email = req.Email
	}
	if req.Phone != "" {
		professional.Phone = req.Phone
	}
	if req.Specialty != "" {
		professional.Specialty = req.Specialty
	}
	if req.RegistrationNum != "" {
		professional.RegistrationNum = req.RegistrationNum
	}
	if req.AvatarURL != "" {
		professional.AvatarURL = req.AvatarURL
	}
	if req.Bio != "" {
		professional.Bio = req.Bio
	}
	if req.Color != "" {
		professional.Color = req.Color
	}
	if req.IsActive != nil {
		professional.IsActive = *req.IsActive
	}
	if req.WorkingHours != nil {
		professional.WorkingHours = *req.WorkingHours
	}

	if err := s.repo.Update(ctx, professional); err != nil {
		return nil, err
	}

	return professional.ToResponse(), nil
}

// Delete deletes a professional
func (s *professionalService) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.Delete(ctx, id, clinicID)
}
