package model

import (
	"time"

	"github.com/google/uuid"
)

// ScheduledMessage represents a message scheduled to be sent at a specific time
type ScheduledMessage struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ClinicID    uuid.UUID  `gorm:"type:uuid;not null;index:idx_scheduled_messages_clinic_scheduled" json:"clinic_id"`
	Instance    string     `gorm:"type:varchar(255);not null" json:"instance"`
	Phone       string     `gorm:"type:varchar(20);not null" json:"phone"`
	Message     string     `gorm:"type:text;not null" json:"message"`
	ScheduledAt time.Time  `gorm:"not null;index:idx_scheduled_messages_clinic_scheduled" json:"scheduled_at"`
	Sent        bool       `gorm:"default:false;index:idx_scheduled_messages_sent" json:"sent"`
	SentAt      *time.Time `gorm:"type:timestamptz" json:"sent_at,omitempty"`
	Error       string     `gorm:"type:text" json:"error,omitempty"`
	CreatedAt   time.Time  `gorm:"not null;default:current_timestamp" json:"created_at"`
}

// TableName specifies the table name for ScheduledMessage
func (ScheduledMessage) TableName() string {
	return "scheduled_messages"
}
