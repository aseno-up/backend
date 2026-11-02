package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/appointment/service"
)

// AppointmentReceiptHandler handles the authenticated endpoint that mints or
// retrieves the public share token for an appointment's receipt.
type AppointmentReceiptHandler interface {
	RegisterRoutes(router gin.IRouter)
}

type appointmentReceiptHandler struct {
	service service.AppointmentReceiptService
}

func NewAppointmentReceiptHandler(s service.AppointmentReceiptService) AppointmentReceiptHandler {
	return &appointmentReceiptHandler{service: s}
}

// RegisterRoutes must be called on the authenticated /appointments group.
func (h *appointmentReceiptHandler) RegisterRoutes(appointmentsGroup gin.IRouter) {
	appointmentsGroup.POST("/:id/receipt-token", h.EnsureToken)
	// Cheap authenticated probe the public /recibo page uses to decide if the
	// viewer is the owning clinic. Lives here (under /appointments) so it
	// inherits the same auth middleware as EnsureToken.
	appointmentsGroup.GET("/receipt-tokens/:token/mine", h.CheckOwnership)
}

func (h *appointmentReceiptHandler) getClinicID(c *gin.Context) (uuid.UUID, error) {
	val, exists := c.Get("clinic_id")
	if !exists {
		return uuid.Nil, errors.New("clinic_id não encontrado no contexto")
	}
	clinicID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("clinic_id no formato incorreto")
	}
	return clinicID, nil
}

func (h *appointmentReceiptHandler) EnsureToken(c *gin.Context) {
	clinicID, err := h.getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	appointmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do agendamento inválido"})
		return
	}

	resp, err := h.service.EnsureToken(c.Request.Context(), clinicID, appointmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agendamento não encontrado"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CheckOwnership answers: is THIS token's clinic the same as the logged-in
// clinic? It returns 200 {owned: bool} in both cases — the frontend only
// needs a boolean. A missing/unparseable token 404s exactly like the public
// endpoint, so this path can't be used to enumerate tokens either.
func (h *appointmentReceiptHandler) CheckOwnership(c *gin.Context) {
	clinicID, err := h.getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	token, err := uuid.Parse(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recibo não encontrado"})
		return
	}

	owned, err := h.service.CheckTokenOwnership(c.Request.Context(), clinicID, token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recibo não encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"owned": owned})
}
