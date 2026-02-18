package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/payment/model"
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
	GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.Customer, error)
	GetByAsaasID(ctx context.Context, asaasID string) (*model.Customer, error)
	Update(ctx context.Context, customer *model.Customer) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PaymentRepository defines the interface for payment persistence
type PaymentRepository interface {
	Create(ctx context.Context, payment *model.Payment) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Payment, error)
	GetByAsaasID(ctx context.Context, asaasID string) (*model.Payment, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*model.Payment, int64, error)
	GetByCustomerID(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*model.Payment, int64, error)
	Update(ctx context.Context, payment *model.Payment) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.PaymentStatus) error
}

// SubscriptionRepository defines the interface for subscription persistence
type SubscriptionRepository interface {
	Create(ctx context.Context, subscription *model.Subscription) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Subscription, error)
	GetByAsaasID(ctx context.Context, asaasID string) (*model.Subscription, error)
	GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.Subscription, error)
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
	CreateSubscriptionForClinic(ctx context.Context, clinicID uuid.UUID, plan string, billing string) error

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
	Value       float64
	Cycle       model.BillingCycle
	Description string
}

// paymentService implements PaymentService
type paymentService struct {
	client           PaymentClient
	customerRepo     CustomerRepository
	paymentRepo      PaymentRepository
	subscriptionRepo SubscriptionRepository
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	client PaymentClient,
	customerRepo CustomerRepository,
	paymentRepo PaymentRepository,
	subscriptionRepo SubscriptionRepository,
) PaymentService {
	return &paymentService{
		client:           client,
		customerRepo:     customerRepo,
		paymentRepo:      paymentRepo,
		subscriptionRepo: subscriptionRepo,
	}
}

// CreateOrGetCustomer creates a new customer or returns existing one
func (s *paymentService) CreateOrGetCustomer(ctx context.Context, clinicID uuid.UUID, req *CreateCustomerInput) (*model.Customer, error) {
	// Check if customer already exists for this clinic
	existing, err := s.customerRepo.GetByClinicID(ctx, clinicID)
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
		ClinicID:        clinicID,
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
	return s.customerRepo.GetByClinicID(ctx, clinicID)
}

// CreatePayment creates a new payment
func (s *paymentService) CreatePayment(ctx context.Context, clinicID uuid.UUID, req *CreatePaymentInput) (*model.Payment, error) {
	// Get customer for clinic
	customer, err := s.customerRepo.GetByClinicID(ctx, clinicID)
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
		ClinicID:       clinicID,
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
	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	// Verify clinic ownership
	if payment.ClinicID != clinicID {
		return nil, errors.ErrForbiddenInstance
	}

	return payment, nil
}

// GetPayments retrieves payments for a clinic
func (s *paymentService) GetPayments(ctx context.Context, clinicID uuid.UUID, page, perPage int) ([]*model.Payment, int64, error) {
	offset := (page - 1) * perPage
	return s.paymentRepo.GetByClinicID(ctx, clinicID, perPage, offset)
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
	// Get customer for clinic
	customer, err := s.customerRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil, errors.ErrNotFoundInstance.WithMessage("Customer not found. Please create billing profile first.")
	}

	// Check if subscription already exists
	existing, _ := s.subscriptionRepo.GetByClinicID(ctx, clinicID)
	if existing != nil && existing.Status == model.SubscriptionStatusActive {
		return nil, errors.ErrConflictInstance.WithMessage("Clinic already has an active subscription")
	}

	// Create subscription in Asaas
	nextDueDate := time.Now().AddDate(0, 0, 1) // Tomorrow
	asaasReq := &model.CreateSubscriptionRequest{
		CustomerID:  customer.AsaasCustomerID,
		BillingType: req.BillingType,
		Value:       req.Value,
		NextDueDate: nextDueDate.Format("2006-01-02"),
		Cycle:       req.Cycle,
		Description: req.Description,
		ExternalRef: clinicID.String(),
	}

	asaasSub, err := s.client.CreateSubscription(ctx, asaasReq)
	if err != nil {
		return nil, errors.ErrPaymentGatewayInstance.WithError(err)
	}

	// Parse next due date
	parsedNextDueDate, _ := time.Parse("2006-01-02", asaasSub.NextDueDate)

	// Save subscription to database
	subscription := &model.Subscription{
		ClinicID:            clinicID,
		CustomerID:          customer.ID,
		AsaasSubscriptionID: asaasSub.ID,
		Value:               asaasSub.Value,
		Status:              model.SubscriptionStatus(asaasSub.Status),
		BillingType:         model.BillingType(asaasSub.BillingType),
		Cycle:               model.BillingCycle(asaasSub.Cycle),
		NextDueDate:         parsedNextDueDate,
		Description:         asaasSub.Description,
		ExternalRef:         asaasSub.ExternalRef,
	}

	if err := s.subscriptionRepo.Create(ctx, subscription); err != nil {
		return nil, err
	}

	return subscription, nil
}

// GetSubscription retrieves the subscription for a clinic
func (s *paymentService) GetSubscription(ctx context.Context, clinicID uuid.UUID) (*model.Subscription, error) {
	return s.subscriptionRepo.GetByClinicID(ctx, clinicID)
}

// CancelSubscription cancels a subscription
func (s *paymentService) CancelSubscription(ctx context.Context, clinicID uuid.UUID) error {
	subscription, err := s.subscriptionRepo.GetByClinicID(ctx, clinicID)
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
		// Payment was received - could update clinic subscription status here
		// This would require injecting a clinic service/repository
	case model.WebhookEventPaymentOverdue:
		// Payment is overdue - could notify the clinic
	}

	return nil
}

// CreateCustomerForClinic creates an Asaas customer for a clinic during registration
func (s *paymentService) CreateCustomerForClinic(ctx context.Context, clinicID uuid.UUID, req *model.CreateCustomerRequest) error {
	// Check if customer already exists
	existing, _ := s.customerRepo.GetByClinicID(ctx, clinicID)
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
		ClinicID:        clinicID,
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
func (s *paymentService) CreateSubscriptionForClinic(ctx context.Context, clinicID uuid.UUID, plan string, billing string) error {
	// Get customer
	customer, err := s.customerRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return errors.ErrNotFoundInstance.WithMessage("Customer not found")
	}

	// Map plan to price (you should get this from config/database)
	var value float64
	switch plan {
	case "basic":
		value = 99.00
	case "professional":
		value = 199.00
	case "advanced":
		value = 349.00
	default:
		return errors.ErrBadRequestInstance.WithMessage("Invalid plan")
	}

	// Adjust for billing cycle
	var cycle model.BillingCycle
	if billing == "annually" {
		cycle = model.BillingCycleYearly
		value = value * 10 // 2 months free on annual billing
	} else {
		cycle = model.BillingCycleMonthly
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
		Description: "Assinatura " + plan,
	}

	asaasSubscription, err := s.client.CreateSubscription(ctx, asaasReq)
	if err != nil {
		return err
	}

	// Parse next due date
	nextDue, _ := time.Parse("2006-01-02", asaasSubscription.NextDueDate)

	// Save subscription in database
	subscription := &model.Subscription{
		ClinicID:            clinicID,
		CustomerID:          customer.ID,
		AsaasSubscriptionID: asaasSubscription.ID,
		Status:              model.SubscriptionStatus(asaasSubscription.Status),
		BillingType:         model.BillingType(asaasSubscription.BillingType),
		Value:               asaasSubscription.Value,
		NextDueDate:         nextDue,
		Cycle:               model.BillingCycle(asaasSubscription.Cycle),
		Description:         asaasSubscription.Description,
	}

	return s.subscriptionRepo.Create(ctx, subscription)
}
