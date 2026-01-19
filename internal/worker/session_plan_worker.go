package worker

import (
	"context"
	"github.com/aseno/backend/internal/logger"
	"time"

	"github.com/aseno/backend/pkg/sessionplan/service"
)

// SessionPlanWorker handles periodic appointment generation
type SessionPlanWorker struct {
	service  service.SessionPlanService
	interval time.Duration
	stopCh   chan struct{}
}

// NewSessionPlanWorker creates a new session plan worker
func NewSessionPlanWorker(svc service.SessionPlanService, interval time.Duration) *SessionPlanWorker {
	return &SessionPlanWorker{
		service:  svc,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the worker
func (w *SessionPlanWorker) Start() {
	logger.Printf("Starting session plan worker (interval: %v)", w.interval)

	// Run immediately on start
	w.runGeneration()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.runGeneration()
		case <-w.stopCh:
			logger.Println("Session plan worker stopped")
			return
		}
	}
}

// Stop stops the worker
func (w *SessionPlanWorker) Stop() {
	close(w.stopCh)
}

// runGeneration runs the appointment generation for all active plans
func (w *SessionPlanWorker) runGeneration() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Println("Running session plan appointment generation...")

	err := w.service.GenerateAllPending(ctx, 2) // Generate 2 weeks ahead
	if err != nil {
		logger.Printf("Error generating appointments: %v", err)
		return
	}

	logger.Println("Session plan appointment generation completed")
}
