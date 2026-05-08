package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// WorkerConfig holds all worker-related configurations
// These can be changed via environment variables without rebuilding
type WorkerConfig struct {
	// Reminder Worker
	ReminderWorkerEnabled  bool
	ReminderWorkerInterval time.Duration

	// Reminder Thresholds
	Reminder48hEnabled bool
	Reminder48hBefore  time.Duration
	Reminder24hEnabled bool
	Reminder24hBefore  time.Duration
	Reminder2hEnabled  bool
	Reminder2hBefore   time.Duration

	// Timeout Worker - marks appointments as no_response if patient doesn't confirm
	TimeoutWorkerEnabled        bool
	TimeoutWorkerInterval       time.Duration
	ConfirmationTimeoutDuration time.Duration

	// Payment Reminder Worker - monthly payment due-date reminders for session
	// plans. The kill switches are per-channel so operations can shut down
	// WhatsApp while leaving email working (or vice versa) without a deploy.
	PaymentReminderWhatsAppEnabled bool
	PaymentReminderEmailEnabled    bool

	// Message Sender
	MessageRateLimitSeconds int
	MessageRetryMax         int
	MessageRetryDelay       time.Duration

	// WhatsApp Rate Limiting & Message Segmentation
	MinDelayBetweenPatientsSeconds int
	MinDelayBetweenMessagesSeconds int
	MaxDelayBetweenMessagesSeconds int
	MessageSegmentationEnabled     bool
	// ResponseDebounceSeconds is the idle window (in seconds) to wait before
	// processing a patient's message with the AI Agent. If a new message
	// arrives within this window the timer resets, preventing immediate bot
	// responses that can trigger WhatsApp account blocking.
	ResponseDebounceSeconds int

	// Redis
	RedisURL string

	// RabbitMQ
	RabbitMQURL   string
	RabbitMQQueue string
	RabbitMQDLQ   string

	// AI Agent Configuration
	AIAgentEnabled        bool
	AIAgentProvider       string // openrouter, gemini, claude, openai
	AIAgentModel          string
	AIAgentFallbackModels []string // List of fallback models to try in order
	AIAgentMaxTokens      int
	AIAgentTemperature    float32
	// AIAgentHandoffTemperature is the stricter temperature used only when
	// the AI runs in handoff mode (clinic with action tool disabled, AI
	// gathering context for the human team). Lower than AIAgentTemperature
	// because handoff requires strict rule-following.
	AIAgentHandoffTemperature float32
	AIAgentTimeout            time.Duration
	ConversationMaxTurns      int
	ConversationRedisTTL      time.Duration
	ConversationPostgresTTL   time.Duration
	// ConversationPromptHistoryTurns = how many past turns (user+assistant
	// mixed) are injected into the LLM prompt. Different from MaxTurns, which
	// is the storage cap. Lower values reduce "memory-faulty" hallucination
	// on cheaper models; higher values give better continuity on strong ones.
	ConversationPromptHistoryTurns int

	// AI Agent Feature Flags (gradual rollout)
	AIAgentSchedulingEnabled bool
	AIAgentConfirmEnabled    bool
	AIAgentCancelEnabled     bool
	AIAgentRescheduleEnabled bool
	AIAgentRemindersEnabled  bool
}

// NewWorkerConfig creates a new worker configuration from environment variables
func NewWorkerConfig() *WorkerConfig {
	return &WorkerConfig{
		// Reminder Worker
		ReminderWorkerEnabled:  getEnvAsBoolW("REMINDER_WORKER_ENABLED", true),
		ReminderWorkerInterval: getEnvAsDurationW("REMINDER_WORKER_INTERVAL", 15*time.Minute),

		// Reminder Thresholds
		Reminder48hEnabled: getEnvAsBoolW("REMINDER_48H_ENABLED", true),
		Reminder48hBefore:  getEnvAsDurationW("REMINDER_48H_BEFORE", 48*time.Hour),
		Reminder24hEnabled: getEnvAsBoolW("REMINDER_24H_ENABLED", true),
		Reminder24hBefore:  getEnvAsDurationW("REMINDER_24H_BEFORE", 24*time.Hour),
		Reminder2hEnabled:  getEnvAsBoolW("REMINDER_2H_ENABLED", true),
		Reminder2hBefore:   getEnvAsDurationW("REMINDER_2H_BEFORE", 2*time.Hour),

		// Timeout Worker
		TimeoutWorkerEnabled:        getEnvAsBoolW("TIMEOUT_WORKER_ENABLED", true),
		TimeoutWorkerInterval:       getEnvAsDurationW("TIMEOUT_WORKER_INTERVAL", 5*time.Minute),
		ConfirmationTimeoutDuration: getEnvAsDurationW("CONFIRMATION_TIMEOUT_DURATION", 16*time.Hour),

		// Payment Reminder kill switches (default on; ops flips to false to shut down a channel)
		PaymentReminderWhatsAppEnabled: getEnvAsBoolW("PAYMENT_REMINDER_WHATSAPP_ENABLED", true),
		PaymentReminderEmailEnabled:    getEnvAsBoolW("PAYMENT_REMINDER_EMAIL_ENABLED", true),

		// Message Sender
		MessageRateLimitSeconds: getEnvAsIntW("MESSAGE_RATE_LIMIT_SECONDS", 1),
		MessageRetryMax:         getEnvAsIntW("MESSAGE_RETRY_MAX", 3),
		MessageRetryDelay:       getEnvAsDurationW("MESSAGE_RETRY_DELAY", 1*time.Minute),

		// WhatsApp Rate Limiting & Message Segmentation
		MinDelayBetweenPatientsSeconds: getEnvAsIntW("WHATSAPP_MIN_DELAY_BETWEEN_PATIENTS_SECONDS", 60),
		MinDelayBetweenMessagesSeconds: getEnvAsIntW("WHATSAPP_MIN_DELAY_BETWEEN_MESSAGES_SECONDS", 3),
		MaxDelayBetweenMessagesSeconds: getEnvAsIntW("WHATSAPP_MAX_DELAY_BETWEEN_MESSAGES_SECONDS", 10),
		MessageSegmentationEnabled:     getEnvAsBoolW("WHATSAPP_MESSAGE_SEGMENTATION_ENABLED", true),
		ResponseDebounceSeconds:        getEnvAsIntW("WHATSAPP_RESPONSE_DEBOUNCE_SECONDS", 25),

		// Redis
		RedisURL: getEnvW("REDIS_URL", "redis://localhost:6379"),

		// RabbitMQ
		RabbitMQURL:   getEnvW("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQQueue: getEnvW("RABBITMQ_QUEUE", "whatsapp_messages"),
		RabbitMQDLQ:   getEnvW("RABBITMQ_DLQ", "whatsapp_messages_dlq"),

		// AI Agent Configuration
		AIAgentEnabled:  getEnvAsBoolW("AI_AGENT_ENABLED", false),
		AIAgentProvider: getEnvW("AI_AGENT_PROVIDER", "openrouter"),
		AIAgentModel:    getEnvW("AI_AGENT_MODEL", "google/gemini-2.0-flash-001"),
		AIAgentFallbackModels: getEnvAsSliceW("AI_AGENT_FALLBACK_MODELS", []string{
			"google/gemini-2.0-flash-lite-001",
			"qwen/qwen-2.5-72b-instruct",
		}),
		AIAgentMaxTokens:               getEnvAsIntW("AI_AGENT_MAX_TOKENS", 2048),
		AIAgentTemperature:             getEnvAsFloat32W("AI_AGENT_TEMPERATURE", 0.5),
		AIAgentHandoffTemperature:      getEnvAsFloat32W("AI_AGENT_HANDOFF_TEMPERATURE", 0.3),
		AIAgentTimeout:                 getEnvAsDurationW("AI_AGENT_TIMEOUT", 30*time.Second),
		ConversationMaxTurns:           getEnvAsIntW("CONVERSATION_MAX_TURNS", 30),
		ConversationRedisTTL:           getEnvAsDurationW("CONVERSATION_REDIS_TTL", 30*time.Minute),
		ConversationPostgresTTL:        getEnvAsDurationW("CONVERSATION_POSTGRES_TTL", 90*24*time.Hour),
		ConversationPromptHistoryTurns: getEnvAsIntW("CONVERSATION_PROMPT_HISTORY_TURNS", 15),

		// AI Agent Feature Flags (all disabled by default for gradual rollout)
		AIAgentSchedulingEnabled: getEnvAsBoolW("AI_AGENT_SCHEDULING_ENABLED", false),
		AIAgentConfirmEnabled:    getEnvAsBoolW("AI_AGENT_CONFIRM_ENABLED", false),
		AIAgentCancelEnabled:     getEnvAsBoolW("AI_AGENT_CANCEL_ENABLED", false),
		AIAgentRescheduleEnabled: getEnvAsBoolW("AI_AGENT_RESCHEDULE_ENABLED", false),
		AIAgentRemindersEnabled:  getEnvAsBoolW("AI_AGENT_REMINDERS_ENABLED", false),
	}
}

// IsRabbitMQConfigured returns true if RabbitMQ URL is configured
func (c *WorkerConfig) IsRabbitMQConfigured() bool {
	return c.RabbitMQURL != ""
}

// GetReminderThresholds returns all enabled reminder thresholds
func (c *WorkerConfig) GetReminderThresholds() []ReminderThreshold {
	var thresholds []ReminderThreshold

	if c.Reminder48hEnabled {
		thresholds = append(thresholds, ReminderThreshold{
			Name:     "48h",
			Duration: c.Reminder48hBefore,
			Type:     "reminder_48h",
		})
	}

	if c.Reminder24hEnabled {
		thresholds = append(thresholds, ReminderThreshold{
			Name:     "24h",
			Duration: c.Reminder24hBefore,
			Type:     "reminder_24h",
		})
	}

	if c.Reminder2hEnabled {
		thresholds = append(thresholds, ReminderThreshold{
			Name:     "2h",
			Duration: c.Reminder2hBefore,
			Type:     "reminder_2h",
		})
	}

	return thresholds
}

// ReminderThreshold represents a reminder threshold configuration
type ReminderThreshold struct {
	Name     string
	Duration time.Duration
	Type     string
}

// Helper functions (with W suffix to avoid conflict with main config)

func getEnvW(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsIntW(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBoolW(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvAsDurationW(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvAsFloat32W(key string, defaultValue float32) float32 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 32); err == nil {
			return float32(floatValue)
		}
	}
	return defaultValue
}

// getEnvAsSliceW reads an environment variable as a comma-separated list
func getEnvAsSliceW(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma and trim whitespace
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}
