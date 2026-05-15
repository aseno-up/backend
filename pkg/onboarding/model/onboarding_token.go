package model

import (
	"time"

	"github.com/google/uuid"
)

// OnboardingToken is a one-time magic link issued when a visitor submits the
// landing-page "Receba seu link de acesso" form. Clicking the link in the
// e-mail takes them to /onboarding/<token> where they finish creating the
// account (name + password + clinic name) — the e-mail is implicitly verified
// because they had to receive and click the link.
type OnboardingToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Token     string     `gorm:"size:64;uniqueIndex:idx_onboarding_tokens_token;not null" json:"token"`
	Email     string     `gorm:"size:255;index;not null" json:"email"`
	Name      string     `gorm:"size:255" json:"name"`
	WhatsApp  string     `gorm:"size:32" json:"whatsapp"`
	Origem    string     `gorm:"size:64" json:"origem"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	UsedAt    *time.Time `gorm:"index" json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (OnboardingToken) TableName() string {
	return "onboarding_tokens"
}

// IsValid returns whether the token can still be used (not expired, not consumed).
func (t *OnboardingToken) IsValid(now time.Time) bool {
	if t.UsedAt != nil {
		return false
	}
	return now.Before(t.ExpiresAt)
}
