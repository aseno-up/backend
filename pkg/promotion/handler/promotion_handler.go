package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/promotion/model"
	"github.com/aseno/backend/pkg/promotion/service"
)

type PromotionHandler interface {
	RegisterAdminRoutes(router gin.IRouter)
	RegisterPublicRoutes(router gin.IRouter)
	List(c *gin.Context)
	Get(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	GetActive(c *gin.Context)
}

type promotionHandler struct {
	service service.PromotionService
}

func NewPromotionHandler(svc service.PromotionService) PromotionHandler {
	return &promotionHandler{service: svc}
}

// RegisterAdminRoutes mounts the CRUD under the admin group (auth + dev-only enforced
// by the parent router in main.go).
func (h *promotionHandler) RegisterAdminRoutes(router gin.IRouter) {
	group := router.Group("/promotions")
	{
		group.GET("", h.List)
		group.GET("/:id", h.Get)
		group.POST("", h.Create)
		group.PUT("/:id", h.Update)
		group.DELETE("/:id", h.Delete)
	}
}

// RegisterPublicRoutes mounts the read-only "active promotion" endpoint under /public.
// No auth required — this is consumed by the landing page.
func (h *promotionHandler) RegisterPublicRoutes(router gin.IRouter) {
	router.GET("/public/promotions/active", h.GetActive)
}

func (h *promotionHandler) List(c *gin.Context) {
	result, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *promotionHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido")))
		return
	}
	p, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *promotionHandler) Create(c *gin.Context) {
	var req model.CreatePromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}
	p, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *promotionHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido")))
		return
	}
	var req model.UpdatePromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}
	p, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *promotionHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("ID inválido")))
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Promoção removida com sucesso"})
}

// GetActive returns the single active promotion (or null) for the public landing
// page. Cached at the edge (Cache-Control header) so traffic spikes don't hit
// the DB on every request.
func (h *promotionHandler) GetActive(c *gin.Context) {
	p, err := h.service.GetActive(c.Request.Context())
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.Header("Cache-Control", "public, max-age=60, s-maxage=60")
	// 200 com null em vez de 404 mantém o frontend simples e evita ruído no Sentry.
	c.JSON(http.StatusOK, gin.H{"data": p})
}
