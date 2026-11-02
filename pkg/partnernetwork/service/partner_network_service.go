package service

import (
	"errors"

	"github.com/aseno/backend/pkg/partnernetwork/model"
	"github.com/aseno/backend/pkg/partnernetwork/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPartnerNetworkNotFound   = errors.New("rede de convênio não encontrada")
	ErrPartnerNetworkNameExists = errors.New("já existe uma rede de convênio com esse nome para esta clínica")
)

type PartnerNetworkService interface {
	Create(clinicID uuid.UUID, req *model.CreatePartnerNetworkRequest) (*model.PartnerNetworkResponse, error)
	GetByID(clinicID, id uuid.UUID) (*model.PartnerNetworkResponse, error)
	Update(clinicID, id uuid.UUID, req *model.UpdatePartnerNetworkRequest) (*model.PartnerNetworkResponse, error)
	Delete(clinicID, id uuid.UUID) error
	List(clinicID uuid.UUID) ([]*model.PartnerNetworkResponse, error)
	ListActive(clinicID uuid.UUID) ([]*model.PartnerNetworkResponse, error)
}

type partnerNetworkService struct {
	repo repository.PartnerNetworkRepository
}

func NewPartnerNetworkService(repo repository.PartnerNetworkRepository) PartnerNetworkService {
	return &partnerNetworkService{repo: repo}
}

func (s *partnerNetworkService) Create(clinicID uuid.UUID, req *model.CreatePartnerNetworkRequest) (*model.PartnerNetworkResponse, error) {
	existing, err := s.repo.GetByName(req.Name, clinicID)
	if err == nil && existing != nil && existing.ID != uuid.Nil {
		return nil, ErrPartnerNetworkNameExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	n := &model.PartnerNetwork{
		ClinicID: clinicID,
		Name:     req.Name,
		IsActive: true,
	}
	if err := s.repo.Create(n); err != nil {
		return nil, err
	}
	return n.ToResponse(), nil
}

func (s *partnerNetworkService) GetByID(clinicID, id uuid.UUID) (*model.PartnerNetworkResponse, error) {
	n, err := s.repo.GetByIDWithPlans(id, clinicID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPartnerNetworkNotFound
		}
		return nil, err
	}
	return n.ToResponse(), nil
}

func (s *partnerNetworkService) Update(clinicID, id uuid.UUID, req *model.UpdatePartnerNetworkRequest) (*model.PartnerNetworkResponse, error) {
	n, err := s.repo.GetByID(id, clinicID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPartnerNetworkNotFound
		}
		return nil, err
	}
	if req.Name != nil && *req.Name != n.Name {
		conflict, errLookup := s.repo.GetByName(*req.Name, clinicID)
		if errLookup == nil && conflict != nil && conflict.ID != uuid.Nil && conflict.ID != id {
			return nil, ErrPartnerNetworkNameExists
		}
		n.Name = *req.Name
	}
	if req.IsActive != nil {
		n.IsActive = *req.IsActive
	}
	if err := s.repo.Update(n); err != nil {
		return nil, err
	}
	return n.ToResponse(), nil
}

func (s *partnerNetworkService) Delete(clinicID, id uuid.UUID) error {
	if _, err := s.repo.GetByID(id, clinicID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPartnerNetworkNotFound
		}
		return err
	}
	return s.repo.Delete(id, clinicID)
}

func (s *partnerNetworkService) List(clinicID uuid.UUID) ([]*model.PartnerNetworkResponse, error) {
	ns, err := s.repo.ListByClinic(clinicID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.PartnerNetworkResponse, len(ns))
	for i, n := range ns {
		out[i] = n.ToResponse()
	}
	return out, nil
}

func (s *partnerNetworkService) ListActive(clinicID uuid.UUID) ([]*model.PartnerNetworkResponse, error) {
	ns, err := s.repo.ListActiveByClinic(clinicID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.PartnerNetworkResponse, len(ns))
	for i, n := range ns {
		out[i] = n.ToResponse()
	}
	return out, nil
}
