package handler

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	lead_model "github.com/aseno/backend/pkg/lead/model"
	lead_repo "github.com/aseno/backend/pkg/lead/repository"
	onboarding_service "github.com/aseno/backend/pkg/onboarding/service"
)

// OnboardingHandler exposes the magic-link signup endpoints:
//
//   POST /leads/landing-link         -> capture e-mail, save lead, send link
//   GET  /onboarding-tokens/:token   -> validate token (returns email)
//   POST /onboarding-tokens/:token/complete -> finish signup
type OnboardingHandler interface {
	RegisterRoutes(router gin.IRouter)
	CreateLandingLink(c *gin.Context)
	GetToken(c *gin.Context)
	Complete(c *gin.Context)
}

type onboardingHandler struct {
	svc      onboarding_service.OnboardingService
	leadRepo lead_repo.LeadRepository
}

// NewOnboardingHandler wires the handler. `leadRepo` is optional — when
// supplied, every landing-link request also drops a row in the `leads` table
// so the sales team sees the capture exactly like the kit-planilhas flow.
func NewOnboardingHandler(svc onboarding_service.OnboardingService, leadRepo lead_repo.LeadRepository) OnboardingHandler {
	return &onboardingHandler{svc: svc, leadRepo: leadRepo}
}

func (h *onboardingHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/leads/landing-link", h.CreateLandingLink)
	router.GET("/onboarding-tokens/:token", h.GetToken)
	router.POST("/onboarding-tokens/:token/complete", h.Complete)
}

type createLandingLinkRequest struct {
	Nome     string `json:"nome"`
	Email    string `json:"email" binding:"required,email"`
	WhatsApp string `json:"whatsapp"`
	Origem   string `json:"origem"`
}

func (h *onboardingHandler) CreateLandingLink(c *gin.Context) {
	var req createLandingLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "E-mail válido é obrigatório"})
		return
	}
	if req.Origem == "" {
		req.Origem = "landing_unknown"
	}

	// Best-effort lead drop so the team sees the capture in the leads table.
	// Failures here don't block the signup link — they just cost us a row.
	if h.leadRepo != nil {
		now := time.Now()
		_ = h.leadRepo.Create(c.Request.Context(), &lead_model.Lead{
			ClinicID:       uuid.Nil,
			ClinicName:     "Landing Page — " + req.Origem,
			ContactName:    fallbackName(req.Nome, req.Email),
			Email:          req.Email,
			Phone:          req.WhatsApp,
			Channel:        lead_model.LeadChannelInbound,
			Source:         lead_model.LeadSource(req.Origem),
			Status:         lead_model.LeadStatusIsca,
			FirstContactAt: &now,
			Notes:          "Magic-link enviado pelo formulário da landing page",
		})
	}

	if _, _, err := h.svc.GenerateForLead(c.Request.Context(), req.Email, req.Nome, req.WhatsApp, req.Origem); err != nil {
		log.Printf("[ONBOARDING] GenerateForLead failed for %s: %v", req.Email, err)
		c.JSON(httpStatusFromError(err), gin.H{"error": "Não foi possível enviar o link agora. Tente de novo em instantes."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Link enviado para o seu e-mail"})
}

type tokenResponse struct {
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	WhatsApp string    `json:"whatsapp"`
	Expires  time.Time `json:"expires_at"`
}

func (h *onboardingHandler) GetToken(c *gin.Context) {
	token := c.Param("token")
	t, err := h.svc.Validate(c.Request.Context(), token)
	if err != nil {
		c.JSON(httpStatusFromError(err), gin.H{"error": userMessage(err, "Link inválido ou expirado")})
		return
	}
	c.JSON(http.StatusOK, tokenResponse{
		Email:    t.Email,
		Name:     t.Name,
		WhatsApp: t.WhatsApp,
		Expires:  t.ExpiresAt,
	})
}

type completeRequest struct {
	Name       string `json:"name" binding:"required,min=2,max=100"`
	Password   string `json:"password" binding:"required,min=8"`
	ClinicName string `json:"clinic_name"`
	Phone      string `json:"phone"`
}

func (h *onboardingHandler) Complete(c *gin.Context) {
	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nome (mín. 2) e senha (mín. 8) são obrigatórios"})
		return
	}
	token := c.Param("token")

	resp, err := h.svc.Complete(c.Request.Context(), token, onboarding_service.CompleteRequest{
		Name:       req.Name,
		Password:   req.Password,
		ClinicName: req.ClinicName,
		Phone:      req.Phone,
	})
	if err != nil {
		c.JSON(httpStatusFromError(err), gin.H{"error": userMessage(err, "Não foi possível concluir o cadastro")})
		return
	}

	// Set the same HttpOnly auth cookies that /auth/register sets, otherwise
	// the auto-login redirect to /agenda fails the middleware and bounces to
	// /login. MaxAge mirrors auth_handler.go: 15min access, 7d refresh.
	// Secure flag flips on in production so cookies only travel over HTTPS.
	secure := os.Getenv("ENVIRONMENT") == "production"
	c.SetCookie("access_token", resp.AccessToken, 900, "/", "", secure, true)
	c.SetCookie("refresh_token", resp.RefreshToken, 604800, "/", "", secure, true)

	c.JSON(http.StatusCreated, resp)
}

// fallbackName picks a usable display name when the form didn't include one.
// Returns "Visitante" so the leads table doesn't store the e-mail local part
// as a fake contact name (looked like a bug to users seeing it in the magic-
// link e-mail and the onboarding form).
func fallbackName(nome, _ string) string {
	if n := strings.TrimSpace(nome); n != "" {
		return n
	}
	return "Visitante"
}

// httpStatusFromError extracts a status code from project AppError when
// available, falling back to 500.
func httpStatusFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if appErr, ok := err.(*errors.AppError); ok && appErr.StatusCode > 0 {
		return appErr.StatusCode
	}
	return http.StatusInternalServerError
}

// userMessage returns the user-safe message embedded in AppError, falling back
// to the supplied default. Avoids leaking internal error text.
func userMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	if appErr, ok := err.(*errors.AppError); ok && appErr.Message != "" {
		return appErr.Message
	}
	return fallback
}

