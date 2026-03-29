package model

import (
	payment_model "github.com/aseno/backend/pkg/payment/model"
)

// CreateSubaccountRequest represents a request to create an Asaas subaccount
type CreateSubaccountRequest struct {
	Name          string  `json:"name" binding:"required"`
	Email         string  `json:"email" binding:"required,email"`
	CpfCnpj       string  `json:"cpfCnpj" binding:"required"`
	CompanyType   string  `json:"companyType,omitempty"` // MEI, LIMITED, INDIVIDUAL, ASSOCIATION
	BirthDate     string  `json:"birthDate,omitempty"`    // YYYY-MM-DD, required for PF
	IncomeValue   float64 `json:"incomeValue"`           // Required by Asaas
	Phone         string  `json:"phone,omitempty"`
	MobilePhone   string  `json:"mobilePhone,omitempty"`
	PostalCode    string  `json:"postalCode,omitempty"`
	Address       string  `json:"address,omitempty"`
	AddressNumber string  `json:"addressNumber,omitempty"`
	Complement    string  `json:"complement,omitempty"`
	Province      string  `json:"province,omitempty"`
	City          string  `json:"city,omitempty"`
	State         string  `json:"state,omitempty"`
	Webhooks      string  `json:"webhooks,omitempty"` // Webhook URL for the subaccount
}

// AsaasSubaccountResponse represents the response from Asaas when creating/getting a subaccount
type AsaasSubaccountResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	CpfCnpj     string `json:"cpfCnpj"`
	CompanyType string `json:"companyType"`
	Phone       string `json:"phone"`
	MobilePhone string `json:"mobilePhone"`
	PostalCode  string `json:"postalCode"`
	Address     string `json:"address"`
	Province    string `json:"province"`
	City        string `json:"city"`
	State       string `json:"state"`
	WalletID    string `json:"walletId"`
	APIKey      string `json:"apiKey"`
	AccountNumber *AccountNumber `json:"accountNumber,omitempty"`
}

type AccountNumber struct {
	Agency  string `json:"agency"`
	Account string `json:"account"`
}

// SplitItem represents a single split entry for a payment
type SplitItem struct {
	WalletID        string  `json:"walletId"`
	FixedValue      float64 `json:"fixedValue,omitempty"`
	PercentualValue float64 `json:"percentualValue,omitempty"`
}

// CreatePaymentWithSplitRequest extends CreatePaymentRequest with split and credit card support
type CreatePaymentWithSplitRequest struct {
	CustomerID           string                          `json:"customer" binding:"required"`
	BillingType          string                          `json:"billingType" binding:"required"`
	Value                float64                         `json:"value" binding:"required"`
	DueDate              string                          `json:"dueDate" binding:"required"`
	PixExpirationDate    string                          `json:"pixExpirationDate,omitempty"` // ISO datetime for PIX expiration
	Description          string                          `json:"description,omitempty"`
	ExternalRef          string                          `json:"externalReference,omitempty"`
	Split                []SplitItem                     `json:"split,omitempty"`
	CreditCard           *payment_model.CreditCardInfo       `json:"creditCard,omitempty"`
	CreditCardHolderInfo *payment_model.CreditCardHolderInfo `json:"creditCardHolderInfo,omitempty"`
	InstallmentCount     *int                            `json:"installmentCount,omitempty"`
	PostalService        bool                            `json:"postalService,omitempty"`
}

// BalanceResponse represents the response from Asaas finance/balance endpoint
type BalanceResponse struct {
	Balance       float64 `json:"balance"`
	TotalPending  float64 `json:"totalPending,omitempty"`
	TotalReceived float64 `json:"totalReceived,omitempty"`
}

// FinancialSummaryResponse contains aggregated financial data for a clinic
type FinancialSummaryResponse struct {
	TotalReceived    float64            `json:"total_received"`
	TotalPending     float64            `json:"total_pending"`
	TotalFees        float64            `json:"total_fees"`
	NetReceived      float64            `json:"net_received"`
	PaymentsByMethod map[string]float64 `json:"payments_by_method"`
	PaymentCount     int64              `json:"payment_count"`
}

// PaymentFilters contains filters for querying clinic payments
type PaymentFilters struct {
	Status      string `form:"status"`
	BillingType string `form:"billing_type"`
	DateFrom    string `form:"date_from"`
	DateTo      string `form:"date_to"`
	Page        int    `form:"page"`
	PerPage     int    `form:"per_page"`
}

// PaymentPageData is returned by the public /pagar/:token endpoint
type PaymentPageData struct {
	AppointmentID    string  `json:"appointment_id"`
	ClinicID         string  `json:"clinic_id"`
	ClinicName       string  `json:"clinic_name,omitempty"`
	Professional     string  `json:"professional,omitempty"`
	StartTime        string  `json:"start_time"`
	EndTime          string  `json:"end_time"`
	Price            float64 `json:"price"`
	PaymentStatus    string  `json:"payment_status"`
	Status           string  `json:"status"`
	RemainingSeconds int     `json:"remaining_seconds"`
	HasSubaccount    bool    `json:"has_subaccount"`
	ExistingPayment  *ClinicPaymentResponse `json:"existing_payment,omitempty"`
}

// ClinicPaymentConfig is returned by the public endpoint to check payment capabilities
type ClinicPaymentConfig struct {
	HasSubaccount         bool   `json:"has_subaccount"`
	RequirePayment        bool   `json:"require_payment"`
	PaymentTimeoutMinutes int    `json:"payment_timeout_minutes,omitempty"`
	Status                string `json:"status,omitempty"`
}

// SubaccountResponse is the API response for a subaccount
type SubaccountResponse struct {
	ID             string           `json:"id"`
	ClinicID       string           `json:"clinic_id"`
	AsaasAccountID string           `json:"asaas_account_id"`
	WalletID       string           `json:"wallet_id"`
	Status         SubaccountStatus `json:"status"`
	CompanyName    string           `json:"company_name"`
	CpfCnpj        string           `json:"cpf_cnpj"`
	Email          string           `json:"email"`
	CreatedAt      string           `json:"created_at"`
}

// SplitConfigResponse is the API response for a split config
type SplitConfigResponse struct {
	ID       string  `json:"id"`
	ClinicID string  `json:"clinic_id"`
	FeeType  FeeType `json:"fee_type"`
	FeeValue float64 `json:"fee_value"`
	Active   bool    `json:"active"`
}
