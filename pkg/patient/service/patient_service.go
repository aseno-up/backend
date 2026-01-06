package service

import (
	"context"
	"math"

	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/patient/model"
	"github.com/aseno/backend/pkg/patient/repository"
)

// PatientService defines the interface for patient business logic
type PatientService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreatePatientRequest) (*model.PatientResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, page, perPage int) (*model.PatientListResponse, error)
	Search(ctx context.Context, clinicID uuid.UUID, query string, page, perPage int) (*model.PatientListResponse, error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdatePatientRequest) (*model.PatientResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
}

// patientService implements PatientService
type patientService struct {
	repo repository.PatientRepository
}

// NewPatientService creates a new patient service
func NewPatientService(repo repository.PatientRepository) PatientService {
	return &patientService{repo: repo}
}

// Create creates a new patient
func (s *patientService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreatePatientRequest) (*model.PatientResponse, error) {
	patient := &model.Patient{
		ClinicID:  clinicID,
		Name:      req.Name,
		Email:     req.Email,
		Phone:     req.Phone,
		WhatsApp:  req.WhatsApp,
		Document:  req.Document,
		BirthDate: req.BirthDate,
		Gender:    req.Gender,
		Address:   req.Address,
		City:      req.City,
		State:     req.State,
		ZipCode:   req.ZipCode,
		Notes:     req.Notes,
		IsActive:  true,
	}

	// If WhatsApp not provided, use phone
	if patient.WhatsApp == "" {
		patient.WhatsApp = patient.Phone
	}

	if err := s.repo.Create(ctx, patient); err != nil {
		return nil, err
	}

	return patient.ToResponse(), nil
}

// GetByID retrieves a patient by ID
func (s *patientService) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientResponse, error) {
	patient, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}
	return patient.ToResponse(), nil
}

// List retrieves a paginated list of patients
func (s *patientService) List(ctx context.Context, clinicID uuid.UUID, page, perPage int) (*model.PatientListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	patients, total, err := s.repo.GetByClinicID(ctx, clinicID, perPage, offset)
	if err != nil {
		return nil, err
	}

	data := make([]*model.PatientResponse, len(patients))
	for i, p := range patients {
		data[i] = p.ToResponse()
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	return &model.PatientListResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

// Search searches patients by name, phone, or email
func (s *patientService) Search(ctx context.Context, clinicID uuid.UUID, query string, page, perPage int) (*model.PatientListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	patients, total, err := s.repo.Search(ctx, clinicID, query, perPage, offset)
	if err != nil {
		return nil, err
	}

	data := make([]*model.PatientResponse, len(patients))
	for i, p := range patients {
		data[i] = p.ToResponse()
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	return &model.PatientListResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

// Update updates a patient
func (s *patientService) Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdatePatientRequest) (*model.PatientResponse, error) {
	patient, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Name != "" {
		patient.Name = req.Name
	}
	if req.Email != "" {
		patient.Email = req.Email
	}
	if req.Phone != "" {
		patient.Phone = req.Phone
	}
	if req.WhatsApp != "" {
		patient.WhatsApp = req.WhatsApp
	}
	if req.Document != "" {
		patient.Document = req.Document
	}
	if req.BirthDate != nil {
		patient.BirthDate = req.BirthDate
	}
	if req.Gender != "" {
		patient.Gender = req.Gender
	}
	if req.Address != "" {
		patient.Address = req.Address
	}
	if req.City != "" {
		patient.City = req.City
	}
	if req.State != "" {
		patient.State = req.State
	}
	if req.ZipCode != "" {
		patient.ZipCode = req.ZipCode
	}
	if req.Notes != "" {
		patient.Notes = req.Notes
	}
	if req.IsActive != nil {
		patient.IsActive = *req.IsActive
	}

	if err := s.repo.Update(ctx, patient); err != nil {
		return nil, err
	}

	return patient.ToResponse(), nil
}

// Delete deletes a patient
func (s *patientService) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.Delete(ctx, id, clinicID)
}
