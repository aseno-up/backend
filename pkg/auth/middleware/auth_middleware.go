package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/auth/service"
	user_model "github.com/aseno/backend/pkg/user/model"
)

// Context keys for storing auth data
const (
	ContextKeyUserID   = "user_id"
	ContextKeyClinicID = "clinic_id"
	ContextKeyUserRole = "user_role"
	ContextKeyEmail    = "email"
)

// AuthMiddleware defines the interface for authentication middleware
type AuthMiddleware interface {
	Authenticate() gin.HandlerFunc
	RequireRole(roles ...user_model.UserRole) gin.HandlerFunc
	RequireAdmin() gin.HandlerFunc
	RequireOwner() gin.HandlerFunc
}

// authMiddleware implements AuthMiddleware
type authMiddleware struct {
	jwtService service.JWTService
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtService service.JWTService) AuthMiddleware {
	return &authMiddleware{
		jwtService: jwtService,
	}
}

// Authenticate validates the JWT token and sets user context
func (m *authMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// 1. Try to get from Cookie
		cookie, err := c.Cookie("access_token")
		if err == nil && cookie != "" {
			tokenString = cookie
		}

		// 2. If no cookie, try Authorization header
		if tokenString == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					tokenString = parts[1]
				}
			}
		}

		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance.WithMessage("Missing authorization token")))
			c.Abort()
			return
		}

		// Validate token
		claims, err := m.jwtService.ValidateAccessToken(tokenString)
		if err != nil {
			statusCode := errors.GetStatusCode(err)
			c.JSON(statusCode, errors.ToResponse(err))
			c.Abort()
			return
		}

		// Set user context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyClinicID, claims.ClinicID)
		c.Set(ContextKeyUserRole, claims.Role)
		c.Set(ContextKeyEmail, claims.Email)

		c.Next()
	}
}

// RequireRole ensures the user has one of the specified roles
func (m *authMiddleware) RequireRole(roles ...user_model.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get(ContextKeyUserRole)
		if !exists {
			c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
			c.Abort()
			return
		}

		role := userRole.(user_model.UserRole)
		hasRole := false
		for _, r := range roles {
			if r == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, errors.ToResponse(errors.ErrForbiddenInstance.WithMessage("Insufficient permissions")))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin ensures the user has admin or owner role
func (m *authMiddleware) RequireAdmin() gin.HandlerFunc {
	return m.RequireRole(user_model.UserRoleAdmin, user_model.UserRoleOwner)
}

// RequireOwner ensures the user is the clinic owner
func (m *authMiddleware) RequireOwner() gin.HandlerFunc {
	return m.RequireRole(user_model.UserRoleOwner)
}

// OptionalAuth tries to authenticate but doesn't fail if no token is provided
func (m *authMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// 1. Try to get from Cookie
		cookie, err := c.Cookie("access_token")
		if err == nil && cookie != "" {
			tokenString = cookie
		}

		// 2. If no cookie, try Authorization header
		if tokenString == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					tokenString = parts[1]
				}
			}
		}

		if tokenString == "" {
			c.Next()
			return
		}

		claims, err := m.jwtService.ValidateAccessToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		// Set user context if token is valid
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyClinicID, claims.ClinicID)
		c.Set(ContextKeyUserRole, claims.Role)
		c.Set(ContextKeyEmail, claims.Email)

		c.Next()
	}
}
