package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	clinicmodel "github.com/aseno/backend/pkg/clinic/model"
	paymentmethodmodel "github.com/aseno/backend/pkg/paymentmethod/model"
	"github.com/aseno/backend/pkg/salespackage/model"
	"github.com/aseno/backend/pkg/salespackage/repository"
)

// Narrow collaborators — keep dependency surface small for testability.

type receiptClinicReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*clinicmodel.Clinic, error)
}

type receiptPaymentMethodReader interface {
	GetByID(id, clinicID uuid.UUID) (*paymentmethodmodel.PaymentMethod, error)
}

type receiptTokenStore interface {
	GetOrCreateBySalesPackage(ctx context.Context, clinicID, salesPackageID uuid.UUID) (*model.SalesPackageReceiptToken, error)
	GetByToken(ctx context.Context, token uuid.UUID) (*model.SalesPackageReceiptToken, error)
}

// SalesPackageReceiptService manages the opaque share token + builds the
// payload rendered on the public /recibo/venda page.
type SalesPackageReceiptService interface {
	EnsureToken(ctx context.Context, clinicID, salesPackageID uuid.UUID) (*model.EnsureReceiptTokenResponse, error)
	GetPublicReceipt(ctx context.Context, token uuid.UUID) (*model.PublicSalesReceiptResponse, error)
}

type salesPackageReceiptService struct {
	tokenRepo     receiptTokenStore
	salesRepo     repository.SalesPackageRepository
	clinicRepo    receiptClinicReader
	pmRepo        receiptPaymentMethodReader
	publicBaseURL string
}

func NewSalesPackageReceiptService(
	tokenRepo receiptTokenStore,
	salesRepo repository.SalesPackageRepository,
	clinicRepo receiptClinicReader,
	pmRepo receiptPaymentMethodReader,
	publicBaseURL string,
) SalesPackageReceiptService {
	return &salesPackageReceiptService{
		tokenRepo:     tokenRepo,
		salesRepo:     salesRepo,
		clinicRepo:    clinicRepo,
		pmRepo:        pmRepo,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

func (s *salesPackageReceiptService) EnsureToken(ctx context.Context, clinicID, salesPackageID uuid.UUID) (*model.EnsureReceiptTokenResponse, error) {
	// Confirm the sale exists and belongs to the clinic.
	if _, err := s.salesRepo.GetByID(ctx, salesPackageID, clinicID); err != nil {
		return nil, err
	}

	row, err := s.tokenRepo.GetOrCreateBySalesPackage(ctx, clinicID, salesPackageID)
	if err != nil {
		return nil, err
	}

	return &model.EnsureReceiptTokenResponse{
		Token:     row.Token,
		PublicURL: s.buildPublicURL(row.Token),
	}, nil
}

func (s *salesPackageReceiptService) GetPublicReceipt(ctx context.Context, token uuid.UUID) (*model.PublicSalesReceiptResponse, error) {
	row, err := s.tokenRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	pkg, err := s.salesRepo.GetByID(ctx, row.SalesPackageID, row.ClinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance
	}

	clinic, err := s.clinicRepo.GetByID(ctx, row.ClinicID)
	if err != nil {
		return nil, err
	}

	items := make([]model.PublicReceiptItem, 0, len(pkg.Items))
	var subtotal float64
	for _, it := range pkg.Items {
		name := "Procedimento"
		if it.Procedure != nil && it.Procedure.Name != "" {
			name = it.Procedure.Name
		}
		items = append(items, model.PublicReceiptItem{
			Name:      name,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
			LineTotal: it.LineTotal,
		})
		subtotal += it.LineTotal
	}

	payments := make([]model.PublicReceiptPayment, 0, len(pkg.Payments))
	var totalPaid float64
	for _, p := range pkg.Payments {
		methodName := ""
		if p.PaymentMethodID != nil && s.pmRepo != nil {
			if m, err := s.pmRepo.GetByID(*p.PaymentMethodID, p.ClinicID); err == nil && m != nil {
				methodName = m.Name
			}
		}
		payments = append(payments, model.PublicReceiptPayment{
			Method:   methodName,
			Amount:   p.Amount,
			Discount: p.Discount,
			PaidAt:   p.PaidAt,
			DueDate:  p.DueDate,
		})
		if p.PaidAt != nil {
			totalPaid += p.Amount - p.Discount
		}
	}

	balance := pkg.Total - totalPaid
	if balance < 0 {
		balance = 0
	}

	patientName := ""
	if pkg.Patient != nil {
		patientName = pkg.Patient.Name
	}

	resp := &model.PublicSalesReceiptResponse{
		ReceiptNumber: buildSalesReceiptNumber(pkg.Code, pkg.SaleDate),
		Code:          pkg.Code,
		Status:        pkg.Status,
		Clinic: model.PublicReceiptClinic{
			Name:     clinic.Name,
			Document: clinic.Document,
			Address:  clinic.Address,
			Phone:    clinic.Phone,
		},
		PatientName: patientName,
		SaleDate:    pkg.SaleDate,
		ValidUntil:  pkg.ValidUntil,
		Items:       items,
		Subtotal:    subtotal,
		Discount:    pkg.Discount,
		Total:       pkg.Total,
		Payments:    payments,
		TotalPaid:   totalPaid,
		BalanceDue:  balance,
		GeneratedAt: time.Now(),
	}
	return resp, nil
}

func (s *salesPackageReceiptService) buildPublicURL(token uuid.UUID) string {
	return fmt.Sprintf("%s/recibo/venda/%s", s.publicBaseURL, token.String())
}

// buildSalesReceiptNumber wraps the sale code with an "R-" prefix and the
// sale date for human-friendly display. Deterministic so reloads produce
// the same number.
func buildSalesReceiptNumber(code string, saleDate time.Time) string {
	return fmt.Sprintf("R-%s-%s", saleDate.Format("20060102"), strings.ToUpper(code))
}
