package service

import (
	"errors"

	"github.com/aseno/backend/pkg/partnernetwork/model"
	"github.com/aseno/backend/pkg/partnernetwork/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPartnerPlanNotFound   = errors.New("plano de convênio não encontrado")
	ErrPartnerPlanNameExists = errors.New("já existe um plano com esse nome nesta rede")
)

type PartnerPlanService interface {
	Create(clinicID uuid.UUID, req *model.CreatePartnerPlanRequest) (*model.PartnerPlanResponse, error)
	GetByID(clinicID, id uuid.UUID) (*model.PartnerPlanResponse, error)
	Update(clinicID, id uuid.UUID, req *model.UpdatePartnerPlanRequest) (*model.PartnerPlanResponse, error)
	Delete(clinicID, id uuid.UUID) error
	ListByNetwork(clinicID, networkID uuid.UUID) ([]*model.PartnerPlanResponse, error)
}

type partnerPlanService struct {
	planRepo    repository.PartnerPlanRepository
	networkRepo repository.PartnerNetworkRepository
}

func NewPartnerPlanService(planRepo repository.PartnerPlanRepository, networkRepo repository.PartnerNetworkRepository) PartnerPlanService {
	return &partnerPlanService{planRepo: planRepo, networkRepo: networkRepo}
}

func (s *partnerPlanService) Create(clinicID uuid.UUID, req *model.CreatePartnerPlanRequest) (*model.PartnerPlanResponse, error) {
	if _, err := s.networkRepo.GetByID(req.PartnerNetworkID, clinicID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPartnerNetworkNotFound
		}
		return nil, err
	}

	existing, err := s.planRepo.GetByNetworkAndName(req.PartnerNetworkID, req.Name)
	if err == nil && existing != nil && existing.ID != uuid.Nil {
		return nil, ErrPartnerPlanNameExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	p := &model.PartnerPlan{
		PartnerNetworkID:   req.PartnerNetworkID,
		Name:               req.Name,
		MaxSessionsPerWeek: req.MaxSessionsPerWeek,
		IsActive:           true,
	}
	if err := s.planRepo.Create(p); err != nil {
		return nil, err
	}
	return p.ToResponse(), nil
}

func (s *partnerPlanService) GetByID(clinicID, id uuid.UUID) (*model.PartnerPlanResponse, error) {
	p, err := s.planRepo.GetByIDScoped(id, clinicID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPartnerPlanNotFound
		}
		return nil, err
	}
	return p.ToResponse(), nil
}

func (s *partnerPlanService) Update(clinicID, id uuid.UUID, req *model.UpdatePartnerPlanRequest) (*model.PartnerPlanResponse, error) {
	p, err := s.planRepo.GetByIDScoped(id, clinicID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPartnerPlanNotFound
		}
		return nil, err
	}
	if req.Name != nil && *req.Name != p.Name {
		conflict, errLookup := s.planRepo.GetByNetworkAndName(p.PartnerNetworkID, *req.Name)
		if errLookup == nil && conflict != nil && conflict.ID != uuid.Nil && conflict.ID != id {
			return nil, ErrPartnerPlanNameExists
		}
		p.Name = *req.Name
	}
	if req.MaxSessionsPerWeek != nil {
		p.MaxSessionsPerWeek = req.MaxSessionsPerWeek
	}
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}
	if err := s.planRepo.Update(p); err != nil {
		return nil, err
	}
	return p.ToResponse(), nil
}

func (s *partnerPlanService) Delete(clinicID, id uuid.UUID) error {
	if _, err := s.planRepo.GetByIDScoped(id, clinicID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPartnerPlanNotFound
		}
		return err
	}
	return s.planRepo.Delete(id, clinicID)
}

func (s *partnerPlanService) ListByNetwork(clinicID, networkID uuid.UUID) ([]*model.PartnerPlanResponse, error) {
	if _, err := s.networkRepo.GetByID(networkID, clinicID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPartnerNetworkNotFound
		}
		return nil, err
	}
	plans, err := s.planRepo.ListByNetwork(networkID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.PartnerPlanResponse, len(plans))
	for i, p := range plans {
		out[i] = p.ToResponse()
	}
	return out, nil
}
