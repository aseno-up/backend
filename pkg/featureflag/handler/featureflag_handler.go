package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/featureflag/model"
	"github.com/aseno/backend/pkg/featureflag/service"
)

// FeatureFlagHandler defines the interface for feature flag endpoints
type FeatureFlagHandler interface {
	RegisterRoutes(router gin.IRouter)
	List(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Check(c *gin.Context)
}

type featureFlagHandler struct {
	service service.FeatureFlagService
}

// NewFeatureFlagHandler creates a new feature flag handler
func NewFeatureFlagHandler(service service.FeatureFlagService) FeatureFlagHandler {
	return &featureFlagHandler{service: service}
}

func (h *featureFlagHandler) RegisterRoutes(router gin.IRouter) {
	group := router.Group("/feature-flags")
	{
		group.GET("", h.List)
		group.POST("", h.Create)
		group.PUT("/:id", h.Update)
		group.DELETE("/:id", h.Delete)
		group.GET("/check/:key", h.Check)
	}
}

func (h *featureFlagHandler) List(c *gin.Context) {
	result, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *featureFlagHandler) Create(c *gin.Context) {
	var req model.CreateFeatureFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	flag, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, flag)
}

func (h *featureFlagHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido")))
		return
	}

	var req model.UpdateFeatureFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	flag, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, flag)
}

func (h *featureFlagHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido")))
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Feature flag removida com sucesso"})
}

// Check verifies if a feature flag is enabled for the current clinic
func (h *featureFlagHandler) Check(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Key é obrigatória")))
		return
	}

	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	enabled := h.service.IsEnabled(c.Request.Context(), key, clinicID.(uuid.UUID))
	c.JSON(http.StatusOK, gin.H{"key": key, "enabled": enabled})
}
