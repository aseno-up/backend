package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/notification/model"
	"github.com/aseno/backend/pkg/notification/service"
)

// NotificationHandler defines the interface for notification handlers
type NotificationHandler interface {
	RegisterRoutes(router gin.IRouter)
	List(c *gin.Context)
	CountUnread(c *gin.Context)
	MarkAsRead(c *gin.Context)
	MarkAllAsRead(c *gin.Context)
	Delete(c *gin.Context)
}

type notificationHandler struct {
	service service.NotificationService
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(service service.NotificationService) NotificationHandler {
	return &notificationHandler{service: service}
}

// RegisterRoutes registers notification routes
func (h *notificationHandler) RegisterRoutes(router gin.IRouter) {
	notifications := router.Group("/notifications")
	{
		notifications.GET("", h.List)
		notifications.GET("/unread-count", h.CountUnread)
		notifications.POST("/:id/read", h.MarkAsRead)
		notifications.POST("/read-all", h.MarkAllAsRead)
		notifications.DELETE("/:id", h.Delete)
	}
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
