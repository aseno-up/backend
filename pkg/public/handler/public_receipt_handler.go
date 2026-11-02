package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appointment_service "github.com/aseno/backend/pkg/appointment/service"
)

// PublicReceiptHandler serves the read-only receipt page at
// /public/receipts/:token. It MUST live under the unauthenticated router and
// MUST NOT leak sensitive patient data (CPF, phone, diagnosis, notes).
type PublicReceiptHandler interface {
	RegisterRoutes(router gin.IRouter)
}

type publicReceiptHandler struct {
	receiptSvc appointment_service.AppointmentReceiptService
}

func NewPublicReceiptHandler(receiptSvc appointment_service.AppointmentReceiptService) PublicReceiptHandler {
	return &publicReceiptHandler{receiptSvc: receiptSvc}
}

func (h *publicReceiptHandler) RegisterRoutes(router gin.IRouter) {
	public := router.Group("/public")
	{
		public.GET("/receipts/:token", h.Get)
	}
}

func (h *publicReceiptHandler) Get(c *gin.Context) {
	token, err := uuid.Parse(c.Param("token"))
	if err != nil {
		// An unparseable token is treated as a missing one: the public page
		// must give the same response for "doesn't exist" and "malformed" so
		// the endpoint can't be probed to enumerate valid tokens.
		c.JSON(http.StatusNotFound, gin.H{"error": "Recibo não encontrado"})
		return
	}

	// Discourage any indexer that might stumble into the JSON response.
	c.Header("X-Robots-Tag", "noindex, nofollow")

	resp, err := h.receiptSvc.GetPublicReceipt(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recibo não encontrado"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
