package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	settings_service "github.com/aseno/backend/pkg/settings/service"
)

// PublicHandler defines the interface for public (unauthenticated) endpoints
type PublicHandler interface {
	RegisterRoutes(router gin.IRouter)
	GetConfig(c *gin.Context)
}

// publicHandler implements PublicHandler
type publicHandler struct {
	settings settings_service.SettingsService
}

// NewPublicHandler creates a new public handler
func NewPublicHandler(settings settings_service.SettingsService) PublicHandler {
	return &publicHandler{
		settings: settings,
	}
}

// RegisterRoutes registers public routes (no auth required)
func (h *publicHandler) RegisterRoutes(router gin.IRouter) {
	public := router.Group("/public")
	{
		public.GET("/config", h.GetConfig)
	}
}

// PlanConfigResponse represents a single plan's public configuration
type PlanConfigResponse struct {
	LimitProfessionals int      `json:"limit_professionals"` // -1 = unlimited
	PriceMonthly       int      `json:"price_monthly"`
	ExcessPrice        int      `json:"excess_price_per_professional"`
	Features           []string `json:"features"`
}

// PublicConfigResponse represents the public configuration response
type PublicConfigResponse struct {
	Plans     map[string]PlanConfigResponse `json:"plans"`
	TrialDays int                           `json:"trial_days"`
}

// GetConfig returns public configuration for the landing page
// @Summary Get public configuration
// @Tags Public
// @Success 200 {object} PublicConfigResponse
// @Router /public/config [get]
func (h *publicHandler) GetConfig(c *gin.Context) {
	ctx := c.Request.Context()

	response := PublicConfigResponse{
		TrialDays: h.settings.GetTrialDays(ctx),
		Plans: map[string]PlanConfigResponse{
			"basic": {
				LimitProfessionals: h.settings.GetPlanLimit(ctx, "basic"),
				PriceMonthly:       h.settings.GetPlanPrice(ctx, "basic"),
				ExcessPrice:        h.settings.GetExcessPrice(ctx),
				Features: []string{
					"Até 5 profissionais",
					"Agenda ilimitada",
					"Confirmações WhatsApp",
					"Sessões recorrentes",
					"Gestão de pacientes",
					"Histórico de atendimentos",
					"Acesso via celular/computador",
					"Suporte por email",
				},
			},
			"professional": {
				LimitProfessionals: h.settings.GetPlanLimit(ctx, "professional"),
				PriceMonthly:       h.settings.GetPlanPrice(ctx, "professional"),
				ExcessPrice:        h.settings.GetExcessPrice(ctx),
				Features: []string{
					"Até 15 profissionais",
					"Tudo do Básico",
					"Integração Google Calendar",
					"Lista de Espera Automática",
					"Detector de Risco de Faltas",
					"Relatórios de Prejuízo",
					"Relatórios de Faltas",
					"Personalização de mensagens",
					"Suporte prioritário WhatsApp",
				},
			},
			"advanced": {
				LimitProfessionals: h.settings.GetPlanLimit(ctx, "advanced"),
				PriceMonthly:       h.settings.GetPlanPrice(ctx, "advanced"),
				ExcessPrice:        0, // No excess for advanced plan (unlimited)
				Features: []string{
					"Profissionais ilimitados",
					"Tudo do Profissional",
					"API de integração",
					"Treinamento da equipe",
					"Importação de dados",
					"SLA de 99.9%",
					"Backup diário automático",
				},
			},
		},
	}

	// Add cache headers for CDN/browser caching (5 minutes)
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, response)
}
