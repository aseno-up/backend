package model

// AsaasWebhookPayload represents a webhook payload from Asaas
type AsaasWebhookPayload struct {
	Event   WebhookEvent   `json:"event"`
	Payment *WebhookPayment `json:"payment,omitempty"`
}

// WebhookEvent represents the type of webhook event
type WebhookEvent string

const (
	// Payment events
	WebhookEventPaymentCreated             WebhookEvent = "PAYMENT_CREATED"
	WebhookEventPaymentUpdated             WebhookEvent = "PAYMENT_UPDATED"
	WebhookEventPaymentConfirmed           WebhookEvent = "PAYMENT_CONFIRMED"
	WebhookEventPaymentReceived            WebhookEvent = "PAYMENT_RECEIVED"
	WebhookEventPaymentOverdue             WebhookEvent = "PAYMENT_OVERDUE"
	WebhookEventPaymentDeleted             WebhookEvent = "PAYMENT_DELETED"
	WebhookEventPaymentRestored            WebhookEvent = "PAYMENT_RESTORED"
	WebhookEventPaymentRefunded            WebhookEvent = "PAYMENT_REFUNDED"
	WebhookEventPaymentReceivedInCash      WebhookEvent = "PAYMENT_RECEIVED_IN_CASH_UNDONE"
	WebhookEventPaymentChargebackRequested WebhookEvent = "PAYMENT_CHARGEBACK_REQUESTED"
	WebhookEventPaymentChargebackDispute   WebhookEvent = "PAYMENT_CHARGEBACK_DISPUTE"
	WebhookEventPaymentAwaitingChargeback  WebhookEvent = "PAYMENT_AWAITING_CHARGEBACK_REVERSAL"
	WebhookEventPaymentDunningReceived     WebhookEvent = "PAYMENT_DUNNING_RECEIVED"
	WebhookEventPaymentDunningRequested    WebhookEvent = "PAYMENT_DUNNING_REQUESTED"
	WebhookEventPaymentBankSlipViewed      WebhookEvent = "PAYMENT_BANK_SLIP_VIEWED"
	WebhookEventPaymentCheckoutViewed      WebhookEvent = "PAYMENT_CHECKOUT_VIEWED"
)

// WebhookPayment represents payment data in a webhook
type WebhookPayment struct {
	ID                string  `json:"id"`
	Customer          string  `json:"customer"`
	Subscription      string  `json:"subscription,omitempty"`
	Value             float64 `json:"value"`
	NetValue          float64 `json:"netValue"`
	Status            string  `json:"status"`
	BillingType       string  `json:"billingType"`
	DueDate           string  `json:"dueDate"`
	PaymentDate       string  `json:"paymentDate,omitempty"`
	ClientPaymentDate string  `json:"clientPaymentDate,omitempty"`
	InvoiceURL        string  `json:"invoiceUrl"`
	BankSlipURL       string  `json:"bankSlipUrl"`
	Description       string  `json:"description"`
	ExternalRef       string  `json:"externalReference"`
	DateCreated       string  `json:"dateCreated"`
}
