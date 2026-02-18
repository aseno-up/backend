package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	auth_service "github.com/aseno/backend/pkg/auth/service"
	"github.com/aseno/backend/pkg/notification/model"
	"github.com/aseno/backend/pkg/notification/service"
	"github.com/aseno/backend/pkg/notification/sse"
)

// NotificationHandler defines the interface for notification handlers
type NotificationHandler interface {
	RegisterRoutes(router gin.IRouter)
	RegisterPublicRoutes(router gin.IRouter) // For SSE stream with token auth
	List(c *gin.Context)
	CountUnread(c *gin.Context)
	MarkAsRead(c *gin.Context)
	MarkAllAsRead(c *gin.Context)
	Delete(c *gin.Context)
	Stream(c *gin.Context)
	SSEToken(c *gin.Context)
	GetHub() *sse.Hub
	SetJWTService(jwtService auth_service.JWTService)
}

type notificationHandler struct {
	service    service.NotificationService
	hub        *sse.Hub
	jwtService auth_service.JWTService
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(service service.NotificationService) NotificationHandler {
	hub := sse.NewHub()
	go hub.Run() // Start the hub in a goroutine

	return &notificationHandler{
		service: service,
		hub:     hub,
	}
}

// SetJWTService sets the JWT service for SSE token generation
func (h *notificationHandler) SetJWTService(jwtService auth_service.JWTService) {
	h.jwtService = jwtService
}

// GetHub returns the SSE hub for broadcasting notifications
func (h *notificationHandler) GetHub() *sse.Hub {
	return h.hub
}

// RegisterRoutes registers notification routes (protected - requires auth)
func (h *notificationHandler) RegisterRoutes(router gin.IRouter) {
	notifications := router.Group("/notifications")
	{
		notifications.GET("", h.List)
		notifications.GET("/sse-token", h.SSEToken)
		notifications.GET("/unread-count", h.CountUnread)
		notifications.POST("/:id/read", h.MarkAsRead)
		notifications.POST("/read-all", h.MarkAllAsRead)
		notifications.DELETE("/:id", h.Delete)
	}
}

// RegisterPublicRoutes registers public notification routes (SSE stream with token auth)
func (h *notificationHandler) RegisterPublicRoutes(router gin.IRouter) {
	router.GET("/notifications/stream", h.Stream)
}

// List returns notifications for the current user
// @Summary List notifications
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Param read query bool false "Filter by read status"
// @Success 200 {array} model.NotificationResponse
// @Failure 401 {object} errors.AppError
// @Router /notifications [get]
func (h *notificationHandler) List(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	filter := &model.NotificationFilter{
		Limit:  50,
		Offset: 0,
	}

	// Parse query params
	if limit := c.Query("limit"); limit != "" {
		var l int
		if _, err := c.GetQuery("limit"); err {
			l = 50 // default
		}
		if l > 0 && l <= 100 {
			filter.Limit = l
		}
	}

	notifications, err := h.service.List(c.Request.Context(), clinicID.(uuid.UUID), userID.(uuid.UUID), filter)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, notifications)
}

// CountUnread returns the count of unread notifications
// @Summary Count unread notifications
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]int64
// @Failure 401 {object} errors.AppError
// @Router /notifications/unread-count [get]
func (h *notificationHandler) CountUnread(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	count, err := h.service.CountUnread(c.Request.Context(), clinicID.(uuid.UUID), userID.(uuid.UUID))
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

// MarkAsRead marks a notification as read
// @Summary Mark notification as read
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /notifications/{id}/read [post]
func (h *notificationHandler) MarkAsRead(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid notification ID")))
		return
	}

	if err := h.service.MarkAsRead(c.Request.Context(), id, clinicID.(uuid.UUID)); err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

// MarkAllAsRead marks all notifications as read
// @Summary Mark all notifications as read
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 401 {object} errors.AppError
// @Router /notifications/read-all [post]
func (h *notificationHandler) MarkAllAsRead(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	if err := h.service.MarkAllAsRead(c.Request.Context(), clinicID.(uuid.UUID), userID.(uuid.UUID)); err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read"})
}

// Delete deletes a notification
// @Summary Delete notification
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /notifications/{id} [delete]
func (h *notificationHandler) Delete(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithMessage("Invalid notification ID")))
		return
	}

	if err := h.service.Delete(c.Request.Context(), id, clinicID.(uuid.UUID)); err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification deleted"})
}

// SSEToken generates a short-lived token for SSE connections
// @Summary Generate SSE token
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 401 {object} errors.AppError
// @Router /notifications/sse-token [get]
func (h *notificationHandler) SSEToken(c *gin.Context) {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")

	if h.jwtService == nil {
		c.JSON(http.StatusInternalServerError, errors.ToResponse(errors.ErrInternalInstance.WithMessage("JWT service not configured")))
		return
	}

	token, err := h.jwtService.GenerateSSEToken(userID.(uuid.UUID), clinicID.(uuid.UUID))
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// Stream establishes an SSE connection for real-time notifications
// @Summary Stream notifications via SSE
// @Tags Notifications
// @Security BearerAuth
// @Produce text/event-stream
// @Param token query string false "SSE token for authentication"
// @Success 200 {string} string "SSE stream"
// @Failure 401 {object} errors.AppError
// @Router /notifications/stream [get]
func (h *notificationHandler) Stream(c *gin.Context) {
	var clinicID, userID uuid.UUID

	// Try to get from context (cookie/header auth)
	if cID, exists := c.Get("clinic_id"); exists {
		clinicID = cID.(uuid.UUID)
	}
	if uID, exists := c.Get("user_id"); exists {
		userID = uID.(uuid.UUID)
	}

	// If not authenticated via middleware, try token from query param
	if clinicID == uuid.Nil || userID == uuid.Nil {
		token := c.Query("token")
		if token != "" && h.jwtService != nil {
			claims, err := h.jwtService.ValidateSSEToken(token)
			if err == nil {
				clinicID = claims.ClinicID
				userID = claims.UserID
			}
		}
	}

	// Still not authenticated? Return error
	if clinicID == uuid.Nil || userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance.WithMessage("Authentication required")))
		return
	}

	// Set SSE headers BEFORE any writes
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// Create client with unique ID
	clientID := fmt.Sprintf("%s-%s-%d", clinicID.String(), userID.String(), time.Now().UnixNano())
	client := &sse.Client{
		ID:       clientID,
		ClinicID: clinicID,
		UserID:   userID,
		Channel:  make(chan []byte, 64),
	}

	// Register client
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	// Send initial connection event
	_, err := fmt.Fprintf(c.Writer, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	if err != nil {
		return
	}
	c.Writer.Flush()

	// Keep connection alive with manual loop
	ctx := c.Request.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send keepalive comment
			_, err := fmt.Fprintf(c.Writer, ": keepalive\n\n")
			if err != nil {
				return
			}
			c.Writer.Flush()
		case data, ok := <-client.Channel:
			if !ok {
				return
			}
			_, err := c.Writer.Write(data)
			if err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}
