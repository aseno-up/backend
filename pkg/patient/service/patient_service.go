package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
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
	BulkDelete(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error
	// Risk assessment methods
	GetRisk(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRiskResponse, error)
	CalculateRiskScore(ctx context.Context, patientID, clinicID uuid.UUID) error
	RecalculateAllRiskScores(ctx context.Context, clinicID uuid.UUID) (int, error)
	// Self-registered patient management
	ListSelfRegistered(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.PatientResponse], error)
	MergePatients(ctx context.Context, clinicID, targetID, sourceID uuid.UUID) error
	// Patient merge (bulk) methods
	ListUnlinkedProvisional(ctx context.Context, clinicID uuid.UUID) ([]*model.UnlinkedPatientResponse, error)
	SearchExcludingProvisional(ctx context.Context, clinicID uuid.UUID, query string) ([]*model.PatientSearchResult, error)
	MergePatientsBulk(ctx context.Context, clinicID uuid.UUID, actorID uuid.UUID, req *model.BulkMergePatientRequest) (*model.BulkMergePatientResponse, error)
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
	if clinic.Account != nil && clinic.Account.Plan != nil {
		return clinic.Account.Plan.HasApiAccess, nil
	}
	return false, nil
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

// BulkDelete soft deletes multiple patients
func (s *patientService) BulkDelete(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error {
	if err := s.repo.BulkDelete(ctx, ids, clinicID); err != nil {
		return err
	}
	return nil
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

// ListSelfRegistered returns patients with source=SELF_REGISTERED
func (s *patientService) ListSelfRegistered(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.PatientResponse], error) {
	patients, total, err := s.repo.ListBySource(ctx, clinicID, model.PatientSourceSelfRegistered, filter)
	if err != nil {
		return pagination.Response[*model.PatientResponse]{}, errors.ErrInternalInstance.WithError(err)
	}

	responses := make([]*model.PatientResponse, len(patients))
	for i, p := range patients {
		responses[i] = p.ToResponse()
	}

	return pagination.Response[*model.PatientResponse]{
		Data: responses,
		Meta: pagination.NewMeta(filter.GetPage(), filter.GetLimit(), total),
	}, nil
}

// MergePatients merges a self-registered patient into an existing patient
func (s *patientService) MergePatients(ctx context.Context, clinicID, targetID, sourceID uuid.UUID) error {
	if targetID == sourceID {
		return errors.ErrValidationInstance.WithMessage("Paciente de origem e destino não podem ser o mesmo")
	}

	if err := s.repo.MergePatients(ctx, clinicID, targetID, sourceID); err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// ListUnlinkedProvisional returns provisional patients pending verification
func (s *patientService) ListUnlinkedProvisional(ctx context.Context, clinicID uuid.UUID) ([]*model.UnlinkedPatientResponse, error) {
	patients, err := s.repo.GetUnlinkedProvisionalPatients(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	results := make([]*model.UnlinkedPatientResponse, len(patients))
	for i, p := range patients {
		results[i] = &model.UnlinkedPatientResponse{
			ID:        p.ID,
			Name:      p.Name,
			Email:     p.Email,
			CPF:       p.Document,
			Phone:     p.Phone,
			CreatedAt: p.CreatedAt,
		}
	}
	return results, nil
}

// SearchExcludingProvisional searches for confirmed patients by name or CPF
func (s *patientService) SearchExcludingProvisional(ctx context.Context, clinicID uuid.UUID, query string) ([]*model.PatientSearchResult, error) {
	patients, err := s.repo.SearchPatientsExcludingProvisional(ctx, clinicID, query, 10)
	if err != nil {
		return nil, err
	}

	results := make([]*model.PatientSearchResult, len(patients))
	for i, p := range patients {
		results[i] = &model.PatientSearchResult{
			ID:     p.ID,
			Name:   p.Name,
			CPF:    p.Document,
			Email:  p.Email,
			Phone:  p.Phone,
			Source: p.Source,
		}
	}
	return results, nil
}

// MergePatientsBulk merges multiple provisional patients into a target patient
func (s *patientService) MergePatientsBulk(ctx context.Context, clinicID uuid.UUID, actorID uuid.UUID, req *model.BulkMergePatientRequest) (*model.BulkMergePatientResponse, error) {
	if len(req.ProvisionalIDs) == 0 {
		return nil, errors.ErrValidationInstance.WithMessage("provisional_ids cannot be empty")
	}

	// Check none of the provisional IDs is the same as the target
	for _, pid := range req.ProvisionalIDs {
		if pid == req.TargetPatientID {
			return nil, errors.ErrValidationInstance.WithMessage("Target patient cannot be in the provisional list")
		}
	}

	reassigned, err := s.repo.MergePatientsBulk(ctx, clinicID, req.TargetPatientID, req.ProvisionalIDs, actorID)
	if err != nil {
		return nil, err
	}

	return &model.BulkMergePatientResponse{
		AppointmentsReassigned: reassigned,
		TargetPatientID:        req.TargetPatientID,
		MergedCount:            len(req.ProvisionalIDs),
	}, nil
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
