package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	account_model "github.com/aseno/backend/pkg/account/model"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	plan_model "github.com/aseno/backend/pkg/plan/model"
)

func setupPlanTestRouter(planSlug string, subscriptionStatus string, middleware gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		// Simulate authenticated user with clinic in context
		c.Set(ContextKeyClinicID, uuid.New())
		c.Set("clinic", &clinic_model.Clinic{
			ID: uuid.New(),
			Account: &account_model.Account{
				SubscriptionStatus: account_model.SubscriptionStatus(subscriptionStatus),
				Plan: &plan_model.Plan{
					Slug: planSlug,
				},
			},
		})
		c.Next()
	})

	r.Use(middleware)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	return r
}

func TestRequirePlan_BasicBlocked(t *testing.T) {
	r := setupPlanTestRouter("basic", "active", RequirePlan("professional"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "UPGRADE_REQUIRED")
}

func TestRequirePlan_ProfessionalAllowed(t *testing.T) {
	r := setupPlanTestRouter("professional", "active", RequirePlan("professional"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePlan_AdvancedAllowed(t *testing.T) {
	r := setupPlanTestRouter("advanced", "active", RequirePlan("professional"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePlan_TrialAllowed(t *testing.T) {
	r := setupPlanTestRouter("basic", "trial", RequirePlan("professional"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePlan_NoPlanBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Set(ContextKeyClinicID, uuid.New())
		c.Set("clinic", &clinic_model.Clinic{
			ID:      uuid.New(),
			Account: &account_model.Account{SubscriptionStatus: "active"},
		})
		c.Next()
	})
	r.Use(RequirePlan("professional"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePlan_NoClinicInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(RequirePlan("professional"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePlan_RequireAdvanced_ProfessionalBlocked(t *testing.T) {
	r := setupPlanTestRouter("professional", "active", RequirePlan("advanced"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePlan_RequireAdvanced_AdvancedAllowed(t *testing.T) {
	r := setupPlanTestRouter("advanced", "active", RequirePlan("advanced"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
