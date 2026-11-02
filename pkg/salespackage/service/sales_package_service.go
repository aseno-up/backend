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
	AddPayment(ctx context.Context, salesPackageID, clinicID, userID uuid.UUID, input *model.CreateSalesPackagePaymentInput) (*model.SalesPackageResponse, error)
	UpdatePaymentRow(ctx context.Context, paymentID, clinicID, userID uuid.UUID, req *model.UpdatePaymentRowRequest) (*model.SalesPackagePaymentResponse, error)
	DeletePaymentRow(ctx context.Context, paymentID, clinicID, userID uuid.UUID) error
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

	year := saleDate.Year()
	pkg := &model.SalesPackage{
		ClinicID:   clinicID,
		PatientID:  req.PatientID,
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
	// The clinic+year code lock is taken first so the MAX+1 read inside this
	// tx can't race with parallel Creates picking the same number.
	err := s.repo.WithTx(ctx, func(txRepo repository.SalesPackageRepository) error {
		if err := txRepo.AcquireCodeLock(ctx, clinicID, year); err != nil {
			return err
		}
		next, err := txRepo.NextCodeForYear(ctx, clinicID, year)
		if err != nil {
			return err
		}
		pkg.Code = fmt.Sprintf("V-%d-%03d", year, next)
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

		// First-parcel base due. The user's explicit due_date wins over the
		// method's settlement_days so partner clinics with custom acquirer
		// terms can still drive the schedule by hand.
		baseDue := saleDate.AddDate(0, 0, method.SettlementDays)
		if in.DueDate != nil {
			baseDue = in.DueDate.UTC().Truncate(24 * time.Hour)
		}

		gross := in.Amount - in.Discount
		methodID := in.PaymentMethodID

		// Upfront acquirers pay the clinic the full sale as a single payout,
		// regardless of how many installments the customer chose. Collapse to
		// a single row so the cashier shows ONE receivable.
		if method.SettlementMode == paymentmethodModel.SettlementModeUpfront {
			row := model.SalesPackagePayment{
				ClinicID:          clinicID,
				PaymentMethodID:   &methodID,
				Amount:            round2(gross),
				Discount:          0,
				Installment:       1,
				TotalInstallments: 1,
				DueDate:           baseDue,
				PaidAt:            nil,
			}
			// "Pago em" is a flag: filled = mark as paid. The actual paid_at
			// date is always sale_date + settlement_days — the user backdates
			// by changing the sale date, not by typing a different "Pago em".
			if in.PaidAt != nil {
				row.PaidAt = &baseDue
			}
			out = append(out, row)
			continue
		}

		// Monthly mode (the default for installments). Parcel 1 lands on
		// baseDue (saleDate + settlement_days); each subsequent parcel falls
		// one calendar month after the previous one. Calendar-aligned so a
		// small settlement_days (e.g. 3) doesn't accidentally schedule 12
		// parcels inside one month.
		//
		// Legacy "fixed_days" rows fall through this path too — the user-
		// visible API no longer offers that mode.
		share := round2(gross / float64(installments))
		accumulated := 0.0
		for i := 1; i <= installments; i++ {
			amount := share
			if i == installments {
				amount = round2(gross - accumulated)
			}
			accumulated = round2(accumulated + amount)

			due := baseDue
			if installments > 1 {
				due = baseDue.AddDate(0, i-1, 0)
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
			// Single-installment row marked as paid: paid_at always tracks
			// sale_date + settlement_days. See upfront branch above.
			if in.PaidAt != nil && i == 1 && installments == 1 {
				row.PaidAt = &due
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

	// Resolve gross amount — req.Amount overrides the receivable's amount,
	// useful when settling a receivable for less than the full balance.
	gross := payment.Amount
	if req.Amount != nil {
		gross = *req.Amount
	}
	discount := 0.0
	if req.Discount != nil {
		discount = *req.Discount
	}
	netAmount := round2(gross - discount)
	if netAmount <= 0 {
		return nil, errors.ErrValidationInstance.WithMessage("Valor efetivo deve ser maior que zero (verifique desconto).")
	}

	installments := 1
	if req.Installments != nil && *req.Installments > 0 {
		installments = *req.Installments
	}

	// Resolve method (req override or the row's existing method). The user
	// can register a method-less open receivable by supplying req.PaymentMethodID.
	var methodID *uuid.UUID
	if req.PaymentMethodID != nil {
		id := *req.PaymentMethodID
		methodID = &id
	} else {
		methodID = payment.PaymentMethodID
	}

	// Installments require a method we can validate against. A method is also
	// needed to compute due dates for split rows.
	if installments > 1 && methodID == nil {
		return nil, errors.ErrValidationInstance.WithMessage("Selecione uma forma de pagamento para parcelar.")
	}

	var method *paymentmethodModel.PaymentMethod
	if methodID != nil {
		method, err = s.paymentMethod.GetByID(ctx, *methodID, clinicID)
		if err != nil {
			return nil, errors.ErrValidationInstance.WithMessage("Forma de pagamento inválida.")
		}
		if !method.Active {
			return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' está inativa.", method.Name))
		}
		if installments > 1 {
			if !method.InstallmentsAllowed {
				return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' não aceita parcelamento.", method.Name))
			}
			if installments > method.MaxInstallments {
				return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' aceita no máximo %d parcelas.", method.Name, method.MaxInstallments))
			}
		}
	}

	paidAt := req.PaidAt.UTC()

	// Upfront mode: bank pays full amount as a single payout, regardless of
	// customer-facing installments. Same behavior as expandPayments.
	upfront := method != nil && method.SettlementMode == paymentmethodModel.SettlementModeUpfront

	// Build the parcel rows — first row reuses the existing payment.ID, the
	// rest are inserted. For upfront mode, or installments == 1, we keep one
	// row only.
	type parcel struct {
		Amount      float64
		Installment int
		DueDate     time.Time
		PaidAt      *time.Time
	}
	var parcels []parcel

	if upfront || installments == 1 {
		due := pkg.SaleDate
		if method != nil {
			due = pkg.SaleDate.AddDate(0, 0, method.SettlementDays)
		}
		// "Pago em" is a flag (the user clicked Registrar — they want it
		// marked as paid). The stored paid_at is sale_date + settlement_days
		// so the cashier reflects when the money actually lands in the bank.
		// Single non-upfront with no method falls back to the user's input.
		var pa *time.Time
		if installments == 1 && !upfront {
			if method != nil {
				pa = &due
			} else {
				pa = &paidAt
			}
		}
		parcels = []parcel{{Amount: netAmount, Installment: 1, DueDate: due, PaidAt: pa}}
	} else {
		// Monthly mode: parcel 1 = saleDate + settlement_days, then +1 month
		// per parcel. Calendar-aligned, see expandPayments comment.
		baseDue := pkg.SaleDate.AddDate(0, 0, method.SettlementDays)
		share := round2(netAmount / float64(installments))
		accumulated := 0.0
		for i := 1; i <= installments; i++ {
			amt := share
			if i == installments {
				amt = round2(netAmount - accumulated)
			}
			accumulated = round2(accumulated + amt)
			parcels = append(parcels, parcel{
				Amount:      amt,
				Installment: i,
				DueDate:     baseDue.AddDate(0, i-1, 0),
				PaidAt:      nil,
			})
		}
	}

	totalInstallments := len(parcels)

	// Repurpose the existing row as parcel 1, then append rows 2..N.
	payment.PaymentMethodID = methodID
	payment.Amount = parcels[0].Amount
	payment.Installment = parcels[0].Installment
	payment.TotalInstallments = totalInstallments
	payment.DueDate = parcels[0].DueDate
	payment.PaidAt = parcels[0].PaidAt

	var extraRows []model.SalesPackagePayment
	for i := 1; i < len(parcels); i++ {
		extraRows = append(extraRows, model.SalesPackagePayment{
			ClinicID:          clinicID,
			SalesPackageID:    pkg.ID,
			PaymentMethodID:   methodID,
			Amount:            parcels[i].Amount,
			Discount:          0,
			Installment:       parcels[i].Installment,
			TotalInstallments: totalInstallments,
			DueDate:           parcels[i].DueDate,
			PaidAt:            parcels[i].PaidAt,
		})
	}

	err = s.repo.WithTx(ctx, func(txRepo repository.SalesPackageRepository) error {
		if err := txRepo.UpdatePayment(ctx, payment); err != nil {
			return err
		}
		if len(extraRows) > 0 {
			return txRepo.CreatePayments(ctx, extraRows)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	patientName := s.patientName(ctx, clinicID, pkg.PatientID)
	if err := s.finance.UpsertFromSalesPackagePayment(ctx, payment, pkg, patientName); err != nil {
		logger.Printf("[SalesPackage] finance upsert failed on register for payment %s: %v", payment.ID, err)
		s.systemLog(ctx, &clinicID, "finance_upsert_failed", systemlogModel.LogStatusFailed, map[string]interface{}{"payment_id": payment.ID}, err.Error())
	}
	for i := range extraRows {
		if err := s.finance.UpsertFromSalesPackagePayment(ctx, &extraRows[i], pkg, patientName); err != nil {
			logger.Printf("[SalesPackage] finance upsert failed on register parcel %s: %v", extraRows[i].ID, err)
			s.systemLog(ctx, &clinicID, "finance_upsert_failed", systemlogModel.LogStatusFailed, map[string]interface{}{"payment_id": extraRows[i].ID}, err.Error())
		}
	}

	s.auditLog(ctx, clinicID, userID, auditModel.AuditActionUpdate, payment.ID.String(), &old, payment)
	s.systemLog(ctx, &clinicID, "payment_registered", systemlogModel.LogStatusSuccess, map[string]interface{}{"payment_id": payment.ID, "amount": payment.Amount, "installments": totalInstallments}, "")

	return payment.ToResponse(), nil
}

// --- Add payment to active sale ------------------------------------------

// AddPayment records a brand-new payment line on an active sale (e.g. the
// customer paid R$200 cash on top of an existing R$1000 receivable). The
// new amount is consumed from the sale's open receivables in oldest-first
// order — fully consumed receivables are deleted, partials are shrunk.
func (s *salesPackageService) AddPayment(ctx context.Context, salesPackageID, clinicID, userID uuid.UUID, input *model.CreateSalesPackagePaymentInput) (*model.SalesPackageResponse, error) {
	if input == nil {
		return nil, errors.ErrValidationInstance.WithMessage("Pagamento inválido.")
	}
	if input.Amount <= 0 {
		return nil, errors.ErrValidationInstance.WithMessage("Valor do pagamento deve ser maior que zero.")
	}
	if input.Discount < 0 {
		return nil, errors.ErrValidationInstance.WithMessage("Desconto não pode ser negativo.")
	}

	pkg, err := s.repo.GetByID(ctx, salesPackageID, clinicID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Venda não encontrada.")
		}
		return nil, err
	}
	if pkg.Status != model.SalesPackageStatusActive {
		return nil, errors.ErrValidationInstance.WithMessage("Apenas vendas ativas aceitam novos pagamentos.")
	}

	method, err := s.paymentMethod.GetByID(ctx, input.PaymentMethodID, clinicID)
	if err != nil {
		return nil, errors.ErrValidationInstance.WithMessage("Forma de pagamento inválida.")
	}
	if !method.Active {
		return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' está inativa.", method.Name))
	}

	installments := 1
	if input.Installments > 0 {
		installments = input.Installments
	}
	if installments > 1 {
		if !method.InstallmentsAllowed {
			return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' não aceita parcelamento.", method.Name))
		}
		if installments > method.MaxInstallments {
			return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' aceita no máximo %d parcelas.", method.Name, method.MaxInstallments))
		}
	}

	netAmount := round2(input.Amount - input.Discount)
	if netAmount <= 0 {
		return nil, errors.ErrValidationInstance.WithMessage("Valor efetivo deve ser maior que zero (verifique desconto).")
	}

	// Find open receivables (method-less, not paid).
	allPayments, err := s.repo.ListPaymentsByPackage(ctx, salesPackageID, clinicID)
	if err != nil {
		return nil, err
	}
	var openRows []model.SalesPackagePayment
	var totalOpen float64
	for _, p := range allPayments {
		if p.PaymentMethodID == nil && p.PaidAt == nil {
			openRows = append(openRows, p)
			totalOpen = round2(totalOpen + p.Amount)
		}
	}
	if netAmount-totalOpen > 0.005 {
		return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Valor (R$ %.2f) maior que o saldo em aberto (R$ %.2f).", netAmount, totalOpen))
	}

	// Build new parcel rows for the input (same shape expandPayments uses).
	var saleDate = pkg.SaleDate
	baseDue := saleDate.AddDate(0, 0, method.SettlementDays)
	if input.DueDate != nil {
		baseDue = input.DueDate.UTC().Truncate(24 * time.Hour)
	}

	upfront := method.SettlementMode == paymentmethodModel.SettlementModeUpfront
	var newRows []model.SalesPackagePayment
	if upfront || installments == 1 {
		// "Pago em" preenchido = marca como pago. paid_at sempre = sale_date
		// + settlement_days (data real em que o dinheiro entra no banco).
		var pa *time.Time
		if input.PaidAt != nil && installments == 1 && !upfront {
			pa = &baseDue
		}
		methodID := input.PaymentMethodID
		newRows = append(newRows, model.SalesPackagePayment{
			ClinicID:          clinicID,
			SalesPackageID:    salesPackageID,
			PaymentMethodID:   &methodID,
			Amount:            netAmount,
			Discount:          0,
			Installment:       1,
			TotalInstallments: 1,
			DueDate:           baseDue,
			PaidAt:            pa,
		})
	} else {
		// Monthly mode: parcel 1 = saleDate + settlement_days, then +1 month
		// per parcel. Calendar-aligned, see expandPayments comment.
		share := round2(netAmount / float64(installments))
		accumulated := 0.0
		methodID := input.PaymentMethodID
		for i := 1; i <= installments; i++ {
			amt := share
			if i == installments {
				amt = round2(netAmount - accumulated)
			}
			accumulated = round2(accumulated + amt)
			due := baseDue.AddDate(0, i-1, 0)
			newRows = append(newRows, model.SalesPackagePayment{
				ClinicID:          clinicID,
				SalesPackageID:    salesPackageID,
				PaymentMethodID:   &methodID,
				Amount:            amt,
				Discount:          0,
				Installment:       i,
				TotalInstallments: installments,
				DueDate:           due,
				PaidAt:            nil,
			})
		}
	}

	// Reduce open receivables by netAmount, oldest first.
	remaining := netAmount
	var rowsToUpdate []model.SalesPackagePayment
	var rowsToDelete []uuid.UUID
	for i := range openRows {
		if remaining <= 0.005 {
			break
		}
		row := openRows[i]
		if row.Amount-remaining <= 0.005 {
			rowsToDelete = append(rowsToDelete, row.ID)
			remaining = round2(remaining - row.Amount)
		} else {
			row.Amount = round2(row.Amount - remaining)
			rowsToUpdate = append(rowsToUpdate, row)
			remaining = 0
		}
	}

	err = s.repo.WithTx(ctx, func(txRepo repository.SalesPackageRepository) error {
		if len(newRows) > 0 {
			if err := txRepo.CreatePayments(ctx, newRows); err != nil {
				return err
			}
		}
		for i := range rowsToUpdate {
			if err := txRepo.UpdatePayment(ctx, &rowsToUpdate[i]); err != nil {
				return err
			}
		}
		for _, id := range rowsToDelete {
			if err := txRepo.DeletePayment(ctx, id, clinicID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Reflect to finance.
	patientName := s.patientName(ctx, clinicID, pkg.PatientID)
	for i := range newRows {
		if err := s.finance.UpsertFromSalesPackagePayment(ctx, &newRows[i], pkg, patientName); err != nil {
			logger.Printf("[SalesPackage] finance upsert failed on AddPayment new row %s: %v", newRows[i].ID, err)
			s.systemLog(ctx, &clinicID, "finance_upsert_failed", systemlogModel.LogStatusFailed, map[string]interface{}{"payment_id": newRows[i].ID}, err.Error())
		}
	}
	for i := range rowsToUpdate {
		if err := s.finance.UpsertFromSalesPackagePayment(ctx, &rowsToUpdate[i], pkg, patientName); err != nil {
			logger.Printf("[SalesPackage] finance upsert failed on AddPayment updated row %s: %v", rowsToUpdate[i].ID, err)
		}
	}
	for _, id := range rowsToDelete {
		if err := s.finance.DeleteFromSalesPackagePayment(ctx, id, clinicID); err != nil {
			logger.Printf("[SalesPackage] finance delete failed on AddPayment consumed row %s: %v", id, err)
		}
	}

	s.auditLog(ctx, clinicID, userID, auditModel.AuditActionUpdate, pkg.ID.String(), nil, map[string]interface{}{"added_payment_amount": netAmount, "installments": installments})
	s.systemLog(ctx, &clinicID, "payment_added", systemlogModel.LogStatusSuccess, map[string]interface{}{"sales_package_id": pkg.ID, "amount": netAmount, "installments": installments}, "")

	return s.fetchResponse(ctx, pkg.ID, clinicID)
}

// --- Update / Delete payment row -----------------------------------------

// UpdatePaymentRow edits one payment row in place: change method, change
// amount (with rebalance against open receivables), or toggle paid/unpaid.
// Keeps sum(payments) == pkg.total true and mirrors every change to finance.
func (s *salesPackageService) UpdatePaymentRow(ctx context.Context, paymentID, clinicID, userID uuid.UUID, req *model.UpdatePaymentRowRequest) (*model.SalesPackagePaymentResponse, error) {
	if req == nil {
		return nil, errors.ErrValidationInstance.WithMessage("Requisição inválida.")
	}
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
	if pkg.Status == model.SalesPackageStatusCanceled {
		return nil, errors.ErrValidationInstance.WithMessage("Não é possível editar pagamentos de uma venda cancelada.")
	}

	old := *payment

	// Resolve method (override or current).
	methodID := payment.PaymentMethodID
	if req.PaymentMethodID != nil {
		id := *req.PaymentMethodID
		methodID = &id
	}
	var method *paymentmethodModel.PaymentMethod
	if methodID != nil {
		method, err = s.paymentMethod.GetByID(ctx, *methodID, clinicID)
		if err != nil {
			return nil, errors.ErrValidationInstance.WithMessage("Forma de pagamento inválida.")
		}
		if !method.Active {
			return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Forma de pagamento '%s' está inativa.", method.Name))
		}
	}

	// Resolve new amount.
	newAmount := payment.Amount
	if req.Amount != nil {
		newAmount = round2(*req.Amount)
	}
	delta := round2(newAmount - payment.Amount)

	// Rebalance open receivables to keep sum(payments) == pkg.total.
	var rowsToUpdate []model.SalesPackagePayment
	var rowsToDelete []uuid.UUID
	var newReceivable *model.SalesPackagePayment
	if delta > 0.005 {
		// Need to consume delta from existing open receivables.
		all, err := s.repo.ListPaymentsByPackage(ctx, pkg.ID, clinicID)
		if err != nil {
			return nil, err
		}
		var totalOpen float64
		for _, p := range all {
			if p.ID == payment.ID {
				continue
			}
			if p.PaymentMethodID == nil && p.PaidAt == nil {
				totalOpen = round2(totalOpen + p.Amount)
			}
		}
		if delta-totalOpen > 0.005 {
			return nil, errors.ErrValidationInstance.WithMessage(fmt.Sprintf("Aumento de R$ %.2f maior que o saldo em aberto disponível (R$ %.2f).", delta, totalOpen))
		}
		remaining := delta
		for _, p := range all {
			if remaining <= 0.005 {
				break
			}
			if p.ID == payment.ID {
				continue
			}
			if p.PaymentMethodID != nil || p.PaidAt != nil {
				continue
			}
			if p.Amount-remaining <= 0.005 {
				rowsToDelete = append(rowsToDelete, p.ID)
				remaining = round2(remaining - p.Amount)
			} else {
				p.Amount = round2(p.Amount - remaining)
				rowsToUpdate = append(rowsToUpdate, p)
				remaining = 0
			}
		}
	} else if delta < -0.005 {
		// Free up |delta| as an open receivable — merge into existing or create.
		all, err := s.repo.ListPaymentsByPackage(ctx, pkg.ID, clinicID)
		if err != nil {
			return nil, err
		}
		merged := false
		for _, p := range all {
			if p.ID == payment.ID {
				continue
			}
			if p.PaymentMethodID == nil && p.PaidAt == nil {
				p.Amount = round2(p.Amount + (-delta))
				rowsToUpdate = append(rowsToUpdate, p)
				merged = true
				break
			}
		}
		if !merged {
			newReceivable = &model.SalesPackagePayment{
				ClinicID:          clinicID,
				SalesPackageID:    pkg.ID,
				PaymentMethodID:   nil,
				Amount:            round2(-delta),
				Discount:          0,
				Installment:       1,
				TotalInstallments: 1,
				DueDate:           pkg.SaleDate,
				PaidAt:            nil,
			}
		}
	}

	// Apply changes to the row itself.
	payment.PaymentMethodID = methodID
	payment.Amount = newAmount
	if req.Paid != nil {
		if *req.Paid {
			if methodID == nil {
				return nil, errors.ErrValidationInstance.WithMessage("Para marcar como pago, selecione uma forma de pagamento.")
			}
			paidAt := pkg.SaleDate.AddDate(0, 0, method.SettlementDays)
			payment.PaidAt = &paidAt
		} else {
			payment.PaidAt = nil
		}
	}

	// Persist atomically.
	err = s.repo.WithTx(ctx, func(txRepo repository.SalesPackageRepository) error {
		if err := txRepo.UpdatePayment(ctx, payment); err != nil {
			return err
		}
		for i := range rowsToUpdate {
			if err := txRepo.UpdatePayment(ctx, &rowsToUpdate[i]); err != nil {
				return err
			}
		}
		for _, id := range rowsToDelete {
			if err := txRepo.DeletePayment(ctx, id, clinicID); err != nil {
				return err
			}
		}
		if newReceivable != nil {
			return txRepo.CreatePayments(ctx, []model.SalesPackagePayment{*newReceivable})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Mirror everything to finance.
	patientName := s.patientName(ctx, clinicID, pkg.PatientID)
	if err := s.finance.UpsertFromSalesPackagePayment(ctx, payment, pkg, patientName); err != nil {
		logger.Printf("[SalesPackage] finance upsert failed on UpdatePaymentRow %s: %v", payment.ID, err)
	}
	for i := range rowsToUpdate {
		if err := s.finance.UpsertFromSalesPackagePayment(ctx, &rowsToUpdate[i], pkg, patientName); err != nil {
			logger.Printf("[SalesPackage] finance upsert failed on UpdatePaymentRow rebalance %s: %v", rowsToUpdate[i].ID, err)
		}
	}
	for _, id := range rowsToDelete {
		if err := s.finance.DeleteFromSalesPackagePayment(ctx, id, clinicID); err != nil {
			logger.Printf("[SalesPackage] finance delete failed on UpdatePaymentRow %s: %v", id, err)
		}
	}
	if newReceivable != nil {
		if err := s.finance.UpsertFromSalesPackagePayment(ctx, newReceivable, pkg, patientName); err != nil {
			logger.Printf("[SalesPackage] finance upsert failed on UpdatePaymentRow new receivable %s: %v", newReceivable.ID, err)
		}
	}

	s.auditLog(ctx, clinicID, userID, auditModel.AuditActionUpdate, payment.ID.String(), &old, payment)
	s.systemLog(ctx, &clinicID, "payment_row_updated", systemlogModel.LogStatusSuccess, map[string]interface{}{"payment_id": payment.ID}, "")
	return payment.ToResponse(), nil
}

// DeletePaymentRow removes a single payment row from an active sale and
// restores its amount as an open receivable. Finance ledger drops the
// deleted row's transaction and updates the receivable's transaction.
func (s *salesPackageService) DeletePaymentRow(ctx context.Context, paymentID, clinicID, userID uuid.UUID) error {
	payment, err := s.repo.GetPaymentByID(ctx, paymentID, clinicID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrNotFoundInstance.WithMessage("Pagamento não encontrado.")
		}
		return err
	}
	pkg, err := s.repo.GetByID(ctx, payment.SalesPackageID, clinicID)
	if err != nil {
		return err
	}
	if pkg.Status == model.SalesPackageStatusCanceled {
		return errors.ErrValidationInstance.WithMessage("Não é possível excluir pagamentos de uma venda cancelada.")
	}

	deletedAmount := payment.Amount

	// Either merge restored amount into an existing open receivable, or create
	// a fresh method-less row carrying it.
	all, err := s.repo.ListPaymentsByPackage(ctx, pkg.ID, clinicID)
	if err != nil {
		return err
	}
	var rowsToUpdate []model.SalesPackagePayment
	var newReceivable *model.SalesPackagePayment
	merged := false
	for _, p := range all {
		if p.ID == payment.ID {
			continue
		}
		if p.PaymentMethodID == nil && p.PaidAt == nil {
			p.Amount = round2(p.Amount + deletedAmount)
			rowsToUpdate = append(rowsToUpdate, p)
			merged = true
			break
		}
	}
	if !merged {
		newReceivable = &model.SalesPackagePayment{
			ClinicID:          clinicID,
			SalesPackageID:    pkg.ID,
			PaymentMethodID:   nil,
			Amount:            deletedAmount,
			Discount:          0,
			Installment:       1,
			TotalInstallments: 1,
			DueDate:           pkg.SaleDate,
			PaidAt:            nil,
		}
	}

	err = s.repo.WithTx(ctx, func(txRepo repository.SalesPackageRepository) error {
		if err := txRepo.DeletePayment(ctx, payment.ID, clinicID); err != nil {
			return err
		}
		for i := range rowsToUpdate {
			if err := txRepo.UpdatePayment(ctx, &rowsToUpdate[i]); err != nil {
				return err
			}
		}
		if newReceivable != nil {
			return txRepo.CreatePayments(ctx, []model.SalesPackagePayment{*newReceivable})
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Reflect to finance: drop the deleted tx, upsert the surviving receivable.
	if err := s.finance.DeleteFromSalesPackagePayment(ctx, payment.ID, clinicID); err != nil {
		logger.Printf("[SalesPackage] finance delete failed on DeletePaymentRow %s: %v", payment.ID, err)
	}
	patientName := s.patientName(ctx, clinicID, pkg.PatientID)
	for i := range rowsToUpdate {
		if err := s.finance.UpsertFromSalesPackagePayment(ctx, &rowsToUpdate[i], pkg, patientName); err != nil {
			logger.Printf("[SalesPackage] finance upsert failed on DeletePaymentRow %s: %v", rowsToUpdate[i].ID, err)
		}
	}
	if newReceivable != nil {
		if err := s.finance.UpsertFromSalesPackagePayment(ctx, newReceivable, pkg, patientName); err != nil {
			logger.Printf("[SalesPackage] finance upsert failed on DeletePaymentRow new receivable %s: %v", newReceivable.ID, err)
		}
	}

	s.auditLog(ctx, clinicID, userID, auditModel.AuditActionDelete, payment.ID.String(), payment, nil)
	s.systemLog(ctx, &clinicID, "payment_row_deleted", systemlogModel.LogStatusSuccess, map[string]interface{}{"payment_id": payment.ID}, "")
	return nil
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
