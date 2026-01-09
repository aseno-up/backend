package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/clinic/service"
)

// OnboardingHandler defines the interface for onboarding handlers
type OnboardingHandler interface {
	RegisterRoutes(router gin.IRouter)
	GetStatus(c *gin.Context)
}

// onboardingHandler implements OnboardingHandler
type onboardingHandler struct {
	service service.OnboardingService
}

// NewOnboardingHandler creates a new onboarding handler
func NewOnboardingHandler(service service.OnboardingService) OnboardingHandler {
	return &onboardingHandler{service: service}
}

// RegisterRoutes registers onboarding routes
func (h *onboardingHandler) RegisterRoutes(router gin.IRouter) {
	onboarding := router.Group("/onboarding")
	{
		onboarding.GET("/status", h.GetStatus)
	}
}

// GetStatus handles getting the onboarding status for the current clinic
func (h *onboardingHandler) GetStatus(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	status, err := h.service.GetStatus(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, status)
}
