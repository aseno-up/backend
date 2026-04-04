package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	account_repository "github.com/aseno/backend/pkg/account/repository"
	"github.com/aseno/backend/pkg/lead/model"
	"github.com/aseno/backend/pkg/lead/service"
)

// LeadHandler defines the interface for lead HTTP handlers
type LeadHandler interface {
	RegisterRoutes(router gin.IRouter)
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	ListAccounts(c *gin.Context)
	Dashboard(c *gin.Context)
}

type leadHandler struct {
	service     service.LeadService
	accountRepo account_repository.AccountRepository
	db          *gorm.DB
}

func NewLeadHandler(service service.LeadService, accountRepo account_repository.AccountRepository, db *gorm.DB) LeadHandler {
	return &leadHandler{service: service, accountRepo: accountRepo, db: db}
}

func (h *leadHandler) RegisterRoutes(router gin.IRouter) {
	leads := router.Group("/leads")
	{
		leads.POST("", h.Create)
		leads.GET("", h.List)
		leads.GET("/:id", h.GetByID)
		leads.PUT("/:id", h.Update)
		leads.DELETE("/:id", h.Delete)
	}
	router.GET("/accounts", h.ListAccounts)
	router.GET("/crm-dashboard", h.Dashboard)
}

func (h *leadHandler) Create(c *gin.Context) {
	var req model.CreateLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	lead, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, lead)
}

func (h *leadHandler) GetByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid lead ID")))
		return
	}

	lead, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, lead)
}

func (h *leadHandler) List(c *gin.Context) {
	var filter model.LeadFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *leadHandler) Update(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid lead ID")))
		return
	}

	var req model.UpdateLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	lead, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, lead)
}

func (h *leadHandler) Delete(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid lead ID")))
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Lead deleted successfully"})
}

func (h *leadHandler) ListAccounts(c *gin.Context) {
	type accountRow struct {
		ID                 string  `json:"id"`
		OwnerName          string  `json:"owner_name"`
		ClinicName         string  `json:"clinic_name"`
		SubscriptionStatus string  `json:"subscription_status"`
		PlanName           string  `json:"plan_name"`
		MRR                float64 `json:"mrr"`
	}

	var rows []accountRow
	err := h.db.WithContext(c.Request.Context()).Raw(`
		SELECT
			a.id::text,
			COALESCE(u.name, '') as owner_name,
			COALESCE(c.name, '') as clinic_name,
			a.subscription_status,
			COALESCE(p.name, '') as plan_name,
			COALESCE(
				(SELECT pc.price_brl / CASE WHEN pc.cycle_type = 'yearly' THEN 12 ELSE 1 END
				 FROM plan_cycles pc WHERE pc.plan_id = p.id AND pc.deleted_at IS NULL
				 ORDER BY CASE WHEN pc.cycle_type = 'monthly' THEN 0 ELSE 1 END
				 LIMIT 1),
				0
			) as mrr
		FROM accounts a
		LEFT JOIN users u ON u.id = a.owner_id AND u.deleted_at IS NULL
		INNER JOIN clinics c ON c.account_id = a.id AND c.deleted_at IS NULL
		LEFT JOIN plans p ON p.id = a.plan_id AND p.deleted_at IS NULL
		WHERE a.deleted_at IS NULL
		ORDER BY c.name ASC
	`).Scan(&rows).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(errors.ErrInternalInstance.WithError(err)))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *leadHandler) Dashboard(c *gin.Context) {
	// Lead stats by status
	type statusCount struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	var leadsByStatus []statusCount
	h.db.Raw("SELECT status, count(*) as count FROM leads WHERE deleted_at IS NULL GROUP BY status ORDER BY count DESC").Scan(&leadsByStatus)

	// Total MRR from converted leads
	var totalMRR float64
	h.db.Raw("SELECT COALESCE(SUM(mrr), 0) FROM leads WHERE deleted_at IS NULL AND status = 'convertido'").Scan(&totalMRR)

	// Total leads
	var totalLeads int
	h.db.Raw("SELECT count(*) FROM leads WHERE deleted_at IS NULL").Scan(&totalLeads)

	// Converted leads
	var convertedLeads int
	h.db.Raw("SELECT count(*) FROM leads WHERE deleted_at IS NULL AND status = 'convertido'").Scan(&convertedLeads)

	// Active trials
	var activeTrials int
	h.db.Raw("SELECT count(*) FROM leads WHERE deleted_at IS NULL AND status = 'trial_ativo'").Scan(&activeTrials)

	// All clinics with plan info and MRR
	type clinicRow struct {
		ID                 string  `json:"id"`
		Name               string  `json:"name"`
		OwnerName          string  `json:"owner_name"`
		Email              string  `json:"email"`
		Phone              string  `json:"phone"`
		PlanName           string  `json:"plan_name"`
		SubscriptionStatus string  `json:"subscription_status"`
		MRR                float64 `json:"mrr"`
		CreatedAt          string  `json:"created_at"`
	}
	var clinics []clinicRow
	h.db.Raw(`
		SELECT
			c.id::text,
			c.name,
			COALESCE(u.name, '') as owner_name,
			c.email,
			COALESCE(c.phone, '') as phone,
			COALESCE(p.name, '') as plan_name,
			COALESCE(a.subscription_status, '') as subscription_status,
			COALESCE(
				(SELECT pc.price_brl / CASE WHEN pc.cycle_type = 'yearly' THEN 12 ELSE 1 END
				 FROM plan_cycles pc WHERE pc.plan_id = p.id AND pc.deleted_at IS NULL
				 ORDER BY CASE WHEN pc.cycle_type = 'monthly' THEN 0 ELSE 1 END
				 LIMIT 1),
				0
			) as mrr,
			c.created_at::text
		FROM clinics c
		LEFT JOIN accounts a ON a.id = c.account_id AND a.deleted_at IS NULL
		LEFT JOIN users u ON u.id = a.owner_id AND u.deleted_at IS NULL
		LEFT JOIN plans p ON p.id = a.plan_id AND p.deleted_at IS NULL
		WHERE c.deleted_at IS NULL AND c.type != 'dev'
		ORDER BY c.created_at DESC
	`).Scan(&clinics)

	// Clinics per plan
	type planCount struct {
		PlanName string `json:"plan_name"`
		Count    int    `json:"count"`
	}
	var clinicsPerPlan []planCount
	h.db.Raw(`
		SELECT COALESCE(p.name, 'Sem plano') as plan_name, count(*) as count
		FROM clinics c
		LEFT JOIN accounts a ON a.id = c.account_id AND a.deleted_at IS NULL
		LEFT JOIN plans p ON p.id = a.plan_id AND p.deleted_at IS NULL
		WHERE c.deleted_at IS NULL AND c.type != 'dev'
		GROUP BY p.name
		ORDER BY count DESC
	`).Scan(&clinicsPerPlan)

	// Total MRR from all clinics
	var totalClinicMRR float64
	h.db.Raw(`
		SELECT COALESCE(SUM(
			COALESCE(
				(SELECT pc.price_brl / CASE WHEN pc.cycle_type = 'yearly' THEN 12 ELSE 1 END
				 FROM plan_cycles pc WHERE pc.plan_id = p.id AND pc.deleted_at IS NULL
				 ORDER BY CASE WHEN pc.cycle_type = 'monthly' THEN 0 ELSE 1 END
				 LIMIT 1),
				0
			)
		), 0)
		FROM clinics c
		LEFT JOIN accounts a ON a.id = c.account_id AND a.deleted_at IS NULL
		LEFT JOIN plans p ON p.id = a.plan_id AND p.deleted_at IS NULL
		WHERE c.deleted_at IS NULL AND c.type != 'dev'
		AND a.subscription_status IN ('active', 'trial')
	`).Scan(&totalClinicMRR)

	c.JSON(http.StatusOK, gin.H{
		"leads_by_status":  leadsByStatus,
		"total_mrr":        totalMRR,
		"total_clinic_mrr": totalClinicMRR,
		"total_leads":      totalLeads,
		"converted_leads":  convertedLeads,
		"active_trials":    activeTrials,
		"clinics":          clinics,
		"clinics_per_plan": clinicsPerPlan,
	})
}
