package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/systemlog/model"
	"github.com/aseno/backend/pkg/systemlog/repository"
)

// SystemLogService defines the interface for system logging
type SystemLogService interface {
	Log(ctx context.Context, entry SystemLogEntry) error
	List(ctx context.Context, filter *model.SystemLogFilter) (*model.SystemLogResponse, error)
}

// SystemLogEntry represents a structured system log entry
type SystemLogEntry struct {
	ClinicID *uuid.UUID
	Service  string
	Event    string
	Status   model.LogStatus
	Payload  interface{}
	Error    string
	Duration int
}

type systemLogService struct {
	repo repository.SystemLogRepository
}

// NewSystemLogService creates a new system log service
func NewSystemLogService(repo repository.SystemLogRepository) SystemLogService {
	return &systemLogService{repo: repo}
}

func (s *systemLogService) Log(ctx context.Context, entry SystemLogEntry) error {
	log := &model.SystemLog{
		ClinicID: entry.ClinicID,
		Service:  entry.Service,
		Event:    entry.Event,
		Status:   entry.Status,
		Error:    entry.Error,
		Duration: entry.Duration,
	}

	if entry.Payload != nil {
		data, err := json.Marshal(entry.Payload)
		if err == nil {
			log.Payload = data
		}
	}

	// Log asynchronously to avoid blocking workers
	go func() {
		if err := s.repo.Create(context.Background(), log); err != nil {
			logger.Printf("[SystemLog] Failed to save system log: %v", err)
		}
	}()

	return nil
}

func (s *systemLogService) List(ctx context.Context, filter *model.SystemLogFilter) (*model.SystemLogResponse, error) {
	return s.repo.List(ctx, filter)
}
