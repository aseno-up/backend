package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
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
	GetPayment(ctx context.Context, clinicID uuid.UUID, paymentID uuid.UUID) (*model.Payment, error)
	GetPayments(ctx context.Context, clinicID uuid.UUID, page, perPage int) ([]*model.Payment, int64, error)
	GetPixQRCode(ctx context.Context, clinicID uuid.UUID, paymentID uuid.UUID) (*model.AsaasPixQRCodeResponse, error)
	CancelPayment(ctx context.Context, clinicID uuid.UUID, paymentID uuid.UUID) error

	// Subscription operations
	CreateSubscription(ctx context.Context, clinicID uuid.UUID, req *CreateSubscriptionInput) (*model.Subscription, error)
	GetSubscription(ctx context.Context, clinicID uuid.UUID) (*model.Subscription, error)
	CancelSubscription(ctx context.Context, clinicID uuid.UUID) error

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
	paymentRepo PaymentRepository,
	subscriptionRepo SubscriptionRepository,
	clinicRepo clinic_repository.ClinicRepository,
	accountRepo account_repository.AccountRepository,
	planService plan_service.PlanService,
) PaymentService {
	return &paymentService{
		client:           client,
		customerRepo:     customerRepo,
		paymentRepo:      paymentRepo,
		subscriptionRepo: subscriptionRepo,
		clinicRepo:       clinicRepo,
		accountRepo:      accountRepo,
		planService:      planService,
	}
}

// CreateOrGetCustomer creates a new customer or returns existing one
func (s *paymentService) CreateOrGetCustomer(ctx context.Context, clinicID uuid.UUID, req *CreateCustomerInput) (*model.Customer, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Clinic not found")
	}

	// Check if customer already exists for this clinic
	existing, err := s.customerRepo.GetByAccountID(ctx, clinic.AccountID)
	if err == nil && existing != nil {
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
		return nil, errors.ErrPaymentGatewayInstance.WithError(err)
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
		Province:        req.Province,
		City:            req.City,
		State:           req.State,
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

// GetPayment retrieves a payment
func (s *paymentService) GetPayment(ctx context.Context, clinicID uuid.UUID, paymentID uuid.UUID) (*model.Payment, error) {
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	// Verify clinic ownership
	if payment.AccountID != clinic.AccountID {
		return nil, errors.ErrForbiddenInstance
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
func (s *paymentService) GetPixQRCode(ctx context.Context, clinicID uuid.UUID, paymentID uuid.UUID) (*model.AsaasPixQRCodeResponse, error) {
	payment, err := s.GetPayment(ctx, clinicID, paymentID)
	if err != nil {
		return nil, err
	}

	return s.client.GetPixQRCode(ctx, payment.AsaasPaymentID)
}

// CancelPayment cancels a payment
func (s *paymentService) CancelPayment(ctx context.Context, clinicID uuid.UUID, paymentID uuid.UUID) error {
	payment, err := s.GetPayment(ctx, clinicID, paymentID)
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

	// Find payment by Asaas ID
	payment, err := s.paymentRepo.GetByAsaasID(ctx, payload.Payment.ID)
	if err != nil {
		// Payment not found in our database, might be from another source
		return nil
	}

	// Update payment status
	newStatus := model.PaymentStatus(payload.Payment.Status)
	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID, newStatus); err != nil {
		return err
	}

	// Handle specific events
	switch payload.Event {
	case model.WebhookEventPaymentReceived, model.WebhookEventPaymentConfirmed:
		if payment.SubscriptionID != nil {
			// Find subscription
			subscription, err := s.subscriptionRepo.GetByID(ctx, *payment.SubscriptionID)
			if err != nil {
				return err
			}

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
	case model.WebhookEventPaymentOverdue:
		// Payment is overdue - could notify the clinic
	}

	return nil
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
		City:            asaasCustomer.City,
		State:           asaasCustomer.State,
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
