package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/patient/model"
	"github.com/aseno/backend/pkg/patient/repository"
)

// PatientService defines the interface for patient business logic
type PatientService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreatePatientRequest) (*model.PatientResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.PatientResponse], error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdatePatientRequest) (*model.PatientResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	// Risk assessment methods
	GetRisk(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRiskResponse, error)
	CalculateRiskScore(ctx context.Context, patientID, clinicID uuid.UUID) error
	RecalculateAllRiskScores(ctx context.Context, clinicID uuid.UUID) (int, error)
}

// patientService implements PatientService
type patientService struct {
	repo       repository.PatientRepository
	clinicRepo clinic_repository.ClinicRepository
}

// NewPatientService creates a new patient service
func NewPatientService(repo repository.PatientRepository, clinicRepo clinic_repository.ClinicRepository) PatientService {
	return &patientService{
		repo:       repo,
		clinicRepo: clinicRepo,
	}
}

// checkPlanForRiskData returns true if the plan allows accessing risk data
func (s *patientService) checkPlanForRiskData(ctx context.Context, clinicID uuid.UUID) (bool, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return false, err
	}

	// Risk data is available for Professional and Advanced plans
	return clinic.Plan != clinic_model.PlanTypeBasic, nil
}

// maskRiskData hides risk data from the response
func maskRiskData(resp *model.PatientResponse) {
	resp.NoShowRisk = model.NoShowRiskUnknown
	resp.TotalAppointments = 0
	resp.NoShowCount = 0
	resp.AttendanceRate = 0
	resp.RiskUpdatedAt = nil
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

	resp := patient.ToResponse()

	// Check plan and mask if necessary
	allowed, err := s.checkPlanForRiskData(ctx, clinicID)
	if err == nil && !allowed {
		maskRiskData(resp)
	}

	return resp, nil
}

// List retrieves a paginated list of patients
func (s *patientService) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.PatientResponse], error) {
	patients, total, err := s.repo.List(ctx, clinicID, filter)
	if err != nil {
		return pagination.Response[*model.PatientResponse]{}, err
	}

	data := make([]*model.PatientResponse, len(patients))

	// Check plan once for the list
	allowed, _ := s.checkPlanForRiskData(ctx, clinicID)

	for i, p := range patients {
		resp := p.ToResponse()
		if !allowed {
			maskRiskData(resp)
		}
		data[i] = resp
	}

	return pagination.Response[*model.PatientResponse]{
		Data: data,
		Meta: pagination.NewMeta(filter.GetPage(), filter.GetLimit(), total),
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

// GetRisk retrieves the risk assessment for a patient
func (s *patientService) GetRisk(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRiskResponse, error) {
	// Check plan
	allowed, err := s.checkPlanForRiskData(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, errors.ErrUpgradeRequiredInstance.WithMessage("Histórico de faltas e análise de risco disponíveis apenas nos planos Profissional e Avançado")
	}

	patient, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}
	return patient.ToRiskResponse(), nil
}

// CalculateRiskScore calculates and updates the risk score for a patient
func (s *patientService) CalculateRiskScore(ctx context.Context, patientID, clinicID uuid.UUID) error {
	// Get appointment statistics
	stats, err := s.repo.GetAppointmentStats(ctx, patientID, clinicID)
	if err != nil {
		return err
	}

	// Calculate risk level
	risk := calculateRisk(stats.CompletedCount, stats.NoShowCount)

	// Update patient with risk score
	return s.repo.UpdateRiskScore(ctx, patientID, clinicID, risk, stats)
}

// RecalculateAllRiskScores recalculates risk scores for all patients in a clinic
func (s *patientService) RecalculateAllRiskScores(ctx context.Context, clinicID uuid.UUID) (int, error) {
	patients, err := s.repo.GetAllByClinicID(ctx, clinicID)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, patient := range patients {
		if err := s.CalculateRiskScore(ctx, patient.ID, clinicID); err != nil {
			// Log error but continue with other patients
			continue
		}
		updated++
	}

	return updated, nil
}

// calculateRisk determines the risk level based on attendance statistics
func calculateRisk(completed, noShows int) model.NoShowRisk {
	total := completed + noShows

	// Need at least 3 appointments to calculate risk
	if total < 3 {
		return model.NoShowRiskUnknown
	}

	attendanceRate := float64(completed) / float64(total) * 100

	if attendanceRate >= 90 {
		return model.NoShowRiskLow
	} else if attendanceRate >= 70 {
		return model.NoShowRiskMedium
	}
	return model.NoShowRiskHigh
}
