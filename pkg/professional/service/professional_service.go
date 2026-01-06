package service

import (
	"context"
	"math"

	"github.com/google/uuid"

	"github.com/saas/backend/pkg/professional/model"
	"github.com/saas/backend/pkg/professional/repository"
)

// ProfessionalService defines the interface for professional business logic
type ProfessionalService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateProfessionalRequest) (*model.ProfessionalResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.ProfessionalResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, activeOnly bool, page, perPage int) (*model.ProfessionalListResponse, error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateProfessionalRequest) (*model.ProfessionalResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
}

// professionalService implements ProfessionalService
type professionalService struct {
	repo repository.ProfessionalRepository
}

// NewProfessionalService creates a new professional service
func NewProfessionalService(repo repository.ProfessionalRepository) ProfessionalService {
	return &professionalService{repo: repo}
}

// Create creates a new professional
func (s *professionalService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateProfessionalRequest) (*model.ProfessionalResponse, error) {
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
func (s *professionalService) List(ctx context.Context, clinicID uuid.UUID, activeOnly bool, page, perPage int) (*model.ProfessionalListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	professionals, total, err := s.repo.GetByClinicID(ctx, clinicID, activeOnly, perPage, offset)
	if err != nil {
		return nil, err
	}

	data := make([]*model.ProfessionalResponse, len(professionals))
	for i, p := range professionals {
		data[i] = p.ToResponse()
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	return &model.ProfessionalListResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
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
