package service

import (
	"context"
	"errors"

	"github.com/aseno/backend/pkg/pagination"
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
	BulkDelete(clinicID uuid.UUID, ids []uuid.UUID) error
	Restore(clinicID, id uuid.UUID) error
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.ProcedureResponse], error)
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

func (s *procedureService) BulkDelete(clinicID uuid.UUID, ids []uuid.UUID) error {
	return s.repo.BulkDelete(ids, clinicID)
}

func (s *procedureService) Restore(clinicID, id uuid.UUID) error {
	return s.repo.Restore(id, clinicID)
}

func (s *procedureService) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.ProcedureResponse], error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 10
	}

	procedures, total, err := s.repo.List(ctx, clinicID, filter)
	if err != nil {
		return pagination.Response[*model.ProcedureResponse]{}, err
	}

	data := make([]*model.ProcedureResponse, len(procedures))
	for i, proc := range procedures {
		data[i] = proc.ToResponse()
	}

	totalPages := int((total + int64(filter.Limit) - 1) / int64(filter.Limit))

	return pagination.Response[*model.ProcedureResponse]{
		Data: data,
		Meta: pagination.Meta{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}
