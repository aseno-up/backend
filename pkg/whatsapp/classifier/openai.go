package classifier

import (
	"context"
	"fmt"
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
