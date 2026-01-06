package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/saas/backend/internal/errors"
	"github.com/saas/backend/pkg/calendar/model"
	"github.com/saas/backend/pkg/calendar/service"
)

// CalendarHandler defines the interface for calendar handlers
type CalendarHandler interface {
	RegisterRoutes(router gin.IRouter)
	GetAuthURL(c *gin.Context)
	Callback(c *gin.Context)
	GetIntegration(c *gin.Context)
	ListCalendars(c *gin.Context)
	SetCalendar(c *gin.Context)
	UpdateSettings(c *gin.Context)
	Disconnect(c *gin.Context)
}

// calendarHandler implements CalendarHandler
type calendarHandler struct {
	service service.CalendarService
}

// NewCalendarHandler creates a new calendar handler
func NewCalendarHandler(service service.CalendarService) CalendarHandler {
	return &calendarHandler{
		service: service,
	}
}

// RegisterRoutes registers calendar routes
func (h *calendarHandler) RegisterRoutes(router gin.IRouter) {
	calendar := router.Group("/calendar")
	{
		calendar.GET("/auth-url", h.GetAuthURL)
		calendar.GET("/callback", h.Callback)
		calendar.GET("/integration", h.GetIntegration)
		calendar.GET("/calendars", h.ListCalendars)
		calendar.POST("/calendar", h.SetCalendar)
		calendar.PUT("/settings", h.UpdateSettings)
		calendar.DELETE("/disconnect", h.Disconnect)
	}
}

// GetAuthURL returns the Google OAuth authorization URL
func (h *calendarHandler) GetAuthURL(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	authURL := h.service.GetAuthURL(clinicID.(uuid.UUID))

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
	})
}

// Callback handles the OAuth callback from Google
func (h *calendarHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state") // Contains clinic_id

	if code == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Missing authorization code")))
		return
	}

	if state == "" {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Missing state parameter")))
		return
	}

	clinicID, err := uuid.Parse(state)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Invalid state parameter")))
		return
	}

	integration, err := h.service.HandleCallback(c.Request.Context(), clinicID, code)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Redirect back to the frontend calendar settings page
	// In production, this would redirect to a frontend URL
	c.JSON(http.StatusOK, gin.H{
		"status":  "connected",
		"email":   integration.Email,
		"message": "Google Calendar connected successfully",
	})
}

// GetIntegration returns the current calendar integration
func (h *calendarHandler) GetIntegration(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	integration, err := h.service.GetIntegration(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	// Don't expose tokens in response
	c.JSON(http.StatusOK, gin.H{
		"id":             integration.ID,
		"provider":       integration.Provider,
		"email":          integration.Email,
		"calendar_id":    integration.CalendarID,
		"sync_enabled":   integration.SyncEnabled,
		"sync_direction": integration.SyncDirection,
		"last_sync_at":   integration.LastSyncAt,
		"last_sync_error": integration.LastSyncError,
		"connected":      true,
	})
}

// ListCalendars lists available Google Calendars
func (h *calendarHandler) ListCalendars(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	calendars, err := h.service.ListCalendars(c.Request.Context(), clinicID.(uuid.UUID))
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"calendars": calendars,
	})
}

// SetCalendarRequest represents the request to set a calendar
type SetCalendarRequest struct {
	CalendarID string `json:"calendar_id" binding:"required"`
}

// SetCalendar sets the calendar to sync with
func (h *calendarHandler) SetCalendar(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req SetCalendarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.service.SetCalendar(c.Request.Context(), clinicID.(uuid.UUID), req.CalendarID); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "updated",
		"message": "Calendar set successfully",
	})
}

// UpdateSettingsRequest represents the request to update sync settings
type UpdateSettingsRequest struct {
	SyncEnabled   bool                 `json:"sync_enabled"`
	SyncDirection model.SyncDirection `json:"sync_direction"`
}

// UpdateSettings updates sync settings
func (h *calendarHandler) UpdateSettings(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.service.UpdateSyncSettings(c.Request.Context(), clinicID.(uuid.UUID), req.SyncEnabled, req.SyncDirection); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "updated",
		"message": "Settings updated successfully",
	})
}

// Disconnect disconnects the Google Calendar integration
func (h *calendarHandler) Disconnect(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	if err := h.service.DisableIntegration(c.Request.Context(), clinicID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "disconnected",
		"message": "Google Calendar disconnected successfully",
	})
}
