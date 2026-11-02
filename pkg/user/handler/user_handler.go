package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/user/model"
	"github.com/aseno/backend/pkg/user/service"
)

// UserHandler defines the interface for user handlers
type UserHandler interface {
	RegisterRoutes(router gin.IRouter)
	RegisterMeRoutes(router gin.IRouter)
	ListUsers(c *gin.Context)
	GetUser(c *gin.Context)
	UpdateUser(c *gin.Context)
	UpdateRole(c *gin.Context)
	DeactivateUser(c *gin.Context)
	ReactivateUser(c *gin.Context)
	RemoveUser(c *gin.Context)
	BulkRemoveUsers(c *gin.Context)
	GetMyTutorials(c *gin.Context)
	MarkMyTutorialCompleted(c *gin.Context)
	ResetMyTutorials(c *gin.Context)
}

// userHandler implements UserHandler
type userHandler struct {
	userService service.UserService
	auditSvc    audit_service.AuditService
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService service.UserService, auditSvc audit_service.AuditService) UserHandler {
	return &userHandler{
		userService: userService,
		auditSvc:    auditSvc,
	}
}

// RegisterRoutes registers user routes (Admin only)
func (h *userHandler) RegisterRoutes(router gin.IRouter) {
	users := router.Group("/users")
	{
		users.GET("", h.ListUsers)
		users.GET("/:id", h.GetUser)
		users.PUT("/:id", h.UpdateUser)
		users.PATCH("/:id/role", h.UpdateRole)
		users.POST("/:id/deactivate", h.DeactivateUser)
		users.POST("/:id/reactivate", h.ReactivateUser)
		users.DELETE("/:id", h.RemoveUser)
		users.POST("/bulk-delete", h.BulkRemoveUsers)
	}
}

// RegisterMeRoutes registers routes scoped to the currently authenticated user.
// Mounted outside the subscription gate so the in-app tour keeps working even
// when the clinic's subscription is expired.
func (h *userHandler) RegisterMeRoutes(router gin.IRouter) {
	me := router.Group("/me")
	{
		me.GET("/tutorials", h.GetMyTutorials)
		me.POST("/tutorials/:page/complete", h.MarkMyTutorialCompleted)
		me.DELETE("/tutorials", h.ResetMyTutorials)
	}
}

// ListUsers handles listing users for a clinic
// @Summary List users
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} pagination.Response[*model.UserResponse]
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Router /users [get]
func (h *userHandler) ListUsers(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var filter pagination.Filter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	resp, err := h.userService.ListUsers(c.Request.Context(), clinicID.(uuid.UUID), filter)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetUser handles getting a user by ID
// @Summary Get user by ID
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} model.UserResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /users/{id} [get]
func (h *userHandler) GetUser(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Invalid user ID")))
		return
	}

	response, err := h.userService.GetUser(c.Request.Context(), clinicID.(uuid.UUID), userID)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateUser handles updating a user
// @Summary Update a user
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body model.UpdateUserRequest true "Update data"
// @Success 200 {object} model.UserResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /users/{id} [put]
func (h *userHandler) UpdateUser(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Invalid user ID")))
		return
	}

	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	response, err := h.userService.UpdateUser(c.Request.Context(), clinicID.(uuid.UUID), userID, currentUserID.(uuid.UUID), &req)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateRoleRequest represents the request to change user role
type UpdateRoleRequest struct {
	Role model.UserRole `json:"role" binding:"required,oneof=admin professional receptionist"`
}

// UpdateRole handles changing a user's role
// @Summary Change user role
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body UpdateRoleRequest true "New role"
// @Success 200 {object} model.UserResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /users/{id}/role [patch]
func (h *userHandler) UpdateRole(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Invalid user ID")))
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	// Get old data for audit
	var oldData interface{}
	if h.auditSvc != nil {
		old, _ := h.userService.GetUser(c.Request.Context(), clinicID.(uuid.UUID), userID)
		oldData = old
	}

	response, err := h.userService.UpdateRole(c.Request.Context(), clinicID.(uuid.UUID), userID, currentUserID.(uuid.UUID), req.Role)
	if err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityUser, userID.String(), oldData, response)
	}

	c.JSON(http.StatusOK, response)
}

// DeactivateUser handles deactivating a user
// @Summary Deactivate a user
// @Tags Users
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /users/{id}/deactivate [post]
func (h *userHandler) DeactivateUser(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Invalid user ID")))
		return
	}

	if err := h.userService.DeactivateUser(c.Request.Context(), clinicID.(uuid.UUID), userID, currentUserID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionUpdate, audit_model.AuditEntityUser, userID.String(), nil, gin.H{"action": "deactivate"})
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deactivated successfully"})
}

// ReactivateUser handles reactivating a user
// @Summary Reactivate a user
// @Tags Users
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /users/{id}/reactivate [post]
func (h *userHandler) ReactivateUser(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Invalid user ID")))
		return
	}

	if err := h.userService.ReactivateUser(c.Request.Context(), clinicID.(uuid.UUID), userID); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User reactivated successfully"})
}

// RemoveUser handles removing a user
// @Summary Remove a user
// @Tags Users
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /users/{id} [delete]
func (h *userHandler) RemoveUser(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrBadRequestInstance.WithMessage("Invalid user ID")))
		return
	}

	if err := h.userService.RemoveUser(c.Request.Context(), clinicID.(uuid.UUID), userID, currentUserID.(uuid.UUID)); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.LogFromGin(c, audit_model.AuditActionDelete, audit_model.AuditEntityUser, userID.String(), nil, nil)
	}

	c.JSON(http.StatusOK, gin.H{"message": "User removed successfully"})
}

// BulkRemoveUsers handles removing multiple users
func (h *userHandler) BulkRemoveUsers(c *gin.Context) {
	clinicID, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return
	}

	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ToResponse(errors.ErrValidationInstance.WithError(err)))
		return
	}

	if err := h.userService.BulkRemoveUsers(c.Request.Context(), clinicID.(uuid.UUID), currentUserID.(uuid.UUID), req.IDs); err != nil {
		statusCode := errors.GetStatusCode(err)
		c.JSON(statusCode, errors.ToResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Users removed successfully"})
}

// tutorialsResponse is returned by every /me/tutorials endpoint.
type tutorialsResponse struct {
	Pages []string `json:"pages"`
}

// GetMyTutorials returns the tutorial pages the current user already finished.
// @Summary List completed tutorials for the current user
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Success 200 {object} tutorialsResponse
// @Failure 401 {object} errors.AppError
// @Router /me/tutorials [get]
func (h *userHandler) GetMyTutorials(c *gin.Context) {
	clinicID, userID, ok := requireMe(c)
	if !ok {
		return
	}

	pages, err := h.userService.GetTutorialsCompleted(c.Request.Context(), clinicID, userID)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, tutorialsResponse{Pages: pages})
}

// MarkMyTutorialCompleted records that the current user finished a tutorial page.
// @Summary Mark a tutorial page as completed
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param page path string true "Tutorial page identifier (e.g. planos-sessao)"
// @Success 200 {object} tutorialsResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /me/tutorials/{page}/complete [post]
func (h *userHandler) MarkMyTutorialCompleted(c *gin.Context) {
	clinicID, userID, ok := requireMe(c)
	if !ok {
		return
	}

	page := c.Param("page")
	pages, err := h.userService.MarkTutorialCompleted(c.Request.Context(), clinicID, userID, page)
	if err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.JSON(http.StatusOK, tutorialsResponse{Pages: pages})
}

// ResetMyTutorials clears the user's completed tutorials (they'll see the tour again).
// @Summary Reset completed tutorials for the current user
// @Tags Users
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 401 {object} errors.AppError
// @Router /me/tutorials [delete]
func (h *userHandler) ResetMyTutorials(c *gin.Context) {
	clinicID, userID, ok := requireMe(c)
	if !ok {
		return
	}

	if err := h.userService.ResetTutorials(c.Request.Context(), clinicID, userID); err != nil {
		c.JSON(errors.GetStatusCode(err), errors.ToResponse(err))
		return
	}
	c.Status(http.StatusNoContent)
}

// requireMe extracts (clinic_id, user_id) from gin context or writes 401 and returns ok=false.
func requireMe(c *gin.Context) (clinicID uuid.UUID, userID uuid.UUID, ok bool) {
	clinicRaw, exists := c.Get("clinic_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return uuid.Nil, uuid.Nil, false
	}
	userRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
		return uuid.Nil, uuid.Nil, false
	}
	return clinicRaw.(uuid.UUID), userRaw.(uuid.UUID), true
}
