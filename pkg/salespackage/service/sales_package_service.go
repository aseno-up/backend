package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/internal/logger"
	auditModel "github.com/aseno/backend/pkg/audit/model"
	paymentmethodModel "github.com/aseno/backend/pkg/paymentmethod/model"
	"github.com/aseno/backend/pkg/salespackage/model"
	"github.com/aseno/backend/pkg/salespackage/repository"
	systemlogModel "github.com/aseno/backend/pkg/systemlog/model"
)

// --- Collaborators (interfaces wired in main.go) --------------------------

// PaymentMethodLookup reads a payment method's settlement parameters. The
// service uses these to compute due dates and to validate installments.
type PaymentMethodLookup interface {
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*paymentmethodModel.PaymentMethod, error)
}

// FinanceHook owns the FinancialTransaction <-> SalesPackagePayment
// reflection. The service calls Upsert per payment row on sale activation
// and on registerPayment; calls Delete when a payment is removed (currently
// only on package cancel).
type FinanceHook interface {
	UpsertFromSalesPackagePayment(ctx context.Context, payment *model.SalesPackagePayment, pkg *model.SalesPackage, patientName string) error
	DeleteFromSalesPackagePayment(ctx context.Context, salesPackagePaymentID, clinicID uuid.UUID) error
}

// PatientLookup resolves the patient name used for descriptions and search.
type PatientLookup interface {
	GetName(ctx context.Context, patientID, clinicID uuid.UUID) (string, error)
}

// AuditLogger captures CRUD events. Wired against the existing audit service.
type AuditLogger interface {
	Log(ctx context.Context, clinicID, userID uuid.UUID, action auditModel.AuditAction, entityID string, oldData, newData interface{})
}

// SystemLogger captures domain events for observability.
type SystemLogger interface {
	Log(ctx context.Context, clinicID *uuid.UUID, event string, status systemlogModel.LogStatus, payload interface{}, err string)
}

// --- Service contract -----------------------------------------------------

// SalesPackageService is the public API for the salespackage domain.
type SalesPackageService interface {
	Create(ctx context.Context, clinicID, userID uuid.UUID, req *model.CreateSalesPackageRequest) (*model.SalesPackageResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SalesPackageResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, filter model.SalesPackageFilter) (*model.SalesPackageListResponse, error)
	Update(ctx context.Context, id, clinicID, userID uuid.UUID, req *model.UpdateSalesPackageRequest) (*model.SalesPackageResponse, error)
	ConvertQuoteToActive(ctx context.Context, id, clinicID, userID uuid.UUID, payments []model.CreateSalesPackagePaymentInput) (*model.SalesPackageResponse, error)
	Cancel(ctx context.Context, id, clinicID, userID uuid.UUID) (*model.SalesPackageResponse, error)

	// Patient / agenda integration
	ListByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.SalesPackageResponse, error)
	GetBalancesByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]model.ProcedureBalance, error)
	PickFIFOItemID(ctx context.Context, clinicID, patientID, procedureID uuid.UUID) (*uuid.UUID, error)

	// Payment lifecycle
	RegisterPayment(ctx context.Context, paymentID, clinicID, userID uuid.UUID, req *model.RegisterPaymentRequest) (*model.SalesPackagePaymentResponse, error)
}

type salesPackageService struct {
	repo          repository.SalesPackageRepository
	paymentMethod PaymentMethodLookup
	finance       FinanceHook
	patient       PatientLookup
	audit         AuditLogger
	syslog        SystemLogger
}

// NewSalesPackageService builds the service. Audit/syslog/finance/patient may
// be nil only in tests — production wiring always supplies all collaborators.
func NewSalesPackageService(
	repo repository.SalesPackageRepository,
	paymentMethod PaymentMethodLookup,
	finance FinanceHook,
	patient PatientLookup,
	audit AuditLogger,
	syslog SystemLogger,
) SalesPackageService {
	return &salesPackageService{
		repo:          repo,
		paymentMethod: paymentMethod,
		finance:       finance,
		patient:       patient,
		audit:         audit,
		syslog:        syslog,
	}
}

// --- Create ---------------------------------------------------------------

func (s *salesPackageService) Create(ctx context.Context, clinicID, userID uuid.UUID, req *model.CreateSalesPackageRequest) (*model.SalesPackageResponse, error) {
	if req == nil || len(req.Items) == 0 {
		return nil, errors.ErrValidationInstance.WithMessage("Informe pelo menos um item na venda.")
	}
	if req.Status != model.SalesPackageStatusQuote && req.Status != model.SalesPackageStatusActive {
		return nil, errors.ErrValidationInstance.WithMessage("Status inicial deve ser 'quote' ou 'active'.")
	}
	// Active sales no longer require any payment up-front: an unpaid sale
	// becomes an open receivable (one method-less SalesPackagePayment row
	// covering the full total). Same applies to partial payments — the
	// remainder is auto-added by expandPayments.

	saleDate := time.Now().UTC().Truncate(24 * time.Hour)
	if req.SaleDate != nil {
		saleDate = req.SaleDate.UTC().Truncate(24 * time.Hour)
	}
	if req.ValidUntil != nil && req.ValidUntil.Before(saleDate) {
		return nil, errors.ErrValidationInstance.WithMessage("Data de validade não pode ser anterior à data da venda.")
	}

	// Build items + compute subtotal
	items := make([]model.SalesPackageItem, 0, len(req.Items))
	var subtotal float64
	for _, in := range req.Items {
		if in.Quantity <= 0 {
			return nil, errors.ErrValidationInstance.WithMessage("Quantidade de sessões deve ser maior que zero.")
		}
		if in.UnitPrice < 0 {
			return nil, errors.ErrValidationInstance.WithMessage("Preço unitário não pode ser negativo.")
		}
		line := round2(in.UnitPrice * float64(in.Quantity))
		subtotal = round2(subtotal + line)
		items = append(items, model.SalesPackageItem{
			ProcedureID: in.ProcedureID,
			UnitPrice:   in.UnitPrice,
			Quantity:    in.Quantity,
			LineTotal:   line,
		})
	}
	if req.Discount < 0 {
		return nil, errors.ErrValidationInstance.WithMessage("Desconto não pode ser negativo.")
	}
	if req.Discount > subtotal {
		return nil, errors.ErrValidationInstance.WithMessage("Desconto maior que o subtotal.")
	}
	total := round2(subtotal - req.Discount)

	// Build payments — only required for status=active. Quotes ignore the input
	// to avoid persisting receivables for a sale that may never close.
	var payments []model.SalesPackagePayment
	if req.Status == model.SalesPackageStatusActive {
		expanded, err := s.expandPayments(ctx, clinicID, saleDate, total, req.Payments)
		if err != nil {
			return nil, err
		}
		payments = expanded
	}

	// Reserve a clinic-scoped sequential code.
	year := saleDate.Year()
	next, err := s.repo.NextCodeForYear(ctx, clinicID, year)
	if err != nil {
		return nil, err
	}
	code := fmt.Sprintf("V-%d-%03d", year, next)

	pkg := &model.SalesPackage{
		ClinicID:   clinicID,
		PatientID:  req.PatientID,
		Code:       code,
		Status:     req.Status,
		SaleDate:   saleDate,
		ValidUntil: req.ValidUntil,
		Subtotal:   subtotal,
		Discount:   req.Discount,
		Total:      total,
		Notes:      req.Notes,
		CreatedBy:  &userID,
		Items:      items,
		Payments:   payments,
	}

	// One transaction so the sale, items, and payments are all-or-nothing.
	err = s.repo.WithTx(ctx, func(txRepo repository.SalesPackageRepository) error {
		// Re-attach clinic_id on payments before insert (GORM cascades but we
		// need to set the field explicitly because it's not derivable from the
		// parent struct).
		for i := range pkg.Payments {
			pkg.Payments[i].ClinicID = clinicID
		}
		return txRepo.Create(ctx, pkg)
	})
	if err != nil {
		s.systemLog(ctx, &clinicID, "create_failed", systemlogModel.LogStatusFailed, map[string]interface{}{"patient_id": req.PatientID, "status": req.Status}, err.Error())
		return nil, err
	}

	// Reflect each payment into the finance ledger. Done outside the tx so a
	// finance hiccup never rolls back the sale itself — at worst a manual
	// reconciliation is needed, which is preferable to losing the sale.
	if pkg.Status == model.SalesPackageStatusActive {
		patientName := s.patientName(ctx, clinicID, req.PatientID)
		for i := range pkg.Payments {
			if err := s.finance.UpsertFromSalesPackagePayment(ctx, &pkg.Payments[i], pkg, patientName); err != nil {
				logger.Printf("[SalesPackage] finance upsert failed for payment %s: %v", pkg.Payments[i].ID, err)
				s.systemLog(ctx, &clinicID, "finance_upsert_failed", systemlogModel.LogStatusFailed, map[string]interface{}{"payment_id": pkg.Payments[i].ID}, err.Error())
			}
		}
	}

	s.auditLog(ctx, clinicID, userID, auditModel.AuditActionCreate, pkg.ID.String(), nil, pkg)
	event := "sale_created"
	if pkg.Status == model.SalesPackageStatusQuote {
		event = "quote_created"
	}
	s.systemLog(ctx, &clinicID, event, systemlogModel.LogStatusSuccess, map[string]interface{}{"sales_package_id": pkg.ID, "code": pkg.Code, "total": pkg.Total}, "")

	return s.fetchResponse(ctx, pkg.ID, clinicID)
}

// expandPayments turns user-supplied payment lines into one row per parcel,
// validates installments against the payment method, and computes due dates
// from the method's settlement_days. Any remainder between sum(inputs) and
// total is appended as a single "open receivable" row with no payment_method —
// representing the part of the sale the user hasn't decided how to charge yet.
//
// Sum of inputs may be zero (no payments at all → entire total becomes an
// open receivable) or up to total. Anything above total is rejected.
func (s *salesPackageService) expandPayments(ctx context.Context, clinicID uuid.UUID, saleDate time.Time, total float64, inputs []model.CreateSalesPackagePaymentInput) ([]model.SalesPackagePayment, error) {
	var sumGross float64
	for _, in := range inputs {
		if in.Amount <= 0 {
			return nil, errors.ErrValidationInstance.WithMessage("Valor de pagamento deve ser maior que zero.")
		}
		sumGross = round2(sumGross + in.Amount - in.Discount)
	}
	if sumGross-total > 0.005 {
		return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Soma das formas de pagamento (R$ %.2f) é maior que o total da venda (R$ %.2f).", sumGross, total))
	}

	out := make([]model.SalesPackagePayment, 0, len(inputs)+1)
	for _, in := range inputs {
		method, err := s.paymentMethod.GetByID(ctx, in.PaymentMethodID, clinicID)
		if err != nil {
			return nil, errors.ErrValidationInstance.WithMessage("Forma de pagamento inválida.")
		}
		if !method.Active {
			return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' está inativa.", method.Name))
		}

		installments := 1
		if in.Installments > 0 {
			installments = in.Installments
		}
		if installments > 1 {
			if !method.InstallmentsAllowed {
				return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' não aceita parcelamento.", method.Name))
			}
			if installments > method.MaxInstallments {
				return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' aceita no máximo %d parcelas.", method.Name, method.MaxInstallments))
			}
		}

		baseDue := saleDate.AddDate(0, 0, method.SettlementDays)
		if in.DueDate != nil {
			baseDue = in.DueDate.UTC().Truncate(24 * time.Hour)
		}

		gross := in.Amount - in.Discount
		share := round2(gross / float64(installments))
		accumulated := 0.0
		methodID := in.PaymentMethodID
		for i := 1; i <= installments; i++ {
			amount := share
			if i == installments {
				amount = round2(gross - accumulated)
			}
			accumulated = round2(accumulated + amount)

			due := baseDue
			if installments > 1 {
				due = saleDate.AddDate(0, 0, method.SettlementDays*i)
				if in.DueDate != nil {
					due = in.DueDate.AddDate(0, 0, method.SettlementDays*(i-1))
				}
			}

			row := model.SalesPackagePayment{
				ClinicID:          clinicID,
				PaymentMethodID:   &methodID,
				Amount:            amount,
				Discount:          0,
				Installment:       i,
				TotalInstallments: installments,
				DueDate:           due,
				PaidAt:            nil,
			}
			if in.PaidAt != nil && i == 1 && installments == 1 {
				paidAt := in.PaidAt.UTC()
				row.PaidAt = &paidAt
			}
			out = append(out, row)
		}
	}

	// Append the open-receivable row for whatever remains.
	remaining := round2(total - sumGross)
	if remaining > 0.005 {
		out = append(out, model.SalesPackagePayment{
			ClinicID:          clinicID,
			PaymentMethodID:   nil, // method-less: user picks it on RegisterPayment
			Amount:            remaining,
			Discount:          0,
			Installment:       1,
			TotalInstallments: 1,
			DueDate:           saleDate,
			PaidAt:            nil,
		})
	}
	return out, nil
}

// --- Read -----------------------------------------------------------------

func (s *salesPackageService) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SalesPackageResponse, error) {
	return s.fetchResponse(ctx, id, clinicID)
}

func (s *salesPackageService) List(ctx context.Context, clinicID uuid.UUID, filter model.SalesPackageFilter) (*model.SalesPackageListResponse, error) {
	rows, total, err := s.repo.List(ctx, clinicID, filter)
	if err != nil {
		return nil, err
	}
	out := make([]*model.SalesPackageResponse, len(rows))
	for i := range rows {
		out[i] = rows[i].ToResponse()
	}
	if err := s.attachItemsBalance(ctx, out); err != nil {
		return nil, err
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage <= 0 {
		perPage = 20
	}
	totalPages := 0
	if perPage > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}
	return &model.SalesPackageListResponse{
		Data:       out,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

func (s *salesPackageService) ListByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.SalesPackageResponse, error) {
	rows, err := s.repo.ListByPatient(ctx, clinicID, patientID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.SalesPackageResponse, len(rows))
	for i := range rows {
		out[i] = rows[i].ToResponse()
	}
	if err := s.attachItemsBalance(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *salesPackageService) GetBalancesByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]model.ProcedureBalance, error) {
	return s.repo.GetBalancesByPatient(ctx, clinicID, patientID)
}

func (s *salesPackageService) PickFIFOItemID(ctx context.Context, clinicID, patientID, procedureID uuid.UUID) (*uuid.UUID, error) {
	item, err := s.repo.PickFIFOItem(ctx, clinicID, patientID, procedureID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	id := item.ID
	return &id, nil
}

// --- Update ---------------------------------------------------------------

func (s *salesPackageService) Update(ctx context.Context, id, clinicID, userID uuid.UUID, req *model.UpdateSalesPackageRequest) (*model.SalesPackageResponse, error) {
	pkg, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Venda não encontrada.")
		}
		return nil, err
	}
	if pkg.Status == model.SalesPackageStatusCanceled {
		return nil, errors.ErrValidationInstance.WithMessage("Não é possível editar uma venda cancelada.")
	}

	old := *pkg
	if req.SaleDate != nil {
		pkg.SaleDate = req.SaleDate.UTC().Truncate(24 * time.Hour)
	}
	if req.ValidUntil != nil {
		v := req.ValidUntil.UTC().Truncate(24 * time.Hour)
		pkg.ValidUntil = &v
	}
	if req.Discount != nil {
		if *req.Discount < 0 {
			return nil, errors.ErrValidationInstance.WithMessage("Desconto não pode ser negativo.")
		}
		if *req.Discount > pkg.Subtotal {
			return nil, errors.ErrValidationInstance.WithMessage("Desconto maior que o subtotal.")
		}
		pkg.Discount = *req.Discount
		pkg.Total = round2(pkg.Subtotal - pkg.Discount)
	}
	if req.Notes != nil {
		pkg.Notes = *req.Notes
	}
	if pkg.ValidUntil != nil && pkg.ValidUntil.Before(pkg.SaleDate) {
		return nil, errors.ErrValidationInstance.WithMessage("Data de validade não pode ser anterior à data da venda.")
	}

	if err := s.repo.Update(ctx, pkg); err != nil {
		return nil, err
	}

	s.auditLog(ctx, clinicID, userID, auditModel.AuditActionUpdate, pkg.ID.String(), &old, pkg)
	s.systemLog(ctx, &clinicID, "sale_updated", systemlogModel.LogStatusSuccess, map[string]interface{}{"sales_package_id": pkg.ID}, "")
	return s.fetchResponse(ctx, pkg.ID, clinicID)
}

// --- Convert quote → active ----------------------------------------------

func (s *salesPackageService) ConvertQuoteToActive(ctx context.Context, id, clinicID, userID uuid.UUID, payments []model.CreateSalesPackagePaymentInput) (*model.SalesPackageResponse, error) {
	pkg, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Venda não encontrada.")
		}
		return nil, err
	}
	if pkg.Status != model.SalesPackageStatusQuote {
		return nil, errors.ErrValidationInstance.WithMessage("Apenas orçamentos podem ser convertidos.")
	}

	expanded, err := s.expandPayments(ctx, clinicID, pkg.SaleDate, pkg.Total, payments)
	if err != nil {
		return nil, err
	}
	for i := range expanded {
		expanded[i].SalesPackageID = pkg.ID
		expanded[i].ClinicID = clinicID
	}

	err = s.repo.WithTx(ctx, func(txRepo repository.SalesPackageRepository) error {
		if err := txRepo.UpdateStatus(ctx, pkg.ID, clinicID, model.SalesPackageStatusActive); err != nil {
			return err
		}
		return txRepo.CreatePayments(ctx, expanded)
	})
	if err != nil {
		return nil, err
	}

	patientName := s.patientName(ctx, clinicID, pkg.PatientID)
	pkg.Status = model.SalesPackageStatusActive
	for i := range expanded {
		if err := s.finance.UpsertFromSalesPackagePayment(ctx, &expanded[i], pkg, patientName); err != nil {
			logger.Printf("[SalesPackage] finance upsert failed on convert for payment %s: %v", expanded[i].ID, err)
			s.systemLog(ctx, &clinicID, "finance_upsert_failed", systemlogModel.LogStatusFailed, map[string]interface{}{"payment_id": expanded[i].ID}, err.Error())
		}
	}

	s.auditLog(ctx, clinicID, userID, auditModel.AuditActionUpdate, pkg.ID.String(), map[string]interface{}{"status": "quote"}, map[string]interface{}{"status": "active"})
	s.systemLog(ctx, &clinicID, "quote_converted", systemlogModel.LogStatusSuccess, map[string]interface{}{"sales_package_id": pkg.ID}, "")

	return s.fetchResponse(ctx, pkg.ID, clinicID)
}

// --- Cancel ---------------------------------------------------------------

func (s *salesPackageService) Cancel(ctx context.Context, id, clinicID, userID uuid.UUID) (*model.SalesPackageResponse, error) {
	pkg, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Venda não encontrada.")
		}
		return nil, err
	}
	if pkg.Status == model.SalesPackageStatusCanceled {
		return pkg.ToResponse(), nil
	}

	if err := s.repo.UpdateStatus(ctx, pkg.ID, clinicID, model.SalesPackageStatusCanceled); err != nil {
		return nil, err
	}

	// Finance entries are NOT auto-deleted: keeping them lets the user
	// reconcile manually if a partial refund applies (per requirements,
	// refund logic is out of scope).
	s.auditLog(ctx, clinicID, userID, auditModel.AuditActionUpdate, pkg.ID.String(), map[string]interface{}{"status": pkg.Status}, map[string]interface{}{"status": model.SalesPackageStatusCanceled})
	s.systemLog(ctx, &clinicID, "sale_canceled", systemlogModel.LogStatusSuccess, map[string]interface{}{"sales_package_id": pkg.ID}, "")

	return s.fetchResponse(ctx, pkg.ID, clinicID)
}

// --- Register payment -----------------------------------------------------

func (s *salesPackageService) RegisterPayment(ctx context.Context, paymentID, clinicID, userID uuid.UUID, req *model.RegisterPaymentRequest) (*model.SalesPackagePaymentResponse, error) {
	payment, err := s.repo.GetPaymentByID(ctx, paymentID, clinicID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Pagamento não encontrado.")
		}
		return nil, err
	}

	pkg, err := s.repo.GetByID(ctx, payment.SalesPackageID, clinicID)
	if err != nil {
		return nil, err
	}

	old := *payment
	paidAt := req.PaidAt.UTC()
	payment.PaidAt = &paidAt
	if req.PaymentMethodID != nil {
		id := *req.PaymentMethodID
		payment.PaymentMethodID = &id
	}
	if req.Amount != nil {
		payment.Amount = *req.Amount
	}
	if err := s.repo.UpdatePayment(ctx, payment); err != nil {
		return nil, err
	}

	patientName := s.patientName(ctx, clinicID, pkg.PatientID)
	if err := s.finance.UpsertFromSalesPackagePayment(ctx, payment, pkg, patientName); err != nil {
		logger.Printf("[SalesPackage] finance upsert failed on register for payment %s: %v", payment.ID, err)
		s.systemLog(ctx, &clinicID, "finance_upsert_failed", systemlogModel.LogStatusFailed, map[string]interface{}{"payment_id": payment.ID}, err.Error())
	}

	s.auditLog(ctx, clinicID, userID, auditModel.AuditActionUpdate, payment.ID.String(), &old, payment)
	s.systemLog(ctx, &clinicID, "payment_registered", systemlogModel.LogStatusSuccess, map[string]interface{}{"payment_id": payment.ID, "amount": payment.Amount}, "")

	return payment.ToResponse(), nil
}

// --- Helpers --------------------------------------------------------------

func (s *salesPackageService) fetchResponse(ctx context.Context, id, clinicID uuid.UUID) (*model.SalesPackageResponse, error) {
	pkg, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Venda não encontrada.")
		}
		return nil, err
	}
	resp := pkg.ToResponse()
	if name := s.patientName(ctx, clinicID, pkg.PatientID); name != "" {
		resp.PatientName = name
	}
	if err := s.attachItemsBalance(ctx, []*model.SalesPackageResponse{resp}); err != nil {
		return nil, err
	}
	return resp, nil
}

// attachItemsBalance fills items_balance on each response in a single batched
// query so the list view doesn't fan out to N+1 lookups.
func (s *salesPackageService) attachItemsBalance(ctx context.Context, responses []*model.SalesPackageResponse) error {
	itemIDs := make([]uuid.UUID, 0)
	for _, r := range responses {
		for _, it := range r.Items {
			itemIDs = append(itemIDs, it.ID)
		}
	}
	if len(itemIDs) == 0 {
		return nil
	}
	usage, err := s.repo.GetItemUsage(ctx, itemIDs)
	if err != nil {
		return err
	}
	for _, r := range responses {
		balances := make([]*model.SalesPackageItemBalanceResponse, 0, len(r.Items))
		for _, it := range r.Items {
			balances = append(balances, &model.SalesPackageItemBalanceResponse{
				ItemID:      it.ID,
				ProcedureID: it.ProcedureID,
				Used:        usage[it.ID],
				Quantity:    it.Quantity,
			})
		}
		r.ItemsBalance = balances
	}
	return nil
}

func (s *salesPackageService) patientName(ctx context.Context, clinicID, patientID uuid.UUID) string {
	if s.patient == nil {
		return ""
	}
	name, err := s.patient.GetName(ctx, patientID, clinicID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

func (s *salesPackageService) auditLog(ctx context.Context, clinicID, userID uuid.UUID, action auditModel.AuditAction, entityID string, oldData, newData interface{}) {
	if s.audit == nil {
		return
	}
	s.audit.Log(ctx, clinicID, userID, action, entityID, oldData, newData)
}

func (s *salesPackageService) systemLog(ctx context.Context, clinicID *uuid.UUID, event string, status systemlogModel.LogStatus, payload interface{}, errStr string) {
	if s.syslog == nil {
		return
	}
	s.syslog.Log(ctx, clinicID, event, status, payload, errStr)
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func nearlyEqual(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.005
}
