package asaas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saas/backend/pkg/payment/model"
)

// Config holds the Asaas client configuration
type Config struct {
	BaseURL   string
	APIKey    string
	IsSandbox bool
}

// Client implements the Asaas API client
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient creates a new Asaas client
func NewClient(config Config) *Client {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request to the Asaas API
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := c.config.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("access_token", c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiError AsaasError
		if err := json.Unmarshal(respBody, &apiError); err == nil && len(apiError.Errors) > 0 {
			return nil, fmt.Errorf("asaas API error: %s - %s", apiError.Errors[0].Code, apiError.Errors[0].Description)
		}
		return nil, fmt.Errorf("asaas API error: status %d - %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// AsaasError represents an error response from Asaas
type AsaasError struct {
	Errors []struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"errors"`
}

// Customer methods

// CreateCustomer creates a new customer in Asaas
func (c *Client) CreateCustomer(ctx context.Context, req *model.CreateCustomerRequest) (*model.AsaasCustomerResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/customers", req)
	if err != nil {
		return nil, err
	}

	var customer model.AsaasCustomerResponse
	if err := json.Unmarshal(respBody, &customer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal customer response: %w", err)
	}

	return &customer, nil
}

// GetCustomer retrieves a customer by ID
func (c *Client) GetCustomer(ctx context.Context, customerID string) (*model.AsaasCustomerResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/customers/"+customerID, nil)
	if err != nil {
		return nil, err
	}

	var customer model.AsaasCustomerResponse
	if err := json.Unmarshal(respBody, &customer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal customer response: %w", err)
	}

	return &customer, nil
}

// UpdateCustomer updates a customer
func (c *Client) UpdateCustomer(ctx context.Context, customerID string, req *model.CreateCustomerRequest) (*model.AsaasCustomerResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodPut, "/customers/"+customerID, req)
	if err != nil {
		return nil, err
	}

	var customer model.AsaasCustomerResponse
	if err := json.Unmarshal(respBody, &customer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal customer response: %w", err)
	}

	return &customer, nil
}

// DeleteCustomer deletes a customer
func (c *Client) DeleteCustomer(ctx context.Context, customerID string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/customers/"+customerID, nil)
	return err
}

// Payment methods

// CreatePayment creates a new payment in Asaas
func (c *Client) CreatePayment(ctx context.Context, req *model.CreatePaymentRequest) (*model.AsaasPaymentResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/payments", req)
	if err != nil {
		return nil, err
	}

	var payment model.AsaasPaymentResponse
	if err := json.Unmarshal(respBody, &payment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payment response: %w", err)
	}

	return &payment, nil
}

// GetPayment retrieves a payment by ID
func (c *Client) GetPayment(ctx context.Context, paymentID string) (*model.AsaasPaymentResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/payments/"+paymentID, nil)
	if err != nil {
		return nil, err
	}

	var payment model.AsaasPaymentResponse
	if err := json.Unmarshal(respBody, &payment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payment response: %w", err)
	}

	return &payment, nil
}

// GetPaymentsByCustomer retrieves payments for a customer
func (c *Client) GetPaymentsByCustomer(ctx context.Context, customerID string, limit, offset int) (*model.PaymentListResponse, error) {
	endpoint := fmt.Sprintf("/payments?customer=%s&limit=%d&offset=%d", customerID, limit, offset)
	respBody, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var response model.PaymentListResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payments response: %w", err)
	}

	return &response, nil
}

// DeletePayment deletes/cancels a payment
func (c *Client) DeletePayment(ctx context.Context, paymentID string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/payments/"+paymentID, nil)
	return err
}

// GetPixQRCode retrieves the PIX QR code for a payment
func (c *Client) GetPixQRCode(ctx context.Context, paymentID string) (*model.AsaasPixQRCodeResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/payments/"+paymentID+"/pixQrCode", nil)
	if err != nil {
		return nil, err
	}

	var qrCode model.AsaasPixQRCodeResponse
	if err := json.Unmarshal(respBody, &qrCode); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PIX QR code response: %w", err)
	}

	return &qrCode, nil
}

// Subscription methods

// CreateSubscription creates a new subscription in Asaas
func (c *Client) CreateSubscription(ctx context.Context, req *model.CreateSubscriptionRequest) (*model.AsaasSubscriptionResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/subscriptions", req)
	if err != nil {
		return nil, err
	}

	var subscription model.AsaasSubscriptionResponse
	if err := json.Unmarshal(respBody, &subscription); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subscription response: %w", err)
	}

	return &subscription, nil
}

// GetSubscription retrieves a subscription by ID
func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (*model.AsaasSubscriptionResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/subscriptions/"+subscriptionID, nil)
	if err != nil {
		return nil, err
	}

	var subscription model.AsaasSubscriptionResponse
	if err := json.Unmarshal(respBody, &subscription); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subscription response: %w", err)
	}

	return &subscription, nil
}

// GetSubscriptionsByCustomer retrieves subscriptions for a customer
func (c *Client) GetSubscriptionsByCustomer(ctx context.Context, customerID string) (*model.SubscriptionListResponse, error) {
	endpoint := fmt.Sprintf("/subscriptions?customer=%s", customerID)
	respBody, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var response model.SubscriptionListResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subscriptions response: %w", err)
	}

	return &response, nil
}

// UpdateSubscription updates a subscription
func (c *Client) UpdateSubscription(ctx context.Context, subscriptionID string, req *model.CreateSubscriptionRequest) (*model.AsaasSubscriptionResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodPut, "/subscriptions/"+subscriptionID, req)
	if err != nil {
		return nil, err
	}

	var subscription model.AsaasSubscriptionResponse
	if err := json.Unmarshal(respBody, &subscription); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subscription response: %w", err)
	}

	return &subscription, nil
}

// DeleteSubscription cancels a subscription
func (c *Client) DeleteSubscription(ctx context.Context, subscriptionID string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/subscriptions/"+subscriptionID, nil)
	return err
}

// GetSubscriptionPayments retrieves payments for a subscription
func (c *Client) GetSubscriptionPayments(ctx context.Context, subscriptionID string) (*model.PaymentListResponse, error) {
	endpoint := fmt.Sprintf("/subscriptions/%s/payments", subscriptionID)
	respBody, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var response model.PaymentListResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payments response: %w", err)
	}

	return &response, nil
}
