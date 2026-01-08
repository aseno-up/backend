package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/waitlist/model"
	"github.com/aseno/backend/pkg/waitlist/service"
)

// WaitlistHandler defines the interface for waitlist handlers
type WaitlistHandler interface {
	RegisterRoutes(router gin.IRouter)
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
}

// waitlistHandler implements WaitlistHandler
type waitlistHandler struct {
	service service.WaitlistService
}

// NewWaitlistHandler creates a new waitlist handler
func NewWaitlistHandler(service service.WaitlistService) WaitlistHandler {
	return &waitlistHandler{service: service}
}

// RegisterRoutes registers waitlist routes
func (h *waitlistHandler) RegisterRoutes(router gin.IRouter) {
	waitlist := router.Group("/waitlist")
	{
		waitlist.POST("", h.Create)
		waitlist.GET("", h.List)
		waitlist.GET("/:id", h.GetByID)
		waitlist.PUT("/:id", h.Update)
		waitlist.DELETE("/:id", h.Delete)
	}
}

// Create handles adding a patient to the waitlist
// @Summary Add patient to waitlist
// @Description Adds a patient to the appointment waitlist
// @Tags Waitlist
// @Accept json
// @Produce json
// @Param request body model.CreateWaitlistRequest true "Waitlist entry"
// @Success 201 {object} model.WaitlistEntryResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Router /waitlist [post]
func (h *waitlistHandler) Create(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	var req model.CreateWaitlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	entry, err := h.service.Create(c.Request.Context(), clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusCreated, entry)
}

// GetByID handles getting a waitlist entry by ID
// @Summary Get waitlist entry
// @Description Get a waitlist entry by ID
// @Tags Waitlist
// @Accept json
// @Produce json
// @Param id path string true "Waitlist Entry ID"
// @Success 200 {object} model.WaitlistEntryResponse
// @Failure 404 {object} errors.ErrorResponse
// @Router /waitlist/{id} [get]
func (h *waitlistHandler) GetByID(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid waitlist entry ID")))
		return
	}

	entry, err := h.service.GetByID(c.Request.Context(), id, clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, entry)
}

// List handles listing waitlist entries
// @Summary List waitlist entries
// @Description Get a paginated list of waitlist entries
// @Tags Waitlist
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param status query string false "Filter by status (waiting, notified, scheduled, declined)"
// @Success 200 {object} model.WaitlistListResponse
// @Router /waitlist [get]
func (h *waitlistHandler) List(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	status := c.Query("status")

	result, err := h.service.List(c.Request.Context(), clinicID.(uuid.UUID), status, page, perPage)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// Update handles updating a waitlist entry
// @Summary Update waitlist entry
// @Description Update a waitlist entry's preferences
// @Tags Waitlist
// @Accept json
// @Produce json
// @Param id path string true "Waitlist Entry ID"
// @Param request body model.UpdateWaitlistRequest true "Update data"
// @Success 200 {object} model.WaitlistEntryResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 404 {object} errors.ErrorResponse
// @Router /waitlist/{id} [put]
func (h *waitlistHandler) Update(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid waitlist entry ID")))
		return
	}

	var req model.UpdateWaitlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	entry, err := h.service.Update(c.Request.Context(), id, clinicID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, entry)
}

// Delete handles removing a patient from the waitlist
// @Summary Remove from waitlist
// @Description Remove a patient from the waitlist
// @Tags Waitlist
// @Accept json
// @Produce json
// @Param id path string true "Waitlist Entry ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} errors.ErrorResponse
// @Router /waitlist/{id} [delete]
func (h *waitlistHandler) Delete(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid waitlist entry ID")))
		return
	}

	if err := h.service.Delete(c.Request.Context(), id, clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Removed from waitlist successfully"})
}
