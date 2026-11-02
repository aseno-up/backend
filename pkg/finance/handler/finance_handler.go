package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/finance/model"
	"github.com/aseno/backend/pkg/finance/service"
)

type FinanceHandler interface {
	RegisterRoutes(router gin.IRouter)
}

type financeHandler struct {
	service  service.FinanceService
	auditSvc audit_service.AuditService
}

func NewFinanceHandler(svc service.FinanceService, auditSvc audit_service.AuditService) FinanceHandler {
	return &financeHandler{service: svc, auditSvc: auditSvc}
}

func (h *financeHandler) RegisterRoutes(router gin.IRouter) {
	g := router.Group("/finance")
	{
		g.GET("/transactions", h.listTransactions)
		g.POST("/transactions", h.createTransaction)
		g.GET("/transactions/:id", h.getTransaction)
		g.PATCH("/transactions/:id", h.updateTransaction)
		g.DELETE("/transactions/:id", h.deleteTransaction)

		g.GET("/summary", h.summary)

		g.GET("/categories", h.listCategories)
		g.POST("/categories", h.createCategory)
		g.PATCH("/categories/:id", h.updateCategory)
		g.DELETE("/categories/:id", h.deleteCategory)
	}
}

// ---- Helpers -------------------------------------------------------------

func clinicIDFrom(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get("clinic_id")
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

func userIDFrom(c *gin.Context) uuid.UUID {
	v, ok := c.Get("user_id")
	if !ok {
		return uuid.Nil
	}
	id, ok := v.(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}

func parsePeriod(c *gin.Context) (time.Time, time.Time, error) {
	if month := c.Query("month"); month != "" {
		t, err := time.Parse("2006-01", month)
		if err != nil {
			return time.Time{}, time.Time{}, errors.ErrValidationInstance.WithMessage("Formato de mês inválido. Use YYYY-MM.")
		}
		return t, t.AddDate(0, 1, 0), nil
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		now := time.Now()
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return from, from.AddDate(0, 1, 0), nil
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.ErrValidationInstance.WithMessage("Data 'from' inválida. Use YYYY-MM-DD.")
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.ErrValidationInstance.WithMessage("Data 'to' inválida. Use YYYY-MM-DD.")
	}
	return from, to, nil
}

// ---- Transactions --------------------------------------------------------

func (h *financeHandler) listTransactions(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	from, to, err := parsePeriod(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(err))
		return
	}

	filter := model.TransactionFilter{From: &from, To: &to}
	if v := c.Query("type"); v != "" {
		tt := model.TransactionType(v)
		filter.Type = &tt
	}
	if v := c.Query("category_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.CategoryID = &id
		}
	}
	if v := c.Query("source"); v != "" {
		src := model.TransactionSource(v)
		filter.Source = &src
	}
	if c.Query("only_paid") == "true" {
		filter.OnlyPaid = true
	}

	list, err := h.service.ListTransactions(c.Request.Context(), clinicID, filter)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (h *financeHandler) createTransaction(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req model.CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	resp, err := h.service.CreateTransaction(c.Request.Context(), &req, clinicID, userIDFrom(c))
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntityFinancialTransaction, resp.ID.String(), nil, resp)
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *financeHandler) getTransaction(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido.")))
		return
	}
	resp, err := h.service.GetTransaction(c.Request.Context(), id, clinicID)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *financeHandler) updateTransaction(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido.")))
		return
	}

	var req model.UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}
	resp, err := h.service.UpdateTransaction(c.Request.Context(), id, &req, clinicID)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityFinancialTransaction, id.String(), nil, resp)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *financeHandler) deleteTransaction(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido.")))
		return
	}
	if err := h.service.DeleteTransaction(c.Request.Context(), id, clinicID); err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityFinancialTransaction, id.String(), nil, nil)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lançamento excluído."})
}

// ---- Summary -------------------------------------------------------------

func (h *financeHandler) summary(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	from, to, err := parsePeriod(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(err))
		return
	}
	summary, err := h.service.GetCashFlowSummary(c.Request.Context(), clinicID, from, to)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, summary)
}

// ---- Categories ----------------------------------------------------------

func (h *financeHandler) listCategories(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	list, err := h.service.ListCategories(c.Request.Context(), clinicID)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (h *financeHandler) createCategory(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	var req model.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}
	cat, err := h.service.CreateCategory(c.Request.Context(), &req, clinicID)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionCreate, audit_model.AuditEntityFinancialCategory, cat.ID.String(), nil, cat)
	}
	c.JSON(http.StatusCreated, cat)
}

func (h *financeHandler) updateCategory(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido.")))
		return
	}
	var req model.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}
	cat, err := h.service.UpdateCategory(c.Request.Context(), id, &req, clinicID)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityFinancialCategory, id.String(), nil, cat)
	}
	c.JSON(http.StatusOK, cat)
}

func (h *financeHandler) deleteCategory(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido.")))
		return
	}
	if err := h.service.DeleteCategory(c.Request.Context(), id, clinicID); err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityFinancialCategory, id.String(), nil, nil)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Categoria excluída."})
}
