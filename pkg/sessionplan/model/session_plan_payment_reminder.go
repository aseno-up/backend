package model

import (
	"time"

	"github.com/google/uuid"
)

// PaymentReminderChannel identifies the delivery channel that carried a
// monthly payment reminder. The worker inserts one row per (plan, month,
// channel) so each channel is tracked independently.
type PaymentReminderChannel string

const (
	PaymentReminderChannelWhatsApp PaymentReminderChannel = "whatsapp"
	PaymentReminderChannelEmail    PaymentReminderChannel = "email"
)

// SessionPlanPaymentReminder records that a monthly payment reminder was
// dispatched to a patient for a given reference month. The worker consults
// this table before enqueuing so the same patient never receives more than
// one reminder per month per channel.
type SessionPlanPaymentReminder struct {
	ID             uuid.UUID              `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ClinicID       uuid.UUID              `json:"clinic_id" gorm:"type:uuid;not null;index"`
	SessionPlanID  uuid.UUID              `json:"session_plan_id" gorm:"type:uuid;not null;index"`
	EnrollmentID   *uuid.UUID             `json:"enrollment_id,omitempty" gorm:"type:uuid;index"`
	PatientID      uuid.UUID              `json:"patient_id" gorm:"type:uuid;not null;index"`
	ReferenceMonth string                 `json:"reference_month" gorm:"size:7;not null"` // YYYY-MM
	Channel        PaymentReminderChannel `json:"channel" gorm:"size:20;not null"`
	SentAt         time.Time              `json:"sent_at" gorm:"not null"`
	CreatedAt      time.Time              `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the table name
func (SessionPlanPaymentReminder) TableName() string {
	return "session_plan_payment_reminders"
}
