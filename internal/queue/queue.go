package queue

import (
	"context"
	"encoding/json"
	"time"
)

// MessageJob represents a message to be sent via WhatsApp
type MessageJob struct {
	ID            string    `json:"id"`
	ClinicID      string    `json:"clinic_id"`
	AppointmentID string    `json:"appointment_id"`
	Instance      string    `json:"instance"`
	Phone         string    `json:"phone"`
	PatientName   string    `json:"patient_name"`
	Message       string    `json:"message"`
	MessageType   string    `json:"message_type"` // reminder_48h, reminder_24h, reminder_2h, confirmation
	Attempts      int       `json:"attempts"`
	MaxAttempts   int       `json:"max_attempts"`
	CreatedAt     time.Time `json:"created_at"`
	ScheduledAt   time.Time `json:"scheduled_at"`

	// Retry fields for exponential backoff
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	FirstFailAt *time.Time `json:"first_fail_at,omitempty"`
}

// ToJSON serializes the job to JSON
func (j *MessageJob) ToJSON() ([]byte, error) {
	return json.Marshal(j)
}

// FromJSON deserializes a job from JSON
func FromJSON(data []byte) (*MessageJob, error) {
	var job MessageJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// ShouldRetry returns true if the job should be retried
func (j *MessageJob) ShouldRetry() bool {
	if j.MaxAttempts == 0 {
		return true
	}
	return j.Attempts < j.MaxAttempts
}

// IsReadyForRetry returns true if it's time to process this retry
func (j *MessageJob) IsReadyForRetry() bool {
	if j.NextRetryAt == nil {
		return true
	}
	return time.Now().After(*j.NextRetryAt)
}

// TimeUntilRetry returns the duration until the next retry is ready
func (j *MessageJob) TimeUntilRetry() time.Duration {
	if j.NextRetryAt == nil {
		return 0
	}
	until := time.Until(*j.NextRetryAt)
	if until < 0 {
		return 0
	}
	return until
}

// MarkFailed updates the job with failure information for retry
func (j *MessageJob) MarkFailed(err string, nextRetryAt time.Time) {
	j.Attempts++
	j.LastError = err
	j.NextRetryAt = &nextRetryAt
	if j.FirstFailAt == nil {
		now := time.Now()
		j.FirstFailAt = &now
	}
}

// Queue defines the interface for message queue operations
// This abstraction allows switching between RabbitMQ, Redis, or other implementations
type Queue interface {
	// Connect establishes connection to the queue
	Connect(ctx context.Context) error

	// Close closes the connection
	Close() error

	// Publish publishes a message to the queue
	Publish(ctx context.Context, job *MessageJob) error

	// PublishWithDelay publishes a message with a delay before processing
	// Used for exponential backoff retry
	PublishWithDelay(ctx context.Context, job *MessageJob, delay time.Duration) error

	// Consume starts consuming messages from the queue
	// The handler is called for each message received
	Consume(ctx context.Context, handler func(job *MessageJob) error) error

	// PublishToDeadLetter moves a failed message to dead letter queue
	PublishToDeadLetter(ctx context.Context, job *MessageJob, reason string) error

	// IsConnected returns true if connected to the queue
	IsConnected() bool

	// QueueLength returns the number of messages in the queue
	QueueLength(ctx context.Context) (int, error)
}

// DeadLetterJob represents a failed message in the dead letter queue
type DeadLetterJob struct {
	MessageJob
	FailedAt time.Time `json:"failed_at"`
	Reason   string    `json:"reason"`
}

// RetryStrategy defines the interface for retry delay calculation
type RetryStrategy interface {
	// NextDelay calculates the next retry delay based on the attempt number
	NextDelay(attempt int) time.Duration
}

// exponentialBackoff implements RetryStrategy with exponential backoff and jitter
type exponentialBackoff struct {
	initialDelay time.Duration
	maxDelay     time.Duration
	multiplier   float64
	jitterFactor float64
}

// DefaultRetryStrategy returns a default exponential backoff strategy
func DefaultRetryStrategy() RetryStrategy {
	return &exponentialBackoff{
		initialDelay: 5 * time.Second,
		maxDelay:     10 * time.Minute,
		multiplier:   2.0,
		jitterFactor: 0.25,
	}
}

// NextDelay calculates the next retry delay with exponential backoff and jitter
func (e *exponentialBackoff) NextDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate base delay: initialDelay * multiplier^attempt
	delay := float64(e.initialDelay)
	for i := 0; i < attempt; i++ {
		delay *= e.multiplier
	}

	// Cap at max delay
	if delay > float64(e.maxDelay) {
		delay = float64(e.maxDelay)
	}

	// Apply jitter (random variation between -jitter and +jitter)
	if e.jitterFactor > 0 {
		jitter := delay * e.jitterFactor
		// Simple jitter using modulo (deterministic but varied)
		variation := float64(attempt%10) / 10.0 * 2 * jitter // 0 to 2*jitter
		delay = delay - jitter + variation
	}

	// Ensure positive
	if delay < float64(e.initialDelay) {
		delay = float64(e.initialDelay)
	}

	return time.Duration(delay)
}
