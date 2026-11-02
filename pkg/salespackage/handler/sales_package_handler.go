package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/salespackage/model"
	"github.com/aseno/backend/pkg/salespackage/service"
)

type SalesPackageHandler interface {
	RegisterRoutes(router gin.IRouter)
	RegisterPublicRoutes(router gin.IRouter)
}

type salesPackageHandler struct {
	service        service.SalesPackageService
	receiptService service.SalesPackageReceiptService
}

func NewSalesPackageHandler(svc service.SalesPackageService, receiptSvc service.SalesPackageReceiptService) SalesPackageHandler {
	return &salesPackageHandler{service: svc, receiptService: receiptSvc}
}

func (h *salesPackageHandler) RegisterRoutes(router gin.IRouter) {
	g := router.Group("/sales-packages")
	{
		g.GET("", h.list)
		g.POST("", h.create)
		g.GET("/balances", h.balances) // ?patient_id=...&procedure_id=...(optional)
		g.GET("/:id", h.getByID)
		g.PATCH("/:id", h.update)
		g.POST("/:id/convert", h.convert)
		g.POST("/:id/cancel", h.cancel)
		g.POST("/:id/share-receipt", h.shareReceipt)
		g.POST("/payments/:paymentId/register", h.registerPayment)
		g.PUT("/payments/:paymentId", h.updatePaymentRow)
		g.DELETE("/payments/:paymentId", h.deletePaymentRow)
		g.POST("/:id/payments", h.addPayment)
	}
}

// RegisterPublicRoutes mounts the unauthenticated endpoint that backs the
// public /recibo/venda/:token page.
func (h *salesPackageHandler) RegisterPublicRoutes(router gin.IRouter) {
	router.GET("/public/sales-packages/recibo/:token", h.publicReceipt)
}

// --- Helpers --------------------------------------------------------------

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

// bindError converts a gin binding error into a 400 response whose message
// surfaces the actual reason (e.g. "Key: 'CreateSalesPackageRequest.Items[0].UnitPrice'
// Error:Field validation for 'UnitPrice' failed on the 'required' tag") instead
// of the generic "Validation failed". Helps the user — and us — diagnose
// payload mismatches without spelunking through system_logs.
func bindError(err error) error {
	return errors.ErrValidationInstance.
		WithError(err).
		WithMessage("Dados inválidos: " + err.Error())
}

// --- Endpoints ------------------------------------------------------------

func (h *salesPackageHandler) list(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	filter := model.SalesPackageFilter{
		Query: c.Query("q"),
	}
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Page = n
		}
	}
	if v := c.Query("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.PerPage = n
		}
	}
	if v := c.Query("status"); v != "" {
		s := model.SalesPackageStatus(v)
		filter.Status = &s
	}
	if v := c.Query("patient_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.PatientID = &id
		}
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			filter.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			filter.To = &t
		}
	}

	resp, err := h.service.List(c.Request.Context(), clinicID, filter)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *salesPackageHandler) create(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	userID := userIDFrom(c)

	var req model.CreateSalesPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(bindError(err)))
		return
	}

	resp, err := h.service.Create(c.Request.Context(), clinicID, userID, &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *salesPackageHandler) getByID(c *gin.Context) {
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

	resp, err := h.service.GetByID(c.Request.Context(), id, clinicID)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *salesPackageHandler) update(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	userID := userIDFrom(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido.")))
		return
	}

	var req model.UpdateSalesPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(bindError(err)))
		return
	}

	resp, err := h.service.Update(c.Request.Context(), id, clinicID, userID, &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *salesPackageHandler) convert(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	userID := userIDFrom(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido.")))
		return
	}

	var req struct {
		Payments []model.CreateSalesPackagePaymentInput `json:"payments" binding:"required,min=1,dive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(bindError(err)))
		return
	}

	resp, err := h.service.ConvertQuoteToActive(c.Request.Context(), id, clinicID, userID, req.Payments)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *salesPackageHandler) cancel(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	userID := userIDFrom(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido.")))
		return
	}

	resp, err := h.service.Cancel(c.Request.Context(), id, clinicID, userID)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// updatePaymentRow edits a single payment line in place. Method/amount
// changes rebalance against the sale's open receivables.
func (h *salesPackageHandler) updatePaymentRow(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	userID := userIDFrom(c)
	paymentID, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID de pagamento inválido.")))
		return
	}
	var req model.UpdatePaymentRowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(bindError(err)))
		return
	}
	resp, err := h.service.UpdatePaymentRow(c.Request.Context(), paymentID, clinicID, userID, &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// deletePaymentRow removes a payment line and restores its amount as an
// open receivable so the sale's total stays consistent.
func (h *salesPackageHandler) deletePaymentRow(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	userID := userIDFrom(c)
	paymentID, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID de pagamento inválido.")))
		return
	}
	if err := h.service.DeletePaymentRow(c.Request.Context(), paymentID, clinicID, userID); err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// addPayment records a brand-new payment line on an existing active sale.
// The new amount is consumed from the sale's open receivables.
func (h *salesPackageHandler) addPayment(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	userID := userIDFrom(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido.")))
		return
	}
	var req model.CreateSalesPackagePaymentInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(bindError(err)))
		return
	}
	resp, err := h.service.AddPayment(c.Request.Context(), id, clinicID, userID, &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *salesPackageHandler) registerPayment(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	userID := userIDFrom(c)
	paymentID, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID de pagamento inválido.")))
		return
	}

	var req model.RegisterPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(bindError(err)))
		return
	}

	resp, err := h.service.RegisterPayment(c.Request.Context(), paymentID, clinicID, userID, &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// shareReceipt returns the public receipt link for a sale, creating the
// share token on first request (idempotent on subsequent calls).
func (h *salesPackageHandler) shareReceipt(c *gin.Context) {
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
	resp, err := h.receiptService.EnsureToken(c.Request.Context(), clinicID, id)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// publicReceipt is the unauthenticated endpoint that returns the live receipt
// payload for a given share token. Errors normalize to 404 to avoid leaking
// the existence of a token.
func (h *salesPackageHandler) publicReceipt(c *gin.Context) {
	token, err := uuid.Parse(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, errors.ToResponse(errors.ErrNotFoundInstance))
		return
	}
	resp, err := h.receiptService.GetPublicReceipt(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusNotFound, errors.ToResponse(errors.ErrNotFoundInstance))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// balances returns either:
//   - patient packages aggregated per procedure (when only patient_id is given)
//   - the FIFO item id for (patient_id, procedure_id) — used by the agenda when
//     creating an appointment, so the backend picks the consumed item.
func (h *salesPackageHandler) balances(c *gin.Context) {
	clinicID, ok := clinicIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}
	patientIDStr := c.Query("patient_id")
	if patientIDStr == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("patient_id é obrigatório.")))
		return
	}
	patientID, err := uuid.Parse(patientIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("patient_id inválido.")))
		return
	}

	if procStr := c.Query("procedure_id"); procStr != "" {
		procedureID, err := uuid.Parse(procStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("procedure_id inválido.")))
			return
		}
		itemID, err := h.service.PickFIFOItemID(c.Request.Context(), clinicID, patientID, procedureID)
		if err != nil {
			c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
			return
		}
		c.JSON(http.StatusOK, gin.H{"item_id": itemID})
		return
	}

	balances, err := h.service.GetBalancesByPatient(c.Request.Context(), clinicID, patientID)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": balances})
}
