package model

import (
	"time"

	"github.com/google/uuid"
)

// PasswordReset represents a password reset token
type PasswordReset struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;index"`
	TokenHash string     `json:"-" gorm:"not null;index"`
	ExpiresAt time.Time  `json:"expires_at" gorm:"not null"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the table name
func (PasswordReset) TableName() string {
	return "password_resets"
}

// RequestPasswordResetRequest represents the request to initiate password reset
type RequestPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest represents the request to complete password reset
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}
