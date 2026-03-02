package handler

import (
	"errors"
	"net/http"

	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/procedure/model"
	"github.com/aseno/backend/pkg/procedure/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProcedureHandler handles HTTP requests for procedures
type ProcedureHandler struct {
	service service.ProcedureService
}

// NewProcedureHandler creates a new procedure handler
func NewProcedureHandler(s service.ProcedureService) *ProcedureHandler {
	return &ProcedureHandler{service: s}
}

// RegisterRoutes registers procedure routes
func (h *ProcedureHandler) RegisterRoutes(r *gin.RouterGroup) {
	procedures := r.Group("/procedures")
	{
		procedures.POST("", h.Create)
		procedures.GET("", h.List)
		procedures.GET("/:id", h.GetByID)
		procedures.PUT("/:id", h.Update)
		procedures.DELETE("/:id", h.Delete)
		procedures.POST("/bulk-delete", h.BulkDelete)
	}
}

// getClinicID extracts clinic ID from Gin context
func getClinicID(c *gin.Context) (uuid.UUID, error) {
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

// Create handles creating a new procedure
func (h *ProcedureHandler) Create(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req model.CreateProcedureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	proc, err := h.service.Create(clinicID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, proc)
}

// List handles retrieving a paginated list of procedures
func (h *ProcedureHandler) List(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var filter pagination.Filter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parâmetros de paginação/filtro inválidos"})
		return
	}

	resp, err := h.service.List(c.Request.Context(), clinicID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetByID handles retrieving a specific procedure
func (h *ProcedureHandler) GetByID(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	proc, err := h.service.GetByID(clinicID, id)
	if err != nil {
		if errors.Is(err, service.ErrProcedureNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Procedimento não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, proc)
}

// Update handles updating an existing procedure
func (h *ProcedureHandler) Update(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	var req model.UpdateProcedureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	proc, err := h.service.Update(clinicID, id, &req)
	if err != nil {
		if errors.Is(err, service.ErrProcedureNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Procedimento não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, proc)
}

// Delete handles removing a procedure
func (h *ProcedureHandler) Delete(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
		return
	}

	if err := h.service.Delete(clinicID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Procedimento não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Procedimento removido com sucesso"})
}

// BulkDelete handles removing multiple procedures
func (h *ProcedureHandler) BulkDelete(c *gin.Context) {
	clinicID, err := getClinicID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	if err := h.service.BulkDelete(clinicID, req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Procedimentos removidos com sucesso"})
}
