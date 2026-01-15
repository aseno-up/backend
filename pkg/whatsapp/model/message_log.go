package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// MessageLogEvent represents lifecycle events for messages
type MessageLogEvent string

const (
	MessageLogEventCreated   MessageLogEvent = "created"   // Message created in system
	MessageLogEventQueued    MessageLogEvent = "queued"    // Queued for sending
	MessageLogEventSent      MessageLogEvent = "sent"      // Provider confirmed sent
	MessageLogEventDelivered MessageLogEvent = "delivered" // Delivered to phone
	MessageLogEventRead      MessageLogEvent = "read"      // Read by patient
	MessageLogEventReplied   MessageLogEvent = "replied"   // Patient replied
	MessageLogEventFailed    MessageLogEvent = "failed"    // Failed to send
	MessageLogEventRetried   MessageLogEvent = "retried"   // Retry attempt
	MessageLogEventProcessed MessageLogEvent = "processed" // System processed response
)

// MessageLogChannel represents the message channel
type MessageLogChannel string

const (
	MessageLogChannelWhatsApp MessageLogChannel = "whatsapp"
)

// MessageLogPayload is a custom type for JSONB columns
type MessageLogPayload map[string]interface{}

// Value implements driver.Valuer for MessageLogPayload
func (p MessageLogPayload) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return json.Marshal(p)
}

// Scan implements sql.Scanner for MessageLogPayload
func (p *MessageLogPayload) Scan(value interface{}) error {
	if value == nil {
		*p = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for MessageLogPayload")
	}

	return json.Unmarshal(data, p)
}

// MessageLog represents a lifecycle event for a message
type MessageLog struct {
	ID            uuid.UUID           `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID      uuid.UUID           `json:"clinic_id" gorm:"type:uuid;not null;index"`
	MessageID     uuid.UUID           `json:"message_id" gorm:"type:uuid;not null;index"`
	PatientID     *uuid.UUID          `json:"patient_id,omitempty" gorm:"type:uuid;index"`
	AppointmentID *uuid.UUID          `json:"appointment_id,omitempty" gorm:"type:uuid;index"`
	Event         MessageLogEvent     `json:"event" gorm:"type:varchar(50);not null;index"`
	Channel       MessageLogChannel   `json:"channel" gorm:"type:varchar(50);not null;default:'whatsapp'"`
	Direction     MessageDirection    `json:"direction" gorm:"type:varchar(50);not null"`
	Payload       MessageLogPayload   `json:"payload,omitempty" gorm:"type:jsonb"`
	Error         *string             `json:"error,omitempty"`
	CreatedAt     time.Time           `json:"created_at" gorm:"index"`
}

// TableName returns the table name for GORM
func (MessageLog) TableName() string {
	return "message_logs"
}
