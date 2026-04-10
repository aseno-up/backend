package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StringArray is a custom type for storing []string as JSONB in PostgreSQL
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = StringArray{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if !ok {
			return nil
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, s)
}

// Plan represents the base plan definition with its global limits.
type Plan struct {
	ID                     uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name                   string         `json:"name" gorm:"not null"`
	Slug                   string         `json:"slug" gorm:"uniqueIndex;not null"`
	IsActive               bool           `json:"is_active" gorm:"not null;default:true"`
	MaxClinics             int            `json:"max_clinics" gorm:"not null;default:1"`              // -1 for unlimited
	MaxProfessionals       int            `json:"max_professionals" gorm:"not null;default:1"`        // -1 for unlimited
	MaxWhatsappConnections int            `json:"max_whatsapp_connections" gorm:"not null;default:1"` // -1 for unlimited
	MaxGoogleCalendars     int            `json:"max_google_calendars" gorm:"not null;default:1"`     // -1 for unlimited
	HasApiAccess           bool           `json:"has_api_access" gorm:"not null;default:false"`
	Features               StringArray    `json:"features" gorm:"type:jsonb;default:'[]'"`
	CreatedAt              time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt              gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	Cycles []PlanCycle `json:"cycles,omitempty" gorm:"foreignKey:PlanID"`
}

// TableName returns the table name for GORM
func (Plan) TableName() string {
	return "plans"
}
