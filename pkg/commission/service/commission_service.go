package service

import (
	"context"
	"log"
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

// GroupSessionCommissionInput holds the data needed to calculate commission for a group session
type GroupSessionCommissionInput struct {
	SessionPlanID  uuid.UUID
	SessionDate    string // YYYY-MM-DD
	ProfessionalID uuid.UUID
	ProcedureID    uuid.UUID
	ProcedureName  string
	SessionPrice   float64
}

// CommissionService handles commission business logic
type CommissionService interface {
	// CalculateAndStore calculates commission for a completed appointment and stores it.
	CalculateAndStore(ctx context.Context, appointment *appointmentModel.Appointment, clinicID uuid.UUID) error

	// CalculateAndStoreForGroupSession calculates commission for a group session attendance.
	CalculateAndStoreForGroupSession(ctx context.Context, input *GroupSessionCommissionInput, clinicID uuid.UUID) error

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

	// VoidByGroupSession removes commission for a group session date.
	VoidByGroupSession(ctx context.Context, sessionPlanID, clinicID uuid.UUID, date string) error
}

type commissionService struct {
	repo      repository.CommissionRepository
	ppLookup  ProfessionalProcedureLookup
	aptLookup AppointmentLookup
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

		aptID := appointment.ID
		commission := &model.Commission{
			ClinicID:        clinicID,
			AppointmentID:   &aptID,
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
		detail := model.CommissionDetailResponse{
			ID:              c.ID,
			AppointmentID:   c.AppointmentID,
			SessionPlanID:   c.SessionPlanID,
			SessionDate:     c.SessionDate,
			SessionPrice:    c.SessionPrice,
			CommissionType:  c.CommissionType,
			CommissionRate:  c.CommissionRate,
			CommissionValue: c.CommissionValue,
			Status:          c.Status,
			PaidAt:          c.PaidAt,
		}

		if c.AppointmentID != nil {
			// Appointment-based commission
			detail.Source = "appointment"
			apt, err := s.aptLookup.GetByID(ctx, *c.AppointmentID, clinicID)
			if err != nil {
				continue
			}
			detail.AppointmentDate = apt.StartTime
			if apt.Patient != nil {
				detail.PatientName = apt.Patient.Name
			}
			for _, p := range apt.Procedures {
				if p.ProcedureID == c.ProcedureID && p.Procedure != nil {
					detail.ProcedureName = p.Procedure.Name
					break
				}
			}
		} else if c.SessionPlanID != nil {
			// Group session commission
			detail.Source = "group_session"
			detail.AppointmentDate = c.CreatedAt
			if c.SessionDate != "" {
				if t, err := time.Parse("2006-01-02", c.SessionDate); err == nil {
					detail.AppointmentDate = t
				}
			}
			detail.PatientName = "Sessão de grupo"
			detail.ProcedureName = c.ProcedureName // snapshot stored at creation time
		}

		results = append(results, detail)
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

func (s *commissionService) CalculateAndStoreForGroupSession(ctx context.Context, input *GroupSessionCommissionInput, clinicID uuid.UUID) error {
	// Check if commission already exists for this session plan + date
	existing, err := s.repo.GetBySessionPlanAndDate(ctx, input.SessionPlanID, clinicID, input.SessionDate)
	if err != nil {
		log.Printf("[Commission] Error checking existing commission for plan %s date %s: %v", input.SessionPlanID, input.SessionDate, err)
		return err
	}
	if len(existing) > 0 {
		log.Printf("[Commission] SKIP: commission already exists for plan %s date %s (found %d)", input.SessionPlanID, input.SessionDate, len(existing))
		return nil // Already calculated — idempotent
	}

	// Get commission config for this professional + procedure
	profProcs, err := s.ppLookup.ListByProfessional(ctx, input.ProfessionalID, clinicID)
	if err != nil {
		log.Printf("[Commission] Error fetching professional procedures for prof %s: %v", input.ProfessionalID, err)
		return err
	}

	log.Printf("[Commission] Found %d professional-procedure configs for prof %s", len(profProcs), input.ProfessionalID)

	var config *professionalModel.ProfessionalProcedure
	for _, pp := range profProcs {
		if pp.ProcedureID == input.ProcedureID {
			config = &pp
			break
		}
	}

	if config == nil {
		log.Printf("[Commission] SKIP: no commission config for prof %s + procedure %s", input.ProfessionalID, input.ProcedureID)
		return nil // No commission configured for this professional-procedure pair
	}

	log.Printf("[Commission] Found config: type=%s rate=%.2f for prof %s + procedure %s", config.CommissionType, config.CommissionValue, input.ProfessionalID, input.ProcedureID)

	var commissionValue float64
	if config.CommissionType == "percentage" {
		commissionValue = input.SessionPrice * (config.CommissionValue / 100)
	} else {
		commissionValue = config.CommissionValue
	}

	if commissionValue > input.SessionPrice {
		commissionValue = input.SessionPrice
	}

	commission := &model.Commission{
		ClinicID:        clinicID,
		SessionPlanID:   &input.SessionPlanID,
		SessionDate:     input.SessionDate,
		ProfessionalID:  input.ProfessionalID,
		ProcedureID:     input.ProcedureID,
		ProcedureName:   input.ProcedureName,
		SessionPrice:    input.SessionPrice,
		CommissionType:  config.CommissionType,
		CommissionRate:  config.CommissionValue,
		CommissionValue: commissionValue,
		Status:          model.CommissionStatusPending,
	}

	if err := s.repo.Create(ctx, commission); err != nil {
		log.Printf("[Commission] Error creating commission for plan %s date %s: %v", input.SessionPlanID, input.SessionDate, err)
		return err
	}

	log.Printf("[Commission] Successfully created commission: plan=%s date=%s value=%.2f", input.SessionPlanID, input.SessionDate, commissionValue)
	return nil
}

func (s *commissionService) VoidByAppointment(ctx context.Context, appointmentID, clinicID uuid.UUID) error {
	return s.repo.DeleteByAppointmentID(ctx, appointmentID, clinicID)
}

func (s *commissionService) VoidByGroupSession(ctx context.Context, sessionPlanID, clinicID uuid.UUID, date string) error {
	return s.repo.DeleteBySessionPlanAndDate(ctx, sessionPlanID, clinicID, date)
}
