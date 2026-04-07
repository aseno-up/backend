package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	appointmentModel "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/commission/model"
	"github.com/aseno/backend/pkg/commission/repository"
	professionalModel "github.com/aseno/backend/pkg/professional/model"
)

// ProfessionalProcedureLookup provides commission configuration for a professional-procedure pair
type ProfessionalProcedureLookup interface {
	ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID) ([]professionalModel.ProfessionalProcedure, error)
}

// AppointmentLookup provides appointment details for commission reports
type AppointmentLookup interface {
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*appointmentModel.Appointment, error)
}

// CommissionService handles commission business logic
type CommissionService interface {
	// CalculateAndStore calculates commission for a completed appointment and stores it.
	// Called from appointment_service.Complete() within the same transaction context.
	CalculateAndStore(ctx context.Context, appointment *appointmentModel.Appointment, clinicID uuid.UUID) error

	// GetDetailedReport returns commission details for a professional in a period,
	// enriched with appointment/patient/procedure info.
	GetDetailedReport(ctx context.Context, professionalID, clinicID uuid.UUID, from, to time.Time) ([]model.CommissionDetailResponse, error)

	// GetSummary returns consolidated commission summary for all professionals in a period.
	GetSummary(ctx context.Context, clinicID uuid.UUID, from, to time.Time) (*model.CommissionSummaryResponse, error)

	// BulkMarkPaid marks selected commissions as paid.
	BulkMarkPaid(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error

	// MarkAllPendingPaid marks all pending commissions for a professional in a period as paid.
	MarkAllPendingPaid(ctx context.Context, professionalID, clinicID uuid.UUID, from, to time.Time) error

	// VoidByAppointment removes commission when appointment is canceled/no-show.
	VoidByAppointment(ctx context.Context, appointmentID, clinicID uuid.UUID) error
}

type commissionService struct {
	repo        repository.CommissionRepository
	ppLookup    ProfessionalProcedureLookup
	aptLookup   AppointmentLookup
}

func NewCommissionService(
	repo repository.CommissionRepository,
	ppLookup ProfessionalProcedureLookup,
	aptLookup AppointmentLookup,
) CommissionService {
	return &commissionService{
		repo:      repo,
		ppLookup:  ppLookup,
		aptLookup: aptLookup,
	}
}

func (s *commissionService) CalculateAndStore(ctx context.Context, appointment *appointmentModel.Appointment, clinicID uuid.UUID) error {
	if len(appointment.Procedures) == 0 {
		return nil // No procedures → no commission to calculate
	}

	// Get commission config for this professional
	profProcs, err := s.ppLookup.ListByProfessional(ctx, appointment.ProfessionalID, clinicID)
	if err != nil {
		return err
	}

	// Build lookup map
	configMap := make(map[uuid.UUID]professionalModel.ProfessionalProcedure)
	for _, pp := range profProcs {
		configMap[pp.ProcedureID] = pp
	}

	// Calculate and store commission for each procedure in the appointment
	for _, aptProc := range appointment.Procedures {
		config, exists := configMap[aptProc.ProcedureID]
		if !exists {
			continue // No commission configured for this procedure — skip
		}

		var commissionValue float64
		if config.CommissionType == "percentage" {
			commissionValue = aptProc.Price * (config.CommissionValue / 100)
		} else {
			commissionValue = config.CommissionValue
		}

		// Cap fixed commission at session price
		if commissionValue > aptProc.Price {
			commissionValue = aptProc.Price
		}

		commission := &model.Commission{
			ClinicID:        clinicID,
			AppointmentID:   appointment.ID,
			ProfessionalID:  appointment.ProfessionalID,
			ProcedureID:     aptProc.ProcedureID,
			SessionPrice:    aptProc.Price,
			CommissionType:  config.CommissionType,
			CommissionRate:  config.CommissionValue,
			CommissionValue: commissionValue,
			Status:          model.CommissionStatusPending,
		}

		if err := s.repo.Create(ctx, commission); err != nil {
			return err
		}
	}

	return nil
}

func (s *commissionService) GetDetailedReport(ctx context.Context, professionalID, clinicID uuid.UUID, from, to time.Time) ([]model.CommissionDetailResponse, error) {
	commissions, err := s.repo.ListByProfessional(ctx, professionalID, clinicID, from, to)
	if err != nil {
		return nil, err
	}

	results := make([]model.CommissionDetailResponse, 0, len(commissions))
	for _, c := range commissions {
		// Fetch appointment for date and patient name
		apt, err := s.aptLookup.GetByID(ctx, c.AppointmentID, clinicID)
		if err != nil {
			continue // Skip if appointment not found
		}

		patientName := ""
		if apt.Patient != nil {
			patientName = apt.Patient.Name
		}

		procedureName := ""
		for _, p := range apt.Procedures {
			if p.ProcedureID == c.ProcedureID && p.Procedure != nil {
				procedureName = p.Procedure.Name
				break
			}
		}

		results = append(results, model.CommissionDetailResponse{
			ID:              c.ID,
			AppointmentID:   c.AppointmentID,
			AppointmentDate: apt.StartTime,
			PatientName:     patientName,
			ProcedureName:   procedureName,
			SessionPrice:    c.SessionPrice,
			CommissionType:  c.CommissionType,
			CommissionRate:  c.CommissionRate,
			CommissionValue: c.CommissionValue,
			Status:          c.Status,
			PaidAt:          c.PaidAt,
		})
	}

	return results, nil
}

func (s *commissionService) GetSummary(ctx context.Context, clinicID uuid.UUID, from, to time.Time) (*model.CommissionSummaryResponse, error) {
	professionals, err := s.repo.Summary(ctx, clinicID, from, to)
	if err != nil {
		return nil, err
	}

	if professionals == nil {
		professionals = []model.ProfessionalCommissionSummary{}
	}

	resp := &model.CommissionSummaryResponse{
		Professionals: professionals,
	}

	for _, p := range professionals {
		resp.TotalSessions += p.TotalSessions
		resp.TotalCommission += p.TotalCommission
		resp.TotalPaid += p.TotalPaid
		resp.TotalPending += p.TotalPending
	}

	return resp, nil
}

func (s *commissionService) BulkMarkPaid(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error {
	if len(ids) == 0 {
		return errors.ErrValidationInstance.WithMessage("Nenhuma comissao selecionada.")
	}
	return s.repo.BulkMarkPaid(ctx, ids, clinicID)
}

func (s *commissionService) MarkAllPendingPaid(ctx context.Context, professionalID, clinicID uuid.UUID, from, to time.Time) error {
	commissions, err := s.repo.ListByProfessional(ctx, professionalID, clinicID, from, to)
	if err != nil {
		return err
	}

	var pendingIDs []uuid.UUID
	for _, c := range commissions {
		if c.Status == model.CommissionStatusPending {
			pendingIDs = append(pendingIDs, c.ID)
		}
	}

	if len(pendingIDs) == 0 {
		return nil
	}

	return s.repo.BulkMarkPaid(ctx, pendingIDs, clinicID)
}

func (s *commissionService) VoidByAppointment(ctx context.Context, appointmentID, clinicID uuid.UUID) error {
	return s.repo.DeleteByAppointmentID(ctx, appointmentID, clinicID)
}
