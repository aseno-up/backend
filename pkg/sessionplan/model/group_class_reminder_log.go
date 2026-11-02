package model

import (
	"time"

	"github.com/google/uuid"
)

// GroupClassReminderLog records that a WhatsApp reminder was enqueued for
// (clinic, plan, patient, session_date, reminder_type). The unique index on
// those five columns is the dedup primary mechanism: a second worker tick
// inside the reminder window collides on insert and skips, preventing the
// same reminder from going out twice.
//
// Group plan classes never materialize as appointments, so the appointment
// reminder fields can't be used. This separate log lives parallel to
// `appointments.reminder_*_sent_at`.
type GroupClassReminderLog struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClinicID      uuid.UUID `json:"clinic_id" gorm:"type:uuid;not null;uniqueIndex:idx_group_class_reminder_log_unique"`
	SessionPlanID uuid.UUID `json:"session_plan_id" gorm:"type:uuid;not null;uniqueIndex:idx_group_class_reminder_log_unique"`
	PatientID     uuid.UUID `json:"patient_id" gorm:"type:uuid;not null;uniqueIndex:idx_group_class_reminder_log_unique"`
	SessionDate   string    `json:"session_date" gorm:"size:10;not null;uniqueIndex:idx_group_class_reminder_log_unique"` // YYYY-MM-DD
	ReminderType  string    `json:"reminder_type" gorm:"size:20;not null;uniqueIndex:idx_group_class_reminder_log_unique"`
	SentAt        time.Time `json:"sent_at" gorm:"not null;default:current_timestamp"`
}

// TableName returns the table name for GORM.
func (GroupClassReminderLog) TableName() string {
	return "group_class_reminder_log"
}
