package model

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeAppointmentReminder    NotificationType = "appointment_reminder"
	NotificationTypeAppointmentConfirmed   NotificationType = "appointment_confirmed"
	NotificationTypeAppointmentCancelled   NotificationType = "appointment_cancelled"
	NotificationTypeAppointmentRescheduled NotificationType = "appointment_rescheduled"
	NotificationTypeWhatsAppConnected      NotificationType = "whatsapp_connected"
	NotificationTypeWhatsAppDisconnected   NotificationType = "whatsapp_disconnected"
	NotificationTypeTrialEnding            NotificationType = "trial_ending"
	NotificationTypeTrialExpired           NotificationType = "trial_expired"
	NotificationTypeNewPatient             NotificationType = "new_patient"
	NotificationTypeSystem                 NotificationType = "system"
)

// Notification represents an in-app notification
type Notification struct {
	ID        uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID  uuid.UUID        `json:"clinic_id" gorm:"type:uuid;not null;index"`
	UserID    *uuid.UUID       `json:"user_id,omitempty" gorm:"type:uuid;index"` // nil = all users in clinic
	Type      NotificationType `json:"type" gorm:"type:varchar(50);not null;index"`
	Title     string           `json:"title" gorm:"type:varchar(200);not null"`
	Message   string           `json:"message" gorm:"type:text;not null"`
	Data      JSON             `json:"data,omitempty" gorm:"type:jsonb"` // Additional data (e.g., appointment ID)
	ReadAt    *time.Time       `json:"read_at,omitempty"`
	CreatedAt time.Time        `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the table name for GORM
func (Notification) TableName() string {
	return "notifications"
}

// IsRead returns whether the notification has been read
func (n *Notification) IsRead() bool {
	return n.ReadAt != nil
}

// JSON is a custom type for JSONB columns
type JSON map[string]interface{}

// NotificationResponse is the API response for a notification
type NotificationResponse struct {
	ID        uuid.UUID        `json:"id"`
	Type      NotificationType `json:"type"`
	Title     string           `json:"title"`
	Message   string           `json:"message"`
	Data      JSON             `json:"data,omitempty"`
	Read      bool             `json:"read"`
	CreatedAt time.Time        `json:"created_at"`
}

// ToResponse converts a Notification to NotificationResponse
func (n *Notification) ToResponse() *NotificationResponse {
	return &NotificationResponse{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Message:   n.Message,
		Data:      n.Data,
		Read:      n.IsRead(),
		CreatedAt: n.CreatedAt,
	}
}

// NotificationFilter represents filter options for listing notifications
type NotificationFilter struct {
	Type   *NotificationType
	Read   *bool
	Limit  int
	Offset int
}
