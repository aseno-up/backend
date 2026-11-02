package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/aseno/backend/internal/errors"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
)

var planHierarchy = map[string]int{
	"basic":        0,
	"professional": 1,
	"advanced":     2,
}

// RequirePlan returns a middleware that ensures the clinic's plan meets the minimum requirement.
// Trial users have full access regardless of plan.
// The clinic must already be in the context (set by ValidateTenant or ValidateSubscription).
func RequirePlan(minimumPlan string) gin.HandlerFunc {
	return func(c *gin.Context) {
		clinicVal, exists := c.Get("clinic")
		if !exists {
			c.JSON(http.StatusForbidden, errors.ToResponse(
				errors.ErrUpgradeRequiredInstance.WithMessage("Não foi possível verificar o plano"),
			))
			c.Abort()
			return
		}

		clinic, ok := clinicVal.(*clinic_model.Clinic)
		if !ok || clinic.Account == nil {
			c.JSON(http.StatusForbidden, errors.ToResponse(
				errors.ErrUpgradeRequiredInstance.WithMessage("Não foi possível verificar o plano"),
			))
			c.Abort()
			return
		}

		// Trial users have full access
		if clinic.Account.SubscriptionStatus == "trial" {
			c.Next()
			return
		}

		// Check plan level
		currentPlan := "basic"
		if clinic.Account.Plan != nil {
			currentPlan = clinic.Account.Plan.Slug
		}

		currentLevel, currentOk := planHierarchy[currentPlan]
		requiredLevel, requiredOk := planHierarchy[minimumPlan]

		if !currentOk || !requiredOk || currentLevel < requiredLevel {
			c.JSON(http.StatusForbidden, errors.ToResponse(
				errors.ErrUpgradeRequiredInstance,
			))
			c.Abort()
			return
		}

		c.Next()
	}
}
