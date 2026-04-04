package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/aseno/backend/internal/logger"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	plan_service "github.com/aseno/backend/pkg/plan/service"
	"github.com/aseno/backend/pkg/settings/model"
	"github.com/aseno/backend/pkg/settings/service"
)

// SettingsHandler defines the interface for admin settings endpoints
type SettingsHandler interface {
	RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc)
	GetAll(c *gin.Context)
	Update(c *gin.Context)
}

// settingsHandler implements SettingsHandler
type settingsHandler struct {
	service service.SettingsService
	planSvc plan_service.PlanService
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(service service.SettingsService, planSvc plan_service.PlanService) SettingsHandler {
	return &settingsHandler{
		service: service,
		planSvc: planSvc,
	}
}

// RegisterRoutes registers admin settings routes
func (h *settingsHandler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	admin := router.Group("/admin")
	admin.Use(authMiddleware)
	admin.Use(h.devOnlyMiddleware())
	{
		admin.GET("/settings", h.GetAll)
		admin.PUT("/settings/:key", h.Update)
	}
}

// devOnlyMiddleware restricts access to dev-type clinics only
func (h *settingsHandler) devOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clinicRaw, exists := c.Get("clinic")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autorizado"})
			c.Abort()
			return
		}

		clinic, ok := clinicRaw.(*clinic_model.Clinic)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro interno"})
			c.Abort()
			return
		}

		if clinic.Type != clinic_model.ClinicTypeDev {
			c.JSON(http.StatusForbidden, gin.H{"error": "Acesso restrito a clínicas de desenvolvimento"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetAll returns all system settings
// @Summary Get all system settings
// @Tags Admin
// @Security BearerAuth
// @Success 200 {object} model.SettingsResponse
// @Router /admin/settings [get]
func (h *settingsHandler) GetAll(c *gin.Context) {
	settings, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar configurações"})
		return
	}

	c.JSON(http.StatusOK, model.SettingsResponse{Settings: settings})
}

// Update updates a single setting
// @Summary Update a system setting
// @Tags Admin
// @Security BearerAuth
// @Param key path string true "Setting key"
// @Param body body model.UpdateSettingRequest true "New value"
// @Success 200 {object} map[string]string
// @Router /admin/settings/{key} [put]
func (h *settingsHandler) Update(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chave é obrigatória"})
		return
	}

	var req model.UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Valor é obrigatório"})
		return
	}

	if err := h.service.Update(c.Request.Context(), key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar configuração"})
		return
	}

	// Propagate plan limit changes to the Plan entity so new accounts inherit updated values
	h.propagatePlanLimit(c, key, req.Value)

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuração atualizada com sucesso",
		"key":     key,
		"value":   req.Value,
	})
}

// planLimitSettings maps setting keys to plan slugs for propagation
var planLimitSettings = map[string]string{
	model.SettingPlanBasicLimit:    "basic",
	model.SettingPlanProLimit:      "professional",
	model.SettingPlanAdvancedLimit: "advanced",
}

// propagatePlanLimit updates the Plan entity when a plan limit setting changes.
// This ensures new accounts created after the change inherit the updated limits.
// Existing accounts are NOT affected (they keep their Account.Max* values).
func (h *settingsHandler) propagatePlanLimit(c *gin.Context, key, value string) {
	slug, ok := planLimitSettings[key]
	if !ok {
		return
	}

	limit, err := strconv.Atoi(value)
	if err != nil {
		logger.Printf("[Settings] Failed to parse plan limit value '%s': %v", value, err)
		return
	}

	plan, err := h.planSvc.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		logger.Printf("[Settings] Failed to find plan '%s' for limit propagation: %v", slug, err)
		return
	}

	plan.MaxProfessionals = limit
	if err := h.planSvc.UpdatePlan(c.Request.Context(), plan); err != nil {
		logger.Printf("[Settings] Failed to propagate limit to plan '%s': %v", slug, err)
	}
}
