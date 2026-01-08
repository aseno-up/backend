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

// Queue defines the interface for message queue operations
// This abstraction allows switching between RabbitMQ, Redis, or other implementations
type Queue interface {
	// Connect establishes connection to the queue
	Connect(ctx context.Context) error

	// Close closes the connection
	Close() error

	// Publish publishes a message to the queue
	Publish(ctx context.Context, job *MessageJob) error

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
