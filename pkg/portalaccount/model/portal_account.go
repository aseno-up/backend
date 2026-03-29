package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// PortalAccount represents a patient's login credentials for the public portal
type PortalAccount struct {
	ID                        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PatientID                 uuid.UUID  `json:"patient_id" gorm:"type:uuid;not null;index"`
	ClinicID                  uuid.UUID  `json:"clinic_id" gorm:"type:uuid;not null;index"`
	Email                     string     `json:"email" gorm:"type:varchar(255);not null;index"`
	PasswordHash              *string    `json:"-" gorm:"type:text"`
	EmailVerifiedAt           *time.Time `json:"email_verified_at"`
	RegistrationToken         string     `json:"-" gorm:"type:varchar(128);uniqueIndex"`
	RegistrationTokenExpiresAt *time.Time `json:"-"`
	EmailVerificationToken    *string    `json:"-" gorm:"type:varchar(128);index"`
	CreatedAt                 time.Time  `json:"created_at"`
	LastLoginAt               *time.Time `json:"last_login_at,omitempty"`
}

func (PortalAccount) TableName() string {
	return "portal_accounts"
}

// IsRegistrationTokenValid checks if the registration token is still valid
func (p *PortalAccount) IsRegistrationTokenValid() bool {
	if p.RegistrationTokenExpiresAt == nil {
		return false
	}
	return time.Now().Before(*p.RegistrationTokenExpiresAt)
}

// IsEmailVerified checks if the email has been verified
func (p *PortalAccount) IsEmailVerified() bool {
	return p.EmailVerifiedAt != nil
}

// HasPassword checks if the account has a password set
func (p *PortalAccount) HasPassword() bool {
	return p.PasswordHash != nil && *p.PasswordHash != ""
}

// GenerateToken generates a secure random token (64 hex chars)
func GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// RegistrationTokenTTL is 7 days
const RegistrationTokenTTL = 7 * 24 * time.Hour

// EmailVerificationTokenTTL is 24 hours
const EmailVerificationTokenTTL = 24 * time.Hour

// RegisterRequest is the request to create a password for a portal account
type RegisterRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginRequest is the request to login to the portal
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// VerifyEmailRequest is the request to verify email
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// PortalMeResponse is the response for GET /portal/me
type PortalMeResponse struct {
	PatientID  uuid.UUID `json:"patient_id"`
	ClinicID   uuid.UUID `json:"clinic_id"`
	ClinicName string    `json:"clinic_name"`
	ClinicSlug string    `json:"clinic_slug"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
}

// PortalStatsResponse contains appointment statistics for the patient
type PortalStatsResponse struct {
	TotalAppointments    int        `json:"total_appointments"`
	NoShowCount          int        `json:"no_show_count"`
	AppointmentsLastMonth int       `json:"appointments_last_month"`
	NextAppointment      *time.Time `json:"next_appointment,omitempty"`
	NextProfessionalName string     `json:"next_professional_name,omitempty"`
}

// PortalCreateAppointmentRequest is the request to create an appointment from the portal
type PortalCreateAppointmentRequest struct {
	ProfessionalID uuid.UUID  `json:"professional_id" binding:"required"`
	StartTime      time.Time  `json:"start_time" binding:"required"`
	ProcedureID    *uuid.UUID `json:"procedure_id"`
	Notes          string     `json:"notes"`
}

// PortalAppointmentResponse is the response after creating an appointment from the portal
type PortalAppointmentResponse struct {
	ID           uuid.UUID `json:"id"`
	Professional string    `json:"professional"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Status       string    `json:"status"`
}

// ValidateTokenResponse is returned when validating a registration token
type ValidateTokenResponse struct {
	Email      string `json:"email"`
	PatientName string `json:"patient_name"`
	ClinicName  string `json:"clinic_name"`
}
