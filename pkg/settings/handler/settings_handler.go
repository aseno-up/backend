package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
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
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(service service.SettingsService) SettingsHandler {
	return &settingsHandler{
		service: service,
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

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuração atualizada com sucesso",
		"key":     key,
		"value":   req.Value,
	})
}
