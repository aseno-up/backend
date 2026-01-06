package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/aseno/backend/internal/database"
)

// HealthHandler defines the interface for health check endpoints
type HealthHandler interface {
	RegisterRoutes(router gin.IRouter)
	Health(c *gin.Context)
	Ready(c *gin.Context)
}

// healthHandler implements HealthHandler
type healthHandler struct {
	db        *database.Database
	startTime time.Time
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *database.Database) HealthHandler {
	return &healthHandler{
		db:        db,
		startTime: time.Now(),
	}
}

// RegisterRoutes registers health check routes
func (h *healthHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/health", h.Health)
	router.GET("/ready", h.Ready)
}

// Health returns basic health status
func (h *healthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).String(),
	})
}

// Ready returns readiness status including database connectivity
func (h *healthHandler) Ready(c *gin.Context) {
	checks := make(map[string]interface{})

	// Check database connection
	dbStatus := "ok"
	if err := h.db.Ping(); err != nil {
		dbStatus = "error: " + err.Error()
	}
	checks["database"] = dbStatus

	// Determine overall status
	status := http.StatusOK
	overallStatus := "ok"
	if dbStatus != "ok" {
		status = http.StatusServiceUnavailable
		overallStatus = "degraded"
	}

	c.JSON(status, gin.H{
		"status":    overallStatus,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).String(),
		"checks":    checks,
	})
}
