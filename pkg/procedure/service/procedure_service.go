package service

import (
	"errors"

	"github.com/aseno/backend/pkg/procedure/model"
	"github.com/aseno/backend/pkg/procedure/repository"
	"github.com/google/uuid"
)

var (
	ErrProcedureNotFound = errors.New("procedimento não encontrado")
	ErrInvalidProcedure  = errors.New("dados inválidos para o procedimento")
)

// ProcedureService defines the interface for procedure business logic
type ProcedureService interface {
	Create(clinicID uuid.UUID, req *model.CreateProcedureRequest) (*model.ProcedureResponse, error)
	GetByID(clinicID, id uuid.UUID) (*model.ProcedureResponse, error)
	Update(clinicID, id uuid.UUID, req *model.UpdateProcedureRequest) (*model.ProcedureResponse, error)
	Delete(clinicID, id uuid.UUID) error
	List(clinicID uuid.UUID, page, perPage int) (*model.ProcedureListResponse, error)
}

type procedureService struct {
	repo repository.ProcedureRepository
}

// NewProcedureService creates a new procedure service
func NewProcedureService(repo repository.ProcedureRepository) ProcedureService {
	return &procedureService{repo: repo}
}

func (s *procedureService) Create(clinicID uuid.UUID, req *model.CreateProcedureRequest) (*model.ProcedureResponse, error) {
	proc := &model.Procedure{
		ClinicID:        clinicID,
		Name:            req.Name,
		Price:           *req.Price,
		DurationMinutes: *req.DurationMinutes,
	}

	if err := s.repo.Create(proc); err != nil {
		return nil, err
	}

	return proc.ToResponse(), nil
}

func (s *procedureService) GetByID(clinicID, id uuid.UUID) (*model.ProcedureResponse, error) {
	proc, err := s.repo.GetByID(id, clinicID)
	if err != nil {
		return nil, ErrProcedureNotFound
	}
	return proc.ToResponse(), nil
}

func (s *procedureService) Update(clinicID, id uuid.UUID, req *model.UpdateProcedureRequest) (*model.ProcedureResponse, error) {
	proc, err := s.repo.GetByID(id, clinicID)
	if err != nil {
		return nil, ErrProcedureNotFound
	}

	if req.Name != nil {
		proc.Name = *req.Name
	}
	if req.Price != nil {
		proc.Price = *req.Price
	}
	if req.DurationMinutes != nil {
		proc.DurationMinutes = *req.DurationMinutes
	}

	if err := s.repo.Update(proc); err != nil {
		return nil, err
	}

	return proc.ToResponse(), nil
}

func (s *procedureService) Delete(clinicID, id uuid.UUID) error {
	// Let the repo handle scoping and checking existence
	err := s.repo.Delete(id, clinicID)
	if err != nil {
		return err // Or map to ErrProcedureNotFound based on db error
	}
	return nil
}

func (s *procedureService) List(clinicID uuid.UUID, page, perPage int) (*model.ProcedureListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	procedures, total, err := s.repo.List(clinicID, page, perPage)
	if err != nil {
		return nil, err
	}

	resp := &model.ProcedureListResponse{
		Data:       make([]*model.ProcedureResponse, len(procedures)),
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: int((total + int64(perPage) - 1) / int64(perPage)),
	}

	for i, proc := range procedures {
		resp.Data[i] = proc.ToResponse()
	}

	return resp, nil
}
