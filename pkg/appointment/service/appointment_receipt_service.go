package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/appointment/model"
	clinicmodel "github.com/aseno/backend/pkg/clinic/model"
)

// Narrow interfaces — consumers (main.go, tests) may pass either the full
// repository or a focused mock. This keeps the service easy to test without
// recreating the entire AppointmentRepository surface.

type receiptAppointmentReader interface {
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Appointment, error)
}

type receiptPaymentLister interface {
	ListByAppointment(appointmentID, clinicID uuid.UUID) ([]*model.AppointmentPayment, error)
}

type receiptClinicReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*clinicmodel.Clinic, error)
}

type receiptTokenStore interface {
	GetOrCreateByAppointment(ctx context.Context, clinicID, appointmentID uuid.UUID) (*model.AppointmentReceiptToken, error)
	GetByToken(ctx context.Context, token uuid.UUID) (*model.AppointmentReceiptToken, error)
}

// AppointmentReceiptService exposes the "share public receipt" domain. It
// centralizes token creation/lookup and the live aggregation that powers
// the public /recibo page.
type AppointmentReceiptService interface {
	// EnsureToken returns (creates if missing) the share token for a clinic's
	// appointment. Idempotent.
	EnsureToken(ctx context.Context, clinicID, appointmentID uuid.UUID) (*model.EnsureTokenResponse, error)
	// GetPublicReceipt resolves the token to a fresh, live-view receipt. It
	// must not leak sensitive data (CPF, phone, notes, internal IDs).
	GetPublicReceipt(ctx context.Context, token uuid.UUID) (*model.PublicReceiptResponse, error)
	// CheckTokenOwnership returns true iff the token belongs to the given
	// clinic. Used by the /recibo page to decide between the 2-via clinic
	// layout and the 1-via patient layout: a third-party clinic that happens
	// to be logged in must not see the "via da clínica" signature slot.
	CheckTokenOwnership(ctx context.Context, clinicID, token uuid.UUID) (bool, error)
}

type appointmentReceiptService struct {
	tokenRepo       receiptTokenStore
	appointmentRepo receiptAppointmentReader
	paymentRepo     receiptPaymentLister
	clinicRepo      receiptClinicReader
	publicBaseURL   string
}

// NewAppointmentReceiptService wires the receipt service. publicBaseURL is the
// frontend origin (e.g. "https://app.aseno.com.br") used to build share links.
func NewAppointmentReceiptService(
	tokenRepo receiptTokenStore,
	appointmentRepo receiptAppointmentReader,
	paymentRepo receiptPaymentLister,
	clinicRepo receiptClinicReader,
	publicBaseURL string,
) AppointmentReceiptService {
	return &appointmentReceiptService{
		tokenRepo:       tokenRepo,
		appointmentRepo: appointmentRepo,
		paymentRepo:     paymentRepo,
		clinicRepo:      clinicRepo,
		publicBaseURL:   strings.TrimRight(publicBaseURL, "/"),
	}
}

func (s *appointmentReceiptService) EnsureToken(ctx context.Context, clinicID, appointmentID uuid.UUID) (*model.EnsureTokenResponse, error) {
	// Confirm the appointment exists and belongs to the clinic. GetByID
	// already filters deleted_at IS NULL, so soft-deleted appointments can't
	// mint new tokens.
	if _, err := s.appointmentRepo.GetByID(ctx, appointmentID, clinicID); err != nil {
		return nil, err
	}

	row, err := s.tokenRepo.GetOrCreateByAppointment(ctx, clinicID, appointmentID)
	if err != nil {
		return nil, err
	}

	return &model.EnsureTokenResponse{
		Token:     row.Token,
		PublicURL: s.buildPublicURL(row.Token),
	}, nil
}

func (s *appointmentReceiptService) GetPublicReceipt(ctx context.Context, token uuid.UUID) (*model.PublicReceiptResponse, error) {
	row, err := s.tokenRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// The appointment repo already filters soft-deleted rows. If the
	// appointment was (soft-)deleted after the token was created, this
	// returns not-found and the public page renders a 404 — which is what
	// the user asked for.
	apt, err := s.appointmentRepo.GetByID(ctx, row.AppointmentID, row.ClinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance
	}

	clinic, err := s.clinicRepo.GetByID(ctx, row.ClinicID)
	if err != nil {
		return nil, err
	}

	payments, err := s.paymentRepo.ListByAppointment(apt.ID, apt.ClinicID)
	if err != nil {
		return nil, err
	}

	// Build procedure lines. Historical prices live on the AppointmentProcedure
	// row; the procedure name falls back to the deleted procedure (loaded via
	// the Unscoped preload in GetByID).
	// Initialized as empty (not nil) so the DTO marshals to `[]` instead of
	// `null` — the frontend reads `.length` directly.
	procedures := make([]model.PublicReceiptProcedure, 0, len(apt.Procedures))
	var subtotal float64
	for _, ap := range apt.Procedures {
		name := ""
		if ap.Procedure != nil {
			name = ap.Procedure.Name
		}
		if name == "" {
			name = "Procedimento"
		}
		procedures = append(procedures, model.PublicReceiptProcedure{
			Name:  name,
			Price: ap.Price,
		})
		subtotal += ap.Price
	}

	// payment lines + totals. See procedures above — empty slice, never nil.
	paymentLines := make([]model.PublicReceiptPayment, 0, len(payments))
	var totalPaid, totalDiscount float64
	for _, p := range payments {
		methodName := ""
		if p.PaymentMethod != nil {
			methodName = p.PaymentMethod.Name
		}
		paymentLines = append(paymentLines, model.PublicReceiptPayment{
			Method:   methodName,
			Amount:   p.Amount,
			Discount: p.Discount,
			PaidAt:   p.PaidAt,
		})
		totalPaid += p.Amount
		totalDiscount += p.Discount
	}

	// Total follows the appointment Price when it differs from the
	// procedures subtotal (manual overrides). Otherwise use the subtotal.
	total := apt.Price
	if total == 0 {
		total = subtotal
	}
	balance := total - totalDiscount - totalPaid
	if balance < 0 {
		balance = 0
	}

	patientName := ""
	if apt.Patient != nil {
		patientName = apt.Patient.Name
	}
	professionalName := ""
	if apt.Professional != nil {
		professionalName = apt.Professional.Name
	}

	resp := &model.PublicReceiptResponse{
		ReceiptNumber: buildReceiptNumber(row.Token, apt.StartTime),
		Clinic: model.PublicReceiptClinic{
			Name:     clinic.Name,
			Document: clinic.Document,
			Address:  clinic.Address,
			Phone:    clinic.Phone,
		},
		PatientName:      patientName,
		ProfessionalName: professionalName,
		AppointmentDate:  apt.StartTime,
		Procedures:       procedures,
		Subtotal:         subtotal,
		Discount:         totalDiscount,
		Total:            total,
		Payments:         paymentLines,
		TotalPaid:        totalPaid,
		BalanceDue:       balance,
		GeneratedAt:      time.Now(),
	}
	return resp, nil
}

func (s *appointmentReceiptService) CheckTokenOwnership(ctx context.Context, clinicID, token uuid.UUID) (bool, error) {
	row, err := s.tokenRepo.GetByToken(ctx, token)
	if err != nil {
		return false, err
	}
	return row.ClinicID == clinicID, nil
}

func (s *appointmentReceiptService) buildPublicURL(token uuid.UUID) string {
	base := s.publicBaseURL
	if base == "" {
		base = ""
	}
	return fmt.Sprintf("%s/recibo/%s", base, token.String())
}

// buildReceiptNumber formats a deterministic, human-friendly identifier for
// the receipt. It uses the appointment date and the first 6 hex chars of the
// token so two clicks on the same link always render the same number, and no
// additional counter state is required.
func buildReceiptNumber(token uuid.UUID, start time.Time) string {
	raw := strings.ReplaceAll(token.String(), "-", "")
	suffix := raw
	if len(suffix) > 6 {
		suffix = suffix[:6]
	}
	return fmt.Sprintf("R-%s-%s", start.Format("20060102"), strings.ToUpper(suffix))
}
