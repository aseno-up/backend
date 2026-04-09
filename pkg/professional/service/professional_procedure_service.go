package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	procedure_repository "github.com/aseno/backend/pkg/procedure/repository"
	"github.com/aseno/backend/pkg/professional/model"
	"github.com/aseno/backend/pkg/professional/repository"
)

// ProfessionalProcedureService handles commission business logic
type ProfessionalProcedureService interface {
	ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID) ([]model.ProfessionalProcedureResponse, error)
	ListProfessionalsByProcedure(ctx context.Context, procedureID, clinicID uuid.UUID) ([]model.ProfessionalWithCommission, error)
	BulkUpsert(ctx context.Context, clinicID, professionalID uuid.UUID, req *model.BulkUpsertProfessionalProcedureRequest) ([]model.ProfessionalProcedureResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
}

type professionalProcedureService struct {
	ppRepo           repository.ProfessionalProcedureRepository
	professionalRepo repository.ProfessionalRepository
	procedureRepo    procedure_repository.ProcedureRepository
}

func NewProfessionalProcedureService(
	ppRepo repository.ProfessionalProcedureRepository,
	professionalRepo repository.ProfessionalRepository,
	procedureRepo procedure_repository.ProcedureRepository,
) ProfessionalProcedureService {
	return &professionalProcedureService{
		ppRepo:           ppRepo,
		professionalRepo: professionalRepo,
		procedureRepo:    procedureRepo,
	}
}

func (s *professionalProcedureService) ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID) ([]model.ProfessionalProcedureResponse, error) {
	// Verify professional exists in this clinic
	_, err := s.professionalRepo.GetByID(ctx, professionalID, clinicID)
	if err != nil {
		return nil, err
	}

	items, err := s.ppRepo.ListByProfessional(ctx, professionalID, clinicID)
	if err != nil {
		return nil, err
	}

	// Collect procedure IDs to fetch names and prices
	procedureIDs := make([]uuid.UUID, len(items))
	for i, item := range items {
		procedureIDs[i] = item.ProcedureID
	}

	if len(procedureIDs) == 0 {
		return []model.ProfessionalProcedureResponse{}, nil
	}

	procedures, err := s.procedureRepo.GetByIDs(procedureIDs, clinicID)
	if err != nil {
		return nil, err
	}

	procMap := make(map[uuid.UUID]struct {
		Name  string
		Price float64
	})
	for _, p := range procedures {
		procMap[p.ID] = struct {
			Name  string
			Price float64
		}{Name: p.Name, Price: p.Price}
	}

	responses := make([]model.ProfessionalProcedureResponse, 0, len(items))
	for _, item := range items {
		proc := procMap[item.ProcedureID]
		responses = append(responses, model.ProfessionalProcedureResponse{
			ID:              item.ID,
			ProfessionalID:  item.ProfessionalID,
			ProcedureID:     item.ProcedureID,
			ProcedureName:   proc.Name,
			ProcedurePrice:  proc.Price,
			CommissionType:  item.CommissionType,
			CommissionValue: item.CommissionValue,
			IsActive:        item.IsActive,
		})
	}

	return responses, nil
}

func (s *professionalProcedureService) BulkUpsert(ctx context.Context, clinicID, professionalID uuid.UUID, req *model.BulkUpsertProfessionalProcedureRequest) ([]model.ProfessionalProcedureResponse, error) {
	// Verify professional exists in this clinic
	_, err := s.professionalRepo.GetByID(ctx, professionalID, clinicID)
	if err != nil {
		return nil, err
	}

	// Validate each item
	procedureIDs := make([]uuid.UUID, len(req.Items))
	for i, item := range req.Items {
		if item.CommissionType != "percentage" && item.CommissionType != "fixed" {
			return nil, errors.ErrValidationInstance.WithMessage(
				fmt.Sprintf("Tipo de repasse invalido: '%s'. Use 'percentage' ou 'fixed'.", item.CommissionType))
		}
		if item.CommissionType == "percentage" && (item.CommissionValue <= 0 || item.CommissionValue > 100) {
			return nil, errors.ErrValidationInstance.WithMessage(
				fmt.Sprintf("Percentual de repasse deve ser entre 0 e 100. Recebido: %.2f", item.CommissionValue))
		}
		if item.CommissionType == "fixed" && item.CommissionValue < 0 {
			return nil, errors.ErrValidationInstance.WithMessage(
				fmt.Sprintf("Valor fixo de repasse nao pode ser negativo. Recebido: %.2f", item.CommissionValue))
		}
		procedureIDs[i] = item.ProcedureID
	}

	// Verify all procedures exist in this clinic
	procedures, err := s.procedureRepo.GetByIDs(procedureIDs, clinicID)
	if err != nil {
		return nil, err
	}
	if len(procedures) != len(procedureIDs) {
		return nil, errors.ErrValidationInstance.WithMessage("Um ou mais procedimentos nao foram encontrados nesta clinica.")
	}

	// Build price map for fixed value validation
	priceMap := make(map[uuid.UUID]float64)
	for _, p := range procedures {
		priceMap[p.ID] = p.Price
	}

	// Validate fixed commission does not exceed procedure price
	for _, item := range req.Items {
		if item.CommissionType == "fixed" {
			price := priceMap[item.ProcedureID]
			if item.CommissionValue > price {
				return nil, errors.ErrValidationInstance.WithMessage(
					fmt.Sprintf("Valor fixo de repasse (R$ %.2f) nao pode ser maior que o preco do procedimento (R$ %.2f).", item.CommissionValue, price))
			}
		}
	}

	// Build entities
	entities := make([]model.ProfessionalProcedure, len(req.Items))
	for i, item := range req.Items {
		entities[i] = model.ProfessionalProcedure{
			ClinicID:        clinicID,
			ProfessionalID:  professionalID,
			ProcedureID:     item.ProcedureID,
			CommissionType:  item.CommissionType,
			CommissionValue: item.CommissionValue,
			IsActive:        true,
		}
	}

	if err := s.ppRepo.BulkUpsert(ctx, clinicID, professionalID, entities); err != nil {
		return nil, err
	}

	// Return updated list
	return s.ListByProfessional(ctx, professionalID, clinicID)
}

func (s *professionalProcedureService) ListProfessionalsByProcedure(ctx context.Context, procedureID, clinicID uuid.UUID) ([]model.ProfessionalWithCommission, error) {
	items, err := s.ppRepo.ListByProcedure(ctx, procedureID, clinicID)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return []model.ProfessionalWithCommission{}, nil
	}

	// Fetch professional names
	results := make([]model.ProfessionalWithCommission, 0, len(items))
	for _, item := range items {
		if !item.IsActive {
			continue
		}
		prof, err := s.professionalRepo.GetByID(ctx, item.ProfessionalID, clinicID)
		if err != nil {
			continue
		}
		results = append(results, model.ProfessionalWithCommission{
			ProfessionalID:   item.ProfessionalID,
			ProfessionalName: prof.Name,
			CommissionType:   item.CommissionType,
			CommissionValue:  item.CommissionValue,
		})
	}

	return results, nil
}

func (s *professionalProcedureService) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.ppRepo.Delete(ctx, id, clinicID)
}
