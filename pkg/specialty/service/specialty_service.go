package service

import (
	"errors"

	"github.com/aseno/backend/pkg/specialty/model"
	"github.com/aseno/backend/pkg/specialty/repository"
	"github.com/google/uuid"
)

var (
	ErrSpecialtyNotFound = errors.New("especialidade não encontrada")
)

type SpecialtyService interface {
	Create(clinicID uuid.UUID, req *model.CreateSpecialtyRequest) (*model.SpecialtyResponse, error)
	GetByID(clinicID, id uuid.UUID) (*model.SpecialtyResponse, error)
	Update(clinicID, id uuid.UUID, req *model.UpdateSpecialtyRequest) (*model.SpecialtyResponse, error)
	Delete(clinicID, id uuid.UUID) error
	List(clinicID uuid.UUID) ([]*model.SpecialtyResponse, error)
	ListActive(clinicID uuid.UUID) ([]*model.SpecialtyResponse, error)
}

type specialtyService struct {
	repo repository.SpecialtyRepository
}

func NewSpecialtyService(repo repository.SpecialtyRepository) SpecialtyService {
	return &specialtyService{repo: repo}
}

func (s *specialtyService) Create(clinicID uuid.UUID, req *model.CreateSpecialtyRequest) (*model.SpecialtyResponse, error) {
	specialty := &model.Specialty{
		ClinicID: clinicID,
		Name:     req.Name,
		Active:   true,
	}

	if err := s.repo.Create(specialty); err != nil {
		return nil, err
	}

	return specialty.ToResponse(), nil
}

func (s *specialtyService) GetByID(clinicID, id uuid.UUID) (*model.SpecialtyResponse, error) {
	specialty, err := s.repo.GetByID(id, clinicID)
	if err != nil {
		return nil, ErrSpecialtyNotFound
	}
	return specialty.ToResponse(), nil
}

func (s *specialtyService) Update(clinicID, id uuid.UUID, req *model.UpdateSpecialtyRequest) (*model.SpecialtyResponse, error) {
	specialty, err := s.repo.GetByID(id, clinicID)
	if err != nil {
		return nil, ErrSpecialtyNotFound
	}

	if req.Name != nil {
		specialty.Name = *req.Name
	}
	if req.Active != nil {
		specialty.Active = *req.Active
	}

	if err := s.repo.Update(specialty); err != nil {
		return nil, err
	}

	return specialty.ToResponse(), nil
}

func (s *specialtyService) Delete(clinicID, id uuid.UUID) error {
	return s.repo.Delete(id, clinicID)
}

func (s *specialtyService) List(clinicID uuid.UUID) ([]*model.SpecialtyResponse, error) {
	specialties, err := s.repo.ListByClinic(clinicID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.SpecialtyResponse, len(specialties))
	for i, sp := range specialties {
		result[i] = sp.ToResponse()
	}
	return result, nil
}

func (s *specialtyService) ListActive(clinicID uuid.UUID) ([]*model.SpecialtyResponse, error) {
	specialties, err := s.repo.ListActiveByClinic(clinicID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.SpecialtyResponse, len(specialties))
	for i, sp := range specialties {
		result[i] = sp.ToResponse()
	}
	return result, nil
}
