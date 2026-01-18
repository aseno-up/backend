package classifier

import (
	"context"
	"fmt"

	"github.com/aseno/backend/pkg/whatsapp/model"
)

type claudeClassifier struct {
	// TODO: Implement when needed
}

// NewClaudeClassifier creates a new Claude-based classifier
func NewClaudeClassifier(cfg Config) (MessageClassifier, error) {
	return nil, fmt.Errorf("claude classifier not implemented yet - set AI_CLASSIFIER_PROVIDER=gemini")
}

func (c *claudeClassifier) Provider() string {
	return "claude"
}

func (c *claudeClassifier) Classify(ctx context.Context, content string) (*ClassificationResult, error) {
	return nil, fmt.Errorf("claude classifier not implemented")
}

func (c *claudeClassifier) MatchSlot(ctx context.Context, message string, slots []model.SlotOption) (*SlotMatchResult, error) {
	return nil, fmt.Errorf("claude slot matching not implemented yet")
}
