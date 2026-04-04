package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/email/service"
	"github.com/aseno/backend/pkg/lead/model"
	"github.com/aseno/backend/pkg/lead/repository"
)

// LeadIscaHandler handles the public lead capture (isca) endpoint
type LeadIscaHandler interface {
	RegisterRoutes(router gin.IRouter)
	CreateIsca(c *gin.Context)
}

type leadIscaHandler struct {
	repo      repository.LeadRepository
	emailSvc  service.EmailService
	sheetsDir string
}

// NewLeadIscaHandler creates a new handler for the public isca endpoint
func NewLeadIscaHandler(repo repository.LeadRepository, emailSvc service.EmailService, sheetsDir string) LeadIscaHandler {
	return &leadIscaHandler{
		repo:      repo,
		emailSvc:  emailSvc,
		sheetsDir: sheetsDir,
	}
}

func (h *leadIscaHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/leads/isca", h.CreateIsca)
}

// createIscaRequest represents the public isca form submission
type createIscaRequest struct {
	Nome     string `json:"nome" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	WhatsApp string `json:"whatsapp"`
	Origem   string `json:"origem"`
}

func (h *leadIscaHandler) CreateIsca(c *gin.Context) {
	var req createIscaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nome e email são obrigatórios"})
		return
	}

	now := time.Now()

	lead := &model.Lead{
		ClinicID:       uuid.Nil,
		ClinicName:     "Isca — " + req.Origem,
		ContactName:    req.Nome,
		Email:          req.Email,
		Phone:          req.WhatsApp,
		Channel:        model.LeadChannelInbound,
		Source:         model.LeadSource(req.Origem),
		Status:         model.LeadStatusIsca,
		FirstContactAt: &now,
		Notes:          "Lead capturado via isca, sem vínculo com clínica específica",
	}

	if err := h.repo.Create(c.Request.Context(), lead); err != nil {
		log.Printf("[ISCA] failed to save lead: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao processar sua solicitação"})
		return
	}

	go h.sendIscaEmail(req.Nome, req.Email)

	c.JSON(http.StatusCreated, gin.H{"message": "Kit enviado com sucesso"})
}

func (h *leadIscaHandler) sendIscaEmail(name, email string) {
	if h.emailSvc == nil {
		log.Println("[ISCA] email service not configured, skipping")
		return
	}

	sheets := []string{
		"Controle_Agenda_NoShow_Aseno.xlsx",
		"Controle_Financeiro_Clinica_Aseno.xlsx",
		"Calculadora_Precificacao_Procedimentos_Aseno.xlsx",
	}

	var attachments []service.EmailAttachment
	for _, filename := range sheets {
		path := filepath.Join(h.sheetsDir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[ISCA] failed to read sheet %s: %v", filename, err)
			return
		}
		attachments = append(attachments, service.EmailAttachment{
			Name:    filename,
			Content: base64.StdEncoding.EncodeToString(data),
		})
	}

	err := h.emailSvc.SendLeadIsca(context.Background(), &service.LeadIscaEmailData{
		RecipientEmail: email,
		RecipientName:  name,
		Attachments:    attachments,
	})
	if err != nil {
		log.Printf("[ISCA] failed to send email to %s: %v", email, err)
	} else {
		fmt.Printf("[ISCA] kit sent to %s (%s)\n", name, email)
	}
}
