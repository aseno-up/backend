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
}

type salesPackageHandler struct {
	service service.SalesPackageService
}

func NewSalesPackageHandler(svc service.SalesPackageService) SalesPackageHandler {
	return &salesPackageHandler{service: svc}
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
		g.POST("/payments/:paymentId/register", h.registerPayment)
	}
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
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
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
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
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
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
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
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	resp, err := h.service.RegisterPayment(c.Request.Context(), paymentID, clinicID, userID, &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
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
