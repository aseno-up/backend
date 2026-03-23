package handler

import (
	"log"
	"net/http"

	"github.com/aseno/backend/pkg/auth/middleware"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	"github.com/aseno/backend/pkg/plan/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PlanHandler handles HTTP requests for plans and offers
type PlanHandler struct {
	planService service.PlanService
}

// NewPlanHandler creates a new plan handler
func NewPlanHandler(planService service.PlanService) *PlanHandler {
	return &PlanHandler{planService: planService}
}

// RegisterPublicRoutes registers the public plan routes
func (h *PlanHandler) RegisterPublicRoutes(router *gin.RouterGroup) {
	public := router.Group("/plans")
	{
		public.GET("", h.ListPlans)
		public.POST("/validate-offer", h.ValidateOffer)
	}
}

// RegisterRoutes registers the protected admin routes for plans
func (h *PlanHandler) RegisterRoutes(router *gin.RouterGroup, authMiddleware middleware.AuthMiddleware) {
	admin := router.Group("/admin/plans")
	admin.Use(authMiddleware.RequireAdmin())
	{
		admin.POST("", h.CreatePlan)
		admin.PUT("/:id", h.UpdatePlan)
	}
}

// ListPlans returns all active plans
func (h *PlanHandler) ListPlans(c *gin.Context) {
	plans, err := h.planService.ListPlans(c.Request.Context())
	if err != nil {
		log.Printf("Error listing plans: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plans)
}

// ValidateOfferRequest represents the request to validate an offer
type ValidateOfferRequest struct {
	PlanID string `json:"plan_id" binding:"required"`
	Token  string `json:"token" binding:"required"`
}

// ValidateOffer validates a coupon token and returns the discounted price
func (h *PlanHandler) ValidateOffer(c *gin.Context) {
	var req ValidateOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Use standard bad request error
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan ID format"})
		return
	}

	plan, err := h.planService.GetPlanByID(c.Request.Context(), planID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	offer, err := h.planService.ValidateOffer(c.Request.Context(), req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired offer", "details": err.Error()})
		return
	}

	// This validates the offer against the plan and returns the best price
	priceInfo, err := h.planService.CalculateBestPrice(c.Request.Context(), plan, offer, "monthly")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"price": priceInfo,
	})
}

// CreatePlan creates a new plan
func (h *PlanHandler) CreatePlan(c *gin.Context) {
	var req plan_model.Plan
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if err := h.planService.CreatePlan(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}

// UpdatePlan updates an existing plan
func (h *PlanHandler) UpdatePlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan ID"})
		return
	}

	var req plan_model.Plan
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}
	req.ID = id

	if err := h.planService.UpdatePlan(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, req)
}
