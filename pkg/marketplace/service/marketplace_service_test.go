package service

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/internal/crypto"
	"github.com/aseno/backend/pkg/marketplace/model"
	payment_model "github.com/aseno/backend/pkg/payment/model"
)

// Test encryption key (32 bytes hex)
func testEncryptionKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return hex.EncodeToString(key)
}

// --- Mocks ---

type mockSubaccountRepo struct {
	subaccounts map[uuid.UUID]*model.Subaccount
}

func newMockSubaccountRepo() *mockSubaccountRepo {
	return &mockSubaccountRepo{subaccounts: make(map[uuid.UUID]*model.Subaccount)}
}

func (m *mockSubaccountRepo) Create(_ context.Context, sub *model.Subaccount) error {
	sub.ID = uuid.New()
	sub.CreatedAt = time.Now()
	m.subaccounts[sub.ClinicID] = sub
	return nil
}

func (m *mockSubaccountRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Subaccount, error) {
	for _, s := range m.subaccounts {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, &notFoundError{}
}

func (m *mockSubaccountRepo) GetByClinicID(_ context.Context, clinicID uuid.UUID) (*model.Subaccount, error) {
	if s, ok := m.subaccounts[clinicID]; ok {
		return s, nil
	}
	return nil, &notFoundError{}
}

func (m *mockSubaccountRepo) GetByAsaasID(_ context.Context, asaasID string) (*model.Subaccount, error) {
	for _, s := range m.subaccounts {
		if s.AsaasAccountID == asaasID {
			return s, nil
		}
	}
	return nil, &notFoundError{}
}

func (m *mockSubaccountRepo) Update(_ context.Context, sub *model.Subaccount) error {
	m.subaccounts[sub.ClinicID] = sub
	return nil
}

func (m *mockSubaccountRepo) UpdateStatus(_ context.Context, id uuid.UUID, status model.SubaccountStatus) error {
	for _, s := range m.subaccounts {
		if s.ID == id {
			s.Status = status
			return nil
		}
	}
	return nil
}

type mockSplitConfigRepo struct {
	configs map[uuid.UUID]*model.SplitConfig
}

func newMockSplitConfigRepo() *mockSplitConfigRepo {
	return &mockSplitConfigRepo{configs: make(map[uuid.UUID]*model.SplitConfig)}
}

func (m *mockSplitConfigRepo) Create(_ context.Context, config *model.SplitConfig) error {
	config.ID = uuid.New()
	m.configs[config.ClinicID] = config
	return nil
}

func (m *mockSplitConfigRepo) GetByClinicID(_ context.Context, clinicID uuid.UUID) (*model.SplitConfig, error) {
	if c, ok := m.configs[clinicID]; ok {
		return c, nil
	}
	return nil, &notFoundError{}
}

func (m *mockSplitConfigRepo) Update(_ context.Context, config *model.SplitConfig) error {
	m.configs[config.ClinicID] = config
	return nil
}

type mockClinicPaymentRepo struct {
	payments  map[uuid.UUID]*model.ClinicPayment
	byAsaasID map[string]*model.ClinicPayment
}

func newMockClinicPaymentRepo() *mockClinicPaymentRepo {
	return &mockClinicPaymentRepo{
		payments:  make(map[uuid.UUID]*model.ClinicPayment),
		byAsaasID: make(map[string]*model.ClinicPayment),
	}
}

func (m *mockClinicPaymentRepo) Create(_ context.Context, p *model.ClinicPayment) error {
	p.ID = uuid.New()
	m.payments[p.ID] = p
	m.byAsaasID[p.AsaasPaymentID] = p
	return nil
}

func (m *mockClinicPaymentRepo) GetByID(_ context.Context, id uuid.UUID) (*model.ClinicPayment, error) {
	if p, ok := m.payments[id]; ok {
		return p, nil
	}
	return nil, &notFoundError{}
}

func (m *mockClinicPaymentRepo) GetByAsaasID(_ context.Context, asaasID string) (*model.ClinicPayment, error) {
	if p, ok := m.byAsaasID[asaasID]; ok {
		return p, nil
	}
	return nil, &notFoundError{}
}

func (m *mockClinicPaymentRepo) GetByClinicID(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.ClinicPayment, int64, error) {
	return nil, 0, nil
}

func (m *mockClinicPaymentRepo) GetByAppointmentID(_ context.Context, aptID uuid.UUID) (*model.ClinicPayment, error) {
	for _, p := range m.payments {
		if p.AppointmentID != nil && *p.AppointmentID == aptID {
			return p, nil
		}
	}
	return nil, &notFoundError{}
}

func (m *mockClinicPaymentRepo) ListWithFilters(_ context.Context, _ uuid.UUID, _ model.PaymentFilters) ([]*model.ClinicPayment, int64, error) {
	return nil, 0, nil
}

func (m *mockClinicPaymentRepo) Update(_ context.Context, p *model.ClinicPayment) error {
	m.payments[p.ID] = p
	m.byAsaasID[p.AsaasPaymentID] = p
	return nil
}

func (m *mockClinicPaymentRepo) UpdateStatus(_ context.Context, id uuid.UUID, status string) error {
	if p, ok := m.payments[id]; ok {
		p.Status = status
	}
	return nil
}

func (m *mockClinicPaymentRepo) GetSummary(_ context.Context, _ uuid.UUID, _, _ time.Time) (*model.FinancialSummaryResponse, error) {
	return &model.FinancialSummaryResponse{PaymentsByMethod: make(map[string]float64)}, nil
}

type mockAsaasClient struct {
	lastPaymentReq *model.CreatePaymentWithSplitRequest
}

func (m *mockAsaasClient) CreateCustomerWithKey(_ context.Context, _ string, req *payment_model.CreateCustomerRequest) (*payment_model.AsaasCustomerResponse, error) {
	return &payment_model.AsaasCustomerResponse{ID: "cus_test_mock"}, nil
}

func (m *mockAsaasClient) CreateSubaccount(_ context.Context, req *model.CreateSubaccountRequest) (*model.AsaasSubaccountResponse, error) {
	return &model.AsaasSubaccountResponse{
		ID:       "acc_test123",
		WalletID: "wal_test456",
		APIKey:   "sk_test_subaccount_key",
	}, nil
}

func (m *mockAsaasClient) GetSubaccount(_ context.Context, _ string) (*model.AsaasSubaccountResponse, error) {
	return &model.AsaasSubaccountResponse{ID: "acc_test123"}, nil
}

func (m *mockAsaasClient) CreatePaymentWithKey(_ context.Context, _ string, req *model.CreatePaymentWithSplitRequest) (*payment_model.AsaasPaymentResponse, error) {
	m.lastPaymentReq = req
	return &payment_model.AsaasPaymentResponse{
		ID:          "pay_test789",
		Value:       req.Value,
		NetValue:    req.Value * 0.97,
		Status:      "PENDING",
		BillingType: req.BillingType,
	}, nil
}

func (m *mockAsaasClient) GetPixQRCodeWithKey(_ context.Context, _, _ string) (*payment_model.AsaasPixQRCodeResponse, error) {
	return &payment_model.AsaasPixQRCodeResponse{
		EncodedImage: "base64image",
		Payload:      "pix-copy-paste-code",
	}, nil
}

func (m *mockAsaasClient) GetPaymentWithKey(_ context.Context, _, _ string) (*payment_model.AsaasPaymentResponse, error) {
	return &payment_model.AsaasPaymentResponse{ID: "pay_test789", Status: "RECEIVED"}, nil
}

func (m *mockAsaasClient) GetBalanceWithKey(_ context.Context, _ string) (*model.BalanceResponse, error) {
	return &model.BalanceResponse{Balance: 1000.00}, nil
}

type mockClinicRepo struct{}

func (m *mockClinicRepo) GetByID(_ context.Context, id uuid.UUID) (ClinicInfo, error) {
	return &mockClinicInfo{id: id, accountID: uuid.New()}, nil
}

type mockClinicInfo struct {
	id        uuid.UUID
	accountID uuid.UUID
}

func (c *mockClinicInfo) GetID() uuid.UUID                    { return c.id }
func (c *mockClinicInfo) GetAccountID() uuid.UUID              { return c.accountID }
func (c *mockClinicInfo) GetRequirePaymentBeforeBooking() bool { return false }
func (c *mockClinicInfo) GetPlanSlug() string                  { return "professional" }
func (c *mockClinicInfo) GetSubscriptionStatus() string        { return "active" }

type mockAppointmentRepo struct {
	price float64
}

func (m *mockAppointmentRepo) GetByID(_ context.Context, aptID, clinicID uuid.UUID) (AppointmentInfo, error) {
	return &mockAptInfo{id: aptID, clinicID: clinicID, patientID: uuid.New(), price: m.price}, nil
}

func (m *mockAppointmentRepo) GetByPaymentToken(_ context.Context, token string) (AppointmentInfo, error) {
	return &mockAptInfo{id: uuid.New(), clinicID: uuid.New(), patientID: uuid.New(), price: m.price}, nil
}

func (m *mockAppointmentRepo) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockAppointmentRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

type mockAptInfo struct {
	id        uuid.UUID
	clinicID  uuid.UUID
	patientID uuid.UUID
	price     float64
}

func (a *mockAptInfo) GetID() uuid.UUID              { return a.id }
func (a *mockAptInfo) GetClinicID() uuid.UUID        { return a.clinicID }
func (a *mockAptInfo) GetPatientID() uuid.UUID       { return a.patientID }
func (a *mockAptInfo) GetPrice() float64              { return a.price }
func (a *mockAptInfo) GetStatus() string              { return "awaiting_payment" }
func (a *mockAptInfo) GetPaymentToken() string        { return "test-token" }
func (a *mockAptInfo) GetPaymentExpiresAt() *time.Time {
	t := time.Now().Add(15 * time.Minute)
	return &t
}

type mockAptPaymentRepo struct {
	sum     float64
	created bool
}

func (m *mockAptPaymentRepo) SumByAppointment(_ uuid.UUID) (float64, error) {
	return m.sum, nil
}

func (m *mockAptPaymentRepo) CreateFromWebhook(_ context.Context, _, _ uuid.UUID, _ *uuid.UUID, _ float64, _ time.Time) error {
	m.created = true
	return nil
}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }

// --- Tests ---

func TestCreateSubaccount_Success(t *testing.T) {
	encKey := testEncryptionKey()
	subRepo := newMockSubaccountRepo()
	splitRepo := newMockSplitConfigRepo()

	svc := NewMarketplaceService(
		&mockAsaasClient{},
		subRepo,
		splitRepo,
		newMockClinicPaymentRepo(),
		&mockClinicRepo{},
		&mockAppointmentRepo{price: 100},
		&mockAptPaymentRepo{},
		MarketplaceServiceConfig{
			EncryptionKey:     encKey,
			AsenoWalletID:     "wal_aseno",
			DefaultFeePercent: 0,
		},
	)

	clinicID := uuid.New()
	resp, err := svc.CreateSubaccount(context.Background(), clinicID, &CreateSubaccountInput{
		CompanyName: "Test Clinic",
		CpfCnpj:     "12345678000100",
		Email:       "test@clinic.com",
	})

	require.NoError(t, err)
	assert.Equal(t, "acc_test123", resp.AsaasAccountID)
	assert.Equal(t, "wal_test456", resp.WalletID)
	assert.Equal(t, model.SubaccountStatusActive, resp.Status)

	// Verify subaccount was stored
	stored, err := subRepo.GetByClinicID(context.Background(), clinicID)
	require.NoError(t, err)
	assert.Equal(t, "acc_test123", stored.AsaasAccountID)

	// Verify API key was encrypted
	decrypted, err := crypto.Decrypt(stored.EncryptedAPIKey, encKey)
	require.NoError(t, err)
	assert.Equal(t, "sk_test_subaccount_key", decrypted)

	// Verify split config was created
	splitConfig, err := splitRepo.GetByClinicID(context.Background(), clinicID)
	require.NoError(t, err)
	assert.Equal(t, model.FeeTypePercentage, splitConfig.FeeType)
	assert.Equal(t, 0.0, splitConfig.FeeValue)
}

func TestCreateSubaccount_AlreadyExists(t *testing.T) {
	encKey := testEncryptionKey()
	subRepo := newMockSubaccountRepo()

	svc := NewMarketplaceService(
		&mockAsaasClient{},
		subRepo,
		newMockSplitConfigRepo(),
		newMockClinicPaymentRepo(),
		&mockClinicRepo{},
		&mockAppointmentRepo{price: 100},
		&mockAptPaymentRepo{},
		MarketplaceServiceConfig{EncryptionKey: encKey},
	)

	clinicID := uuid.New()

	// Create first time
	_, err := svc.CreateSubaccount(context.Background(), clinicID, &CreateSubaccountInput{
		CompanyName: "Test", CpfCnpj: "123", Email: "a@b.com",
	})
	require.NoError(t, err)

	// Try again — should conflict
	_, err = svc.CreateSubaccount(context.Background(), clinicID, &CreateSubaccountInput{
		CompanyName: "Test", CpfCnpj: "123", Email: "a@b.com",
	})
	assert.Error(t, err)
}

func TestBuildSplitItems_NoFee(t *testing.T) {
	svc := &marketplaceService{
		asenoWalletID:     "wal_aseno",
		defaultFeePercent: 0,
		splitConfigRepo:   newMockSplitConfigRepo(),
	}

	splits := svc.buildSplitItems(context.Background(), uuid.New(), 100.0)
	assert.Nil(t, splits, "No split when fee is 0 and no per-clinic config")
}

func TestBuildSplitItems_WithPercentageFee(t *testing.T) {
	svc := &marketplaceService{
		asenoWalletID:     "wal_aseno",
		defaultFeePercent: 5.0,
		splitConfigRepo:   newMockSplitConfigRepo(),
	}

	splits := svc.buildSplitItems(context.Background(), uuid.New(), 200.0)
	require.Len(t, splits, 1)
	assert.Equal(t, "wal_aseno", splits[0].WalletID)
	assert.Equal(t, 5.0, splits[0].PercentualValue)
}

func TestBuildSplitItems_NoWallet(t *testing.T) {
	svc := &marketplaceService{
		asenoWalletID:     "",
		defaultFeePercent: 5.0,
		splitConfigRepo:   newMockSplitConfigRepo(),
	}

	splits := svc.buildSplitItems(context.Background(), uuid.New(), 100.0)
	assert.Nil(t, splits, "No split when wallet ID is empty")
}

func TestBuildSplitItems_PerClinicConfig(t *testing.T) {
	clinicID := uuid.New()
	splitRepo := newMockSplitConfigRepo()
	_ = splitRepo.Create(context.Background(), &model.SplitConfig{
		ClinicID:      clinicID,
		AsenoWalletID: "wal_clinic_specific",
		FeeType:       model.FeeTypePercentage,
		FeeValue:      3.5,
		Active:        true,
	})

	svc := &marketplaceService{
		asenoWalletID:     "wal_aseno",
		defaultFeePercent: 5.0, // global default is 5%, but clinic has 3.5%
		splitConfigRepo:   splitRepo,
	}

	splits := svc.buildSplitItems(context.Background(), clinicID, 100.0)
	require.Len(t, splits, 1)
	assert.Equal(t, "wal_clinic_specific", splits[0].WalletID)
	assert.Equal(t, 3.5, splits[0].PercentualValue)
}

func TestHandleWebhook_PaymentReceived_BridgesAppointmentPayment(t *testing.T) {
	encKey := testEncryptionKey()
	cpRepo := newMockClinicPaymentRepo()
	aptPaymentRepo := &mockAptPaymentRepo{sum: 0}

	clinicID := uuid.New()
	aptID := uuid.New()

	// Pre-create a clinic payment
	cp := &model.ClinicPayment{
		ClinicID:       clinicID,
		AppointmentID:  &aptID,
		AsaasPaymentID: "pay_webhook_test",
		Value:          150.0,
		Status:         "PENDING",
		BillingType:    "PIX",
		DueDate:        time.Now(),
	}
	_ = cpRepo.Create(context.Background(), cp)

	svc := NewMarketplaceService(
		&mockAsaasClient{},
		newMockSubaccountRepo(),
		newMockSplitConfigRepo(),
		cpRepo,
		&mockClinicRepo{},
		&mockAppointmentRepo{price: 150},
		aptPaymentRepo,
		MarketplaceServiceConfig{EncryptionKey: encKey},
	)

	err := svc.HandleSubaccountWebhook(context.Background(), &payment_model.AsaasWebhookPayload{
		Event: payment_model.WebhookEventPaymentReceived,
		Payment: &payment_model.WebhookPayment{
			ID:          "pay_webhook_test",
			Status:      "RECEIVED",
			Value:       150.0,
			NetValue:    145.50,
			PaymentDate: "2026-03-29",
		},
	})

	require.NoError(t, err)

	// Verify clinic payment was updated
	updated, _ := cpRepo.GetByAsaasID(context.Background(), "pay_webhook_test")
	assert.Equal(t, "RECEIVED", updated.Status)
	assert.Equal(t, 145.50, updated.NetValue)

	// Verify bridge created appointment payment
	assert.True(t, aptPaymentRepo.created, "AppointmentPayment should have been created")
}

func TestHandleWebhook_Idempotent(t *testing.T) {
	encKey := testEncryptionKey()
	cpRepo := newMockClinicPaymentRepo()

	cp := &model.ClinicPayment{
		ClinicID:       uuid.New(),
		AsaasPaymentID: "pay_idempotent",
		Value:          100.0,
		Status:         "RECEIVED", // Already in target status
		BillingType:    "PIX",
		DueDate:        time.Now(),
	}
	_ = cpRepo.Create(context.Background(), cp)

	svc := NewMarketplaceService(
		&mockAsaasClient{},
		newMockSubaccountRepo(),
		newMockSplitConfigRepo(),
		cpRepo,
		&mockClinicRepo{},
		&mockAppointmentRepo{price: 100},
		&mockAptPaymentRepo{},
		MarketplaceServiceConfig{EncryptionKey: encKey},
	)

	err := svc.HandleSubaccountWebhook(context.Background(), &payment_model.AsaasWebhookPayload{
		Event: payment_model.WebhookEventPaymentReceived,
		Payment: &payment_model.WebhookPayment{
			ID:     "pay_idempotent",
			Status: "RECEIVED",
		},
	})

	assert.NoError(t, err, "Idempotent webhook should not error")
}

func TestHandleWebhook_AlreadyPaid_NoBridge(t *testing.T) {
	encKey := testEncryptionKey()
	cpRepo := newMockClinicPaymentRepo()
	aptPaymentRepo := &mockAptPaymentRepo{sum: 150} // Already fully paid

	aptID := uuid.New()
	cp := &model.ClinicPayment{
		ClinicID:       uuid.New(),
		AppointmentID:  &aptID,
		AsaasPaymentID: "pay_already_paid",
		Value:          150.0,
		Status:         "PENDING",
		BillingType:    "PIX",
		DueDate:        time.Now(),
	}
	_ = cpRepo.Create(context.Background(), cp)

	svc := NewMarketplaceService(
		&mockAsaasClient{},
		newMockSubaccountRepo(),
		newMockSplitConfigRepo(),
		cpRepo,
		&mockClinicRepo{},
		&mockAppointmentRepo{price: 150},
		aptPaymentRepo,
		MarketplaceServiceConfig{EncryptionKey: encKey},
	)

	_ = svc.HandleSubaccountWebhook(context.Background(), &payment_model.AsaasWebhookPayload{
		Event: payment_model.WebhookEventPaymentReceived,
		Payment: &payment_model.WebhookPayment{
			ID:       "pay_already_paid",
			Status:   "RECEIVED",
			Value:    150.0,
			NetValue: 145.0,
		},
	})

	assert.False(t, aptPaymentRepo.created, "Should not create AppointmentPayment when already fully paid")
}

// --- Payment Security Tests ---

func TestGetPaymentPageData_ValidToken(t *testing.T) {
	encKey := testEncryptionKey()
	subRepo := newMockSubaccountRepo()
	clinicID := uuid.New()

	// Create subaccount
	sub := &model.Subaccount{
		ID: uuid.New(), ClinicID: clinicID, Status: model.SubaccountStatusActive,
		AsaasAccountID: "acc_test", WalletID: "wal_test",
	}
	encrypted, _ := crypto.Encrypt("sk_test", encKey)
	sub.EncryptedAPIKey = encrypted
	subRepo.subaccounts[clinicID] = sub

	expiresAt := time.Now().Add(10 * time.Minute)
	aptRepo := &mockAppointmentRepoWithToken{
		apt: &mockAptInfoWithExpiry{
			id: uuid.New(), clinicID: clinicID, patientID: uuid.New(),
			price: 100, status: "awaiting_payment", token: "valid-token", expiresAt: &expiresAt,
		},
	}

	svc := NewMarketplaceService(
		&mockAsaasClient{},
		subRepo, newMockSplitConfigRepo(), newMockClinicPaymentRepo(),
		&mockClinicRepo{}, aptRepo, &mockAptPaymentRepo{},
		MarketplaceServiceConfig{EncryptionKey: encKey, PaymentTimeoutMinutes: 15},
	)

	data, err := svc.GetPaymentPageData(context.Background(), "valid-token")
	require.NoError(t, err)
	assert.Equal(t, 100.0, data.Price)
	assert.True(t, data.RemainingSeconds > 0)
	assert.True(t, data.HasSubaccount)
}

func TestGetPaymentPageData_ExpiredToken(t *testing.T) {
	encKey := testEncryptionKey()

	expiresAt := time.Now().Add(-5 * time.Minute) // expired 5 min ago
	aptRepo := &mockAppointmentRepoWithToken{
		apt: &mockAptInfoWithExpiry{
			id: uuid.New(), clinicID: uuid.New(), patientID: uuid.New(),
			price: 100, status: "awaiting_payment", token: "expired-token", expiresAt: &expiresAt,
		},
	}

	svc := NewMarketplaceService(
		&mockAsaasClient{},
		newMockSubaccountRepo(), newMockSplitConfigRepo(), newMockClinicPaymentRepo(),
		&mockClinicRepo{}, aptRepo, &mockAptPaymentRepo{},
		MarketplaceServiceConfig{EncryptionKey: encKey},
	)

	_, err := svc.GetPaymentPageData(context.Background(), "expired-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expirou")
}

func TestGetPaymentPageData_CanceledAppointment(t *testing.T) {
	encKey := testEncryptionKey()

	expiresAt := time.Now().Add(10 * time.Minute)
	aptRepo := &mockAppointmentRepoWithToken{
		apt: &mockAptInfoWithExpiry{
			id: uuid.New(), clinicID: uuid.New(), patientID: uuid.New(),
			price: 100, status: "canceled", token: "canceled-token", expiresAt: &expiresAt,
		},
	}

	svc := NewMarketplaceService(
		&mockAsaasClient{},
		newMockSubaccountRepo(), newMockSplitConfigRepo(), newMockClinicPaymentRepo(),
		&mockClinicRepo{}, aptRepo, &mockAptPaymentRepo{},
		MarketplaceServiceConfig{EncryptionKey: encKey},
	)

	_, err := svc.GetPaymentPageData(context.Background(), "canceled-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelado")
}

// Mock with token support
type mockAppointmentRepoWithToken struct {
	apt *mockAptInfoWithExpiry
}

func (m *mockAppointmentRepoWithToken) GetByID(_ context.Context, aptID, clinicID uuid.UUID) (AppointmentInfo, error) {
	return m.apt, nil
}
func (m *mockAppointmentRepoWithToken) GetByPaymentToken(_ context.Context, token string) (AppointmentInfo, error) {
	if m.apt.token == token {
		return m.apt, nil
	}
	return nil, &notFoundError{}
}
func (m *mockAppointmentRepoWithToken) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockAppointmentRepoWithToken) UpdateStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

type mockAptInfoWithExpiry struct {
	id        uuid.UUID
	clinicID  uuid.UUID
	patientID uuid.UUID
	price     float64
	status    string
	token     string
	expiresAt *time.Time
}

func (a *mockAptInfoWithExpiry) GetID() uuid.UUID              { return a.id }
func (a *mockAptInfoWithExpiry) GetClinicID() uuid.UUID        { return a.clinicID }
func (a *mockAptInfoWithExpiry) GetPatientID() uuid.UUID       { return a.patientID }
func (a *mockAptInfoWithExpiry) GetPrice() float64              { return a.price }
func (a *mockAptInfoWithExpiry) GetStatus() string              { return a.status }
func (a *mockAptInfoWithExpiry) GetPaymentToken() string        { return a.token }
func (a *mockAptInfoWithExpiry) GetPaymentExpiresAt() *time.Time { return a.expiresAt }
