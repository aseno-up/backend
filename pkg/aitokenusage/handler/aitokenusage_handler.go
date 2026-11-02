package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/aitokenusage/model"
	"github.com/aseno/backend/pkg/aitokenusage/service"
)

// Handler defines the AI token usage HTTP endpoints.
type Handler interface {
	RegisterRoutes(router gin.IRouter)
	SummaryByClinicAndModel(c *gin.Context)
}

type handler struct {
	svc service.Service
}

// NewHandler creates a new AI token usage handler.
func NewHandler(s service.Service) Handler {
	return &handler{svc: s}
}

func (h *handler) RegisterRoutes(router gin.IRouter) {
	g := router.Group("/ai-token-usage")
	{
		g.GET("/by-model", h.SummaryByClinicAndModel)
	}
}

// SummaryByClinicAndModel returns per-clinic per-model token consumption.
// Query params:
//   - start_date (YYYY-MM-DD, default: 30 days ago)
//   - end_date   (YYYY-MM-DD, default: today)
func (h *handler) SummaryByClinicAndModel(c *gin.Context) {
	startStr := c.DefaultQuery("start_date", "")
	endStr := c.DefaultQuery("end_date", "")

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		start = time.Now().AddDate(0, 0, -30)
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		end = time.Now()
	}

	rows, err := h.svc.SummaryByClinicAndModel(c.Request.Context(), start, end)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, model.ClinicModelSummaryResponse{
		Data:   rows,
		Period: start.Format("2006-01-02") + " to " + end.Format("2006-01-02"),
	})
}
