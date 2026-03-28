package service

import (
	"context"
	"errors"
	"time"

	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/appointment/repository"
	paymentmethod_repository "github.com/aseno/backend/pkg/paymentmethod/repository"
	"github.com/google/uuid"
)

var (
	ErrPaymentNotFound      = errors.New("pagamento não encontrado")
	ErrAppointmentNotFound  = errors.New("agendamento não encontrado")
	ErrPaymentExceedsPrice  = errors.New("valor do pagamento excede o saldo restante da sessão")
	ErrInvalidInstallments  = errors.New("parcelas só são permitidas para cartão de crédito")
)

type AppointmentPaymentService interface {
	Create(ctx context.Context, clinicID, appointmentID uuid.UUID, req *model.CreateAppointmentPaymentRequest) (*model.AppointmentPaymentResponse, error)
	Update(ctx context.Context, clinicID, paymentID uuid.UUID, req *model.CreateAppointmentPaymentRequest) (*model.AppointmentPaymentResponse, error)
	Delete(ctx context.Context, clinicID, paymentID uuid.UUID) error
	ListByAppointment(ctx context.Context, clinicID, appointmentID uuid.UUID) ([]*model.AppointmentPaymentResponse, error)
}

type appointmentPaymentService struct {
	paymentRepo       repository.AppointmentPaymentRepository
	appointmentRepo   repository.AppointmentRepository
	paymentMethodRepo paymentmethod_repository.PaymentMethodRepository
}

func NewAppointmentPaymentService(
	paymentRepo repository.AppointmentPaymentRepository,
	appointmentRepo repository.AppointmentRepository,
	paymentMethodRepo paymentmethod_repository.PaymentMethodRepository,
) AppointmentPaymentService {
	return &appointmentPaymentService{
		paymentRepo:       paymentRepo,
		appointmentRepo:   appointmentRepo,
		paymentMethodRepo: paymentMethodRepo,
	}
}

func (s *appointmentPaymentService) validatePayment(ctx context.Context, clinicID uuid.UUID, apt *model.Appointment, req *model.CreateAppointmentPaymentRequest, excludePaymentID *uuid.UUID) error {
	// Validate payment method exists and belongs to clinic
	method, err := s.paymentMethodRepo.GetByID(req.PaymentMethodID, clinicID)
	if err != nil {
		return errors.New("método de pagamento não encontrado")
	}

	// Installments only for credit cards
	if req.Installments > 1 && method.Type != "credit" {
		return ErrInvalidInstallments
	}

	// Calculate how much is already paid (excluding the payment being edited)
	totalPaid, err := s.paymentRepo.SumByAppointment(apt.ID)
	if err != nil {
		return err
	}

	// If editing, subtract the old payment's covered amount from totalPaid
	if excludePaymentID != nil {
		oldPayment, err := s.paymentRepo.GetByID(*excludePaymentID, clinicID)
		if err == nil {
			totalPaid -= (oldPayment.Amount + oldPayment.Discount)
		}
	}

	// amount = what the patient actually pays
	// discount = abatement on the session price
	// amount + discount = how much of the session price this payment covers
	if req.Discount < 0 {
		return errors.New("desconto não pode ser negativo")
	}

	covered := req.Amount + req.Discount
	if apt.Price > 0 && totalPaid+covered > apt.Price {
		return ErrPaymentExceedsPrice
	}

	return nil
}

func (s *appointmentPaymentService) Create(ctx context.Context, clinicID, appointmentID uuid.UUID, req *model.CreateAppointmentPaymentRequest) (*model.AppointmentPaymentResponse, error) {
	// Verify appointment belongs to clinic
	apt, err := s.appointmentRepo.GetByID(ctx, appointmentID, clinicID)
	if err != nil {
		return nil, ErrAppointmentNotFound
	}

	// Validate
	if err := s.validatePayment(ctx, clinicID, apt, req, nil); err != nil {
		return nil, err
	}

	installments := req.Installments
	if installments < 1 {
		installments = 1
	}

	payment := &model.AppointmentPayment{
		ClinicID:        clinicID,
		AppointmentID:   appointmentID,
		PaymentMethodID: req.PaymentMethodID,
		Amount:          req.Amount,
		Discount:        req.Discount,
		Installments:    installments,
		Notes:           req.Notes,
		PaidAt:          time.Now(),
	}

	if err := s.paymentRepo.Create(payment); err != nil {
		return nil, err
	}

	// Update appointment payment_status
	if err := s.updatePaymentStatus(ctx, apt); err != nil {
		return nil, err
	}

	// Reload with preloaded payment method
	created, err := s.paymentRepo.GetByID(payment.ID, clinicID)
	if err != nil {
		return payment.ToResponse(), nil
	}

	return created.ToResponse(), nil
}

func (s *appointmentPaymentService) Update(ctx context.Context, clinicID, paymentID uuid.UUID, req *model.CreateAppointmentPaymentRequest) (*model.AppointmentPaymentResponse, error) {
	payment, err := s.paymentRepo.GetByID(paymentID, clinicID)
	if err != nil {
		return nil, ErrPaymentNotFound
	}

	// Validate against appointment
	apt, err := s.appointmentRepo.GetByID(ctx, payment.AppointmentID, clinicID)
	if err != nil {
		return nil, ErrAppointmentNotFound
	}

	if err := s.validatePayment(ctx, clinicID, apt, req, &paymentID); err != nil {
		return nil, err
	}

	payment.PaymentMethodID = req.PaymentMethodID
	payment.Amount = req.Amount
	payment.Discount = req.Discount
	payment.Notes = req.Notes
	installments := req.Installments
	if installments < 1 {
		installments = 1
	}
	payment.Installments = installments

	if err := s.paymentRepo.Update(payment); err != nil {
		return nil, err
	}

	_ = s.updatePaymentStatus(ctx, apt)

	return payment.ToResponse(), nil
}

func (s *appointmentPaymentService) Delete(ctx context.Context, clinicID, paymentID uuid.UUID) error {
	payment, err := s.paymentRepo.GetByID(paymentID, clinicID)
	if err != nil {
		return ErrPaymentNotFound
	}

	if err := s.paymentRepo.Delete(paymentID, clinicID); err != nil {
		return err
	}

	// Update appointment payment_status
	apt, err := s.appointmentRepo.GetByID(ctx, payment.AppointmentID, clinicID)
	if err == nil {
		_ = s.updatePaymentStatus(ctx, apt)
	}

	return nil
}

func (s *appointmentPaymentService) ListByAppointment(ctx context.Context, clinicID, appointmentID uuid.UUID) ([]*model.AppointmentPaymentResponse, error) {
	payments, err := s.paymentRepo.ListByAppointment(appointmentID, clinicID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.AppointmentPaymentResponse, len(payments))
	for i, p := range payments {
		result[i] = p.ToResponse()
	}
	return result, nil
}

// updatePaymentStatus recalculates and saves the appointment's payment_status
func (s *appointmentPaymentService) updatePaymentStatus(ctx context.Context, apt *model.Appointment) error {
	totalPaid, err := s.paymentRepo.SumByAppointment(apt.ID)
	if err != nil {
		return err
	}

	var status model.PaymentStatus
	switch {
	case totalPaid <= 0:
		status = model.PaymentStatusPending
	case totalPaid >= apt.Price:
		status = model.PaymentStatusPaid
	default:
		status = model.PaymentStatusPartial
	}

	return s.appointmentRepo.UpdatePaymentStatus(ctx, apt.ID, status)
}
