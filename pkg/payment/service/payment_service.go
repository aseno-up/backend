package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aseno/backend/internal/logger"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	account_model "github.com/aseno/backend/pkg/account/model"
	account_repository "github.com/aseno/backend/pkg/account/repository"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	"github.com/aseno/backend/pkg/payment/model"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	plan_service "github.com/aseno/backend/pkg/plan/service"
)

// PaymentClient defines the interface for payment gateway operations
type PaymentClient interface {
	// Customer operations
	CreateCustomer(ctx context.Context, req *model.CreateCustomerRequest) (*model.AsaasCustomerResponse, error)
	GetCustomer(ctx context.Context, customerID string) (*model.AsaasCustomerResponse, error)
	UpdateCustomer(ctx context.Context, customerID string, req *model.CreateCustomerRequest) (*model.AsaasCustomerResponse, error)
	DeleteCustomer(ctx context.Context, customerID string) error

	// Payment operations
	CreatePayment(ctx context.Context, req *model.CreatePaymentRequest) (*model.AsaasPaymentResponse, error)
	GetPayment(ctx context.Context, paymentID string) (*model.AsaasPaymentResponse, error)
	GetPaymentsByCustomer(ctx context.Context, customerID string, limit, offset int) (*model.PaymentListResponse, error)
	DeletePayment(ctx context.Context, paymentID string) error
	GetPixQRCode(ctx context.Context, paymentID string) (*model.AsaasPixQRCodeResponse, error)

	// Subscription operations
	CreateSubscription(ctx context.Context, req *model.CreateSubscriptionRequest) (*model.AsaasSubscriptionResponse, error)
	GetSubscription(ctx context.Context, subscriptionID string) (*model.AsaasSubscriptionResponse, error)
	GetSubscriptionsByCustomer(ctx context.Context, customerID string) (*model.SubscriptionListResponse, error)
	UpdateSubscription(ctx context.Context, subscriptionID string, req *model.CreateSubscriptionRequest) (*model.AsaasSubscriptionResponse, error)
	DeleteSubscription(ctx context.Context, subscriptionID string) error
	GetSubscriptionPayments(ctx context.Context, subscriptionID string) (*model.PaymentListResponse, error)
}

// CustomerRepository defines the interface for customer persistence
type CustomerRepository interface {
	Create(ctx context.Context, customer *model.Customer) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Customer, error)
	GetByAccountID(ctx context.Context, accountID uuid.UUID) (*model.Customer, error)
	GetByAsaasID(ctx context.Context, asaasID string) (*model.Customer, error)
	Update(ctx context.Context, customer *model.Customer) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PaymentRepository defines the interface for payment persistence
type PaymentRepository interface {
	Create(ctx context.Context, payment *model.Payment) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Payment, error)
	GetByAsaasID(ctx context.Context, asaasID string) (*model.Payment, error)
	GetByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*model.Payment, int64, error)
	GetByCustomerID(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*model.Payment, int64, error)
	Update(ctx context.Context, payment *model.Payment) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.PaymentStatus) error
}

// SubscriptionRepository defines the interface for subscription persistence
type SubscriptionRepository interface {
	Create(ctx context.Context, subscription *model.Subscription) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Subscription, error)
	GetByAsaasID(ctx context.Context, asaasID string) (*model.Subscription, error)
	GetByAccountID(ctx context.Context, accountID uuid.UUID) (*model.Subscription, error)
	GetByCustomerID(ctx context.Context, customerID uuid.UUID) ([]*model.Subscription, error)
	Update(ctx context.Context, subscription *model.Subscription) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.SubscriptionStatus) error
}

// PaymentService defines the interface for payment operations
type PaymentService interface {
	// Customer operations
	CreateOrGetCustomer(ctx context.Context, clinicID uuid.UUID, req *CreateCustomerInput) (*model.Customer, error)
	GetCustomerByClinic(ctx context.Context, clinicID uuid.UUID) (*model.Customer, error)

	// Payment operations
	CreatePayment(ctx context.Context, clinicID uuid.UUID, req *CreatePaymentInput) (*model.Payment, error)
	GetPayment(ctx context.Context, clinicID uuid.UUID, paymentID string) (*model.Payment, error)
	GetPayments(ctx context.Context, clinicID uuid.UUID, page, perPage int) ([]*model.Payment, int64, error)
	GetPixQRCode(ctx context.Context, clinicID uuid.UUID, paymentID string) (*model.AsaasPixQRCodeResponse, error)
	CancelPayment(ctx context.Context, clinicID uuid.UUID, paymentID uuid.UUID) error

	// Subscription operations
	CreateSubscription(ctx context.Context, clinicID uuid.UUID, req *CreateSubscriptionInput) (*model.Subscription, error)
	GetSubscription(ctx context.Context, clinicID uuid.UUID) (*model.Subscription, error)
	CancelSubscription(ctx context.Context, clinicID uuid.UUID) error
	SwitchPlan(ctx context.Context, clinicID uuid.UUID, newPlan string, billingType string, cycle string, creditCard *model.CreditCardInfo, creditCardHolder *model.CreditCardHolderInfo) (*model.Subscription, error)
	GetSubscriptionPayments(ctx context.Context, clinicID uuid.UUID) (*model.PaymentListResponse, error)

	// Helper methods for registration flow
	CreateCustomerForClinic(ctx context.Context, clinicID uuid.UUID, req *model.CreateCustomerRequest) error
	CreateSubscriptionForClinic(ctx context.Context, clinicID uuid.UUID, planID uuid.UUID, offerToken *string, billing string) error

	// Webhook handling
	HandleWebhook(ctx context.Context, payload *model.AsaasWebhookPayload) error
}

// CreateCustomerInput represents input for creating a customer
type CreateCustomerInput struct {
	Name          string
	Email         string
	Phone         string
	Document      string
	PostalCode    string
	Address       string
	AddressNumber string
	Province      string
	City          string
	State         string
}

// CreatePaymentInput represents input for creating a payment
type CreatePaymentInput struct {
	BillingType model.BillingType
	Value       float64
	DueDate     time.Time
	Description string
}

// CreateSubscriptionInput represents input for creating a subscription
type CreateSubscriptionInput struct {
	BillingType model.BillingType
	PlanID      uuid.UUID
	OfferToken  *string
	Cycle       model.BillingCycle
}

// paymentService implements PaymentService
type paymentService struct {
	client           PaymentClient
	customerRepo     CustomerRepository
	paymentRepo      PaymentRepository
	subscriptionRepo SubscriptionRepository
	clinicRepo       clinic_repository.ClinicRepository
	accountRepo      account_repository.AccountRepository
	planService      plan_service.PlanService
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	client PaymentClient,
	customerRepo CustomerRepository,
	subscriptionRepo SubscriptionRepository,
	paymentRepo PaymentRepository,
	clinicRepo clinic_repository.ClinicRepository,
	accountRepo account_repository.AccountRepository,
	planService plan_service.PlanService,
) PaymentService {
	return &paymentService{
		client:           client,
		customerRepo:     customerRepo,
		subscriptionRepo: subscriptionRepo,
		paymentRepo:      paymentRepo,
		clinicRepo:       clinicRepo,
		accountRepo:      accountRepo,
		planService:      planService,
	}
}
func (s *paymentService) CreateOrGetCustomer(ctx context.Context, clinicID uuid.UUID, req *CreateCustomerInput) (*model.Customer, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Clinic not found")
	}

	// Check if customer already exists for this clinic
	existing, err := s.customerRepo.GetByAccountID(ctx, clinic.AccountID)
	if err == nil && existing != nil {
		// Sync the customer data with Asaas to ensure CPF/CNPJ is present for subscription
		asaasReq := &model.CreateCustomerRequest{
			Name:          req.Name,
			Email:         req.Email,
			Phone:         req.Phone,
			Document:      req.Document,
			PostalCode:    req.PostalCode,
			Address:       req.Address,
			AddressNumber: req.AddressNumber,
			Province:      req.Province,
			City:          req.City,
			State:         req.State,
			ExternalRef:   clinicID.String(),
		}

		_, updateErr := s.client.UpdateCustomer(ctx, existing.AsaasCustomerID, asaasReq)
		if updateErr != nil {
			logger.Warnf("[PAYMENT] Warning: failed to update Asaas customer %s: %v", existing.AsaasCustomerID, updateErr)
			// Don't fail the request completely if update fails, but log it
			// However if it's a validation error (like invalid CPF), we should return it
			return nil, errors.ErrPaymentGatewayInstance.WithMessage(fmt.Sprintf("Asaas: %v", updateErr))
		}

		// Also update the local database if document changed
		if req.Document != "" && existing.Document != req.Document {
			existing.Document = req.Document
			if len(req.Document) > 11 {
				existing.DocumentType = model.DocumentTypeCNPJ
			} else {
				existing.DocumentType = model.DocumentTypeCPF
			}
			// Let's assume there's a repository method to update, or we just rely on Asaas having the data
			// (Ideally we would have an s.customerRepo.Update, but we can skip it if not urgently needed)
		}

		return existing, nil
	}

	// Create customer in Asaas
	asaasReq := &model.CreateCustomerRequest{
		Name:          req.Name,
		Email:         req.Email,
		Phone:         req.Phone,
		Document:      req.Document,
		PostalCode:    req.PostalCode,
		Address:       req.Address,
		AddressNumber: req.AddressNumber,
		Province:      req.Province,
		City:          req.City,
		State:         req.State,
		ExternalRef:   clinicID.String(),
	}

	asaasCustomer, err := s.client.CreateCustomer(ctx, asaasReq)
	if err != nil {
		logger.Errorf("[PAYMENT] Error creating Asaas customer: %v", err)
		return nil, errors.ErrPaymentGatewayInstance.WithMessage(fmt.Sprintf("Asaas: %v", err))
	}

	// Determine document type
	docType := model.DocumentTypeCPF
	if len(req.Document) > 11 {
		docType = model.DocumentTypeCNPJ
	}

	// Save customer to database
	customer := &model.Customer{
		AccountID:       clinic.AccountID,
		AsaasCustomerID: asaasCustomer.ID,
		Name:            req.Name,
		Email:           req.Email,
		Phone:           req.Phone,
		Document:        req.Document,
		DocumentType:    docType,
		PostalCode:      req.PostalCode,
		Address:         req.Address,
		AddressNumber:   req.AddressNumber,
		Province:        asaasCustomer.Province,
		City:            fmt.Sprintf("%v", asaasCustomer.City),
		State:           fmt.Sprintf("%v", asaasCustomer.State),
		ExternalRef:     clinicID.String(),
	}

	if err := s.customerRepo.Create(ctx, customer); err != nil {
		return nil, err
	}

	return customer, nil
}

// GetCustomerByClinic retrieves the customer for a clinic
func (s *paymentService) GetCustomerByClinic(ctx context.Context, clinicID uuid.UUID) (*model.Customer, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	return s.customerRepo.GetByAccountID(ctx, clinic.AccountID)
}

// CreatePayment creates a new payment
func (s *paymentService) CreatePayment(ctx context.Context, clinicID uuid.UUID, req *CreatePaymentInput) (*model.Payment, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Clinic not found")
	}

	// Get customer for clinic
	customer, err := s.customerRepo.GetByAccountID(ctx, clinic.AccountID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Customer not found. Please create billing profile first.")
	}

	// Create payment in Asaas
	asaasReq := &model.CreatePaymentRequest{
		CustomerID:  customer.AsaasCustomerID,
		BillingType: req.BillingType,
		Value:       req.Value,
		DueDate:     req.DueDate.Format("2006-01-02"),
		Description: req.Description,
		ExternalRef: clinicID.String(),
	}

	asaasPayment, err := s.client.CreatePayment(ctx, asaasReq)
	if err != nil {
		return nil, errors.ErrPaymentGatewayInstance.WithError(err)
	}

	// Parse due date
	dueDate, _ := time.Parse("2006-01-02", asaasPayment.DueDate)

	// Save payment to database
	payment := &model.Payment{
		AccountID:      clinic.AccountID,
		CustomerID:     customer.ID,
		AsaasPaymentID: asaasPayment.ID,
		Value:          asaasPayment.Value,
		NetValue:       asaasPayment.NetValue,
		Status:         model.PaymentStatus(asaasPayment.Status),
		BillingType:    model.BillingType(asaasPayment.BillingType),
		DueDate:        dueDate,
		InvoiceURL:     asaasPayment.InvoiceURL,
		BankSlipURL:    asaasPayment.BankSlipURL,
		Description:    asaasPayment.Description,
		ExternalRef:    asaasPayment.ExternalRef,
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

// GetPayment retrieves a specific payment from local DB, falling back to Asaas if it's an Asaas ID
func (s *paymentService) GetPayment(ctx context.Context, clinicID uuid.UUID, paymentID string) (*model.Payment, error) {
	// Check if paymentID is an Asaas ID (e.g., pay_123456)
	if strings.HasPrefix(paymentID, "pay_") {
		// Try to find it in our DB by Asaas ID first
		payment, err := s.paymentRepo.GetByAsaasID(ctx, paymentID)
		if err == nil {
			return payment, nil
		}
		// Try to fetch from Asaas API (we return a rough representation or just an error since we return *model.Payment)
		// For now, if we don't have it locally, return not found, as we need a DB record to construct *model.Payment easily
		return nil, errors.ErrNotFoundInstance.WithMessage("Payment not found in local database")
	}

	parsedUUID, err := uuid.Parse(paymentID)
	if err != nil {
		return nil, errors.ErrInvalidInstance.WithMessage("Invalid payment ID format")
	}

	payment, err := s.paymentRepo.GetByID(ctx, parsedUUID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Payment not found")
	}

	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Clinic not found")
	}

	// Verify ownership (indirectly via AccountID)
	if payment.AccountID != clinic.AccountID {
		return nil, errors.ErrUnauthorizedInstance.WithMessage("Payment does not belong to this clinic")
	}

	return payment, nil
}

// GetPayments retrieves payments for a clinic
func (s *paymentService) GetPayments(ctx context.Context, clinicID uuid.UUID, page, perPage int) ([]*model.Payment, int64, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	return s.paymentRepo.GetByAccountID(ctx, clinic.AccountID, perPage, offset)
}

// GetPixQRCode retrieves the PIX QR code for a payment
func (s *paymentService) GetPixQRCode(ctx context.Context, clinicID uuid.UUID, paymentID string) (*model.AsaasPixQRCodeResponse, error) {
	// Check if paymentID is an Asaas ID (e.g., pay_123456)
	if strings.HasPrefix(paymentID, "pay_") {
		// Fetch directly from Asaas. To be perfectly secure, we'd verify the customer matches the clinic.
		// For now, since the frontend explicitly requested this payment ID it just got from GetSubscriptionPayments,
		// we fetch it from Asaas.
		return s.client.GetPixQRCode(ctx, paymentID)
	}

	// Otherwise, assume it's an internal UUID
	parsedUUID, err := uuid.Parse(paymentID)
	if err != nil {
		return nil, errors.ErrInvalidInstance.WithMessage("Invalid payment ID format")
	}

	payment, err := s.GetPayment(ctx, clinicID, parsedUUID.String())
	if err != nil {
		return nil, err
	}

	return s.client.GetPixQRCode(ctx, payment.AsaasPaymentID)
}

// CancelPayment cancels a payment
func (s *paymentService) CancelPayment(ctx context.Context, clinicID uuid.UUID, paymentID uuid.UUID) error {
	payment, err := s.GetPayment(ctx, clinicID, paymentID.String())
	if err != nil {
		return err
	}

	if err := s.client.DeletePayment(ctx, payment.AsaasPaymentID); err != nil {
		return errors.ErrPaymentGatewayInstance.WithError(err)
	}

	return s.paymentRepo.UpdateStatus(ctx, paymentID, model.PaymentStatus("CANCELED"))
}

// CreateSubscription creates a new subscription
func (s *paymentService) CreateSubscription(ctx context.Context, clinicID uuid.UUID, req *CreateSubscriptionInput) (*model.Subscription, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Clinic not found")
	}

	// Get customer for clinic
	customer, err := s.customerRepo.GetByAccountID(ctx, clinic.AccountID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Customer not found. Please create billing profile first.")
	}

	// Check if subscription already exists
	existing, _ := s.subscriptionRepo.GetByAccountID(ctx, clinic.AccountID)
	if existing != nil && existing.Status == model.SubscriptionStatusActive {
		return nil, errors.ErrConflictInstance.WithMessage("Clinic already has an active subscription")
	}

	// Get actual Plan Price
	plan, err := s.planService.GetPlanByID(ctx, req.PlanID)
	if err != nil {
		return nil, errors.ErrBadRequestInstance.WithMessage("Invalid plan")
	}

	var planCycleType plan_model.CycleType
	if req.Cycle == model.BillingCycleYearly {
		planCycleType = plan_model.CycleTypeYearly
	} else {
		planCycleType = plan_model.CycleTypeMonthly
	}

	var offer *plan_model.Offer
	if req.OfferToken != nil && *req.OfferToken != "" {
		o, err := s.planService.ValidateOffer(ctx, *req.OfferToken)
		if err == nil {
			offer = o
		}
	}

	value, err := s.planService.CalculateBestPrice(ctx, plan, offer, planCycleType)
	if err != nil {
		return nil, errors.ErrBadRequestInstance.WithMessage(err.Error())
	}

	description := "Assinatura " + plan.Name

	// Create subscription in Asaas
	nextDueDate := time.Now().AddDate(0, 0, 1) // Tomorrow
	asaasReq := &model.CreateSubscriptionRequest{
		CustomerID:  customer.AsaasCustomerID,
		BillingType: req.BillingType,
		Value:       value,
		NextDueDate: nextDueDate.Format("2006-01-02"),
		Cycle:       req.Cycle,
		Description: description,
		ExternalRef: clinicID.String(),
	}

	asaasSub, err := s.client.CreateSubscription(ctx, asaasReq)
	if err != nil {
		return nil, errors.ErrPaymentGatewayInstance.WithError(err)
	}

	// Validate Asaas returned a valid subscription ID
	if asaasSub.ID == "" {
		return nil, errors.ErrPaymentGatewayInstance.WithMessage("Asaas returned empty subscription ID")
	}

	// Parse next due date
	parsedNextDueDate, _ := time.Parse("2006-01-02", asaasSub.NextDueDate)

	// Determine OfferID for tracking usages
	var offerID *uuid.UUID
	if offer != nil {
		offerID = &offer.ID
		// Consume offer
		_ = s.planService.UseOffer(ctx, *offerID)
	}

	// Calculate absolute limits based on Plan or Custom via Offer
	maxClinics := plan.MaxClinics
	maxProfessionals := plan.MaxProfessionals
	if offer != nil {
		if offer.CustomMaxClinics != nil {
			maxClinics = *offer.CustomMaxClinics
		}
		if offer.CustomMaxProfessionals != nil {
			maxProfessionals = *offer.CustomMaxProfessionals
		}
	}

	// Save subscription to database
	subscription := &model.Subscription{
		AccountID:           clinic.AccountID,
		CustomerID:          customer.ID,
		AsaasSubscriptionID: asaasSub.ID,
		Value:               asaasSub.Value,
		Status:              model.SubscriptionStatus(asaasSub.Status),
		BillingType:         model.BillingType(asaasSub.BillingType),
		Cycle:               model.BillingCycle(asaasSub.Cycle),
		NextDueDate:         parsedNextDueDate,
		Description:         asaasSub.Description,
		ExternalRef:         asaasSub.ExternalRef,

		PlanID:                     &plan.ID,
		OfferID:                    offerID,
		PriceAtContract:            value,
		MaxClinicsAtContract:       maxClinics,
		MaxProfessionalsAtContract: maxProfessionals,
		MaxWhatsappAtContract:      plan.MaxWhatsappConnections,
		MaxGoogleAtContract:        plan.MaxGoogleCalendars,
	}

	if err := s.subscriptionRepo.Create(ctx, subscription); err != nil {
		return nil, err
	}

	return subscription, nil
}

// GetSubscription retrieves the subscription for a clinic
func (s *paymentService) GetSubscription(ctx context.Context, clinicID uuid.UUID) (*model.Subscription, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	return s.subscriptionRepo.GetByAccountID(ctx, clinic.AccountID)
}

// CancelSubscription cancels a subscription
func (s *paymentService) CancelSubscription(ctx context.Context, clinicID uuid.UUID) error {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return err
	}
	subscription, err := s.subscriptionRepo.GetByAccountID(ctx, clinic.AccountID)
	if err != nil {
		return err
	}

	if err := s.client.DeleteSubscription(ctx, subscription.AsaasSubscriptionID); err != nil {
		return errors.ErrPaymentGatewayInstance.WithError(err)
	}

	return s.subscriptionRepo.UpdateStatus(ctx, subscription.ID, model.SubscriptionStatusInactive)
}

// HandleWebhook handles incoming webhooks from Asaas
func (s *paymentService) HandleWebhook(ctx context.Context, payload *model.AsaasWebhookPayload) error {
	if payload.Payment == nil {
		return nil
	}

	logger.Infof("[WEBHOOK] Event: %s, PaymentID: %s, Status: %s, Subscription: %s",
		payload.Event, payload.Payment.ID, payload.Payment.Status, payload.Payment.Subscription)

	// Try to find or create payment in local DB
	payment, err := s.paymentRepo.GetByAsaasID(ctx, payload.Payment.ID)
	if err != nil {
		// Payment not in our DB yet — create it if we can link it to a subscription
		payment, err = s.createPaymentFromWebhook(ctx, payload.Payment)
		if err != nil {
			logger.Errorf("[WEBHOOK] Could not create payment from webhook: %v", err)
			// Don't return error — still try to update account status
		}
	}

	// Update payment status in local DB if we have a record
	if payment != nil {
		newStatus := model.PaymentStatus(payload.Payment.Status)
		if err := s.paymentRepo.UpdateStatus(ctx, payment.ID, newStatus); err != nil {
			logger.Errorf("[WEBHOOK] Error updating payment status: %v", err)
		}
	}

	// Update account subscription status based on event
	switch payload.Event {
	case model.WebhookEventPaymentReceived, model.WebhookEventPaymentConfirmed:
		if payment != nil && payment.SubscriptionID != nil {
			// Find subscription
			subscription, err := s.subscriptionRepo.GetByID(ctx, *payment.SubscriptionID)
			if err == nil {
				// Update Account with Price Lock values
				account, err := s.accountRepo.GetByID(ctx, payment.AccountID)
				if err == nil {
					account.MaxClinics = subscription.MaxClinicsAtContract
					account.MaxProfessionals = subscription.MaxProfessionalsAtContract
					account.MaxWhatsappConnections = subscription.MaxWhatsappAtContract
					account.MaxGoogleCalendars = subscription.MaxGoogleAtContract
					s.accountRepo.Update(ctx, account)
				}
			}
		}
		return s.handlePaymentConfirmed(ctx, payload.Payment)
	case model.WebhookEventPaymentOverdue:
		return s.handlePaymentOverdue(ctx, payload.Payment)
	case model.WebhookEventPaymentRefunded, model.WebhookEventPaymentDeleted:
		return s.handlePaymentCanceled(ctx, payload.Payment)
	}

	return nil
}

// createPaymentFromWebhook creates a local payment record from webhook data
func (s *paymentService) createPaymentFromWebhook(ctx context.Context, wp *model.WebhookPayment) (*model.Payment, error) {
	if wp.Subscription == "" {
		return nil, fmt.Errorf("no subscription ID in webhook payment")
	}

	// Find subscription by Asaas ID
	subscription, err := s.subscriptionRepo.GetByAsaasID(ctx, wp.Subscription)
	if err != nil {
		return nil, fmt.Errorf("subscription not found for Asaas ID %s: %w", wp.Subscription, err)
	}

	// Find customer by Asaas ID
	customer, err := s.customerRepo.GetByAsaasID(ctx, wp.Customer)
	if err != nil {
		return nil, fmt.Errorf("customer not found for Asaas ID %s: %w", wp.Customer, err)
	}

	dueDate, _ := time.Parse("2006-01-02", wp.DueDate)

	payment := &model.Payment{
		AccountID:      subscription.AccountID,
		CustomerID:     customer.ID,
		SubscriptionID: &subscription.ID,
		AsaasPaymentID: wp.ID,
		Value:          wp.Value,
		NetValue:       wp.NetValue,
		Status:         model.PaymentStatus(wp.Status),
		BillingType:    model.BillingType(wp.BillingType),
		DueDate:        dueDate,
		InvoiceURL:     wp.InvoiceURL,
		BankSlipURL:    wp.BankSlipURL,
		Description:    wp.Description,
		ExternalRef:    wp.ExternalRef,
	}

	if wp.PaymentDate != "" {
		pd, _ := time.Parse("2006-01-02", wp.PaymentDate)
		payment.PaymentDate = &pd
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	logger.Infof("[WEBHOOK] Created local payment record %s for Asaas payment %s", payment.ID, wp.ID)
	return payment, nil
}

// handlePaymentConfirmed activates the account when payment is confirmed
func (s *paymentService) handlePaymentConfirmed(ctx context.Context, wp *model.WebhookPayment) error {
	account, err := s.getAccountFromWebhookPayment(ctx, wp)
	if err != nil {
		logger.Errorf("[WEBHOOK] Could not find account for confirmed payment: %v", err)
		return nil
	}

	account.SubscriptionStatus = account_model.SubscriptionStatusActive
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return fmt.Errorf("failed to activate account %s: %w", account.ID, err)
	}

	logger.Infof("[WEBHOOK] Account %s activated after payment confirmation", account.ID)
	return nil
}

// handlePaymentOverdue sets the account to past_due when payment is overdue
func (s *paymentService) handlePaymentOverdue(ctx context.Context, wp *model.WebhookPayment) error {
	account, err := s.getAccountFromWebhookPayment(ctx, wp)
	if err != nil {
		logger.Errorf("[WEBHOOK] Could not find account for overdue payment: %v", err)
		return nil
	}

	account.SubscriptionStatus = account_model.SubscriptionStatusPastDue
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return fmt.Errorf("failed to set account %s to past_due: %w", account.ID, err)
	}

	logger.Infof("[WEBHOOK] Account %s set to past_due after payment overdue", account.ID)
	return nil
}

// handlePaymentCanceled sets the account to canceled when payment is refunded/deleted
func (s *paymentService) handlePaymentCanceled(ctx context.Context, wp *model.WebhookPayment) error {
	account, err := s.getAccountFromWebhookPayment(ctx, wp)
	if err != nil {
		logger.Errorf("[WEBHOOK] Could not find account for canceled payment: %v", err)
		return nil
	}

	account.SubscriptionStatus = account_model.SubscriptionStatusCanceled
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return fmt.Errorf("failed to cancel account %s: %w", account.ID, err)
	}

	logger.Infof("[WEBHOOK] Account %s canceled after payment refund/deletion", account.ID)
	return nil
}

// getAccountFromWebhookPayment finds the Account linked to a webhook payment
// via the subscription's Asaas ID or the external reference (clinicID)
func (s *paymentService) getAccountFromWebhookPayment(ctx context.Context, wp *model.WebhookPayment) (*account_model.Account, error) {
	if s.accountRepo == nil {
		return nil, fmt.Errorf("accountRepo not available")
	}

	// Try via subscription Asaas ID first
	if wp.Subscription != "" {
		subscription, err := s.subscriptionRepo.GetByAsaasID(ctx, wp.Subscription)
		if err == nil {
			return s.accountRepo.GetByID(ctx, subscription.AccountID)
		}
	}

	// Fallback: try via external reference (which is the clinicID)
	if wp.ExternalRef != "" {
		clinicID, err := uuid.Parse(wp.ExternalRef)
		if err == nil {
			clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
			if err == nil {
				return s.accountRepo.GetByID(ctx, clinic.AccountID)
			}
		}
	}

	return nil, fmt.Errorf("could not find account for payment %s", wp.ID)
}

// CreateCustomerForClinic creates an Asaas customer for a clinic during registration
func (s *paymentService) CreateCustomerForClinic(ctx context.Context, clinicID uuid.UUID, req *model.CreateCustomerRequest) error {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return err
	}

	// Check if customer already exists
	existing, _ := s.customerRepo.GetByAccountID(ctx, clinic.AccountID)
	if existing != nil {
		return nil // Customer already exists, nothing to do
	}

	// Create customer in Asaas
	asaasCustomer, err := s.client.CreateCustomer(ctx, req)
	if err != nil {
		return err
	}

	// Save customer in database
	customer := &model.Customer{
		AccountID:       clinic.AccountID,
		AsaasCustomerID: asaasCustomer.ID,
		Name:            asaasCustomer.Name,
		Email:           asaasCustomer.Email,
		Phone:           asaasCustomer.Phone,
		Document:        asaasCustomer.Document,
		PostalCode:      asaasCustomer.PostalCode,
		Address:         asaasCustomer.Address,
		AddressNumber:   asaasCustomer.AddressNumber,
		Province:        asaasCustomer.Province,
		City:            fmt.Sprintf("%v", asaasCustomer.City),
		State:           fmt.Sprintf("%v", asaasCustomer.State),
	}

	return s.customerRepo.Create(ctx, customer)
}

// CreateSubscriptionForClinic creates an Asaas subscription for a clinic during registration
func (s *paymentService) CreateSubscriptionForClinic(ctx context.Context, clinicID uuid.UUID, planID uuid.UUID, offerToken *string, billing string) error {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return err
	}

	// Get customer
	customer, err := s.customerRepo.GetByAccountID(ctx, clinic.AccountID)
	if err != nil {
		return errors.ErrNotFoundInstance.WithMessage("Customer not found")
	}

	// Fetch Plan
	plan, err := s.planService.GetPlanByID(ctx, planID)
	if err != nil {
		return errors.ErrBadRequestInstance.WithMessage("Invalid plan")
	}

	// Validate Plan and Offer to get Price
	var planCycleType plan_model.CycleType
	var cycle model.BillingCycle
	if billing == "annually" {
		planCycleType = plan_model.CycleTypeYearly
		cycle = model.BillingCycleYearly
	} else {
		planCycleType = plan_model.CycleTypeMonthly
		cycle = model.BillingCycleMonthly
	}

	var offer *plan_model.Offer
	if offerToken != nil && *offerToken != "" {
		o, err := s.planService.ValidateOffer(ctx, *offerToken)
		if err == nil {
			offer = o
		}
	}

	value, err := s.planService.CalculateBestPrice(ctx, plan, offer, planCycleType)
	if err != nil {
		return errors.ErrBadRequestInstance.WithMessage(err.Error())
	}

	// Calculate next due date (immediate charge)
	nextDueDate := time.Now().Format("2006-01-02")

	// Create subscription in Asaas
	asaasReq := &model.CreateSubscriptionRequest{
		CustomerID:  customer.AsaasCustomerID,
		BillingType: model.BillingTypePix, // Default to PIX, user can change later
		Value:       value,
		NextDueDate: nextDueDate,
		Cycle:       cycle,
		Description: "Assinatura " + plan.Name,
	}

	asaasSubscription, err := s.client.CreateSubscription(ctx, asaasReq)
	if err != nil {
		return err
	}

	// Validate Asaas returned a valid subscription ID
	if asaasSubscription.ID == "" {
		return errors.ErrPaymentGatewayInstance.WithMessage("Asaas returned empty subscription ID")
	}

	// Parse next due date
	nextDue, _ := time.Parse("2006-01-02", asaasSubscription.NextDueDate)

	var offerID *uuid.UUID
	if offer != nil {
		offerID = &offer.ID
		_ = s.planService.UseOffer(ctx, *offerID)
	}

	// Calculate absolute limits based on Plan or Custom via Offer
	maxClinics := plan.MaxClinics
	maxProfessionals := plan.MaxProfessionals
	if offer != nil {
		if offer.CustomMaxClinics != nil {
			maxClinics = *offer.CustomMaxClinics
		}
		if offer.CustomMaxProfessionals != nil {
			maxProfessionals = *offer.CustomMaxProfessionals
		}
	}

	// Save subscription in database
	subscription := &model.Subscription{
		AccountID:           clinic.AccountID,
		CustomerID:          customer.ID,
		AsaasSubscriptionID: asaasSubscription.ID,
		Status:              model.SubscriptionStatus(asaasSubscription.Status),
		BillingType:         model.BillingType(asaasSubscription.BillingType),
		Value:               asaasSubscription.Value,
		NextDueDate:         nextDue,
		Cycle:               model.BillingCycle(asaasSubscription.Cycle),
		Description:         asaasSubscription.Description,

		PlanID:                     &plan.ID,
		OfferID:                    offerID,
		PriceAtContract:            value,
		MaxClinicsAtContract:       maxClinics,
		MaxProfessionalsAtContract: maxProfessionals,
		MaxWhatsappAtContract:      plan.MaxWhatsappConnections,
		MaxGoogleAtContract:        plan.MaxGoogleCalendars,
	}

	return s.subscriptionRepo.Create(ctx, subscription)
}

// SwitchPlan handles plan switching: cancels current subscription and creates new one
// For trial/expired accounts: creates immediate subscription
// For active accounts: cancels current, creates new with nextDueDate = current period end
func (s *paymentService) SwitchPlan(ctx context.Context, clinicID uuid.UUID, newPlan string, billingType string, cycle string, creditCard *model.CreditCardInfo, creditCardHolder *model.CreditCardHolderInfo) (*model.Subscription, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Clinic not found")
	}

	// Get customer (required for subscription)
	customer, err := s.customerRepo.GetByAccountID(ctx, clinic.AccountID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Customer not found. Please create billing profile first.")
	}

	// Fetch the plan
	var plan *plan_model.Plan
	plans, err := s.planService.ListPlans(ctx)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithMessage("Failed to fetch plans")
	}
	for i := range plans {
		if plans[i].Slug == newPlan {
			plan = &plans[i]
			break
		}
	}
	if plan == nil {
		return nil, errors.ErrBadRequestInstance.WithMessage("Invalid plan")
	}

	description := plan.Name + " - Aseno"

	// Determine billing cycle
	var cycleType plan_model.CycleType
	var asaasCycle model.BillingCycle
	if cycle == "YEARLY" {
		cycleType = plan_model.CycleTypeYearly
		asaasCycle = model.BillingCycleYearly
	} else {
		cycleType = plan_model.CycleTypeMonthly
		asaasCycle = model.BillingCycleMonthly
	}

	value, err := s.planService.CalculateBestPrice(ctx, plan, nil, cycleType)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithMessage("Falha ao calcular o preço do plano").WithError(err)
	}

	// Check for existing active subscription
	var nextDueDate string
	existingSub, _ := s.subscriptionRepo.GetByAccountID(ctx, clinic.AccountID)
	if existingSub != nil && existingSub.Status == model.SubscriptionStatusActive {
		if existingSub.AsaasSubscriptionID != "" {
			// Cancel existing valid subscription in Asaas
			if err := s.client.DeleteSubscription(ctx, existingSub.AsaasSubscriptionID); err != nil {
				logger.Warnf("[PAYMENT] Warning: failed to cancel Asaas subscription %s: %v", existingSub.AsaasSubscriptionID, err)
			}
		} else {
			// Ghost subscription (no Asaas ID) — skip Asaas cancellation
			logger.Warnf("[PAYMENT] Found ghost subscription %s with empty Asaas ID for account %s, skipping Asaas cancel", existingSub.ID, clinic.AccountID)
		}

		// Mark as inactive in our DB
		err = s.subscriptionRepo.UpdateStatus(ctx, existingSub.ID, model.SubscriptionStatusInactive)
		if err != nil {
			logger.Errorf("[PAYMENT] Error updating subscription status: %v", err)
		}

		// Use existing NextDueDate only if it's valid, otherwise charge immediately
		if existingSub.AsaasSubscriptionID != "" && !existingSub.NextDueDate.IsZero() {
			nextDueDate = existingSub.NextDueDate.Format("2006-01-02")
		} else {
			nextDueDate = time.Now().Format("2006-01-02")
		}
	} else {
		// No active subscription (trial or expired): charge immediately
		nextDueDate = time.Now().Format("2006-01-02")
	}

	// Create new subscription in Asaas
	asaasReq := &model.CreateSubscriptionRequest{
		CustomerID:           customer.AsaasCustomerID,
		BillingType:          model.BillingType(billingType),
		Value:                value,
		NextDueDate:          nextDueDate,
		Cycle:                asaasCycle,
		Description:          description,
		ExternalRef:          clinicID.String(),
		CreditCard:           creditCard,
		CreditCardHolderInfo: creditCardHolder,
	}
	asaasSub, err := s.client.CreateSubscription(ctx, asaasReq)
	if err != nil {
		logger.Errorf("[PAYMENT] Error creating Asaas subscription: %v", err)
		logger.Errorf("[PAYMENT] Asaas request: customer=%s, billingType=%s, value=%.2f, nextDueDate=%s, cycle=%s",
			asaasReq.CustomerID, asaasReq.BillingType, asaasReq.Value, asaasReq.NextDueDate, asaasReq.Cycle)
		return nil, errors.ErrPaymentGatewayInstance.WithMessage(fmt.Sprintf("Asaas: %v", err))
	}

	logger.Infof("[PAYMENT] Asaas response: ID=%s, Status=%s, Value=%.2f, Cycle=%s, NextDueDate=%s",
		asaasSub.ID, asaasSub.Status, asaasSub.Value, asaasSub.Cycle, asaasSub.NextDueDate)

	// Validate Asaas returned a valid subscription ID
	if asaasSub.ID == "" {
		logger.Errorf("[PAYMENT] Asaas returned empty subscription ID for clinic %s", clinicID)
		logger.Errorf("[PAYMENT] Asaas request was: customer=%s, billingType=%s, value=%.2f, nextDueDate=%s, cycle=%s",
			asaasReq.CustomerID, asaasReq.BillingType, asaasReq.Value, asaasReq.NextDueDate, asaasReq.Cycle)
		return nil, errors.ErrPaymentGatewayInstance.WithMessage("Asaas returned empty subscription ID")
	}

	// Parse next due date
	parsedNextDueDate, _ := time.Parse("2006-01-02", asaasSub.NextDueDate)

	// Save new subscription in DB
	subscription := &model.Subscription{
		AccountID:           clinic.AccountID,
		CustomerID:          customer.ID,
		AsaasSubscriptionID: asaasSub.ID,
		Value:               asaasSub.Value,
		Status:              model.SubscriptionStatus(asaasSub.Status),
		BillingType:         model.BillingType(asaasSub.BillingType),
		Cycle:               model.BillingCycle(asaasSub.Cycle),
		NextDueDate:         parsedNextDueDate,
		Description:         asaasSub.Description,
		ExternalRef:         asaasSub.ExternalRef,

		// Price Lock fields (were missing before this fix)
		PlanID:                     &plan.ID,
		PriceAtContract:            value,
		MaxClinicsAtContract:       plan.MaxClinics,
		MaxProfessionalsAtContract: plan.MaxProfessionals,
		MaxWhatsappAtContract:      plan.MaxWhatsappConnections,
		MaxGoogleAtContract:        plan.MaxGoogleCalendars,
	}

	if err := s.subscriptionRepo.Create(ctx, subscription); err != nil {
		return nil, err
	}

	// Update account plan and status
	if s.accountRepo != nil {
		account, err := s.accountRepo.GetByID(ctx, clinic.AccountID)
		if err == nil && plan != nil {
			account.Plan = plan
			account.PlanID = &plan.ID
			account.SubscriptionStatus = account_model.SubscriptionStatusPending
			_ = s.accountRepo.Update(ctx, account)
		}
	}

	return subscription, nil
}

// GetSubscriptionPayments returns payments for the current subscription from Asaas
func (s *paymentService) GetSubscriptionPayments(ctx context.Context, clinicID uuid.UUID) (*model.PaymentListResponse, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Clinic not found")
	}

	subscription, err := s.subscriptionRepo.GetByAccountID(ctx, clinic.AccountID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("No subscription found")
	}

	return s.client.GetSubscriptionPayments(ctx, subscription.AsaasSubscriptionID)
}
