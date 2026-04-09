package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/attendance/model"
	"github.com/aseno/backend/pkg/attendance/repository"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
)

// sessionPlanReader is the subset of SessionPlanRepository needed here
type sessionPlanReader interface {
	GetByID(ctx context.Context, clinicID, id uuid.UUID) (*sessionplan_model.SessionPlan, error)
}

// GroupSessionProfessionalService handles professional assignment for group sessions
type GroupSessionProfessionalService interface {
	SetProfessional(ctx context.Context, clinicID, sessionPlanID uuid.UUID, date string, professionalID uuid.UUID) (*model.GroupSessionProfessional, error)
	GetProfessional(ctx context.Context, clinicID, sessionPlanID uuid.UUID, date string) (*model.GroupSessionProfessional, error)
}

type groupSessionProfessionalService struct {
	repo     repository.GroupSessionProfessionalRepository
	planRepo sessionPlanReader
}

// NewGroupSessionProfessionalService creates a new service
func NewGroupSessionProfessionalService(
	repo repository.GroupSessionProfessionalRepository,
	planRepo sessionPlanReader,
) GroupSessionProfessionalService {
	return &groupSessionProfessionalService{
		repo:     repo,
		planRepo: planRepo,
	}
}

// SetProfessional assigns a professional to a specific group session date.
// For fixed-professional plans, validates that the professional matches the plan's professional.
// For flexible plans, accepts any professional.
func (s *groupSessionProfessionalService) SetProfessional(
	ctx context.Context,
	clinicID, sessionPlanID uuid.UUID,
	date string,
	professionalID uuid.UUID,
) (*model.GroupSessionProfessional, error) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	if plan.Type != sessionplan_model.PlanTypeGroup {
		return nil, errors.ErrValidationInstance.WithMessage("Somente planos de grupo suportam profissional por sessão.")
	}

	// For fixed professional plans, the professional must match the plan's professional
	if plan.ProfessionalID != nil && *plan.ProfessionalID != professionalID {
		return nil, errors.ErrValidationInstance.WithMessage("Este plano tem profissional fixo. Não é possível alterar.")
	}

	record := &model.GroupSessionProfessional{
		ClinicID:       clinicID,
		SessionPlanID:  sessionPlanID,
		SessionDate:    date,
		ProfessionalID: professionalID,
	}

	if err := s.repo.Upsert(ctx, record); err != nil {
		return nil, err
	}

	// Re-fetch to get relations
	return s.repo.GetByPlanAndDate(ctx, clinicID, sessionPlanID, date)
}

// GetProfessional returns the professional assigned to a specific group session date.
// If no specific assignment exists but the plan has a fixed professional, returns nil
// (the caller should fall back to plan.ProfessionalID).
func (s *groupSessionProfessionalService) GetProfessional(
	ctx context.Context,
	clinicID, sessionPlanID uuid.UUID,
	date string,
) (*model.GroupSessionProfessional, error) {
	return s.repo.GetByPlanAndDate(ctx, clinicID, sessionPlanID, date)
}
