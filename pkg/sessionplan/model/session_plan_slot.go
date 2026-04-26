package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SessionPlanSlot represents a recurring (weekday, start_time) slot of a group
// SessionPlan. Multiple slots per weekday are allowed, which is how a single
// "Pilates" plan can run at 08:00, 09:00, 10:10, ... on the same day.
//
// Capacity is per-slot. ProfessionalID, when set, overrides the plan's
// professional for that specific slot (e.g. Pilates seg 08:00 = Thaiane,
// Pilates qui 08:00 = Graciele). When null, falls back to the plan's
// professional.
//
// SessionPlanSlot is the structural replacement for the legacy DayTimes map
// in group plans. Individual plans continue to use DayTimes.
type SessionPlanSlot struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SessionPlanID  uuid.UUID  `json:"session_plan_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_session_plan_slot_unique"`
	Weekday        int        `json:"weekday" gorm:"not null;uniqueIndex:idx_session_plan_slot_unique"`    // 0=Sunday ... 6=Saturday
	StartTime      string     `json:"start_time" gorm:"size:5;not null;uniqueIndex:idx_session_plan_slot_unique"` // HH:MM
	Capacity       int        `json:"capacity" gorm:"not null;default:1"`
	ProfessionalID *uuid.UUID `json:"professional_id" gorm:"type:uuid;index"`
	IsActive       bool       `json:"is_active" gorm:"default:true"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (SessionPlanSlot) TableName() string {
	return "session_plan_slots"
}

type SessionPlanSlotResponse struct {
	ID             uuid.UUID  `json:"id"`
	SessionPlanID  uuid.UUID  `json:"session_plan_id"`
	Weekday        int        `json:"weekday"`
	StartTime      string     `json:"start_time"`
	Capacity       int        `json:"capacity"`
	ProfessionalID *uuid.UUID `json:"professional_id,omitempty"`
	IsActive       bool       `json:"is_active"`
}

func (s *SessionPlanSlot) ToResponse() *SessionPlanSlotResponse {
	return &SessionPlanSlotResponse{
		ID:             s.ID,
		SessionPlanID:  s.SessionPlanID,
		Weekday:        s.Weekday,
		StartTime:      s.StartTime,
		Capacity:       s.Capacity,
		ProfessionalID: s.ProfessionalID,
		IsActive:       s.IsActive,
	}
}

type CreateSessionPlanSlotRequest struct {
	Weekday        int        `json:"weekday" binding:"min=0,max=6"`
	StartTime      string     `json:"start_time" binding:"required,len=5"` // HH:MM
	Capacity       int        `json:"capacity" binding:"required,min=1,max=200"`
	ProfessionalID *uuid.UUID `json:"professional_id"`
}

type UpdateSessionPlanSlotRequest struct {
	Weekday        *int       `json:"weekday" binding:"omitempty,min=0,max=6"`
	StartTime      *string    `json:"start_time" binding:"omitempty,len=5"`
	Capacity       *int       `json:"capacity" binding:"omitempty,min=1,max=200"`
	ProfessionalID *uuid.UUID `json:"professional_id"`
	IsActive       *bool      `json:"is_active"`
}
