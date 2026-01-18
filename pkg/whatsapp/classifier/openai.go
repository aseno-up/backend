package classifier

import (
	"context"
	"fmt"

	"github.com/aseno/backend/pkg/whatsapp/model"
)

type openaiClassifier struct {
	// TODO: Implement when needed
}

// NewOpenAIClassifier creates a new OpenAI-based classifier
func NewOpenAIClassifier(cfg Config) (MessageClassifier, error) {
	return nil, fmt.Errorf("openai classifier not implemented yet - set AI_CLASSIFIER_PROVIDER=gemini")
}

func (o *openaiClassifier) Provider() string {
	return "openai"
}

func (o *openaiClassifier) Classify(ctx context.Context, content string) (*ClassificationResult, error) {
	return nil, fmt.Errorf("openai classifier not implemented")
}

func (o *openaiClassifier) MatchSlot(ctx context.Context, message string, slots []model.SlotOption) (*SlotMatchResult, error) {
	return nil, fmt.Errorf("openai slot matching not implemented yet")
}
