package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aseno/backend/pkg/plan/handler"
	"github.com/aseno/backend/pkg/plan/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPlanService is a mock implementation of PlanService
type MockPlanService struct {
	mock.Mock
}

func (m *MockPlanService) ListPlans(ctx context.Context) ([]model.Plan, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Plan), args.Error(1)
}

func (m *MockPlanService) GetPlanByID(ctx context.Context, id uuid.UUID) (*model.Plan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Plan), args.Error(1)
}

func (m *MockPlanService) ValidateOffer(ctx context.Context, token string) (*model.Offer, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Offer), args.Error(1)
}

func (m *MockPlanService) CalculateBestPrice(ctx context.Context, plan *model.Plan, offer *model.Offer, cycleType model.CycleType) (float64, error) {
	args := m.Called(ctx, plan, offer, cycleType)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockPlanService) UseOffer(ctx context.Context, offerID uuid.UUID) error       { return nil }
func (m *MockPlanService) CreatePlan(ctx context.Context, plan *model.Plan) error      { return nil }
func (m *MockPlanService) UpdatePlan(ctx context.Context, plan *model.Plan) error      { return nil }
func (m *MockPlanService) CreateOffer(ctx context.Context, offer *model.Offer) error   { return nil }
func (m *MockPlanService) ListOffers(ctx context.Context) ([]model.Offer, error)       { return nil, nil }
func (m *MockPlanService) UpdateOffer(ctx context.Context, offer *model.Offer) error   { return nil }

func TestValidateOffer_WithSlug(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockPlanService)
	h := handler.NewPlanHandler(mockSvc)

	router := gin.New()
	router.POST("/validate-offer", h.ValidateOffer)

	planID := uuid.New()
	mockPlan := model.Plan{
		ID:   planID,
		Slug: "advanced",
	}
	
	mockOffer := &model.Offer{
		ID:     uuid.New(),
		Token:  "MARCO50",
		PlanID: planID,
	}

	// It should first call ListPlans because "advanced" is not a UUID
	mockSvc.On("ListPlans", mock.Anything).Return([]model.Plan{mockPlan}, nil)
	mockSvc.On("ValidateOffer", mock.Anything, "MARCO50").Return(mockOffer, nil)
	mockSvc.On("CalculateBestPrice", mock.Anything, &mockPlan, mockOffer, model.CycleType("monthly")).Return(150.0, nil)

	reqBody := handler.ValidateOfferRequest{
		PlanID: "advanced",
		Token:  "MARCO50",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/validate-offer", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, true, response["valid"])
	assert.Equal(t, float64(150), response["price"])

	mockSvc.AssertExpectations(t)
}
