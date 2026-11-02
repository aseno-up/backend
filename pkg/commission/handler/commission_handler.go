package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/commission/service"
)

type CommissionHandler interface {
	RegisterRoutes(router gin.IRouter)
	GetSummary(c *gin.Context)
	GetDetailedReport(c *gin.Context)
	BulkMarkPaid(c *gin.Context)
	MarkAllPendingPaid(c *gin.Context)
}

type commissionHandler struct {
	service  service.CommissionService
	auditSvc audit_service.AuditService
}

func NewCommissionHandler(service service.CommissionService, auditSvc audit_service.AuditService) CommissionHandler {
	return &commissionHandler{service: service, auditSvc: auditSvc}
}

func (h *commissionHandler) RegisterRoutes(router gin.IRouter) {
	commissions := router.Group("/commissions")
	{
		commissions.GET("/summary", h.GetSummary)
		commissions.GET("/report", h.GetDetailedReport)
		commissions.POST("/bulk-pay", h.BulkMarkPaid)
		commissions.POST("/pay-all", h.MarkAllPendingPaid)
	}
}

// parsePeriod extracts from/to dates from query params.
// Accepts ?from=2026-03-01&to=2026-04-01 or ?month=2026-03
func parsePeriod(c *gin.Context) (time.Time, time.Time, error) {
	if month := c.Query("month"); month != "" {
		t, err := time.Parse("2006-01", month)
		if err != nil {
			return time.Time{}, time.Time{}, errors.ErrValidationInstance.WithMessage("Formato de mes invalido. Use YYYY-MM.")
		}
		return t, t.AddDate(0, 1, 0), nil
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		// Default: current month
		now := time.Now()
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return from, from.AddDate(0, 1, 0), nil
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.ErrValidationInstance.WithMessage("Formato de data 'from' invalido. Use YYYY-MM-DD.")
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.ErrValidationInstance.WithMessage("Formato de data 'to' invalido. Use YYYY-MM-DD.")
	}

	return from, to, nil
}

// GetSummary returns consolidated commission summary for all professionals
func (h *commissionHandler) GetSummary(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	from, to, err := parsePeriod(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(err))
		return
	}

	result, err := h.service.GetSummary(c.Request.Context(), clinicID.(uuid.UUID), from, to)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetDetailedReport returns detailed commission report for a specific professional
func (h *commissionHandler) GetDetailedReport(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	profIDStr := c.Query("professional_id")
	if profIDStr == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("professional_id e obrigatorio.")))
		return
	}
	professionalID, err := uuid.Parse(profIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("professional_id invalido.")))
		return
	}

	from, to, err := parsePeriod(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(err))
		return
	}

	result, err := h.service.GetDetailedReport(c.Request.Context(), professionalID, clinicID.(uuid.UUID), from, to)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// BulkMarkPaid marks selected commissions as paid
func (h *commissionHandler) BulkMarkPaid(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.service.BulkMarkPaid(c.Request.Context(), req.IDs, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityCommission, "bulk-pay", nil, req.IDs)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Commissions marked as paid"})
}

// MarkAllPendingPaid marks all pending commissions for a professional in a period as paid
func (h *commissionHandler) MarkAllPendingPaid(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	var req struct {
		ProfessionalID uuid.UUID `json:"professional_id" binding:"required"`
		Month          string    `json:"month" binding:"required"` // "2026-03"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	from, err := time.Parse("2006-01", req.Month)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Formato de mes invalido. Use YYYY-MM.")))
		return
	}
	to := from.AddDate(0, 1, 0)

	if err := h.service.MarkAllPendingPaid(c.Request.Context(), req.ProfessionalID, clinicID.(uuid.UUID), from, to); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityCommission, req.ProfessionalID.String(), nil, gin.H{"month": req.Month, "action": "pay-all"})
	}

	c.JSON(http.StatusOK, gin.H{"message": "All pending commissions marked as paid"})
}
