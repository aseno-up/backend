package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	user_model "github.com/aseno/backend/pkg/user/model"
)

// TokenType represents the type of JWT token
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// JWTClaims represents the custom claims for JWT tokens
type JWTClaims struct {
	UserID         uuid.UUID           `json:"user_id"`
	ClinicID       uuid.UUID           `json:"clinic_id"`
	Email          string              `json:"email"`
	Role           user_model.UserRole `json:"role"`
	ProfessionalID *uuid.UUID          `json:"professional_id,omitempty"`
	Type           TokenType           `json:"type"`
	jwt.RegisteredClaims
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	ExpiresIn    int       `json:"expires_in"` // seconds
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=1"`
}

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	ClinicName string `json:"clinic_name" binding:"required,min=2,max=100"`
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=8"`
	Name       string `json:"name" binding:"required,min=2,max=100"`
	Phone      string `json:"phone"`
}

// RefreshRequest represents the refresh token request body
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// GoogleAuthRequest represents the Google OAuth request body
type GoogleAuthRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// GoogleLoginRequest is an alias for GoogleAuthRequest (for auth service)
type GoogleLoginRequest = GoogleAuthRequest

// GoogleUserInfo represents user info from Google OAuth
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	User         *user_model.UserResponse `json:"user"`
	AccessToken  string                   `json:"access_token"`
	RefreshToken string                   `json:"refresh_token"`
	ExpiresIn    int                      `json:"expires_in"` // seconds
}

// ChangePasswordRequest represents the change password request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// CurrentUserResponse represents the response for getting current user data
type CurrentUserResponse struct {
	User   *user_model.UserResponse     `json:"user"`
	Clinic *clinic_model.ClinicResponse `json:"clinic"`
}
