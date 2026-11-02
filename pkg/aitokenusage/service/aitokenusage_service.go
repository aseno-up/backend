package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/aitokenusage/model"
	"github.com/aseno/backend/pkg/aitokenusage/repository"
)

// RecordInput is the data captured for one AI generation.
type RecordInput struct {
	ClinicID       uuid.UUID
	ConversationID uuid.UUID
	Provider       string
	Model          string
	InputTokens    int
	OutputTokens   int
	TotalTokens    int
	CostBRL        float64
}

// Service exposes per-call AI token usage operations.
type Service interface {
	Record(ctx context.Context, in RecordInput) error
	SummaryByModel(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.ModelSummary, error)
	SummaryByClinicAndModel(ctx context.Context, start, end time.Time) ([]model.ClinicModelSummary, error)
}

type svc struct {
	repo repository.Repository
}

// NewService builds a new AI token usage service.
func NewService(r repository.Repository) Service {
	return &svc{repo: r}
}

// Record persists one usage row asynchronously. Returns nil immediately so the
// caller (the user-facing AI flow) is never blocked or failed by accounting.
func (s *svc) Record(_ context.Context, in RecordInput) error {
	now := time.Now()
	row := &model.AITokenUsage{
		ClinicID:       in.ClinicID,
		ConversationID: in.ConversationID,
		OccurredAt:     now,
		Date:           now.Truncate(24 * time.Hour),
		Provider:       in.Provider,
		Model:          in.Model,
		InputTokens:    in.InputTokens,
		OutputTokens:   in.OutputTokens,
		TotalTokens:    in.TotalTokens,
		CostBRL:        in.CostBRL,
	}

	go func() {
		if err := s.repo.Create(context.Background(), row); err != nil {
			logger.Printf("[AITokenUsage] Failed to record usage for clinic %s: %v", in.ClinicID, err)
		}
	}()
	return nil
}

func (s *svc) SummaryByModel(ctx context.Context, clinicID uuid.UUID, start, end time.Time) ([]model.ModelSummary, error) {
	return s.repo.SummaryByModel(ctx, clinicID, start, end)
}

func (s *svc) SummaryByClinicAndModel(ctx context.Context, start, end time.Time) ([]model.ClinicModelSummary, error) {
	return s.repo.SummaryByClinicAndModel(ctx, start, end)
}
