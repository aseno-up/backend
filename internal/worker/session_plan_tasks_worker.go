package worker

import (
	"context"
	"time"

	"github.com/aseno/backend/internal/logger"
)

// WorkerTask is a function that runs on each tick of the worker.
// Tasks are registered via RegisterTask and executed sequentially.
type WorkerTask struct {
	Name string
	Fn   func(ctx context.Context) error
}

// SessionPlanTasksWorker runs registered tasks on a configurable interval.
// Replaces the old SessionPlanWorker that generated appointments.
// Designed to be extended with tasks for notifications, billing reminders, etc.
type SessionPlanTasksWorker struct {
	interval time.Duration
	stopCh   chan struct{}
	tasks    []WorkerTask
}

// NewSessionPlanTasksWorker creates a new tasks worker
func NewSessionPlanTasksWorker(interval time.Duration) *SessionPlanTasksWorker {
	return &SessionPlanTasksWorker{
		interval: interval,
		stopCh:   make(chan struct{}),
		tasks:    make([]WorkerTask, 0),
	}
}

// RegisterTask adds a task to be executed on each tick
func (w *SessionPlanTasksWorker) RegisterTask(name string, fn func(ctx context.Context) error) {
	w.tasks = append(w.tasks, WorkerTask{Name: name, Fn: fn})
}

// Start starts the worker
func (w *SessionPlanTasksWorker) Start() {
	logger.Printf("Starting session plan tasks worker (interval: %v, tasks: %d)", w.interval, len(w.tasks))

	if len(w.tasks) == 0 {
		logger.Println("Session plan tasks worker has no registered tasks, waiting for stop signal")
		<-w.stopCh
		logger.Println("Session plan tasks worker stopped")
		return
	}

	// Run immediately on start
	w.runTasks()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.runTasks()
		case <-w.stopCh:
			logger.Println("Session plan tasks worker stopped")
			return
		}
	}
}

// Stop stops the worker
func (w *SessionPlanTasksWorker) Stop() {
	close(w.stopCh)
}

// runTasks executes all registered tasks sequentially
func (w *SessionPlanTasksWorker) runTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for _, task := range w.tasks {
		logger.Printf("Running task: %s", task.Name)
		if err := task.Fn(ctx); err != nil {
			logger.Printf("Task %s failed: %v", task.Name, err)
		}
	}
}
