package config

import (
	"os"
	"strconv"
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

	// Message Sender
	MessageRateLimitSeconds int
	MessageRetryMax         int
	MessageRetryDelay       time.Duration

	// RabbitMQ
	RabbitMQURL   string
	RabbitMQQueue string
	RabbitMQDLQ   string
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

		// Message Sender
		MessageRateLimitSeconds: getEnvAsIntW("MESSAGE_RATE_LIMIT_SECONDS", 1),
		MessageRetryMax:         getEnvAsIntW("MESSAGE_RETRY_MAX", 3),
		MessageRetryDelay:       getEnvAsDurationW("MESSAGE_RETRY_DELAY", 1*time.Minute),

		// RabbitMQ
		RabbitMQURL:   getEnvW("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQQueue: getEnvW("RABBITMQ_QUEUE", "whatsapp_messages"),
		RabbitMQDLQ:   getEnvW("RABBITMQ_DLQ", "whatsapp_messages_dlq"),
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
