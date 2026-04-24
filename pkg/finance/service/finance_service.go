package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	appointmentModel "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/finance/model"
	"github.com/aseno/backend/pkg/finance/repository"
	sessionplanModel "github.com/aseno/backend/pkg/sessionplan/model"
)

// FinanceService orchestrates the finance module (manual + auto transactions
// and categories). Keeping the two domains (transactions, categories) in a
// single service matches the other cross-entity services in the codebase and
// keeps the wiring in main.go small.
type FinanceService interface {
	// Manual transactions (UI)
	CreateTransaction(ctx context.Context, req *model.CreateTransactionRequest, clinicID, userID uuid.UUID) (*model.TransactionResponse, error)
	UpdateTransaction(ctx context.Context, id uuid.UUID, req *model.UpdateTransactionRequest, clinicID uuid.UUID) (*model.TransactionResponse, error)
	DeleteTransaction(ctx context.Context, id, clinicID uuid.UUID) error
	GetTransaction(ctx context.Context, id, clinicID uuid.UUID) (*model.TransactionResponse, error)
	ListTransactions(ctx context.Context, clinicID uuid.UUID, filter model.TransactionFilter) ([]*model.TransactionResponse, error)

	// Cash flow summary
	GetCashFlowSummary(ctx context.Context, clinicID uuid.UUID, from, to time.Time) (*model.CashFlowSummary, error)

	// Auto-generation hooks — one finance row per payment event.
	//
	// Appointment payment (N payments per appointment): keyed by
	// appointment_payment.id. Source routes to 'session_plan' when the
	// appointment belongs to a per-session plan; otherwise 'appointment'.
	UpsertFromAppointmentPayment(ctx context.Context, apt *appointmentModel.Appointment, payment *appointmentModel.AppointmentPayment, clinicID uuid.UUID) error
	DeleteFromAppointmentPayment(ctx context.Context, appointmentPaymentID, clinicID uuid.UUID) error

	// Session plan payment (upfront or monthly plan payments that bypass the
	// appointment flow). Keyed by session_plan_payment.id.
	UpsertFromSessionPlanPayment(ctx context.Context, payment *sessionplanModel.SessionPlanPayment, clinicID uuid.UUID, planTitle, patientName string) error
	DeleteFromSessionPlanPayment(ctx context.Context, sessionPlanPaymentID, clinicID uuid.UUID) error

	// Commission batch — one expense row per pay event (not aggregated).
	// The caller just marked `paidIDs` as paid in this action; `totalPaid`
	// is the sum of those commissions, `sessionCount` is len(paidIDs).
	CreateFromCommissionBatch(ctx context.Context, clinicID, professionalID uuid.UUID, totalPaid float64, professionalName string, sessionCount int, earliest, latest time.Time) error

	// Categories
	ListCategories(ctx context.Context, clinicID uuid.UUID) ([]*model.FinancialCategory, error)
	CreateCategory(ctx context.Context, req *model.CreateCategoryRequest, clinicID uuid.UUID) (*model.FinancialCategory, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, req *model.UpdateCategoryRequest, clinicID uuid.UUID) (*model.FinancialCategory, error)
	DeleteCategory(ctx context.Context, id, clinicID uuid.UUID) error
}

type financeService struct {
	txRepo  repository.TransactionRepository
	catRepo repository.CategoryRepository
}

func NewFinanceService(tx repository.TransactionRepository, cat repository.CategoryRepository) FinanceService {
	return &financeService{txRepo: tx, catRepo: cat}
}

// --- Manual transactions --------------------------------------------------

func (s *financeService) CreateTransaction(ctx context.Context, req *model.CreateTransactionRequest, clinicID, userID uuid.UUID) (*model.TransactionResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.ErrValidationInstance.WithMessage("Valor deve ser maior que zero.")
	}
	if req.Type != model.TransactionTypeIncome && req.Type != model.TransactionTypeExpense {
		return nil, errors.ErrValidationInstance.WithMessage("Tipo inválido (use income ou expense).")
	}

	if req.CategoryID != nil {
		cat, err := s.catRepo.GetByID(ctx, *req.CategoryID, clinicID)
		if err != nil {
			return nil, errors.ErrValidationInstance.WithMessage("Categoria não encontrada.")
		}
		if cat.Type != req.Type {
			return nil, errors.ErrValidationInstance.WithMessage("A categoria escolhida não corresponde ao tipo do lançamento.")
		}
	}

	tx := &model.FinancialTransaction{
		ClinicID:        clinicID,
		Type:            req.Type,
		CategoryID:      req.CategoryID,
		Amount:          req.Amount,
		Description:     req.Description,
		Source:          model.SourceManual,
		CompetenceDate:  req.CompetenceDate,
		DueDate:         req.DueDate,
		PaidAt:          req.PaidAt,
		PaymentMethodID: req.PaymentMethodID,
		CreatedBy:       &userID,
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, err
	}
	return tx.ToResponse(), nil
}

func (s *financeService) UpdateTransaction(ctx context.Context, id uuid.UUID, req *model.UpdateTransactionRequest, clinicID uuid.UUID) (*model.TransactionResponse, error) {
	tx, err := s.txRepo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Lançamento não encontrado.")
	}
	if tx.Source != model.SourceManual {
		return nil, errors.ErrValidationInstance.WithMessage("Lançamentos gerados automaticamente não podem ser editados manualmente.")
	}

	if req.Type != nil {
		tx.Type = *req.Type
	}
	if req.CategoryID != nil {
		cat, catErr := s.catRepo.GetByID(ctx, *req.CategoryID, clinicID)
		if catErr != nil {
			return nil, errors.ErrValidationInstance.WithMessage("Categoria não encontrada.")
		}
		if cat.Type != tx.Type {
			return nil, errors.ErrValidationInstance.WithMessage("A categoria escolhida não corresponde ao tipo do lançamento.")
		}
		tx.CategoryID = req.CategoryID
	}
	if req.Amount != nil {
		if *req.Amount <= 0 {
			return nil, errors.ErrValidationInstance.WithMessage("Valor deve ser maior que zero.")
		}
		tx.Amount = *req.Amount
	}
	if req.Description != nil {
		tx.Description = *req.Description
	}
	if req.CompetenceDate != nil {
		tx.CompetenceDate = *req.CompetenceDate
	}
	if req.DueDate != nil {
		tx.DueDate = req.DueDate
	}
	if req.PaidAt != nil {
		tx.PaidAt = req.PaidAt
	}
	if req.PaymentMethodID != nil {
		tx.PaymentMethodID = req.PaymentMethodID
	}

	if err := s.txRepo.Update(ctx, tx); err != nil {
		return nil, err
	}
	return tx.ToResponse(), nil
}

func (s *financeService) DeleteTransaction(ctx context.Context, id, clinicID uuid.UUID) error {
	tx, err := s.txRepo.GetByID(ctx, id, clinicID)
	if err != nil {
		return errors.ErrNotFoundInstance.WithMessage("Lançamento não encontrado.")
	}
	if tx.Source != model.SourceManual {
		return errors.ErrValidationInstance.WithMessage("Lançamentos gerados automaticamente devem ser removidos pelo seu agendamento de origem.")
	}
	return s.txRepo.Delete(ctx, id, clinicID)
}

func (s *financeService) GetTransaction(ctx context.Context, id, clinicID uuid.UUID) (*model.TransactionResponse, error) {
	tx, err := s.txRepo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Lançamento não encontrado.")
	}
	return tx.ToResponse(), nil
}

func (s *financeService) ListTransactions(ctx context.Context, clinicID uuid.UUID, filter model.TransactionFilter) ([]*model.TransactionResponse, error) {
	list, err := s.txRepo.List(ctx, clinicID, filter)
	if err != nil {
		return nil, err
	}
	out := make([]*model.TransactionResponse, len(list))
	for i := range list {
		out[i] = list[i].ToResponse()
	}
	return out, nil
}

// --- Cash flow ------------------------------------------------------------

func (s *financeService) GetCashFlowSummary(ctx context.Context, clinicID uuid.UUID, from, to time.Time) (*model.CashFlowSummary, error) {
	income, expense, err := s.txRepo.SumByPeriod(ctx, clinicID, from, to)
	if err != nil {
		return nil, err
	}
	return &model.CashFlowSummary{
		From:         from,
		To:           to,
		TotalIncome:  income,
		TotalExpense: expense,
		Balance:      income - expense,
	}, nil
}

// --- Auto-generation from appointment payment ------------------------------

// UpsertFromAppointmentPayment reflects a single appointment_payment event into
// a dedicated financial_transactions row. One finance row per payment event
// means each cash movement (signal, installment, final balance) lands on the
// day it actually happened — Caixa's daily breakdown stays truthful.
//
// Source routes to 'session_plan' when the appointment is tied to a per-session
// plan so the Plano badge lights up in Lançamentos. Keyed by
// appointment_payment.id; re-running the upsert updates the existing row.
func (s *financeService) UpsertFromAppointmentPayment(ctx context.Context, apt *appointmentModel.Appointment, payment *appointmentModel.AppointmentPayment, clinicID uuid.UUID) error {
	existing, err := s.txRepo.GetByAppointmentPaymentID(ctx, payment.ID, clinicID)
	if err != nil {
		return err
	}

	// A zero-value payment shouldn't exist (service blocks amount<=0), but be
	// defensive: if somehow it's zero or deleted, drop the finance row.
	effective := payment.Amount + payment.Discount
	if effective <= 0 || payment.DeletedAt != nil {
		if existing == nil {
			return nil
		}
		return s.txRepo.DeleteByAppointmentPaymentID(ctx, payment.ID, clinicID)
	}

	description := "Atendimento"
	if apt.Patient != nil && apt.Patient.Name != "" {
		description = fmt.Sprintf("Atendimento - %s", apt.Patient.Name)
	}

	source := model.SourceAppointment
	if apt.SessionPlanID != nil {
		source = model.SourceSessionPlan
	}

	paidAt := payment.PaidAt
	if paidAt.IsZero() {
		paidAt = time.Now()
	}
	competence := paidAt

	pmID := payment.PaymentMethodID
	aptID := apt.ID
	paymentID := payment.ID

	if existing == nil {
		var planID *uuid.UUID
		if apt.SessionPlanID != nil {
			id := *apt.SessionPlanID
			planID = &id
		}
		tx := &model.FinancialTransaction{
			ClinicID:             clinicID,
			Type:                 model.TransactionTypeIncome,
			Amount:               effective,
			Description:          description,
			Source:               source,
			AppointmentID:        &aptID,
			AppointmentPaymentID: &paymentID,
			SessionPlanID:        planID,
			CompetenceDate:       competence,
			PaidAt:               &paidAt,
			PaymentMethodID:      &pmID,
		}
		return s.txRepo.Create(ctx, tx)
	}

	// Same shape → no-op (avoids needless writes when status updates replay).
	samePM := existing.PaymentMethodID != nil && *existing.PaymentMethodID == pmID
	if existing.Amount == effective && samePM && existing.Description == description {
		return nil
	}

	existing.Amount = effective
	existing.PaymentMethodID = &pmID
	existing.PaidAt = &paidAt
	existing.CompetenceDate = competence
	existing.Description = description
	existing.Source = source
	return s.txRepo.Update(ctx, existing)
}

// DeleteFromAppointmentPayment removes the finance row tied to a payment that
// was just deleted from the appointment.
func (s *financeService) DeleteFromAppointmentPayment(ctx context.Context, appointmentPaymentID, clinicID uuid.UUID) error {
	return s.txRepo.DeleteByAppointmentPaymentID(ctx, appointmentPaymentID, clinicID)
}

// --- Auto-generation from session plan payment ----------------------------

// UpsertFromSessionPlanPayment handles upfront / monthly plan payments, which
// live in a separate table (session_plan_payments) and never touch
// appointment_payments. Keyed by session_plan_payment.id.
func (s *financeService) UpsertFromSessionPlanPayment(ctx context.Context, payment *sessionplanModel.SessionPlanPayment, clinicID uuid.UUID, planTitle, patientName string) error {
	existing, err := s.txRepo.GetBySessionPlanPaymentID(ctx, payment.ID, clinicID)
	if err != nil {
		return err
	}

	effective := payment.Amount + payment.Discount
	if effective <= 0 || payment.DeletedAt.Valid {
		if existing == nil {
			return nil
		}
		return s.txRepo.DeleteBySessionPlanPaymentID(ctx, payment.ID, clinicID)
	}

	description := "Plano de sessões"
	switch {
	case patientName != "" && planTitle != "":
		description = fmt.Sprintf("Plano %s - %s", planTitle, patientName)
	case patientName != "":
		description = fmt.Sprintf("Plano de sessões - %s", patientName)
	case planTitle != "":
		description = fmt.Sprintf("Plano %s", planTitle)
	}

	paidAt := payment.PaidAt
	if paidAt.IsZero() {
		paidAt = time.Now()
	}

	pmID := payment.PaymentMethodID
	paymentID := payment.ID
	planID := payment.SessionPlanID

	if existing == nil {
		tx := &model.FinancialTransaction{
			ClinicID:             clinicID,
			Type:                 model.TransactionTypeIncome,
			Amount:               effective,
			Description:          description,
			Source:               model.SourceSessionPlan,
			SessionPlanID:        &planID,
			SessionPlanPaymentID: &paymentID,
			CompetenceDate:       paidAt,
			PaidAt:               &paidAt,
			PaymentMethodID:      &pmID,
		}
		return s.txRepo.Create(ctx, tx)
	}

	samePM := existing.PaymentMethodID != nil && *existing.PaymentMethodID == pmID
	if existing.Amount == effective && samePM && existing.Description == description {
		return nil
	}

	existing.Amount = effective
	existing.PaymentMethodID = &pmID
	existing.PaidAt = &paidAt
	existing.CompetenceDate = paidAt
	existing.Description = description
	return s.txRepo.Update(ctx, existing)
}

func (s *financeService) DeleteFromSessionPlanPayment(ctx context.Context, sessionPlanPaymentID, clinicID uuid.UUID) error {
	return s.txRepo.DeleteBySessionPlanPaymentID(ctx, sessionPlanPaymentID, clinicID)
}

// --- Auto-generation from commission payment ------------------------------

// CreateFromCommissionBatch inserts one expense row per BulkMarkPaid event.
// No key, no upsert: each cash movement is a distinct row on its own date —
// matches how Caixa groups by day. Granular per-session commissions live in
// the dedicated Repasse tab.
func (s *financeService) CreateFromCommissionBatch(ctx context.Context, clinicID, professionalID uuid.UUID, totalPaid float64, professionalName string, sessionCount int, earliest, latest time.Time) error {
	if totalPaid <= 0 || sessionCount <= 0 {
		return nil
	}
	if professionalName == "" {
		professionalName = "Profissional"
	}

	now := time.Now()

	// Build description with session count + date range when provided.
	sessionsPart := fmt.Sprintf("%d %s", sessionCount, pluralizeSessions(sessionCount))
	rangePart := ""
	if !earliest.IsZero() && !latest.IsZero() {
		if earliest.Format("2006-01-02") == latest.Format("2006-01-02") {
			rangePart = fmt.Sprintf(" (%s)", earliest.Format("02/01"))
		} else {
			rangePart = fmt.Sprintf(" (%s a %s)", earliest.Format("02/01"), latest.Format("02/01"))
		}
	}
	description := fmt.Sprintf("Repasse %s — %s%s", professionalName, sessionsPart, rangePart)

	profID := professionalID
	tx := &model.FinancialTransaction{
		ClinicID:       clinicID,
		Type:           model.TransactionTypeExpense,
		Amount:         totalPaid,
		Description:    description,
		Source:         model.SourceCommission,
		ProfessionalID: &profID,
		CompetenceDate: now,
		PaidAt:         &now,
	}
	return s.txRepo.Create(ctx, tx)
}

func pluralizeSessions(n int) string {
	if n == 1 {
		return "sessão"
	}
	return "sessões"
}

// --- Categories -----------------------------------------------------------

// ListCategories returns the clinic's categories. If none exist yet, seeds a
// set of defaults so the UI never shows an empty picker on first use.
func (s *financeService) ListCategories(ctx context.Context, clinicID uuid.UUID) ([]*model.FinancialCategory, error) {
	count, err := s.catRepo.CountByClinic(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		if err := s.catRepo.CreateDefaults(ctx, clinicID); err != nil {
			return nil, err
		}
	}

	cats, err := s.catRepo.ListByClinic(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.FinancialCategory, len(cats))
	for i := range cats {
		out[i] = &cats[i]
	}
	return out, nil
}

func (s *financeService) CreateCategory(ctx context.Context, req *model.CreateCategoryRequest, clinicID uuid.UUID) (*model.FinancialCategory, error) {
	if req.Name == "" {
		return nil, errors.ErrValidationInstance.WithMessage("Nome da categoria é obrigatório.")
	}
	if req.Type != model.TransactionTypeIncome && req.Type != model.TransactionTypeExpense {
		return nil, errors.ErrValidationInstance.WithMessage("Tipo inválido (use income ou expense).")
	}

	cat := &model.FinancialCategory{
		ClinicID:  clinicID,
		Name:      req.Name,
		Type:      req.Type,
		Color:     req.Color,
		IsDefault: false,
	}
	if err := s.catRepo.Create(ctx, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *financeService) UpdateCategory(ctx context.Context, id uuid.UUID, req *model.UpdateCategoryRequest, clinicID uuid.UUID) (*model.FinancialCategory, error) {
	cat, err := s.catRepo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Categoria não encontrada.")
	}
	if req.Name != nil {
		cat.Name = *req.Name
	}
	if req.Color != nil {
		cat.Color = *req.Color
	}
	if err := s.catRepo.Update(ctx, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *financeService) DeleteCategory(ctx context.Context, id, clinicID uuid.UUID) error {
	cat, err := s.catRepo.GetByID(ctx, id, clinicID)
	if err != nil {
		return errors.ErrNotFoundInstance.WithMessage("Categoria não encontrada.")
	}
	// Default categories are part of the onboarding experience — blocking deletion
	// keeps the seed list stable and avoids orphaning the UI picker.
	if cat.IsDefault {
		return errors.ErrValidationInstance.WithMessage("Categorias padrão não podem ser excluídas.")
	}
	return s.catRepo.Delete(ctx, id, clinicID)
}
