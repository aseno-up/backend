package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/professional/model"
	"github.com/aseno/backend/pkg/professional/service"
)

// ProfessionalProcedureHandler handles HTTP requests for professional-procedure commissions
type ProfessionalProcedureHandler interface {
	RegisterRoutes(router gin.IRouter)
	List(c *gin.Context)
	BulkUpsert(c *gin.Context)
	Delete(c *gin.Context)
}

type professionalProcedureHandler struct {
	service  service.ProfessionalProcedureService
	auditSvc audit_service.AuditService
}

func NewProfessionalProcedureHandler(service service.ProfessionalProcedureService, auditSvc audit_service.AuditService) ProfessionalProcedureHandler {
	return &professionalProcedureHandler{service: service, auditSvc: auditSvc}
}

// RegisterRoutes registers professional procedure commission routes
func (h *professionalProcedureHandler) RegisterRoutes(router gin.IRouter) {
	professionals := router.Group("/professionals")
	{
		professionals.GET("/:id/procedures", h.List)
		professionals.PUT("/:id/procedures", h.BulkUpsert)
		professionals.DELETE("/:id/procedures/:ppId", h.Delete)
	}

	procedures := router.Group("/procedures")
	{
		procedures.GET("/:id/professionals", h.ListProfessionalsByProcedure)
	}
}

// ListProfessionalsByProcedure returns professionals that have commission configured for a procedure
func (h *professionalProcedureHandler) ListProfessionalsByProcedure(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	procedureID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID do procedimento inválido")))
		return
	}

	result, err := h.service.ListProfessionalsByProcedure(c.Request.Context(), procedureID, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// List returns all procedure commissions for a professional
func (h *professionalProcedureHandler) List(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	professionalID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid professional ID")))
		return
	}

	result, err := h.service.ListByProfessional(c.Request.Context(), professionalID, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// BulkUpsert creates or updates commission settings for a professional's procedures
func (h *professionalProcedureHandler) BulkUpsert(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	professionalID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid professional ID")))
		return
	}

	var req model.BulkUpsertProfessionalProcedureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	result, err := h.service.BulkUpsert(c.Request.Context(), clinicID.(uuid.UUID), professionalID, &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityProfessionalProcedure, professionalID.String(), nil, result)
	}

	c.JSON(http.StatusOK, result)
}

// Delete removes a specific professional-procedure commission
func (h *professionalProcedureHandler) Delete(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	ppID, err := uuid.Parse(c.Param("ppId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid commission ID")))
		return
	}

	if err := h.service.Delete(c.Request.Context(), ppID, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityProfessionalProcedure, ppID.String(), nil, nil)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Commission deleted successfully"})
}
