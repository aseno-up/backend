package model

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserRole represents the user's role in the system
type UserRole string

const (
	UserRoleOwner        UserRole = "owner"        // Clinic owner - full access
	UserRoleAdmin        UserRole = "admin"        // Administrator - manage clinic
	UserRoleProfessional UserRole = "professional" // Professional - own schedule
	UserRoleReceptionist UserRole = "receptionist" // Receptionist - manage appointments
)

// User represents a user in the system
type User struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key"`
	ClinicID       uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Email          string     `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash   string     `json:"-" gorm:"column:password_hash"` // Hidden from JSON
	Name           string     `json:"name" gorm:"not null"`
	Phone          string     `json:"phone"`
	AvatarURL      string     `json:"avatar_url"`
	Role           UserRole   `json:"role" gorm:"not null;default:'receptionist'"`
	GoogleID       string     `json:"-"` // Google OAuth ID (hidden from JSON)
	RefreshToken   string     `json:"-"` // Current refresh token (hidden from JSON)
	EmailVerified  bool       `json:"email_verified" gorm:"default:false"`
	IsActive       bool       `json:"is_active" gorm:"default:true"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	ProfessionalID *uuid.UUID `json:"professional_id,omitempty" gorm:"-"` // Populated from professionals table (source of truth is Professional.UserID)
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for GORM
func (User) TableName() string {
	return "users"
}

// SetPassword hashes and sets the user's password
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword verifies the password against the stored hash
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// UserResponse represents the API response for a user
type UserResponse struct {
	ID             uuid.UUID  `json:"id"`
	ClinicID       uuid.UUID  `json:"clinic_id"`
	Email          string     `json:"email"`
	Name           string     `json:"name"`
	Phone          string     `json:"phone"`
	AvatarURL      string     `json:"avatar_url"`
	Role           UserRole   `json:"role"`
	EmailVerified  bool       `json:"email_verified"`
	IsActive       bool       `json:"is_active"`
	ProfessionalID *uuid.UUID `json:"professional_id,omitempty"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ToResponse converts User to UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:             u.ID,
		ClinicID:       u.ClinicID,
		Email:          u.Email,
		Name:           u.Name,
		Phone:          u.Phone,
		AvatarURL:      u.AvatarURL,
		Role:           u.Role,
		EmailVerified:  u.EmailVerified,
		IsActive:       u.IsActive,
		ProfessionalID: u.ProfessionalID,
		LastLoginAt:    u.LastLoginAt,
		CreatedAt:      u.CreatedAt,
	}
}

// HasPermission checks if user has at least the given role level
func (u *User) HasPermission(requiredRole UserRole) bool {
	roleLevel := map[UserRole]int{
		UserRoleReceptionist: 1,
		UserRoleProfessional: 2,
		UserRoleAdmin:        3,
		UserRoleOwner:        4,
	}

	userLevel, ok := roleLevel[u.Role]
	if !ok {
		return false
	}

	requiredLevel, ok := roleLevel[requiredRole]
	if !ok {
		return false
	}

	return userLevel >= requiredLevel
}

// IsOwner checks if user is the clinic owner
func (u *User) IsOwner() bool {
	return u.Role == UserRoleOwner
}

// IsAdmin checks if user has admin privileges
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleOwner || u.Role == UserRoleAdmin
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Email    string   `json:"email" binding:"required,email"`
	Password string   `json:"password" binding:"required,min=8"`
	Name     string   `json:"name" binding:"required,min=2,max=100"`
	Phone    string   `json:"phone"`
	Role     UserRole `json:"role" binding:"omitempty,oneof=owner admin professional receptionist"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Name      string   `json:"name" binding:"omitempty,min=2,max=100"`
	Phone     string   `json:"phone"`
	AvatarURL string   `json:"avatar_url"`
	Role      UserRole `json:"role" binding:"omitempty,oneof=owner admin professional receptionist"`
	IsActive  *bool    `json:"is_active"`
}

// LoginRequest represents the login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest represents the registration request (creates clinic + owner user)
type RegisterRequest struct {
	ClinicName string `json:"clinic_name" binding:"required,min=2,max=100"`
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=8"`
	Name       string `json:"name" binding:"required,min=2,max=100"`
	Phone      string `json:"phone"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	User         *UserResponse `json:"user"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    int           `json:"expires_in"` // seconds
}
