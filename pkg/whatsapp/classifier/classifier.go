package classifier

import (
	"context"
	"fmt"
	"time"

	"github.com/aseno/backend/pkg/whatsapp/model"
)

// ClassificationResult represents the result of message classification
type ClassificationResult struct {
	Intent     model.MessageIntent      `json:"intent"`
	Confidence float64                  `json:"confidence"`
	Reasoning  string                   `json:"reasoning"`
	Data       *model.ScheduleIntentData `json:"data,omitempty"` // Only for schedule intents
	Provider   string                   `json:"provider"`
	Model      string                   `json:"model"`
	Latency    time.Duration            `json:"latency"`
}

// SlotMatchResult represents the result of slot matching
type SlotMatchResult struct {
	SlotIndex  int           `json:"slot_index"`  // Index of the matched slot (0-based)
	Confidence float64       `json:"confidence"`  // Confidence of the match (0.0-1.0)
	Reasoning  string        `json:"reasoning"`   // Why this slot was chosen
	Provider   string        `json:"provider"`    // "gemini", "openai", "claude"
	Model      string        `json:"model"`       // Model name used
	Latency    time.Duration `json:"latency"`     // Time taken to match
}

// MessageClassifier interface for all AI providers
type MessageClassifier interface {
	Classify(ctx context.Context, content string) (*ClassificationResult, error)
	MatchSlot(ctx context.Context, message string, slots []model.SlotOption) (*SlotMatchResult, error)
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
- schedule: O paciente quer AGENDAR uma NOVA consulta/avaliacao (nao tem consulta marcada ainda)
- confirm: O paciente confirma que VAI comparecer a consulta (mesmo com ressalvas de horario)
- cancel: O paciente quer CANCELAR a consulta ou diz que NAO vai
- reschedule: O paciente quer REMARCAR para outro horario/dia
- question: O paciente esta fazendo uma PERGUNTA
- unknown: Nao e possivel determinar a intencao

IMPORTANTE:
- Se o paciente diz "quero marcar", "agendar", "fazer uma avaliacao", "primeira consulta" = schedule
- Se o paciente diz que vai mas pode atrasar um pouco = confirm
- Se o paciente pede para mudar o horario de consulta existente = reschedule
- Se ha duvida, prefira "unknown"

Se a intencao for "schedule", extraia tambem:
- tipo: "avaliacao", "retorno", "consulta", "exame" (se nao souber, use "consulta")
- especialidade: area medica se mencionada (fisioterapia, psicologia, etc), ou "" se nao mencionada
- urgencia: "normal" ou "urgente"

Mensagem do paciente: "%s"

Responda APENAS com JSON valido (sem markdown, sem crases):
Para schedule: {"intent": "schedule", "confidence": 0.92, "reasoning": "explicacao", "data": {"tipo": "avaliacao", "especialidade": "fisioterapia", "urgencia": "normal"}}
Para outros: {"intent": "confirm", "confidence": 0.95, "reasoning": "explicacao"}`
