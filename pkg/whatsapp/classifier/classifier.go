package classifier

import (
	"context"
	"fmt"
	"time"

	"github.com/aseno/backend/pkg/whatsapp/model"
)

// ClassificationResult represents the result of message classification
type ClassificationResult struct {
	Intent     model.MessageIntent `json:"intent"`
	Confidence float64             `json:"confidence"`
	Reasoning  string              `json:"reasoning"`
	Provider   string              `json:"provider"`
	Model      string              `json:"model"`
	Latency    time.Duration       `json:"latency"`
}

// MessageClassifier interface for all AI providers
type MessageClassifier interface {
	Classify(ctx context.Context, content string) (*ClassificationResult, error)
	Provider() string
}

// Config for creating a classifier
type Config struct {
	Provider string
	Model    string
	APIKey   string
	Timeout  time.Duration
}

// NewClassifier factory that creates the classifier based on provider
func NewClassifier(cfg Config) (MessageClassifier, error) {
	switch cfg.Provider {
	case "gemini":
		return NewGeminiClassifier(cfg)
	case "claude":
		return NewClaudeClassifier(cfg)
	case "openai":
		return NewOpenAIClassifier(cfg)
	default:
		return nil, fmt.Errorf("unknown AI classifier provider: %s", cfg.Provider)
	}
}

// ClassificationPrompt is the shared prompt for all providers
const ClassificationPrompt = `Voce e um assistente de classificacao de mensagens de WhatsApp para uma clinica medica brasileira.

Classifique a intencao da mensagem do paciente em UMA das categorias:
- confirm: O paciente confirma que VAI comparecer a consulta (mesmo com ressalvas de horario)
- cancel: O paciente quer CANCELAR a consulta ou diz que NAO vai
- reschedule: O paciente quer REMARCAR para outro horario/dia
- question: O paciente esta fazendo uma PERGUNTA
- unknown: Nao e possivel determinar a intencao

IMPORTANTE:
- Se o paciente diz que vai mas pode atrasar um pouco, e "confirm"
- Se o paciente pede para mudar o horario, e "reschedule"
- Se ha duvida, prefira "unknown"

Mensagem do paciente: "%s"

Responda APENAS com JSON valido (sem markdown, sem crases):
{"intent": "confirm", "confidence": 0.95, "reasoning": "explicacao curta"}`
