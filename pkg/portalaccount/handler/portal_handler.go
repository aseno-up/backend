package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/portalaccount/middleware"
	"github.com/aseno/backend/pkg/portalaccount/model"
	"github.com/aseno/backend/pkg/portalaccount/service"
)

type PortalHandler struct {
	service          service.PortalService
	portalMiddleware middleware.PortalMiddleware
}

func NewPortalHandler(svc service.PortalService, mw middleware.PortalMiddleware) *PortalHandler {
	return &PortalHandler{service: svc, portalMiddleware: mw}
}

func (h *PortalHandler) RegisterRoutes(router gin.IRouter) {
	portal := router.Group("/portal")
	{
		// Public routes (no auth)
		portal.GET("/validate-token", h.ValidateToken)
		portal.POST("/register", h.Register)
		portal.POST("/verify-email", h.VerifyEmail)
		portal.POST("/login", h.Login)
		portal.POST("/refresh", h.RefreshToken)

		// Protected routes (portal auth)
		protected := portal.Group("")
		protected.Use(h.portalMiddleware.Authenticate())
		{
			protected.GET("/me", h.GetMe)
			protected.GET("/appointments", h.GetAppointments)
			protected.GET("/stats", h.GetStats)
			protected.POST("/appointments", h.CreateAppointment)
			protected.PUT("/appointments/:id/cancel", h.CancelAppointment)
			protected.POST("/logout", h.Logout)
		}
	}
}

func (h *PortalHandler) ValidateToken(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token é obrigatório"})
		return
	}

	result, err := h.service.ValidateRegistrationToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *PortalHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	if err := h.service.Register(c.Request.Context(), req.Token, req.Password); err != nil {
		switch err {
		case service.ErrInvalidToken:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Token inválido ou expirado. Solicite um novo agendamento."})
		case service.ErrAlreadyRegistered:
			c.JSON(http.StatusConflict, gin.H{"error": "Conta já possui senha cadastrada. Faça login."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar conta"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conta criada! Verifique seu email para ativar."})
}

func (h *PortalHandler) VerifyEmail(c *gin.Context) {
	var req model.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	if err := h.service.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token inválido ou expirado"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email verificado com sucesso! Você já pode fazer login."})
}

func (h *PortalHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	tokens, me, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch err {
		case service.ErrInvalidCredentials:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Email ou senha inválidos"})
		case service.ErrEmailNotVerified:
			c.JSON(http.StatusForbidden, gin.H{"error": "Verifique seu email antes de fazer login"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao fazer login"})
		}
		return
	}

	// Set cookies
	c.SetCookie("portal_access_token", tokens.AccessToken, 30*60, "/", "", false, true)
	c.SetCookie("portal_refresh_token", tokens.RefreshToken, 30*24*60*60, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"user":    me,
		"message": "Login realizado com sucesso",
	})
}

func (h *PortalHandler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie("portal_refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token não encontrado"})
		return
	}

	tokens, err := h.service.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sua sessão expirou. Faça login novamente."})
		return
	}

	c.SetCookie("portal_access_token", tokens.AccessToken, 30*60, "/", "", false, true)
	c.SetCookie("portal_refresh_token", tokens.RefreshToken, 30*24*60*60, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Sessão renovada com sucesso"})
}

func (h *PortalHandler) GetMe(c *gin.Context) {
	patientID := middleware.GetPortalPatientID(c)
	clinicID := middleware.GetPortalClinicID(c)

	me, err := h.service.GetMe(c.Request.Context(), patientID, clinicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao carregar dados"})
		return
	}

	c.JSON(http.StatusOK, me)
}

func (h *PortalHandler) GetAppointments(c *gin.Context) {
	patientID := middleware.GetPortalPatientID(c)
	clinicID := middleware.GetPortalClinicID(c)

	appointments, err := h.service.GetAppointments(c.Request.Context(), patientID, clinicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao carregar agendamentos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": appointments})
}

func (h *PortalHandler) GetStats(c *gin.Context) {
	patientID := middleware.GetPortalPatientID(c)
	clinicID := middleware.GetPortalClinicID(c)

	stats, err := h.service.GetStats(c.Request.Context(), patientID, clinicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao carregar estatísticas"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *PortalHandler) CancelAppointment(c *gin.Context) {
	patientID := middleware.GetPortalPatientID(c)
	clinicID := middleware.GetPortalClinicID(c)

	aptID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	if err := h.service.CancelAppointment(c.Request.Context(), patientID, clinicID, aptID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agendamento cancelado com sucesso"})
}

func (h *PortalHandler) CreateAppointment(c *gin.Context) {
	patientID := middleware.GetPortalPatientID(c)
	clinicID := middleware.GetPortalClinicID(c)

	var req model.PortalCreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	result, err := h.service.CreateAppointment(c.Request.Context(), patientID, clinicID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

func (h *PortalHandler) Logout(c *gin.Context) {
	c.SetCookie("portal_access_token", "", -1, "/", "", false, true)
	c.SetCookie("portal_refresh_token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logout realizado"})
}

// Ensure time is imported (used indirectly via cookie max age calculations)
var _ = time.Now
