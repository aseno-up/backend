package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// EmailVerification represents an email verification token
type EmailVerification struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;index"`
	Token     string     `json:"-" gorm:"not null;uniqueIndex"` // Hashed token stored in DB
	ExpiresAt time.Time  `json:"expires_at" gorm:"not null"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// TableName returns the table name for GORM
func (EmailVerification) TableName() string {
	return "email_verifications"
}

// IsExpired checks if the token has expired
func (e *EmailVerification) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// IsUsed checks if the token has been used
func (e *EmailVerification) IsUsed() bool {
	return e.UsedAt != nil
}

// IsValid checks if the token is valid (not expired and not used)
func (e *EmailVerification) IsValid() bool {
	return !e.IsExpired() && !e.IsUsed()
}

// GenerateVerificationToken creates a new random token
func GenerateVerificationToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// VerificationTokenExpiry is the default token expiry duration (24 hours)
const VerificationTokenExpiry = 24 * time.Hour

// VerifyEmailRequest represents the request to verify email
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// ResendVerificationRequest represents the request to resend verification email
type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}
