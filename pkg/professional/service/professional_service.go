package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/internal/planlimit"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/professional/model"
	"github.com/aseno/backend/pkg/professional/repository"
	specialty_repository "github.com/aseno/backend/pkg/specialty/repository"
	user_model "github.com/aseno/backend/pkg/user/model"
	user_repository "github.com/aseno/backend/pkg/user/repository"
)

// ProfessionalService defines the interface for professional business logic
type ProfessionalService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateProfessionalRequest) (*model.ProfessionalResponse, error)
	CreateForExistingUser(ctx context.Context, clinicID, userID uuid.UUID) (*model.ProfessionalResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.ProfessionalResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.ProfessionalResponse], error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateProfessionalRequest) (*model.ProfessionalResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	BulkDelete(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error
}

// professionalService implements ProfessionalService
type professionalService struct {
	repo            repository.ProfessionalRepository
	clinicRepo      clinic_repository.ClinicRepository
	appointmentRepo appointment_repository.AppointmentRepository
	userRepo        user_repository.UserRepository
	specialtyRepo   specialty_repository.SpecialtyRepository
}

// NewProfessionalService creates a new professional service
func NewProfessionalService(repo repository.ProfessionalRepository, clinicRepo clinic_repository.ClinicRepository, appointmentRepo appointment_repository.AppointmentRepository, userRepo user_repository.UserRepository, specialtyRepo specialty_repository.SpecialtyRepository) ProfessionalService {
	return &professionalService{
		repo:            repo,
		clinicRepo:      clinicRepo,
		appointmentRepo: appointmentRepo,
		userRepo:        userRepo,
		specialtyRepo:   specialtyRepo,
	}
}

// Create creates a new professional
func (s *professionalService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateProfessionalRequest) (*model.ProfessionalResponse, error) {
	// 1. Check plan limits via Account (source of truth)
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	if clinic.Account != nil {
		limit := clinic.Account.MaxProfessionals
		if !planlimit.IsUnlimited(limit) {
			count, err := s.repo.CountActiveByClinicID(ctx, clinicID)
			if err != nil {
				return nil, err
			}
			if int(count) >= limit {
				return nil, errors.ErrValidationInstance.WithMessage(
					fmt.Sprintf("Limite de %d profissionais por unidade atingido. Faça upgrade para adicionar mais.", limit))
			}
		}
	}

	// 2. Create professional
	professional := &model.Professional{
		ClinicID:        clinicID,
		Name:            req.Name,
		Email:           req.Email,
		Phone:           req.Phone,
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

	if len(req.SpecialtyIDs) > 0 {
		if err := s.syncSpecialties(professional.ID, clinicID, req.SpecialtyIDs); err != nil {
			return nil, err
		}
	}

	return s.buildResponse(professional), nil
}

// syncSpecialties validates that every specialty belongs to the clinic and
// replaces the professional's specialty set. Passing an empty slice clears all
// specialties; callers that want "no change" must skip this call entirely.
func (s *professionalService) syncSpecialties(professionalID, clinicID uuid.UUID, specialtyIDs []uuid.UUID) error {
	if s.specialtyRepo == nil {
		return nil
	}
	for _, sid := range specialtyIDs {
		if _, err := s.specialtyRepo.GetByID(sid, clinicID); err != nil {
			return errors.ErrValidationInstance.WithMessage(
				fmt.Sprintf("specialty %s does not belong to this clinic", sid))
		}
	}
	if err := s.specialtyRepo.SetProfessionalSpecialties(professionalID, clinicID, specialtyIDs); err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// buildResponse converts a Professional to a response populating the
// Specialties field from the repository, since ToResponse leaves it empty.
func (s *professionalService) buildResponse(p *model.Professional) *model.ProfessionalResponse {
	resp := p.ToResponse()
	if s.specialtyRepo == nil {
		return resp
	}
	specs, err := s.specialtyRepo.GetByProfessionalID(p.ID)
	if err != nil || len(specs) == 0 {
		return resp
	}
	infos := make([]model.SpecialtyInfo, 0, len(specs))
	for _, sp := range specs {
		infos = append(infos, model.SpecialtyInfo{ID: sp.ID, Name: sp.Name})
	}
	resp.Specialties = infos
	return resp
}

// CreateForExistingUser creates a professional linked to an existing user (owner or admin)
func (s *professionalService) CreateForExistingUser(ctx context.Context, clinicID, userID uuid.UUID) (*model.ProfessionalResponse, error) {
	// Validate user exists and belongs to clinic
	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return nil, err
	}

	// Only owner or admin can be linked as professional
	if user.Role != user_model.UserRoleOwner && user.Role != user_model.UserRoleAdmin {
		return nil, errors.ErrValidationInstance.WithMessage(
			"Somente usuários com papel owner ou admin podem ser vinculados como profissional")
	}

	// Check if user already has a professional linked
	existingProf, err := s.repo.GetByUserID(ctx, userID, clinicID)
	if err == nil && existingProf != nil {
		return nil, errors.ErrValidationInstance.WithMessage(
			"Este usuário já possui um profissional vinculado")
	}

	// Check plan limits
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	if clinic.Account != nil {
		limit := clinic.Account.MaxProfessionals
		if !planlimit.IsUnlimited(limit) {
			count, err := s.repo.CountActiveByClinicID(ctx, clinicID)
			if err != nil {
				return nil, err
			}
			if int(count) >= limit {
				return nil, errors.ErrValidationInstance.WithMessage(
					fmt.Sprintf("Limite de %d profissionais por unidade atingido. Faça upgrade para adicionar mais.", limit))
			}
		}
	}

	// Create professional inheriting user data
	defaultSlot := []model.TimeSlot{{Start: "09:00", End: "18:00"}}
	professional := &model.Professional{
		ClinicID: clinicID,
		UserID:   &userID,
		Name:     user.Name,
		Email:    user.Email,
		Phone:    user.Phone,
		Color:    "#7C3AED",
		IsActive: true,
		WorkingHours: model.WorkingHours{
			Monday:    defaultSlot,
			Tuesday:   defaultSlot,
			Wednesday: defaultSlot,
			Thursday:  defaultSlot,
			Friday:    defaultSlot,
		},
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
	return s.buildResponse(professional), nil
}

// List retrieves a paginated list of professionals
func (s *professionalService) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.ProfessionalResponse], error) {
	professionals, total, err := s.repo.List(ctx, clinicID, filter)
	if err != nil {
		return pagination.Response[*model.ProfessionalResponse]{}, err
	}

	data := make([]*model.ProfessionalResponse, len(professionals))
	specialtiesByProf := map[uuid.UUID][]*model.SpecialtyInfo{}
	if s.specialtyRepo != nil && len(professionals) > 0 {
		ids := make([]uuid.UUID, len(professionals))
		for i, p := range professionals {
			ids[i] = p.ID
		}
		if specs, err := s.specialtyRepo.GetByProfessionalIDs(ids); err == nil {
			for profID, list := range specs {
				infos := make([]*model.SpecialtyInfo, 0, len(list))
				for _, sp := range list {
					infos = append(infos, &model.SpecialtyInfo{ID: sp.ID, Name: sp.Name})
				}
				specialtiesByProf[profID] = infos
			}
		}
	}

	for i, p := range professionals {
		resp := p.ToResponse()
		if infos, ok := specialtiesByProf[p.ID]; ok {
			resp.Specialties = make([]model.SpecialtyInfo, len(infos))
			for j, info := range infos {
				resp.Specialties[j] = *info
			}
		}
		data[i] = resp
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

	if req.SpecialtyIDs != nil {
		if err := s.syncSpecialties(professional.ID, clinicID, *req.SpecialtyIDs); err != nil {
			return nil, err
		}
	}

	return s.buildResponse(professional), nil
}

// Delete deletes a professional (blocked if has appointments or linked user)
func (s *professionalService) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	professional, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return err
	}

	// Block delete if professional has a linked user
	if professional.UserID != nil {
		return errors.ErrValidationInstance.WithMessage(
			"Este profissional possui um usuário vinculado. Desative-o ao invés de excluir.")
	}

	// Block delete if professional has appointments
	if s.appointmentRepo != nil {
		count, err := s.appointmentRepo.CountByProfessionalID(ctx, id, clinicID)
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.ErrValidationInstance.WithMessage(
				fmt.Sprintf("Este profissional possui %d agendamento(s). Desative-o ao invés de excluir.", count))
		}
	}

	return s.repo.Delete(ctx, id, clinicID)
}

// BulkDelete soft deletes multiple professionals (blocks any that have appointments or linked users)
func (s *professionalService) BulkDelete(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error {
	for _, id := range ids {
		professional, err := s.repo.GetByID(ctx, id, clinicID)
		if err != nil {
			return err
		}

		if professional.UserID != nil {
			return errors.ErrValidationInstance.WithMessage(
				fmt.Sprintf("O profissional '%s' possui um usuário vinculado. Desative-o ao invés de excluir.", professional.Name))
		}

		if s.appointmentRepo != nil {
			count, err := s.appointmentRepo.CountByProfessionalID(ctx, id, clinicID)
			if err != nil {
				return err
			}
			if count > 0 {
				return errors.ErrValidationInstance.WithMessage(
					fmt.Sprintf("O profissional '%s' possui %d agendamento(s). Desative-o ao invés de excluir.", professional.Name, count))
			}
		}
	}

	return s.repo.BulkDelete(ctx, ids, clinicID)
}
