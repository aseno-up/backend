package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/portalaccount/service"
)

const (
	ContextKeyPortalPatientID = "portal_patient_id"
	ContextKeyPortalClinicID  = "portal_clinic_id"
	ContextKeyPortalEmail     = "portal_email"
)

type PortalMiddleware interface {
	Authenticate() gin.HandlerFunc
}

type portalMiddleware struct {
	portalSvc service.PortalService
}

func NewPortalMiddleware(portalSvc service.PortalService) PortalMiddleware {
	return &portalMiddleware{portalSvc: portalSvc}
}

func (m *portalMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		// Try cookie first
		if cookie, err := c.Cookie("portal_access_token"); err == nil && cookie != "" {
			tokenStr = cookie
		}

		// Try Authorization header
		if tokenStr == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sessão não encontrada. Faça login novamente."})
			c.Abort()
			return
		}

		claims, err := m.portalSvc.ValidateAccessToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sua sessão expirou. Faça login novamente."})
			c.Abort()
			return
		}

		c.Set(ContextKeyPortalPatientID, claims.PatientID)
		c.Set(ContextKeyPortalClinicID, claims.ClinicID)
		c.Set(ContextKeyPortalEmail, claims.Email)
		c.Next()
	}
}

// GetPortalPatientID extracts the patient ID from the context
func GetPortalPatientID(c *gin.Context) uuid.UUID {
	val, _ := c.Get(ContextKeyPortalPatientID)
	if id, ok := val.(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// GetPortalClinicID extracts the clinic ID from the context
func GetPortalClinicID(c *gin.Context) uuid.UUID {
	val, _ := c.Get(ContextKeyPortalClinicID)
	if id, ok := val.(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}
