package model

import (
	"time"

	"github.com/google/uuid"
)

// MessageType represents the type of WhatsApp message
type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeTemplate MessageType = "template"
	MessageTypeButton   MessageType = "button"
	MessageTypeList     MessageType = "list"
)

// MessageStatus represents the delivery status
type MessageStatus string

const (
	MessageStatusPending   MessageStatus = "pending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusFailed    MessageStatus = "failed"
)

// MessageDirection represents inbound vs outbound
type MessageDirection string

const (
	MessageDirectionInbound  MessageDirection = "inbound"
	MessageDirectionOutbound MessageDirection = "outbound"
)

// Message represents a WhatsApp message log
type Message struct {
	ID            uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID      uuid.UUID        `json:"clinic_id" gorm:"type:uuid;not null;index"`
	AppointmentID *uuid.UUID       `json:"appointment_id,omitempty" gorm:"type:uuid;index"`
	PatientID     *uuid.UUID       `json:"patient_id,omitempty" gorm:"type:uuid;index"`
	Phone         string           `json:"phone" gorm:"not null"`
	Direction     MessageDirection `json:"direction" gorm:"not null"`
	Type          MessageType      `json:"type" gorm:"not null;default:'text'"`
	Content       string           `json:"content" gorm:"type:text"`
	Status        MessageStatus    `json:"status" gorm:"not null;default:'pending'"`
	ExternalID    string           `json:"external_id"` // ID from WhatsApp provider
	ErrorMessage  string           `json:"error_message,omitempty"`
	SentAt        *time.Time       `json:"sent_at,omitempty"`
	DeliveredAt   *time.Time       `json:"delivered_at,omitempty"`
	ReadAt        *time.Time       `json:"read_at,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// TableName returns the table name for GORM
func (Message) TableName() string {
	return "whatsapp_messages"
}

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	Phone   string            `json:"phone" binding:"required"`
	Message string            `json:"message" binding:"required"`
	Buttons []MessageButton   `json:"buttons,omitempty"`
	Options *SendMessageOptions `json:"options,omitempty"`
}

// MessageButton represents a button in an interactive message
type MessageButton struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SendMessageOptions represents additional options for sending
type SendMessageOptions struct {
	DelaySeconds int  `json:"delay_seconds,omitempty"`
	LinkPreview  bool `json:"link_preview,omitempty"`
}

// SendMessageResponse represents the response after sending
type SendMessageResponse struct {
	MessageID  string `json:"message_id"`
	ExternalID string `json:"external_id"`
	Status     string `json:"status"`
}

// ConfirmationMessage represents a confirmation message to be sent
type ConfirmationMessage struct {
	ClinicID      uuid.UUID `json:"clinic_id"`
	AppointmentID uuid.UUID `json:"appointment_id"`
	PatientName   string    `json:"patient_name"`
	PatientPhone  string    `json:"patient_phone"`
	Professional  string    `json:"professional"`
	DateTime      time.Time `json:"date_time"`
	ClinicName    string    `json:"clinic_name"`
}

// ReminderType represents the type of reminder
type ReminderType string

const (
	ReminderType48Hours ReminderType = "48h"
	ReminderType24Hours ReminderType = "24h"
	ReminderType2Hours  ReminderType = "2h"
	ReminderType1Hour   ReminderType = "1h"
)

// ReminderMessage represents a reminder to be sent
type ReminderMessage struct {
	ClinicID      uuid.UUID    `json:"clinic_id"`
	AppointmentID uuid.UUID    `json:"appointment_id"`
	PatientName   string       `json:"patient_name"`
	PatientPhone  string       `json:"patient_phone"`
	Professional  string       `json:"professional"`
	DateTime      time.Time    `json:"date_time"`
	ClinicName    string       `json:"clinic_name"`
	ReminderType  ReminderType `json:"reminder_type"`
}
