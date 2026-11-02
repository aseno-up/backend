package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/promotion/model"
	"github.com/aseno/backend/pkg/promotion/repository"
)

// --- Mocks ---

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) Create(ctx context.Context, p *model.Promotion) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}
func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Promotion, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Promotion), args.Error(1)
}
func (m *mockRepo) Update(ctx context.Context, p *model.Promotion) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}
func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockRepo) List(ctx context.Context) ([]model.Promotion, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Promotion), args.Error(1)
}
func (m *mockRepo) GetActive(ctx context.Context, now time.Time) (*model.Promotion, error) {
	args := m.Called(ctx, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Promotion), args.Error(1)
}
func (m *mockRepo) DeactivateAllExcept(ctx context.Context, tx *gorm.DB, exceptID uuid.UUID) error {
	args := m.Called(ctx, tx, exceptID)
	return args.Error(0)
}
func (m *mockRepo) WithTx(tx *gorm.DB) repository.PromotionRepository {
	args := m.Called(tx)
	return args.Get(0).(repository.PromotionRepository)
}
func (m *mockRepo) DB() *gorm.DB {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*gorm.DB)
}

// --- Helpers ---

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func validReq() *model.CreatePromotionRequest {
	return &model.CreatePromotionRequest{
		Name:            "Maio 2026",
		Description:     "Promo de maio",
		DiscountPercent: 25,
		StartsAt:        time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		ExpiresAt:       time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC),
		IsActive:        false,
		PlansAffected:   []string{"basic", "professional"},
		BannerLabel:     "Promoção de Maio",
		BannerShowTimer: true,
	}
}

// --- Model methods ---

func TestPromotion_IsWithinWindow(t *testing.T) {
	p := &model.Promotion{
		StartsAt:  time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	assert.False(t, p.IsWithinWindow(time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC)), "antes do início")
	assert.True(t, p.IsWithinWindow(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)), "início inclusivo")
	assert.True(t, p.IsWithinWindow(time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)), "meio da janela")
	assert.False(t, p.IsWithinWindow(time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)), "expiração exclusiva")
	assert.False(t, p.IsWithinWindow(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)), "após expirar")
}

func TestPromotion_AppliesToPlan(t *testing.T) {
	p := &model.Promotion{PlansAffected: pq.StringArray{"basic", "professional"}}
	assert.True(t, p.AppliesToPlan("basic"))
	assert.True(t, p.AppliesToPlan("professional"))
	assert.False(t, p.AppliesToPlan("advanced"))
	assert.False(t, p.AppliesToPlan(""))

	all := &model.Promotion{PlansAffected: pq.StringArray{"all"}}
	assert.True(t, all.AppliesToPlan("basic"))
	assert.True(t, all.AppliesToPlan("anything-else"))
}

// --- Validations ---

func TestCreate_RejectsStartAfterExpiry(t *testing.T) {
	repo := new(mockRepo)
	svc := NewPromotionService(repo)
	req := validReq()
	req.StartsAt, req.ExpiresAt = req.ExpiresAt, req.StartsAt

	_, err := svc.Create(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "starts_at")
	repo.AssertNotCalled(t, "Create")
}

func TestCreate_RejectsEqualDates(t *testing.T) {
	repo := new(mockRepo)
	svc := NewPromotionService(repo)
	req := validReq()
	req.ExpiresAt = req.StartsAt

	_, err := svc.Create(context.Background(), req)
	assert.Error(t, err)
}

func TestCreate_RejectsDiscountOutOfRange(t *testing.T) {
	repo := new(mockRepo)
	svc := NewPromotionService(repo)

	cases := []int{0, -1, 91, 100, 200}
	for _, p := range cases {
		req := validReq()
		req.DiscountPercent = p
		_, err := svc.Create(context.Background(), req)
		assert.Error(t, err, "discount %d deve falhar", p)
	}
}

func TestCreate_AcceptsBoundaryDiscounts(t *testing.T) {
	for _, p := range []int{1, 50, 90} {
		repo := new(mockRepo)
		svc := NewPromotionService(repo)
		req := validReq()
		req.DiscountPercent = p
		repo.On("Create", mock.Anything, mock.Anything).Return(nil)

		result, err := svc.Create(context.Background(), req)
		assert.NoError(t, err, "discount %d deveria passar", p)
		assert.Equal(t, p, result.DiscountPercent)
	}
}

func TestCreate_RejectsEmptyPlansAffected(t *testing.T) {
	repo := new(mockRepo)
	svc := NewPromotionService(repo)
	req := validReq()
	req.PlansAffected = []string{}

	_, err := svc.Create(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plans_affected")
}

// --- Happy path (no transaction) ---

func TestCreate_InactivePersistsWithoutTransaction(t *testing.T) {
	repo := new(mockRepo)
	svc := NewPromotionService(repo)
	req := validReq()
	req.IsActive = false

	repo.On("Create", mock.Anything, mock.MatchedBy(func(p *model.Promotion) bool {
		return p.Name == "Maio 2026" && !p.IsActive && p.DiscountPercent == 25
	})).Return(nil)

	result, err := svc.Create(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "Maio 2026", result.Name)
	assert.False(t, result.IsActive)
	repo.AssertExpectations(t)
	// Crítico: não deve abrir transação para promoção inativa.
	repo.AssertNotCalled(t, "DB")
	repo.AssertNotCalled(t, "DeactivateAllExcept", mock.Anything, mock.Anything, mock.Anything)
}

// --- GetActive ---

func TestGetActive_DelegatesWithFixedNow(t *testing.T) {
	repo := new(mockRepo)
	fixed := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	svc := NewPromotionServiceWithClock(repo, fixedClock{t: fixed})

	expected := &model.Promotion{Name: "Maio 2026"}
	repo.On("GetActive", mock.Anything, fixed).Return(expected, nil)

	result, err := svc.GetActive(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestGetActive_NoneActiveReturnsNil(t *testing.T) {
	repo := new(mockRepo)
	svc := NewPromotionService(repo)
	repo.On("GetActive", mock.Anything, mock.Anything).Return(nil, nil)

	result, err := svc.GetActive(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, result, "nenhuma promoção ativa deve retornar nil sem erro")
}

// --- Update validations ---

func TestUpdate_RejectsInvalidDateRangeAfterPartialEdit(t *testing.T) {
	repo := new(mockRepo)
	svc := NewPromotionService(repo)
	id := uuid.New()
	existing := &model.Promotion{
		ID:        id,
		StartsAt:  time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	repo.On("GetByID", mock.Anything, id).Return(existing, nil)

	newStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.Update(context.Background(), id, &model.UpdatePromotionRequest{StartsAt: &newStart})
	assert.Error(t, err, "novo starts_at depois do expires_at existente deve falhar")
	repo.AssertNotCalled(t, "Update")
}

func TestUpdate_RejectsInvalidDiscount(t *testing.T) {
	repo := new(mockRepo)
	svc := NewPromotionService(repo)
	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(&model.Promotion{ID: id}, nil)

	invalid := 0
	_, err := svc.Update(context.Background(), id, &model.UpdatePromotionRequest{DiscountPercent: &invalid})
	assert.Error(t, err)
}
