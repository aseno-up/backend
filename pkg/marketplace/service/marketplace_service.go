package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/crypto"
	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/marketplace/model"
	payment_model "github.com/aseno/backend/pkg/payment/model"
)

// Repository interfaces

type SubaccountRepository interface {
	Create(ctx context.Context, sub *model.Subaccount) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Subaccount, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.Subaccount, error)
	GetByAsaasID(ctx context.Context, asaasID string) (*model.Subaccount, error)
	Update(ctx context.Context, sub *model.Subaccount) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.SubaccountStatus) error
}

type SplitConfigRepository interface {
	Create(ctx context.Context, config *model.SplitConfig) error
	GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.SplitConfig, error)
	Update(ctx context.Context, config *model.SplitConfig) error
}

type ClinicPaymentRepository interface {
	Create(ctx context.Context, payment *model.ClinicPayment) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.ClinicPayment, error)
	GetByAsaasID(ctx context.Context, asaasID string) (*model.ClinicPayment, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*model.ClinicPayment, int64, error)
	GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) (*model.ClinicPayment, error)
	ListWithFilters(ctx context.Context, clinicID uuid.UUID, filters model.PaymentFilters) ([]*model.ClinicPayment, int64, error)
	Update(ctx context.Context, payment *model.ClinicPayment) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	GetSummary(ctx context.Context, clinicID uuid.UUID, dateFrom, dateTo time.Time) (*model.FinancialSummaryResponse, error)
}

// External dependencies

type AsaasClient interface {
	CreateSubaccount(ctx context.Context, req *model.CreateSubaccountRequest) (*model.AsaasSubaccountResponse, error)
	GetSubaccount(ctx context.Context, accountID string) (*model.AsaasSubaccountResponse, error)
	CreateCustomerWithKey(ctx context.Context, apiKey string, req *payment_model.CreateCustomerRequest) (*payment_model.AsaasCustomerResponse, error)
	CreatePaymentWithKey(ctx context.Context, apiKey string, req *model.CreatePaymentWithSplitRequest) (*payment_model.AsaasPaymentResponse, error)
	GetPixQRCodeWithKey(ctx context.Context, apiKey, paymentID string) (*payment_model.AsaasPixQRCodeResponse, error)
	GetPaymentWithKey(ctx context.Context, apiKey, paymentID string) (*payment_model.AsaasPaymentResponse, error)
	GetBalanceWithKey(ctx context.Context, apiKey string) (*model.BalanceResponse, error)
}

type ClinicRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (ClinicInfo, error)
}

type ClinicInfo interface {
	GetID() uuid.UUID
	GetAccountID() uuid.UUID
	GetRequirePaymentBeforeBooking() bool
}

type AppointmentRepository interface {
	GetByID(ctx context.Context, appointmentID uuid.UUID, clinicID uuid.UUID) (AppointmentInfo, error)
	GetByPaymentToken(ctx context.Context, token string) (AppointmentInfo, error)
	UpdatePaymentStatus(ctx context.Context, appointmentID uuid.UUID, status string) error
	UpdateStatus(ctx context.Context, appointmentID uuid.UUID, status string) error
}

type AppointmentInfo interface {
	GetID() uuid.UUID
	GetClinicID() uuid.UUID
	GetPatientID() uuid.UUID
	GetPrice() float64
	GetStatus() string
	GetPaymentToken() string
	GetPaymentExpiresAt() *time.Time
}

type AppointmentPaymentRepository interface {
	SumByAppointment(appointmentID uuid.UUID) (float64, error)
	CreateFromWebhook(ctx context.Context, clinicID, appointmentID uuid.UUID, paymentMethodID *uuid.UUID, amount float64, paidAt time.Time) error
}

// Service interface

type MarketplaceService interface {
	// Clinic payment config (public)
	GetClinicPaymentConfig(ctx context.Context, clinicID uuid.UUID) (*model.ClinicPaymentConfig, error)

	// Subaccount
	CreateSubaccount(ctx context.Context, clinicID uuid.UUID, req *CreateSubaccountInput) (*model.SubaccountResponse, error)
	GetSubaccount(ctx context.Context, clinicID uuid.UUID) (*model.SubaccountResponse, error)

	// Split config
	GetSplitConfig(ctx context.Context, clinicID uuid.UUID) (*model.SplitConfigResponse, error)
	UpdateSplitConfig(ctx context.Context, clinicID uuid.UUID, req *UpdateSplitConfigInput) (*model.SplitConfigResponse, error)

	// Patient payments
	CreateAppointmentCharge(ctx context.Context, clinicID, appointmentID uuid.UUID, req *CreateChargeInput) (*model.ClinicPaymentResponse, error)
	GetPaymentPixQRCode(ctx context.Context, clinicID uuid.UUID, asaasPaymentID string) (*payment_model.AsaasPixQRCodeResponse, error)

	// Payment page (public, token-based)
	GetPaymentPageData(ctx context.Context, token string) (*model.PaymentPageData, error)
	CreateChargeByToken(ctx context.Context, token string, req *CreateChargeInput) (*model.ClinicPaymentResponse, error)

	// Payment lookup
	GetClinicPaymentByID(ctx context.Context, id uuid.UUID) (*model.ClinicPaymentResponse, error)

	// Webhook
	HandleSubaccountWebhook(ctx context.Context, payload *payment_model.AsaasWebhookPayload) error

	// Financial dashboard
	GetBalance(ctx context.Context, clinicID uuid.UUID) (*model.BalanceResponse, error)
	GetPaymentHistory(ctx context.Context, clinicID uuid.UUID, filters model.PaymentFilters) ([]*model.ClinicPaymentResponse, int64, error)
	GetFinancialSummary(ctx context.Context, clinicID uuid.UUID, dateFrom, dateTo string) (*model.FinancialSummaryResponse, error)
}

// Input DTOs

type CreateSubaccountInput struct {
	CompanyName   string  `json:"company_name" binding:"required"`
	CpfCnpj       string  `json:"cpf_cnpj" binding:"required"`
	CompanyType   string  `json:"company_type"`
	BirthDate     string  `json:"birth_date"`
	IncomeValue   float64 `json:"income_value" binding:"required,gt=0"`
	Email         string  `json:"email" binding:"required,email"`
	Phone         string  `json:"phone"`
	MobilePhone   string  `json:"mobile_phone"`
	PostalCode    string  `json:"postal_code"`
	Address       string  `json:"address"`
	AddressNumber string  `json:"address_number"`
	Province      string  `json:"province"`
	City          string  `json:"city"`
	State         string  `json:"state"`
}

type UpdateSplitConfigInput struct {
	FeeType  model.FeeType `json:"fee_type" binding:"required,oneof=percentage fixed"`
	FeeValue float64       `json:"fee_value" binding:"gte=0"`
	Active   *bool         `json:"active"`
}

type CreateChargeInput struct {
	BillingType          string                              `json:"billing_type" binding:"required"`
	CustomerName         string                              `json:"customer_name" binding:"required"`
	CustomerCpfCnpj      string                              `json:"customer_cpf_cnpj" binding:"required"`
	CustomerEmail        string                              `json:"customer_email"`
	CustomerPhone        string                              `json:"customer_phone"`
	CreditCard           *payment_model.CreditCardInfo       `json:"credit_card,omitempty"`
	CreditCardHolderInfo *payment_model.CreditCardHolderInfo `json:"credit_card_holder_info,omitempty"`
	InstallmentCount     *int                                `json:"installment_count,omitempty"`
}

// Implementation

type marketplaceService struct {
	asaasClient        AsaasClient
	subaccountRepo     SubaccountRepository
	splitConfigRepo    SplitConfigRepository
	clinicPaymentRepo  ClinicPaymentRepository
	clinicRepo         ClinicRepository
	appointmentRepo    AppointmentRepository
	aptPaymentRepo     AppointmentPaymentRepository
	encryptionKey         string
	asenoWalletID         string
	defaultFeePercent     float64
	webhookBaseURL        string
	paymentTimeoutMinutes int
}

type MarketplaceServiceConfig struct {
	EncryptionKey          string
	AsenoWalletID          string
	DefaultFeePercent      float64
	WebhookBaseURL         string
	PaymentTimeoutMinutes  int
}

func NewMarketplaceService(
	asaasClient AsaasClient,
	subaccountRepo SubaccountRepository,
	splitConfigRepo SplitConfigRepository,
	clinicPaymentRepo ClinicPaymentRepository,
	clinicRepo ClinicRepository,
	appointmentRepo AppointmentRepository,
	aptPaymentRepo AppointmentPaymentRepository,
	cfg MarketplaceServiceConfig,
) MarketplaceService {
	return &marketplaceService{
		asaasClient:       asaasClient,
		subaccountRepo:    subaccountRepo,
		splitConfigRepo:   splitConfigRepo,
		clinicPaymentRepo: clinicPaymentRepo,
		clinicRepo:        clinicRepo,
		appointmentRepo:   appointmentRepo,
		aptPaymentRepo:    aptPaymentRepo,
		encryptionKey:         cfg.EncryptionKey,
		asenoWalletID:         cfg.AsenoWalletID,
		defaultFeePercent:     cfg.DefaultFeePercent,
		paymentTimeoutMinutes: cfg.PaymentTimeoutMinutes,
		webhookBaseURL:    cfg.WebhookBaseURL,
	}
}

// GetClinicPaymentConfig returns the payment configuration for a clinic (public endpoint)
func (s *marketplaceService) GetClinicPaymentConfig(ctx context.Context, clinicID uuid.UUID) (*model.ClinicPaymentConfig, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	sub, err := s.subaccountRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return &model.ClinicPaymentConfig{
			HasSubaccount:  false,
			RequirePayment: false,
		}, nil
	}

	return &model.ClinicPaymentConfig{
		HasSubaccount:         sub.Status == model.SubaccountStatusActive,
		RequirePayment:        clinic.GetRequirePaymentBeforeBooking(),
		PaymentTimeoutMinutes: s.paymentTimeoutMinutes,
		Status:                string(sub.Status),
	}, nil
}

// CreateSubaccount creates an Asaas subaccount for a clinic
func (s *marketplaceService) CreateSubaccount(ctx context.Context, clinicID uuid.UUID, req *CreateSubaccountInput) (*model.SubaccountResponse, error) {
	// Check if subaccount already exists
	existing, err := s.subaccountRepo.GetByClinicID(ctx, clinicID)
	if err == nil && existing != nil {
		return nil, errors.ErrConflictInstance.WithMessage("Subaccount already exists for this clinic")
	}

	// Get clinic to resolve AccountID
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	// Create in Asaas
	asaasReq := &model.CreateSubaccountRequest{
		Name:          req.CompanyName,
		Email:         req.Email,
		CpfCnpj:       req.CpfCnpj,
		CompanyType:   req.CompanyType,
		BirthDate:     req.BirthDate,
		IncomeValue:   req.IncomeValue,
		Phone:         req.Phone,
		MobilePhone:   req.MobilePhone,
		PostalCode:    req.PostalCode,
		Address:       req.Address,
		AddressNumber: req.AddressNumber,
		Province:      req.Province,
		City:          req.City,
		State:         req.State,
		Webhooks:      s.webhookBaseURL,
	}

	asaasResp, err := s.asaasClient.CreateSubaccount(ctx, asaasReq)
	if err != nil {
		logger.Printf("Marketplace: failed to create subaccount in Asaas: %v", err)
		return nil, errors.ErrPaymentGatewayInstance.WithError(err).WithMessage(err.Error())
	}

	// Encrypt the API key before storing
	encryptedKey, err := crypto.Encrypt(asaasResp.APIKey, s.encryptionKey)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err).WithMessage("Failed to encrypt API key")
	}

	// Save locally
	sub := &model.Subaccount{
		ClinicID:        clinicID,
		AccountID:       clinic.GetAccountID(),
		AsaasAccountID:  asaasResp.ID,
		WalletID:        asaasResp.WalletID,
		EncryptedAPIKey: encryptedKey,
		Status:          model.SubaccountStatusActive,
		CompanyName:     req.CompanyName,
		CpfCnpj:         req.CpfCnpj,
		CompanyType:     req.CompanyType,
		IncomeValue:     req.IncomeValue,
		Email:           req.Email,
		Phone:           req.Phone,
		PostalCode:      req.PostalCode,
		Address:         req.Address,
		AddressNumber:   req.AddressNumber,
		Province:        req.Province,
		City:            req.City,
		State:           req.State,
	}

	if err := s.subaccountRepo.Create(ctx, sub); err != nil {
		return nil, err
	}

	// Create default split config
	splitConfig := &model.SplitConfig{
		ClinicID:      clinicID,
		AsenoWalletID: s.asenoWalletID,
		FeeType:       model.FeeTypePercentage,
		FeeValue:      s.defaultFeePercent,
		Active:        true,
	}
	if err := s.splitConfigRepo.Create(ctx, splitConfig); err != nil {
		logger.Printf("Warning: failed to create default split config for clinic %s: %v", clinicID, err)
	}

	return s.toSubaccountResponse(sub), nil
}

// GetSubaccount returns the subaccount for a clinic
func (s *marketplaceService) GetSubaccount(ctx context.Context, clinicID uuid.UUID) (*model.SubaccountResponse, error) {
	sub, err := s.subaccountRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	return s.toSubaccountResponse(sub), nil
}

// GetSplitConfig returns the split configuration for a clinic
func (s *marketplaceService) GetSplitConfig(ctx context.Context, clinicID uuid.UUID) (*model.SplitConfigResponse, error) {
	config, err := s.splitConfigRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	return s.toSplitConfigResponse(config), nil
}

// UpdateSplitConfig updates the split configuration for a clinic
func (s *marketplaceService) UpdateSplitConfig(ctx context.Context, clinicID uuid.UUID, req *UpdateSplitConfigInput) (*model.SplitConfigResponse, error) {
	config, err := s.splitConfigRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	config.FeeType = req.FeeType
	config.FeeValue = req.FeeValue
	if req.Active != nil {
		config.Active = *req.Active
	}

	if err := s.splitConfigRepo.Update(ctx, config); err != nil {
		return nil, err
	}

	return s.toSplitConfigResponse(config), nil
}

// CreateAppointmentCharge creates a payment charge for a patient appointment
func (s *marketplaceService) CreateAppointmentCharge(ctx context.Context, clinicID, appointmentID uuid.UUID, req *CreateChargeInput) (*model.ClinicPaymentResponse, error) {
	// Get subaccount and decrypt API key
	sub, err := s.subaccountRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Clinic has no active subaccount")
	}
	if sub.Status != model.SubaccountStatusActive {
		return nil, errors.ErrBadRequestInstance.WithMessage("Subaccount is not active")
	}

	apiKey, err := crypto.Decrypt(sub.EncryptedAPIKey, s.encryptionKey)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err).WithMessage("Failed to decrypt API key")
	}

	// Get appointment details
	apt, err := s.appointmentRepo.GetByID(ctx, appointmentID, clinicID)
	if err != nil {
		return nil, err
	}

	if apt.GetPrice() <= 0 {
		return nil, errors.ErrBadRequestInstance.WithMessage("Appointment has no price set")
	}

	// Check if charge already exists for this appointment
	existing, _ := s.clinicPaymentRepo.GetByAppointmentID(ctx, appointmentID)
	if existing != nil && (existing.Status == "PENDING" || existing.Status == "RECEIVED" || existing.Status == "CONFIRMED") {
		return existing.ToResponse(), nil
	}

	// First, create/get customer in the subaccount
	customerReq := &payment_model.CreateCustomerRequest{
		Name:     req.CustomerName,
		Document: req.CustomerCpfCnpj,
		Email:    req.CustomerEmail,
		Phone:    req.CustomerPhone,
	}

	// Create customer in subaccount context
	customerResp, err := s.createCustomerWithKey(ctx, apiKey, customerReq)
	if err != nil {
		logger.Printf("Marketplace: failed to create customer in subaccount: %v", err)
		return nil, errors.ErrPaymentGatewayInstance.WithError(err).WithMessage(err.Error())
	}

	// Build split array (per-clinic config from DB, fallback to global default)
	splits := s.buildSplitItems(ctx, clinicID, apt.GetPrice())

	// Calculate split fee
	var splitFee float64
	for _, split := range splits {
		if split.FixedValue > 0 {
			splitFee = split.FixedValue
		} else if split.PercentualValue > 0 {
			splitFee = math.Round(apt.GetPrice()*split.PercentualValue) / 100
		}
	}

	// Build payment request
	dueDate := time.Now().Format("2006-01-02")
	paymentReq := &model.CreatePaymentWithSplitRequest{
		CustomerID:           customerResp,
		BillingType:          req.BillingType,
		Value:                apt.GetPrice(),
		DueDate:              dueDate,
		Description:          fmt.Sprintf("Pagamento consulta - %s", appointmentID.String()[:8]),
		ExternalRef:          fmt.Sprintf("appointment:%s", appointmentID.String()),
		Split:                splits,
		CreditCard:           req.CreditCard,
		CreditCardHolderInfo: req.CreditCardHolderInfo,
		InstallmentCount:     req.InstallmentCount,
	}

	// Set PIX expiration based on appointment's payment_expires_at or fallback to timeout
	if req.BillingType == "PIX" {
		if apt.GetPaymentExpiresAt() != nil {
			paymentReq.PixExpirationDate = apt.GetPaymentExpiresAt().Format("2006-01-02 15:04:05")
		} else if s.paymentTimeoutMinutes > 0 {
			expiration := time.Now().Add(time.Duration(s.paymentTimeoutMinutes) * time.Minute)
			paymentReq.PixExpirationDate = expiration.Format("2006-01-02 15:04:05")
		}
	}

	// Create payment in Asaas via subaccount
	asaasResp, err := s.asaasClient.CreatePaymentWithKey(ctx, apiKey, paymentReq)
	if err != nil {
		logger.Printf("Marketplace: failed to create payment in Asaas: %v", err)
		return nil, errors.ErrPaymentGatewayInstance.WithError(err).WithMessage(err.Error())
	}

	// Save clinic payment locally
	patientID := apt.GetPatientID()
	clinicPayment := &model.ClinicPayment{
		AccountID:      sub.AccountID,
		ClinicID:       clinicID,
		AppointmentID:  &appointmentID,
		PatientID:      &patientID,
		AsaasPaymentID: asaasResp.ID,
		Value:          asaasResp.Value,
		NetValue:       asaasResp.NetValue,
		SplitFeeValue:  splitFee,
		Status:         asaasResp.Status,
		BillingType:    req.BillingType,
		DueDate:        time.Now(),
		Description:    paymentReq.Description,
		ExternalRef:    paymentReq.ExternalRef,
		Installments:   req.InstallmentCount,
	}

	if err := s.clinicPaymentRepo.Create(ctx, clinicPayment); err != nil {
		return nil, err
	}

	// If PIX, fetch QR code
	if req.BillingType == "PIX" {
		qr, err := s.asaasClient.GetPixQRCodeWithKey(ctx, apiKey, asaasResp.ID)
		if err == nil {
			clinicPayment.PixQRCodeImage = qr.EncodedImage
			clinicPayment.PixCopyPaste = qr.Payload
			_ = s.clinicPaymentRepo.Update(ctx, clinicPayment)
		}
	}

	return clinicPayment.ToResponse(), nil
}

// createCustomerWithKey creates a customer in the subaccount via Asaas API and returns the Asaas customer ID.
func (s *marketplaceService) createCustomerWithKey(ctx context.Context, apiKey string, req *payment_model.CreateCustomerRequest) (string, error) {
	resp, err := s.asaasClient.CreateCustomerWithKey(ctx, apiKey, req)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// GetPaymentPixQRCode retrieves PIX QR code for a clinic payment
func (s *marketplaceService) GetPaymentPixQRCode(ctx context.Context, clinicID uuid.UUID, asaasPaymentID string) (*payment_model.AsaasPixQRCodeResponse, error) {
	sub, err := s.subaccountRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	apiKey, err := crypto.Decrypt(sub.EncryptedAPIKey, s.encryptionKey)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return s.asaasClient.GetPixQRCodeWithKey(ctx, apiKey, asaasPaymentID)
}

// GetPaymentPageData returns data for the public payment page, validating token, status, and expiration
func (s *marketplaceService) GetPaymentPageData(ctx context.Context, token string) (*model.PaymentPageData, error) {
	apt, err := s.appointmentRepo.GetByPaymentToken(ctx, token)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Token de pagamento invalido")
	}

	// Check if appointment is canceled
	status := apt.GetStatus()
	if status == "canceled" || status == "no_show" || status == "completed" {
		return nil, errors.ErrBadRequestInstance.WithMessage("Este agendamento foi cancelado ou ja foi concluido")
	}

	// Check expiration
	if apt.GetPaymentExpiresAt() != nil && time.Now().After(*apt.GetPaymentExpiresAt()) {
		return nil, errors.ErrBadRequestInstance.WithMessage("O prazo para pagamento deste agendamento expirou")
	}

	// Calculate remaining seconds
	remaining := 0
	if apt.GetPaymentExpiresAt() != nil {
		remaining = int(time.Until(*apt.GetPaymentExpiresAt()).Seconds())
		if remaining < 0 {
			remaining = 0
		}
	}

	// Check subaccount
	hasSubaccount := false
	sub, err := s.subaccountRepo.GetByClinicID(ctx, apt.GetClinicID())
	if err == nil && sub.Status == model.SubaccountStatusActive {
		hasSubaccount = true
	}

	// Check for existing pending payment
	var existingPayment *model.ClinicPaymentResponse
	aptID := apt.GetID()
	existing, err := s.clinicPaymentRepo.GetByAppointmentID(ctx, aptID)
	if err == nil && (existing.Status == "PENDING" || existing.Status == "RECEIVED" || existing.Status == "CONFIRMED") {
		existingPayment = existing.ToResponse()
	}

	return &model.PaymentPageData{
		AppointmentID:    apt.GetID().String(),
		ClinicID:         apt.GetClinicID().String(),
		StartTime:        "", // filled by handler if needed
		EndTime:          "",
		Price:            apt.GetPrice(),
		PaymentStatus:    "", // filled by handler
		Status:           status,
		RemainingSeconds: remaining,
		HasSubaccount:    hasSubaccount,
		ExistingPayment:  existingPayment,
	}, nil
}

// CreateChargeByToken creates a charge using a payment token, with full validation
func (s *marketplaceService) CreateChargeByToken(ctx context.Context, token string, req *CreateChargeInput) (*model.ClinicPaymentResponse, error) {
	apt, err := s.appointmentRepo.GetByPaymentToken(ctx, token)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Token de pagamento invalido")
	}

	// Validate status
	status := apt.GetStatus()
	if status == "canceled" || status == "no_show" || status == "completed" {
		return nil, errors.ErrBadRequestInstance.WithMessage("Este agendamento foi cancelado ou ja foi concluido")
	}

	// Validate expiration
	if apt.GetPaymentExpiresAt() != nil && time.Now().After(*apt.GetPaymentExpiresAt()) {
		return nil, errors.ErrBadRequestInstance.WithMessage("O prazo para pagamento deste agendamento expirou")
	}

	// Calculate remaining seconds for PIX expiration
	if req.BillingType == "PIX" && apt.GetPaymentExpiresAt() != nil {
		remaining := int(time.Until(*apt.GetPaymentExpiresAt()).Seconds())
		if remaining <= 0 {
			return nil, errors.ErrBadRequestInstance.WithMessage("O prazo para pagamento deste agendamento expirou")
		}
	}

	return s.CreateAppointmentCharge(ctx, apt.GetClinicID(), apt.GetID(), req)
}

// GetClinicPaymentByID returns a clinic payment by its ID
func (s *marketplaceService) GetClinicPaymentByID(ctx context.Context, id uuid.UUID) (*model.ClinicPaymentResponse, error) {
	payment, err := s.clinicPaymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return payment.ToResponse(), nil
}

// HandleSubaccountWebhook processes webhook events from Asaas for subaccount payments
func (s *marketplaceService) HandleSubaccountWebhook(ctx context.Context, payload *payment_model.AsaasWebhookPayload) error {
	if payload.Payment == nil {
		return nil
	}

	// Find the clinic payment by Asaas ID
	clinicPayment, err := s.clinicPaymentRepo.GetByAsaasID(ctx, payload.Payment.ID)
	if err != nil {
		// Payment not found locally — might be a new payment created externally
		logger.Printf("Marketplace webhook: payment %s not found locally, skipping", payload.Payment.ID)
		return nil
	}

	// Idempotency: skip if already in target status
	if clinicPayment.Status == payload.Payment.Status {
		return nil
	}

	// Update payment status
	clinicPayment.Status = payload.Payment.Status
	clinicPayment.NetValue = payload.Payment.NetValue

	if payload.Payment.PaymentDate != "" {
		if t, err := time.Parse("2006-01-02", payload.Payment.PaymentDate); err == nil {
			clinicPayment.PaymentDate = &t
		}
	}

	if err := s.clinicPaymentRepo.Update(ctx, clinicPayment); err != nil {
		return err
	}

	// Bridge: on payment confirmation, auto-create AppointmentPayment and activate appointment
	switch payment_model.WebhookEvent(payload.Event) {
	case payment_model.WebhookEventPaymentReceived, payment_model.WebhookEventPaymentConfirmed:
		if clinicPayment.AppointmentID != nil {
			// If appointment was awaiting_payment, transition to pending
			s.activateAwaitingPaymentAppointment(ctx, *clinicPayment.AppointmentID, clinicPayment.ClinicID)
			s.bridgeToAppointmentPayment(ctx, clinicPayment)
		}
	}

	return nil
}

// activateAwaitingPaymentAppointment transitions an appointment from awaiting_payment to pending
func (s *marketplaceService) activateAwaitingPaymentAppointment(ctx context.Context, appointmentID, clinicID uuid.UUID) {
	apt, err := s.appointmentRepo.GetByID(ctx, appointmentID, clinicID)
	if err != nil {
		return
	}
	// Only transition if currently awaiting payment
	if apt.GetStatus() == "awaiting_payment" {
		if err := s.appointmentRepo.UpdateStatus(ctx, appointmentID, "pending"); err != nil {
			logger.Printf("Marketplace: failed to activate appointment %s: %v", appointmentID, err)
		} else {
			logger.Printf("Marketplace: appointment %s activated (awaiting_payment → pending)", appointmentID)
		}
	}
}

// bridgeToAppointmentPayment creates an AppointmentPayment record so the appointment payment_status
// transitions automatically. This bridges online payments with the manual payment system.
func (s *marketplaceService) bridgeToAppointmentPayment(ctx context.Context, cp *model.ClinicPayment) {
	if cp.AppointmentID == nil {
		return
	}

	// Check if already paid to avoid double-payment
	currentSum, err := s.aptPaymentRepo.SumByAppointment(*cp.AppointmentID)
	if err != nil {
		logger.Printf("Marketplace bridge: failed to sum payments for appointment %s: %v", cp.AppointmentID, err)
		return
	}

	// Get appointment to check price
	apt, err := s.appointmentRepo.GetByID(ctx, *cp.AppointmentID, cp.ClinicID)
	if err != nil {
		logger.Printf("Marketplace bridge: failed to get appointment %s: %v", cp.AppointmentID, err)
		return
	}

	if currentSum >= apt.GetPrice() {
		logger.Printf("Marketplace bridge: appointment %s already fully paid (sum=%.2f, price=%.2f)", cp.AppointmentID, currentSum, apt.GetPrice())
		return
	}

	paidAt := time.Now()
	if cp.PaymentDate != nil {
		paidAt = *cp.PaymentDate
	}

	if err := s.aptPaymentRepo.CreateFromWebhook(ctx, cp.ClinicID, *cp.AppointmentID, nil, cp.Value, paidAt); err != nil {
		logger.Printf("Marketplace bridge: failed to create appointment payment for %s: %v", cp.AppointmentID, err)
		return
	}

	// Update appointment payment status
	newSum := currentSum + cp.Value
	var status string
	switch {
	case newSum <= 0:
		status = "pending"
	case newSum >= apt.GetPrice():
		status = "paid"
	default:
		status = "partial"
	}

	if err := s.appointmentRepo.UpdatePaymentStatus(ctx, *cp.AppointmentID, status); err != nil {
		logger.Printf("Marketplace bridge: failed to update appointment payment status: %v", err)
	}
}

// GetBalance returns the financial balance for a clinic's subaccount
func (s *marketplaceService) GetBalance(ctx context.Context, clinicID uuid.UUID) (*model.BalanceResponse, error) {
	sub, err := s.subaccountRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	apiKey, err := crypto.Decrypt(sub.EncryptedAPIKey, s.encryptionKey)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return s.asaasClient.GetBalanceWithKey(ctx, apiKey)
}

// GetPaymentHistory returns paginated clinic payments with filters
func (s *marketplaceService) GetPaymentHistory(ctx context.Context, clinicID uuid.UUID, filters model.PaymentFilters) ([]*model.ClinicPaymentResponse, int64, error) {
	payments, total, err := s.clinicPaymentRepo.ListWithFilters(ctx, clinicID, filters)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]*model.ClinicPaymentResponse, len(payments))
	for i, p := range payments {
		responses[i] = p.ToResponse()
	}

	return responses, total, nil
}

// GetFinancialSummary returns aggregated financial data for a clinic
func (s *marketplaceService) GetFinancialSummary(ctx context.Context, clinicID uuid.UUID, dateFrom, dateTo string) (*model.FinancialSummaryResponse, error) {
	from, err := time.Parse("2006-01-02", dateFrom)
	if err != nil {
		from = time.Now().AddDate(0, -1, 0) // Default: last 30 days
	}
	to, err := time.Parse("2006-01-02", dateTo)
	if err != nil {
		to = time.Now()
	}

	return s.clinicPaymentRepo.GetSummary(ctx, clinicID, from, to)
}

// buildSplitItems builds the split array for a payment based on the split config
func (s *marketplaceService) buildSplitItems(ctx context.Context, clinicID uuid.UUID, value float64) []model.SplitItem {
	if s.asenoWalletID == "" {
		return nil
	}

	// Try to get per-clinic split config from DB
	config, err := s.splitConfigRepo.GetByClinicID(ctx, clinicID)
	if err == nil && config.Active && config.FeeValue > 0 {
		item := model.SplitItem{WalletID: config.AsenoWalletID}
		if config.FeeType == model.FeeTypePercentage {
			item.PercentualValue = config.FeeValue
		} else {
			item.FixedValue = config.FeeValue
		}
		return []model.SplitItem{item}
	}

	// Fallback to global default
	if s.defaultFeePercent <= 0 {
		return nil
	}
	return []model.SplitItem{
		{
			WalletID:        s.asenoWalletID,
			PercentualValue: s.defaultFeePercent,
		},
	}
}

// Converters

func (s *marketplaceService) toSubaccountResponse(sub *model.Subaccount) *model.SubaccountResponse {
	return &model.SubaccountResponse{
		ID:             sub.ID.String(),
		ClinicID:       sub.ClinicID.String(),
		AsaasAccountID: sub.AsaasAccountID,
		WalletID:       sub.WalletID,
		Status:         sub.Status,
		CompanyName:    sub.CompanyName,
		CpfCnpj:        sub.CpfCnpj,
		Email:          sub.Email,
		CreatedAt:      sub.CreatedAt.Format(time.RFC3339),
	}
}

func (s *marketplaceService) toSplitConfigResponse(config *model.SplitConfig) *model.SplitConfigResponse {
	return &model.SplitConfigResponse{
		ID:       config.ID.String(),
		ClinicID: config.ClinicID.String(),
		FeeType:  config.FeeType,
		FeeValue: config.FeeValue,
		Active:   config.Active,
	}
}
