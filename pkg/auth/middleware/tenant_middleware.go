package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
)

// TenantMiddleware defines the interface for tenant middleware
type TenantMiddleware interface {
	ValidateTenant() gin.HandlerFunc
	ValidateSubscription() gin.HandlerFunc
}

// tenantMiddleware implements TenantMiddleware
type tenantMiddleware struct {
	clinicRepo clinic_repository.ClinicRepository
}

// NewTenantMiddleware creates a new tenant middleware
func NewTenantMiddleware(clinicRepo clinic_repository.ClinicRepository) TenantMiddleware {
	return &tenantMiddleware{
		clinicRepo: clinicRepo,
	}
}

// ValidateTenant ensures the clinic (tenant) exists and is active
func (m *tenantMiddleware) ValidateTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		clinicIDVal, exists := c.Get(ContextKeyClinicID)
		if !exists {
			c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
			c.Abort()
			return
		}

		clinicID := clinicIDVal.(uuid.UUID)

		clinic, err := m.clinicRepo.GetByID(c.Request.Context(), clinicID)
		if err != nil {
			if errors.IsNotFound(err) {
				c.JSON(http.StatusNotFound, errors.ToResponse(errors.ErrTenantNotFoundInstance))
			} else {
				c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
			}
			c.Abort()
			return
		}

		// Store clinic in context for later use
		c.Set("clinic", clinic)

		c.Next()
	}
}

// ValidateSubscription ensures the clinic has an active subscription
func (m *tenantMiddleware) ValidateSubscription() gin.HandlerFunc {
	return func(c *gin.Context) {
		clinicIDVal, exists := c.Get(ContextKeyClinicID)
		if !exists {
			c.JSON(http.StatusUnauthorized, errors.ToResponse(errors.ErrUnauthorizedInstance))
			c.Abort()
			return
		}

		clinicID := clinicIDVal.(uuid.UUID)

		clinic, err := m.clinicRepo.GetByID(c.Request.Context(), clinicID)
		if err != nil {
			if errors.IsNotFound(err) {
				c.JSON(http.StatusNotFound, errors.ToResponse(errors.ErrTenantNotFoundInstance))
			} else {
				c.JSON(http.StatusInternalServerError, errors.ToResponse(err))
			}
			c.Abort()
			return
		}

		if !clinic.IsSubscriptionActive() {
			c.JSON(http.StatusForbidden, errors.ToResponse(
				errors.ErrForbiddenInstance.WithMessage("Subscription has expired. Please renew to continue using the service."),
			))
			c.Abort()
			return
		}

		// Store clinic in context for later use
		c.Set("clinic", clinic)

		c.Next()
	}
}
