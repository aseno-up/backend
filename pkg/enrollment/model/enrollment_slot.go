package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EnrollmentSlot is the junction between SessionPlanEnrollment and
// SessionPlanSlot. A patient enrolled in a group plan picks N slots
// (each = (weekday, time)); each pick is one row here.
type EnrollmentSlot struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	EnrollmentID      uuid.UUID `json:"enrollment_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_enrollment_slot_unique"`
	SessionPlanSlotID uuid.UUID `json:"session_plan_slot_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_enrollment_slot_unique"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (EnrollmentSlot) TableName() string {
	return "enrollment_slots"
}
