package service

import (
	"context"

	"github.com/google/uuid"

	auditModel "github.com/aseno/backend/pkg/audit/model"
	auditService "github.com/aseno/backend/pkg/audit/service"
	financeService "github.com/aseno/backend/pkg/finance/service"
	patientModel "github.com/aseno/backend/pkg/patient/model"
	paymentmethodModel "github.com/aseno/backend/pkg/paymentmethod/model"
	"github.com/aseno/backend/pkg/salespackage/model"
	systemlogModel "github.com/aseno/backend/pkg/systemlog/model"
	systemlogService "github.com/aseno/backend/pkg/systemlog/service"
)

// --- PaymentMethodLookup --------------------------------------------------

// paymentMethodRepoLike covers the (id, clinicID) GetByID signature exposed by
// paymentmethod.repository — the repo doesn't currently take a context, so the
// adapter discards ours when calling through.
type paymentMethodRepoLike interface {
	GetByID(id, clinicID uuid.UUID) (*paymentmethodModel.PaymentMethod, error)
}

type paymentMethodAdapter struct {
	repo paymentMethodRepoLike
}

// NewPaymentMethodLookup wraps a payment method repository so the salespackage
// service can read settlement parameters (settlement_days, installments...).
func NewPaymentMethodLookup(repo paymentMethodRepoLike) PaymentMethodLookup {
	return &paymentMethodAdapter{repo: repo}
}

func (a *paymentMethodAdapter) GetByID(_ context.Context, id, clinicID uuid.UUID) (*paymentmethodModel.PaymentMethod, error) {
	return a.repo.GetByID(id, clinicID)
}

// --- FinanceHook ----------------------------------------------------------

type financeAdapter struct {
	svc financeService.FinanceService
}

// NewFinanceHook adapts the finance service so the salespackage service can
// reflect each SalesPackagePayment into a FinancialTransaction.
func NewFinanceHook(svc financeService.FinanceService) FinanceHook {
	return &financeAdapter{svc: svc}
}

func (a *financeAdapter) UpsertFromSalesPackagePayment(ctx context.Context, payment *model.SalesPackagePayment, pkg *model.SalesPackage, patientName string) error {
	return a.svc.UpsertFromSalesPackagePayment(ctx, financeService.UpsertSalesPackagePaymentArgs{
		ClinicID:              payment.ClinicID,
		SalesPackageID:        payment.SalesPackageID,
		SalesPackagePaymentID: payment.ID,
		PaymentMethodID:       payment.PaymentMethodID, // already nullable
		Amount:                payment.Amount,
		Discount:              payment.Discount,
		DueDate:               payment.DueDate,
		PaidAt:                payment.PaidAt,
		Installment:           payment.Installment,
		TotalInstallments:     payment.TotalInstallments,
		PackageCode:           pkg.Code,
		PatientName:           patientName,
		IsCanceled:            pkg.Status == model.SalesPackageStatusCanceled,
	})
}

func (a *financeAdapter) DeleteFromSalesPackagePayment(ctx context.Context, salesPackagePaymentID, clinicID uuid.UUID) error {
	return a.svc.DeleteFromSalesPackagePayment(ctx, salesPackagePaymentID, clinicID)
}

// --- PatientLookup --------------------------------------------------------

type patientRepoLike interface {
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*patientModel.Patient, error)
}

type patientAdapter struct {
	repo patientRepoLike
}

// NewPatientLookup wraps the patient repository so the service can fetch the
// patient name for descriptions/search results.
func NewPatientLookup(repo patientRepoLike) PatientLookup {
	return &patientAdapter{repo: repo}
}

func (a *patientAdapter) GetName(ctx context.Context, patientID, clinicID uuid.UUID) (string, error) {
	p, err := a.repo.GetByID(ctx, patientID, clinicID)
	if err != nil {
		return "", err
	}
	return p.Name, nil
}

// --- AuditLogger ----------------------------------------------------------

type auditAdapter struct {
	svc auditService.AuditService
}

// NewAuditLogger wraps the audit service. The salespackage service hands off
// CRUD events through this lighter interface so it doesn't take a hard
// dependency on the gin context.
func NewAuditLogger(svc auditService.AuditService) AuditLogger {
	return &auditAdapter{svc: svc}
}

func (a *auditAdapter) Log(ctx context.Context, clinicID, userID uuid.UUID, action auditModel.AuditAction, entityID string, oldData, newData interface{}) {
	_ = a.svc.Log(ctx, auditService.AuditEntry{
		ClinicID:   clinicID,
		UserID:     userID,
		Action:     action,
		EntityType: auditModel.AuditEntitySalesPackage,
		EntityID:   entityID,
		OldData:    oldData,
		NewData:    newData,
	})
}

// --- SystemLogger ---------------------------------------------------------

type syslogAdapter struct {
	svc systemlogService.SystemLogService
}

// NewSystemLogger wraps the system log service.
func NewSystemLogger(svc systemlogService.SystemLogService) SystemLogger {
	return &syslogAdapter{svc: svc}
}

func (a *syslogAdapter) Log(ctx context.Context, clinicID *uuid.UUID, event string, status systemlogModel.LogStatus, payload interface{}, errStr string) {
	_ = a.svc.Log(ctx, systemlogService.SystemLogEntry{
		ClinicID: clinicID,
		Service:  "salespackage",
		Event:    event,
		Status:   status,
		Payload:  payload,
		Error:    errStr,
	})
}
