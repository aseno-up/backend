package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	email_service "github.com/aseno/backend/pkg/email/service"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	plan_service "github.com/aseno/backend/pkg/plan/service"
	settings_service "github.com/aseno/backend/pkg/settings/service"
)

// PublicHandler defines the interface for public (unauthenticated) endpoints
type PublicHandler interface {
	RegisterRoutes(router gin.IRouter)
	GetConfig(c *gin.Context)
	SubmitContact(c *gin.Context)
}

type publicHandler struct {
	settings settings_service.SettingsService
	planSvc  plan_service.PlanService
	emailSvc email_service.EmailService
}

// NewPublicHandler creates a new public handler
func NewPublicHandler(settings settings_service.SettingsService, planSvc plan_service.PlanService, emailSvc email_service.EmailService) PublicHandler {
	return &publicHandler{
		settings: settings,
		planSvc:  planSvc,
		emailSvc: emailSvc,
	}
}

// RegisterRoutes registers public routes (no auth required)
func (h *publicHandler) RegisterRoutes(router gin.IRouter) {
	public := router.Group("/public")
	{
		public.GET("/config", h.GetConfig)
		public.POST("/contact", h.SubmitContact)
	}
}

// ContactRequest represents the contact form request body
type ContactRequest struct {
	Name    string `json:"name" binding:"required,min=2"`
	Email   string `json:"email" binding:"required,email"`
	Message string `json:"message" binding:"required,min=10"`
}

// SubmitContact handles contact form submissions
// @Summary Submit contact form
// @Tags Public
// @Accept json
// @Produce json
// @Param request body ContactRequest true "Contact form data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /public/contact [post]
func (h *publicHandler) SubmitContact(c *gin.Context) {
	var req ContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados invalidos. Preencha todos os campos corretamente."})
		return
	}

	err := h.emailSvc.SendContactForm(c.Request.Context(), &email_service.ContactFormData{
		Name:    req.Name,
		Email:   req.Email,
		Message: req.Message,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao enviar mensagem. Tente novamente."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Mensagem enviada com sucesso."})
}

// PlanConfigResponse represents a single plan's public configuration
type PlanConfigResponse struct {
	LimitProfessionals int      `json:"limit_professionals"` // -1 = unlimited
	PriceMonthly       int      `json:"price_monthly"`
	PriceYearly        int      `json:"price_yearly"`
	// SoloPriceMonthly / SoloPriceYearly are the discounted prices for a single-professional
	// contract. A value of 0 means this plan does not offer a solo variant for that cycle.
	SoloPriceMonthly int      `json:"solo_price_monthly"`
	SoloPriceYearly  int      `json:"solo_price_yearly"`
	ExcessPrice      int      `json:"excess_price_per_professional"`
	Features         []string `json:"features"`
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

	// Fallback features (used only if plan has no features in DB)
	fallbackFeatures := map[string][]string{
		"basic": {
			"Até 5 profissionais",
			"Agenda ilimitada",
			"Confirmações WhatsApp",
			"Sessões recorrentes",
			"Gestão de pacientes",
			"Histórico de atendimentos",
			"Acesso via celular/computador",
			"Suporte por email",
		},
		"professional": {
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
		"advanced": {
			"Profissionais ilimitados",
			"Tudo do Profissional",
			"API de integração",
			"Treinamento da equipe",
			"Importação de dados",
			"SLA de 99.9%",
			"Backup diário automático",
		},
	}

	response := PublicConfigResponse{
		TrialDays: h.settings.GetTrialDays(ctx),
		Plans:     make(map[string]PlanConfigResponse),
	}

	// Fetch dynamic plans from DB
	dbPlans, err := h.planSvc.ListPlans(ctx)
	if err == nil && len(dbPlans) > 0 {
		for _, p := range dbPlans {
			var priceMonthly, priceYearly float64
			var soloMonthly, soloYearly float64
			for _, cycle := range p.Cycles {
				switch cycle.CycleType {
				case plan_model.CycleTypeMonthly:
					priceMonthly = cycle.PriceBRL
					if cycle.SoloPriceBRL != nil {
						soloMonthly = *cycle.SoloPriceBRL
					}
				case plan_model.CycleTypeYearly:
					priceYearly = cycle.PriceBRL
					if cycle.SoloPriceBRL != nil {
						soloYearly = *cycle.SoloPriceBRL
					}
				}
			}

			features := []string(p.Features)
			if len(features) == 0 {
				features = fallbackFeatures[p.Slug]
			}

			excessPrice := 0
			if p.Slug != "advanced" {
				excessPrice = h.settings.GetExcessPrice(ctx)
			}

			// Solo variant is incompatible with advanced (per-unit) plan.
			if p.Slug == "advanced" {
				soloMonthly = 0
				soloYearly = 0
			}

			response.Plans[p.Slug] = PlanConfigResponse{
				LimitProfessionals: p.MaxProfessionals,
				PriceMonthly:       int(priceMonthly),
				PriceYearly:        int(priceYearly),
				SoloPriceMonthly:   int(soloMonthly),
				SoloPriceYearly:    int(soloYearly),
				ExcessPrice:        excessPrice,
				Features:           features,
			}
		}
	} else {
		// Fallback to settings if DB fails
		for slug, features := range fallbackFeatures {
			excessPrice := 0
			if slug != "advanced" {
				excessPrice = h.settings.GetExcessPrice(ctx)
			}
			response.Plans[slug] = PlanConfigResponse{
				LimitProfessionals: h.settings.GetPlanLimit(ctx, slug),
				PriceMonthly:       h.settings.GetPlanPrice(ctx, slug),
				PriceYearly:        h.settings.GetPlanPrice(ctx, slug) * 10,
				ExcessPrice:        excessPrice,
				Features:           features,
			}
		}
	}

	// No cache — prices are managed dynamically via admin
	c.Header("Cache-Control", "no-cache")
	c.JSON(http.StatusOK, response)
}
