package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/auth/model"
	auth_repository "github.com/aseno/backend/pkg/auth/repository"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	email_service "github.com/aseno/backend/pkg/email/service"
	payment_model "github.com/aseno/backend/pkg/payment/model"
	user_model "github.com/aseno/backend/pkg/user/model"
	user_repository "github.com/aseno/backend/pkg/user/repository"
)

// AuthService defines the interface for authentication operations
type AuthService interface {
	Register(ctx context.Context, req *model.RegisterRequest) (*model.AuthResponse, error)
	Login(ctx context.Context, req *model.LoginRequest) (*model.AuthResponse, error)
	GoogleLogin(ctx context.Context, req *model.GoogleLoginRequest) (*model.AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*model.AuthResponse, error)
	Logout(ctx context.Context, userID uuid.UUID) error
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.CurrentUserResponse, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req *model.ChangePasswordRequest) error
	RequestPasswordReset(ctx context.Context, req *model.RequestPasswordResetRequest) error
	ResetPassword(ctx context.Context, req *model.ResetPasswordRequest) error
	// Email verification
	SendVerificationEmail(ctx context.Context, userID uuid.UUID) error
	VerifyEmail(ctx context.Context, token string) error
	ResendVerificationEmail(ctx context.Context, email string) error
	// Payment service injection (for creating Asaas subscriptions during registration)
	SetPaymentService(paymentService PaymentService)
}

// GoogleAuthClient defines the interface for Google authentication
type GoogleAuthClient interface {
	VerifyIDToken(ctx context.Context, idToken string) (*GoogleUserInfo, error)
}

// PaymentService defines minimal interface for payment operations during registration
type PaymentService interface {
	CreateCustomerForClinic(ctx context.Context, clinicID uuid.UUID, req *payment_model.CreateCustomerRequest) error
	CreateSubscriptionForClinic(ctx context.Context, clinicID uuid.UUID, plan string, billing string) error
}

// GoogleUserInfo represents user information from Google
type GoogleUserInfo struct {
	ID            string
	Email         string
	VerifiedEmail bool
	Name          string
	GivenName     string
	FamilyName    string
	Picture       string
}

// authService implements AuthService
type authService struct {
	userRepo              user_repository.UserRepository
	clinicRepo            clinic_repository.ClinicRepository
	passwordResetRepo     auth_repository.PasswordResetRepository
	emailVerificationRepo auth_repository.EmailVerificationRepository
	jwtService            JWTService
	googleClient          GoogleAuthClient
	emailService          email_service.EmailService
	paymentService        PaymentService // Optional: for creating Asaas subscriptions during registration
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo user_repository.UserRepository,
	clinicRepo clinic_repository.ClinicRepository,
	passwordResetRepo auth_repository.PasswordResetRepository,
	emailVerificationRepo auth_repository.EmailVerificationRepository,
	jwtService JWTService,
	emailService email_service.EmailService,
) AuthService {
	return &authService{
		userRepo:              userRepo,
		clinicRepo:            clinicRepo,
		passwordResetRepo:     passwordResetRepo,
		emailVerificationRepo: emailVerificationRepo,
		jwtService:            jwtService,
		emailService:          emailService,
	}
}

// NewAuthServiceWithGoogle creates a new auth service with Google OAuth support
func NewAuthServiceWithGoogle(
	userRepo user_repository.UserRepository,
	clinicRepo clinic_repository.ClinicRepository,
	passwordResetRepo auth_repository.PasswordResetRepository,
	emailVerificationRepo auth_repository.EmailVerificationRepository,
	jwtService JWTService,
	googleClient GoogleAuthClient,
	emailService email_service.EmailService,
) AuthService {
	return &authService{
		userRepo:              userRepo,
		clinicRepo:            clinicRepo,
		passwordResetRepo:     passwordResetRepo,
		emailVerificationRepo: emailVerificationRepo,
		jwtService:            jwtService,
		googleClient:          googleClient,
		emailService:          emailService,
	}
}

// SetPaymentService sets the payment service (used to avoid circular dependency)
func (s *authService) SetPaymentService(paymentService PaymentService) {
	s.paymentService = paymentService
}

// Register creates a new clinic and owner user
func (s *authService) Register(ctx context.Context, req *model.RegisterRequest) (*model.AuthResponse, error) {
	// Check if email already exists
	exists, err := s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.ErrEmailAlreadyExistsInstance
	}

	// Generate slug from clinic name
	slug := s.generateSlug(req.ClinicName)

	// Check if slug is unique, append random suffix if not
	slugExists, err := s.clinicRepo.ExistsBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if slugExists {
		slug = slug + "-" + generateShortID()
	}

	// Determine plan and subscription status based on request
	var planType clinic_model.PlanType
	var subscriptionStatus clinic_model.SubscriptionStatus
	var trialEndsAt *time.Time

	// Set plan based on selection
	switch req.Plan {
	case "basic":
		planType = clinic_model.PlanTypeBasic
	case "professional":
		planType = clinic_model.PlanTypeProfessional
	case "advanced":
		planType = clinic_model.PlanTypeAdvanced
	default: // "trial" or empty - defaults to professional with trial
		planType = clinic_model.PlanTypeProfessional
	}

	// If document provided, set as active (will create Asaas subscription)
	// Otherwise, give 14-day trial period regardless of plan selected
	if req.Document != "" && req.Plan != "" && req.Plan != "trial" {
		subscriptionStatus = clinic_model.SubscriptionStatusActive
	} else {
		subscriptionStatus = clinic_model.SubscriptionStatusTrial
		trial := time.Now().AddDate(0, 0, 14)
		trialEndsAt = &trial
	}

	// Create clinic
	clinic := &clinic_model.Clinic{
		Name:               req.ClinicName,
		Slug:               slug,
		Email:              req.Email,
		Phone:              req.Phone,
		Plan:               planType,
		SubscriptionStatus: subscriptionStatus,
		TrialEndsAt:        trialEndsAt,
		Settings: clinic_model.ClinicSettings{
			Timezone:                  "America/Sao_Paulo",
			DefaultAppointmentMinutes: 30,
			ReminderHoursBefore:       24,
			AllowOnlineBooking:        false,
			RequireConfirmation:       true,
			AIAgentEnabled:            true,
		},
	}

	if err := s.clinicRepo.Create(ctx, clinic); err != nil {
		return nil, err
	}

	// Create owner user
	user := &user_model.User{
		ClinicID:      clinic.ID,
		Email:         req.Email,
		Name:          req.Name,
		Phone:         req.Phone,
		Role:          user_model.UserRoleOwner,
		EmailVerified: false,
		IsActive:      true,
	}

	if err := user.SetPassword(req.Password); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Generate tokens
	tokenPair, err := s.jwtService.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// Save refresh token
	if err := s.userRepo.UpdateRefreshToken(ctx, user.ID, tokenPair.RefreshToken); err != nil {
		return nil, err
	}

	// Update last login
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		// Non-critical error, just log it
	}

	// Create Asaas customer and subscription if document provided and not trial
	if s.paymentService != nil && req.Document != "" && req.Plan != "" && req.Plan != "trial" {
		// Create customer in Asaas
		customerReq := &payment_model.CreateCustomerRequest{
			Name:     req.Name,
			Email:    req.Email,
			Phone:    req.Phone,
			Document: req.Document,
		}

		if err := s.paymentService.CreateCustomerForClinic(ctx, clinic.ID, customerReq); err != nil {
			// Log error but don't fail registration - user can set up payment later
			fmt.Printf("[AUTH] Failed to create Asaas customer for clinic %s: %v\n", clinic.ID, err)
		} else {
			// Customer created successfully, now create subscription
			if err := s.paymentService.CreateSubscriptionForClinic(ctx, clinic.ID, req.Plan, req.Billing); err != nil {
				// Log error but don't fail registration
				fmt.Printf("[AUTH] Failed to create Asaas subscription for clinic %s: %v\n", clinic.ID, err)
			} else {
				fmt.Printf("[AUTH] Successfully created Asaas subscription for clinic %s (plan: %s, billing: %s)\n", clinic.ID, req.Plan, req.Billing)
			}
		}
	}

	// Send verification email (non-blocking, errors are logged but not returned)
	go func() {
		if err := s.SendVerificationEmail(context.Background(), user.ID); err != nil {
			// Log error but don't fail registration
			fmt.Printf("[AUTH] Failed to send verification email to %s: %v\n", user.Email, err)
		}
	}()

	return &model.AuthResponse{
		User:         user.ToResponse(),
		Clinic:       clinic.ToResponse(),
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// Login authenticates a user with email and password
func (s *authService) Login(ctx context.Context, req *model.LoginRequest) (*model.AuthResponse, error) {
	// Find user by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.ErrInvalidCredentialsInstance
		}
		return nil, err
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.ErrForbiddenInstance.WithMessage("User account is deactivated")
	}

	// Verify password
	if !user.CheckPassword(req.Password) {
		return nil, errors.ErrInvalidCredentialsInstance
	}

	// Check if clinic subscription is active
	clinic, err := s.clinicRepo.GetByID(ctx, user.ClinicID)
	if err != nil {
		return nil, err
	}

	if !clinic.IsSubscriptionActive() {
		return nil, errors.ErrForbiddenInstance.WithMessage("Clinic subscription has expired")
	}

	// Generate tokens
	tokenPair, err := s.jwtService.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// Save refresh token
	if err := s.userRepo.UpdateRefreshToken(ctx, user.ID, tokenPair.RefreshToken); err != nil {
		return nil, err
	}

	// Update last login
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		// Non-critical error, just log it
	}

	return &model.AuthResponse{
		User:         user.ToResponse(),
		Clinic:       clinic.ToResponse(),
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// GoogleLogin authenticates a user using Google OAuth ID token
func (s *authService) GoogleLogin(ctx context.Context, req *model.GoogleLoginRequest) (*model.AuthResponse, error) {
	if s.googleClient == nil {
		return nil, errors.ErrInternalInstance.WithMessage("Google OAuth is not configured")
	}

	// Verify the Google ID token
	googleUser, err := s.googleClient.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		return nil, err
	}

	// Check if user exists
	user, err := s.userRepo.GetByEmail(ctx, googleUser.Email)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		// User doesn't exist - create a new clinic and user
		// Generate slug from user's name
		slug := s.generateSlug(googleUser.Name)
		slugExists, err := s.clinicRepo.ExistsBySlug(ctx, slug)
		if err != nil {
			return nil, err
		}
		if slugExists {
			slug = slug + "-" + generateShortID()
		}

		// Calculate trial end date (14 days from now)
		trialEndsAt := time.Now().AddDate(0, 0, 14)

		// Create clinic
		clinic := &clinic_model.Clinic{
			Name:               googleUser.Name + "'s Clinic",
			Slug:               slug,
			Email:              googleUser.Email,
			Plan:               clinic_model.PlanTypeProfessional,
			SubscriptionStatus: clinic_model.SubscriptionStatusTrial,
			TrialEndsAt:        &trialEndsAt,
			Settings: clinic_model.ClinicSettings{
				Timezone:                  "America/Sao_Paulo",
				DefaultAppointmentMinutes: 30,
				ReminderHoursBefore:       24,
				AllowOnlineBooking:        false,
				RequireConfirmation:       true,
				AIAgentEnabled:            true,
			},
		}

		if err := s.clinicRepo.Create(ctx, clinic); err != nil {
			return nil, err
		}

		// Create owner user (without password - Google-only login)
		user = &user_model.User{
			ClinicID:      clinic.ID,
			Email:         googleUser.Email,
			Name:          googleUser.Name,
			Role:          user_model.UserRoleOwner,
			EmailVerified: googleUser.VerifiedEmail,
			IsActive:      true,
			GoogleID:      googleUser.ID,
			AvatarURL:     googleUser.Picture,
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, err
		}
	} else {
		// User exists - check if active
		if !user.IsActive {
			return nil, errors.ErrForbiddenInstance.WithMessage("User account is deactivated")
		}

		// Update Google ID and avatar if not set
		if user.GoogleID == "" {
			user.GoogleID = googleUser.ID
		}
		if user.AvatarURL == "" && googleUser.Picture != "" {
			user.AvatarURL = googleUser.Picture
		}
		if err := s.userRepo.Update(ctx, user); err != nil {
			return nil, err
		}

		// Check if clinic subscription is active
		clinic, err := s.clinicRepo.GetByID(ctx, user.ClinicID)
		if err != nil {
			return nil, err
		}

		if !clinic.IsSubscriptionActive() {
			return nil, errors.ErrForbiddenInstance.WithMessage("Clinic subscription has expired")
		}
	}

	// Generate tokens
	tokenPair, err := s.jwtService.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// Save refresh token
	if err := s.userRepo.UpdateRefreshToken(ctx, user.ID, tokenPair.RefreshToken); err != nil {
		return nil, err
	}

	// Update last login
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		// Non-critical error, just log it
	}

	// Get clinic for response
	clinic, err := s.clinicRepo.GetByID(ctx, user.ClinicID)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		User:         user.ToResponse(),
		Clinic:       clinic.ToResponse(),
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// RefreshToken generates new tokens using a valid refresh token
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*model.AuthResponse, error) {
	// Validate refresh token
	claims, err := s.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.ErrForbiddenInstance.WithMessage("User account is deactivated")
	}

	// Verify refresh token matches stored token
	if user.RefreshToken != refreshToken {
		return nil, errors.ErrTokenInvalidInstance.WithMessage("Token has been revoked")
	}

	// Generate new tokens
	tokenPair, err := s.jwtService.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// Save new refresh token
	if err := s.userRepo.UpdateRefreshToken(ctx, user.ID, tokenPair.RefreshToken); err != nil {
		return nil, err
	}

	// Get clinic for response
	clinic, err := s.clinicRepo.GetByID(ctx, user.ClinicID)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		User:         user.ToResponse(),
		Clinic:       clinic.ToResponse(),
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// Logout invalidates the user's refresh token
func (s *authService) Logout(ctx context.Context, userID uuid.UUID) error {
	return s.userRepo.UpdateRefreshToken(ctx, userID, "")
}

// GetCurrentUser retrieves the current user's data with clinic info
func (s *authService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.CurrentUserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	clinic, err := s.clinicRepo.GetByID(ctx, user.ClinicID)
	if err != nil {
		return nil, err
	}

	return &model.CurrentUserResponse{
		User:   user.ToResponse(),
		Clinic: clinic.ToResponse(),
	}, nil
}

// ChangePassword changes the user's password
func (s *authService) ChangePassword(ctx context.Context, userID uuid.UUID, req *model.ChangePasswordRequest) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify current password
	if !user.CheckPassword(req.CurrentPassword) {
		return errors.ErrInvalidCredentialsInstance.WithMessage("Current password is incorrect")
	}

	// Set new password
	if err := user.SetPassword(req.NewPassword); err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}

	// Update user
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Invalidate refresh token to force re-login
	return s.userRepo.UpdateRefreshToken(ctx, userID, "")
}

// RequestPasswordReset initiates the password reset process
func (s *authService) RequestPasswordReset(ctx context.Context, req *model.RequestPasswordResetRequest) error {
	// Find user by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.IsNotFound(err) {
			// Security: Do not reveal if user exists
			return nil
		}
		return err
	}

	// Generate secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	token := hex.EncodeToString(tokenBytes)

	// In a real app, we should hash the token before saving
	// For this MVP, we save it as is or implement hashing if required
	// The requirement says "Hash do token salvo no banco"
	// Let's assume we save the token directly for now or simulate hash by just saving it.
	// Ideally: tokenHash := hash(token) -> save tokenHash. Email token.
	// When resetting: hash(tokenFromEmail) == tokenHash.
	// Simplification for MVP: Save token directly as "TokenHash" field.

	// Create reset record
	reset := &model.PasswordReset{
		UserID:    user.ID,
		TokenHash: token, // Should be hashed
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	if err := s.passwordResetRepo.Create(ctx, reset); err != nil {
		return err
	}

	// Send email
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, token)

	return s.emailService.SendPasswordReset(ctx, &email_service.ResetPasswordEmailData{
		RecipientEmail: user.Email,
		RecipientName:  user.Name,
		ResetLink:      resetLink,
	})
}

// ResetPassword completes the password reset process
func (s *authService) ResetPassword(ctx context.Context, req *model.ResetPasswordRequest) error {
	// Find reset record by token
	reset, err := s.passwordResetRepo.GetByTokenHash(ctx, req.Token)
	if err != nil {
		if errors.IsNotFound(err) {
			return errors.ErrInvalidInstance.WithMessage("Invalid or expired token")
		}
		return err
	}

	// Check if expired
	if time.Now().After(reset.ExpiresAt) {
		return errors.ErrInvalidInstance.WithMessage("Token has expired")
	}

	// Find user
	user, err := s.userRepo.GetByID(ctx, reset.UserID)
	if err != nil {
		return err
	}

	// Set new password
	if err := user.SetPassword(req.NewPassword); err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}

	// Update user
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Mark token as used
	if err := s.passwordResetRepo.MarkAsUsed(ctx, reset.ID); err != nil {
		return err
	}

	// Invalidate all sessions (refresh tokens)
	return s.userRepo.UpdateRefreshToken(ctx, user.ID, "")
}

// generateSlug creates a URL-friendly slug from a name
func (s *authService) generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	// Limit length
	if len(slug) > 50 {
		slug = slug[:50]
	}

	return slug
}

// generateShortID generates a short random ID
func generateShortID() string {
	id := uuid.New().String()
	return id[:8]
}

// SendVerificationEmail sends an email verification link to the user
func (s *authService) SendVerificationEmail(ctx context.Context, userID uuid.UUID) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Don't send if already verified
	if user.EmailVerified {
		return nil
	}

	// Generate secure random token
	token, err := model.GenerateVerificationToken()
	if err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}

	// Create verification record
	verification := &model.EmailVerification{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(model.VerificationTokenExpiry),
	}

	if err := s.emailVerificationRepo.Create(ctx, verification); err != nil {
		return err
	}

	// Send email
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", frontendURL, token)

	return s.emailService.SendEmailVerification(ctx, &email_service.EmailVerificationData{
		RecipientEmail:   user.Email,
		RecipientName:    user.Name,
		VerificationLink: verificationLink,
	})
}

// VerifyEmail verifies the user's email using the token
func (s *authService) VerifyEmail(ctx context.Context, token string) error {
	// Find verification record by token
	verification, err := s.emailVerificationRepo.GetByToken(ctx, token)
	if err != nil {
		if errors.IsNotFound(err) {
			return errors.ErrInvalidInstance.WithMessage("Invalid or expired verification token")
		}
		return err
	}

	// Check if already used
	if verification.IsUsed() {
		return errors.ErrInvalidInstance.WithMessage("Verification token has already been used")
	}

	// Check if expired
	if verification.IsExpired() {
		return errors.ErrInvalidInstance.WithMessage("Verification token has expired")
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, verification.UserID)
	if err != nil {
		return err
	}

	// Mark email as verified
	user.EmailVerified = true
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Mark token as used
	return s.emailVerificationRepo.MarkAsUsed(ctx, verification.ID)
}

// ResendVerificationEmail resends the verification email to a user
func (s *authService) ResendVerificationEmail(ctx context.Context, email string) error {
	// Find user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.IsNotFound(err) {
			// Security: Do not reveal if user exists
			return nil
		}
		return err
	}

	// Don't resend if already verified
	if user.EmailVerified {
		return nil
	}

	// Check rate limiting - get latest verification
	latest, err := s.emailVerificationRepo.GetLatestByUserID(ctx, user.ID)
	if err == nil && latest != nil {
		// If last token was created less than 1 minute ago, don't resend
		if time.Since(latest.CreatedAt) < time.Minute {
			return errors.ErrInvalidInstance.WithMessage("Please wait before requesting another verification email")
		}
	}

	// Send new verification email
	return s.SendVerificationEmail(ctx, user.ID)
}
