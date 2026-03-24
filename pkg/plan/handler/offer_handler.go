package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/auth/middleware"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	"github.com/aseno/backend/pkg/plan/service"
)

// OfferHandler handles HTTP requests for offers
type OfferHandler struct {
	planService service.PlanService
}

// NewOfferHandler creates a new offer handler
func NewOfferHandler(planService service.PlanService) *OfferHandler {
	return &OfferHandler{planService: planService}
}

// RegisterRoutes registers the offer routes
func (h *OfferHandler) RegisterRoutes(router *gin.RouterGroup, authMiddleware middleware.AuthMiddleware) {
	// Protected admin routes
	admin := router.Group("/admin/offers")
	admin.Use(authMiddleware.RequireAdmin())
	{
		admin.GET("", h.ListOffers)
		admin.POST("", h.CreateOffer)
		admin.PUT("/:id", h.UpdateOffer)
	}
}

// ListOffers returns all active offers
func (h *OfferHandler) ListOffers(c *gin.Context) {
	offers, err := h.planService.ListOffers(c.Request.Context())
	if err != nil {
		log.Printf("Error listing offers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, offers)
}

// CreateOffer creates a new offer
func (h *OfferHandler) CreateOffer(c *gin.Context) {
	var req plan_model.Offer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if err := h.planService.CreateOffer(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}

// UpdateOffer updates an existing offer
func (h *OfferHandler) UpdateOffer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offer ID"})
		return
	}

	var req plan_model.Offer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}
	req.ID = id

	if err := h.planService.UpdateOffer(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, req)
}
