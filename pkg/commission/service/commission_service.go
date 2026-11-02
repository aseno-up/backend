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

	// SetFinanceCommissionUpserter wires the finance module so that paying
	// commissions reflects as an aggregated expense row per professional/month.
	SetFinanceCommissionUpserter(u FinanceCommissionUpserter)
}

// FinanceCommissionUpserter is a narrow port into the finance module. Each
// "Pagar" action (BulkMarkPaid / MarkAllPendingPaid) produces one expense row
// with paid_at=now, description carrying the session count + date range.
// Not aggregated across pay events so Caixa's daily totals stay truthful.
type FinanceCommissionUpserter interface {
	CreateFromCommissionBatch(ctx context.Context, clinicID, professionalID uuid.UUID, totalPaid float64, professionalName string, sessionCount int, earliest, latest time.Time) error
}

type commissionService struct {
	repo            repository.CommissionRepository
	ppLookup        ProfessionalProcedureLookup
	aptLookup       AppointmentLookup
	financeUpserter FinanceCommissionUpserter
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

func (s *commissionService) SetFinanceCommissionUpserter(u FinanceCommissionUpserter) {
	s.financeUpserter = u
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

	// Only the pending rows actually transition to paid; ignore already-paid
	// entries so the finance hook doesn't double-count on a replay.
	fetched, err := s.repo.GetByIDs(ctx, ids, clinicID)
	if err != nil {
		return err
	}
	pending := filterPending(fetched)
	if len(pending) == 0 {
		return nil
	}

	pendingIDs := make([]uuid.UUID, len(pending))
	for i, c := range pending {
		pendingIDs[i] = c.ID
	}

	if err := s.repo.BulkMarkPaid(ctx, pendingIDs, clinicID); err != nil {
		return err
	}
	s.syncFinanceForCommissions(ctx, pending, clinicID)
	return nil
}

func (s *commissionService) MarkAllPendingPaid(ctx context.Context, professionalID, clinicID uuid.UUID, from, to time.Time) error {
	commissions, err := s.repo.ListByProfessional(ctx, professionalID, clinicID, from, to)
	if err != nil {
		return err
	}

	pending := filterPending(commissions)
	if len(pending) == 0 {
		return nil
	}

	pendingIDs := make([]uuid.UUID, len(pending))
	for i, c := range pending {
		pendingIDs[i] = c.ID
	}

	if err := s.repo.BulkMarkPaid(ctx, pendingIDs, clinicID); err != nil {
		return err
	}
	s.syncFinanceForCommissions(ctx, pending, clinicID)
	return nil
}

func filterPending(in []model.Commission) []model.Commission {
	out := make([]model.Commission, 0, len(in))
	for _, c := range in {
		if c.Status == model.CommissionStatusPending {
			out = append(out, c)
		}
	}
	return out
}

// syncFinanceForCommissions inserts one finance expense row per professional
// touched in this pay event. Each call (BulkMarkPaid / MarkAllPendingPaid)
// produces fresh rows dated "now" — Caixa's daily breakdown matches reality.
//
// Only commissions that were PENDING when the batch ran get counted, so
// replaying the same ids (most are already paid) doesn't double-count.
//
// Description carries the session count + date range so the finance row
// explains itself without requiring a drill-down to the Repasse tab.
//
// Errors are logged but not returned: the user-facing pay flow must not fail
// because the finance sync misbehaved.
func (s *commissionService) syncFinanceForCommissions(ctx context.Context, paidCommissions []model.Commission, clinicID uuid.UUID) {
	if s.financeUpserter == nil || len(paidCommissions) == 0 {
		return
	}

	// Group by professional — typically a single professional per pay event,
	// but MarkAllPendingPaid across a period with multiple profs could span
	// more than one.
	type bucket struct {
		total    float64
		count    int
		earliest time.Time
		latest   time.Time
		name     string
	}
	byProf := make(map[uuid.UUID]*bucket)

	for _, c := range paidCommissions {
		b := byProf[c.ProfessionalID]
		if b == nil {
			b = &bucket{}
			byProf[c.ProfessionalID] = b
		}
		b.total += c.CommissionValue
		b.count++
		// Session date: prefer explicit SessionDate for group sessions, else
		// fall back to CreatedAt (correlates with the appointment date in the
		// per-session flow).
		sessionTime := c.CreatedAt
		if c.SessionDate != "" {
			if parsed, err := time.Parse("2006-01-02", c.SessionDate); err == nil {
				sessionTime = parsed
			}
		}
		if b.earliest.IsZero() || sessionTime.Before(b.earliest) {
			b.earliest = sessionTime
		}
		if sessionTime.After(b.latest) {
			b.latest = sessionTime
		}
	}

	// Fetch professional names via a summary covering all involved sessions.
	// We use the widest span so one call resolves every prof touched here.
	var minTime, maxTime time.Time
	for _, b := range byProf {
		if minTime.IsZero() || b.earliest.Before(minTime) {
			minTime = b.earliest
		}
		if b.latest.After(maxTime) {
			maxTime = b.latest
		}
	}
	if !minTime.IsZero() && !maxTime.IsZero() {
		// Summary uses half-open range; widen `to` to include the last session day.
		to := maxTime.AddDate(0, 0, 1)
		if rows, err := s.repo.Summary(ctx, clinicID, minTime, to); err == nil {
			for _, row := range rows {
				if b := byProf[row.ProfessionalID]; b != nil {
					b.name = row.ProfessionalName
				}
			}
		}
	}

	for profID, b := range byProf {
		if err := s.financeUpserter.CreateFromCommissionBatch(ctx, clinicID, profID, b.total, b.name, b.count, b.earliest, b.latest); err != nil {
			log.Printf("commission: finance sync: insert failed for prof=%s: %v", profID, err)
		}
	}
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
