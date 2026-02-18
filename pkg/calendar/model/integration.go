package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CalendarIntegration stores OAuth tokens and settings for a clinic's Google Calendar
type CalendarIntegration struct {
	ID            uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID      uuid.UUID      `json:"clinic_id" gorm:"type:uuid;not null;uniqueIndex"`
	Provider      string         `json:"provider" gorm:"not null;default:'google'"` // google, outlook, etc.
	AccessToken   string         `json:"-" gorm:"type:text"`                        // encrypted
	RefreshToken  string         `json:"-" gorm:"type:text"`                        // encrypted
	TokenExpiry   time.Time      `json:"token_expiry"`
	CalendarID    string         `json:"calendar_id"` // primary calendar ID to sync with
	Email         string         `json:"email"`       // Google account email
	SyncEnabled   bool           `json:"sync_enabled" gorm:"default:true"`
	SyncDirection SyncDirection  `json:"sync_direction" gorm:"default:'bidirectional'"`
	LastSyncAt    *time.Time     `json:"last_sync_at,omitempty"`
	LastSyncError string         `json:"last_sync_error,omitempty"`
	CreatedAt     time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

// SyncDirection determines how appointments are synced
type SyncDirection string

const (
	SyncDirectionToGoogle      SyncDirection = "to_google"     // Only push to Google
	SyncDirectionFromGoogle    SyncDirection = "from_google"   // Only pull from Google
	SyncDirectionBidirectional SyncDirection = "bidirectional" // Both ways
)

// CalendarSync tracks sync status for individual appointments
type CalendarSync struct {
	ID            uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID      uuid.UUID      `json:"clinic_id" gorm:"type:uuid;not null;index"`
	AppointmentID uuid.UUID      `json:"appointment_id" gorm:"type:uuid;not null;index"`
	GoogleEventID string         `json:"google_event_id" gorm:"index"`
	CalendarID    string         `json:"calendar_id"`
	SyncStatus    SyncStatus     `json:"sync_status" gorm:"not null;default:'pending'"`
	LastSyncAt    *time.Time     `json:"last_sync_at,omitempty"`
	LastSyncError string         `json:"last_sync_error,omitempty"`
	CreatedAt     time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

// SyncStatus represents the sync status of an appointment
type SyncStatus string

const (
	SyncStatusPending SyncStatus = "pending"
	SyncStatusSynced  SyncStatus = "synced"
	SyncStatusFailed  SyncStatus = "failed"
	SyncStatusDeleted SyncStatus = "deleted"
)

// GoogleOAuthTokens represents the tokens received from Google OAuth
type GoogleOAuthTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

// GoogleCalendar represents a Google Calendar
type GoogleCalendar struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	TimeZone    string `json:"timeZone,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
}

// GoogleEvent represents a Google Calendar event
type GoogleEvent struct {
	ID          string           `json:"id,omitempty"`
	Summary     string           `json:"summary"`
	Description string           `json:"description,omitempty"`
	Location    string           `json:"location,omitempty"`
	Start       *GoogleEventTime `json:"start"`
	End         *GoogleEventTime `json:"end"`
	Attendees   []GoogleAttendee `json:"attendees,omitempty"`
	Status      string           `json:"status,omitempty"` // confirmed, tentative, cancelled
	HTMLLink    string           `json:"htmlLink,omitempty"`
}

// GoogleEventTime represents the start/end time of an event
type GoogleEventTime struct {
	DateTime string `json:"dateTime,omitempty"` // RFC3339 format
	Date     string `json:"date,omitempty"`     // For all-day events (YYYY-MM-DD)
	TimeZone string `json:"timeZone,omitempty"`
}

// GoogleAttendee represents an event attendee
type GoogleAttendee struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName,omitempty"`
	ResponseStatus string `json:"responseStatus,omitempty"` // needsAction, declined, tentative, accepted
}

// CalendarListResponse represents the response from listing calendars
type CalendarListResponse struct {
	Items []GoogleCalendar `json:"items"`
}

// EventListResponse represents the response from listing events
type EventListResponse struct {
	Items         []GoogleEvent `json:"items"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
	NextSyncToken string        `json:"nextSyncToken,omitempty"`
}
