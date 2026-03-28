package service

import (
	"errors"

	"github.com/aseno/backend/pkg/paymentmethod/model"
	"github.com/aseno/backend/pkg/paymentmethod/repository"
	"github.com/google/uuid"
)

var (
	ErrPaymentMethodNotFound = errors.New("método de pagamento não encontrado")
)

type PaymentMethodService interface {
	Create(clinicID uuid.UUID, req *model.CreatePaymentMethodRequest) (*model.PaymentMethodResponse, error)
	GetByID(clinicID, id uuid.UUID) (*model.PaymentMethodResponse, error)
	Update(clinicID, id uuid.UUID, req *model.UpdatePaymentMethodRequest) (*model.PaymentMethodResponse, error)
	Delete(clinicID, id uuid.UUID) error
	List(clinicID uuid.UUID) ([]*model.PaymentMethodResponse, error)
	ListActive(clinicID uuid.UUID) ([]*model.PaymentMethodResponse, error)
	EnsureDefaults(clinicID uuid.UUID) error
}

type paymentMethodService struct {
	repo repository.PaymentMethodRepository
}

func NewPaymentMethodService(repo repository.PaymentMethodRepository) PaymentMethodService {
	return &paymentMethodService{repo: repo}
}

func (s *paymentMethodService) Create(clinicID uuid.UUID, req *model.CreatePaymentMethodRequest) (*model.PaymentMethodResponse, error) {
	method := &model.PaymentMethod{
		ClinicID: clinicID,
		Name:     req.Name,
		Type:     req.Type,
		Active:   true,
	}

	if err := s.repo.Create(method); err != nil {
		return nil, err
	}

	return method.ToResponse(), nil
}

func (s *paymentMethodService) GetByID(clinicID, id uuid.UUID) (*model.PaymentMethodResponse, error) {
	method, err := s.repo.GetByID(id, clinicID)
	if err != nil {
		return nil, ErrPaymentMethodNotFound
	}
	return method.ToResponse(), nil
}

func (s *paymentMethodService) Update(clinicID, id uuid.UUID, req *model.UpdatePaymentMethodRequest) (*model.PaymentMethodResponse, error) {
	method, err := s.repo.GetByID(id, clinicID)
	if err != nil {
		return nil, ErrPaymentMethodNotFound
	}

	if req.Name != nil {
		method.Name = *req.Name
	}
	if req.Type != nil {
		method.Type = *req.Type
	}
	if req.Active != nil {
		method.Active = *req.Active
	}

	if err := s.repo.Update(method); err != nil {
		return nil, err
	}

	return method.ToResponse(), nil
}

func (s *paymentMethodService) Delete(clinicID, id uuid.UUID) error {
	return s.repo.Delete(id, clinicID)
}

func (s *paymentMethodService) List(clinicID uuid.UUID) ([]*model.PaymentMethodResponse, error) {
	methods, err := s.repo.ListByClinic(clinicID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.PaymentMethodResponse, len(methods))
	for i, m := range methods {
		result[i] = m.ToResponse()
	}
	return result, nil
}

func (s *paymentMethodService) ListActive(clinicID uuid.UUID) ([]*model.PaymentMethodResponse, error) {
	methods, err := s.repo.ListActiveByClinic(clinicID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.PaymentMethodResponse, len(methods))
	for i, m := range methods {
		result[i] = m.ToResponse()
	}
	return result, nil
}

func (s *paymentMethodService) EnsureDefaults(clinicID uuid.UUID) error {
	return s.repo.CreateDefaults(clinicID)
}
