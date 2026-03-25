package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	auth_model "github.com/aseno/backend/pkg/auth/model"
	auth_service "github.com/aseno/backend/pkg/auth/service"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	email_service "github.com/aseno/backend/pkg/email/service"
	"github.com/aseno/backend/pkg/invite/model"
	"github.com/aseno/backend/pkg/invite/repository"
	user_model "github.com/aseno/backend/pkg/user/model"
	user_repository "github.com/aseno/backend/pkg/user/repository"
)

const (
	// InviteExpirationDays is the number of days an invite is valid
	InviteExpirationDays = 7
	// TokenLength is the length of the invite token in bytes (will be hex encoded)
	TokenLength = 32
)

// InviteService defines the interface for invite business logic
type InviteService interface {
	CreateInvite(ctx context.Context, clinicID, invitedByID uuid.UUID, req *model.CreateInviteRequest) (*model.InviteResponse, error)
	AcceptInvite(ctx context.Context, req *model.AcceptInviteRequest) (*auth_model.AuthResponse, error)
	GetInviteByToken(ctx context.Context, token string) (*model.InviteDetailsResponse, error)
	ListInvites(ctx context.Context, clinicID uuid.UUID, page, perPage int) (*model.InviteListResponse, error)
	CancelInvite(ctx context.Context, inviteID, clinicID uuid.UUID) error
	ResendInvite(ctx context.Context, inviteID, clinicID uuid.UUID, resenderID uuid.UUID) error
}

// inviteService implements InviteService
type inviteService struct {
	inviteRepo   repository.InviteRepository
	userRepo     user_repository.UserRepository
	clinicRepo   clinic_repository.ClinicRepository
	emailService email_service.EmailService
	jwtService   auth_service.JWTService
	frontendURL  string
}

// NewInviteService creates a new invite service
func NewInviteService(
	inviteRepo repository.InviteRepository,
	userRepo user_repository.UserRepository,
	clinicRepo clinic_repository.ClinicRepository,
	emailService email_service.EmailService,
	jwtService auth_service.JWTService,
	frontendURL string,
) InviteService {
	return &inviteService{
		inviteRepo:   inviteRepo,
		userRepo:     userRepo,
		clinicRepo:   clinicRepo,
		emailService: emailService,
		jwtService:   jwtService,
		frontendURL:  frontendURL,
	}
}

// CreateInvite creates a new invitation
func (s *inviteService) CreateInvite(ctx context.Context, clinicID, invitedByID uuid.UUID, req *model.CreateInviteRequest) (*model.InviteResponse, error) {
	// Check if user already exists in this clinic
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if existingUser != nil && existingUser.ClinicID == clinicID {
		return nil, errors.ErrEmailAlreadyExistsInstance.WithMessage("User already exists in this clinic")
	}

	// Check for pending invite
	pendingInvite, err := s.inviteRepo.GetPendingByEmail(ctx, req.Email, clinicID)
	if err != nil {
		return nil, err
	}
	if pendingInvite != nil {
		return nil, errors.ErrInvitePendingInstance
	}

	// Get clinic for email
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	// Get inviter for email
	inviter, err := s.userRepo.GetByID(ctx, invitedByID, clinicID)
	if err != nil {
		return nil, err
	}

	// Generate secure token
	token, err := generateSecureToken()
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Create invite
	invite := &model.Invite{
		ClinicID:  clinicID,
		Email:     req.Email,
		Role:      req.Role,
		Token:     token,
		Status:    model.InviteStatusPending,
		InvitedBy: invitedByID,
		ExpiresAt: time.Now().AddDate(0, 0, InviteExpirationDays),
	}

	if err := s.inviteRepo.Create(ctx, invite); err != nil {
		return nil, err
	}

	// Send invite email
	inviteLink := fmt.Sprintf("%s/pt-BR/accept-invite?token=%s", s.frontendURL, token)
	roleLabel := getRoleLabel(req.Role)

	if err := s.emailService.SendInvite(ctx, &email_service.InviteEmailData{
		RecipientEmail: req.Email,
		ClinicName:     clinic.Name,
		InviterName:    inviter.Name,
		Role:           roleLabel,
		InviteLink:     inviteLink,
		ExpiresAt:      invite.ExpiresAt,
	}); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to send invite email: %v\n", err)
	}

	return invite.ToResponse(), nil
}

// AcceptInvite accepts an invitation and creates a new user
func (s *inviteService) AcceptInvite(ctx context.Context, req *model.AcceptInviteRequest) (*auth_model.AuthResponse, error) {
	// Get invite by token
	invite, err := s.inviteRepo.GetByToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	// Validate invite status
	if invite.Status != model.InviteStatusPending {
		return nil, errors.ErrInviteAlreadyAcceptedInstance
	}

	// Check expiration
	if invite.IsExpired() {
		// Update status to expired
		invite.Status = model.InviteStatusExpired
		s.inviteRepo.Update(ctx, invite)
		return nil, errors.ErrInviteExpiredInstance
	}

	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, invite.Email)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	var user *user_model.User

	if existingUser != nil {
		// User exists - they might be in another clinic
		// For now, we don't allow users in multiple clinics
		return nil, errors.ErrEmailAlreadyExistsInstance.WithMessage("An account with this email already exists")
	}

	// Create new user
	user = &user_model.User{
		ClinicID:      invite.ClinicID,
		Email:         invite.Email,
		Name:          req.Name,
		Role:          invite.Role,
		EmailVerified: true, // Verified via invite
		IsActive:      true,
	}

	if err := user.SetPassword(req.Password); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Update invite status
	now := time.Now()
	invite.Status = model.InviteStatusAccepted
	invite.AcceptedAt = &now
	if err := s.inviteRepo.Update(ctx, invite); err != nil {
		// Log but don't fail
		fmt.Printf("Failed to update invite status: %v\n", err)
	}

	// Get clinic for response
	clinic, err := s.clinicRepo.GetByID(ctx, invite.ClinicID)
	if err != nil {
		return nil, err
	}

	// Generate tokens
	tokenPair, err := s.jwtService.GenerateTokenPair(user)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Save refresh token
	if err := s.userRepo.UpdateRefreshToken(ctx, user.ID, tokenPair.RefreshToken); err != nil {
		return nil, err
	}

	// Update last login
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		// Log but don't fail
		fmt.Printf("Failed to update last login: %v\n", err)
	}

	// Send welcome email
	loginLink := fmt.Sprintf("%s/pt-BR/login", s.frontendURL)
	if err := s.emailService.SendWelcome(ctx, &email_service.WelcomeEmailData{
		RecipientEmail: user.Email,
		RecipientName:  user.Name,
		ClinicName:     clinic.Name,
		LoginLink:      loginLink,
	}); err != nil {
		// Log but don't fail
		fmt.Printf("Failed to send welcome email: %v\n", err)
	}

	return &auth_model.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// GetInviteByToken retrieves invite details for display
func (s *inviteService) GetInviteByToken(ctx context.Context, token string) (*model.InviteDetailsResponse, error) {
	invite, err := s.inviteRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Get clinic name
	clinic, err := s.clinicRepo.GetByID(ctx, invite.ClinicID)
	if err != nil {
		return nil, err
	}

	// Get inviter name
	inviter, err := s.userRepo.GetByID(ctx, invite.InvitedBy, invite.ClinicID)
	if err != nil {
		return nil, err
	}

	isExpired := invite.IsExpired()
	isValid := invite.Status == model.InviteStatusPending && !isExpired

	return &model.InviteDetailsResponse{
		ID:          invite.ID,
		Email:       invite.Email,
		Role:        invite.Role,
		ClinicName:  clinic.Name,
		InviterName: inviter.Name,
		ExpiresAt:   invite.ExpiresAt,
		IsValid:     isValid,
		IsExpired:   isExpired,
	}, nil
}

// ListInvites returns paginated invites for a clinic
func (s *inviteService) ListInvites(ctx context.Context, clinicID uuid.UUID, page, perPage int) (*model.InviteListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	invites, total, err := s.inviteRepo.GetByClinicID(ctx, clinicID, perPage, offset)
	if err != nil {
		return nil, err
	}

	responses := make([]*model.InviteResponse, len(invites))
	for i, invite := range invites {
		responses[i] = invite.ToResponse()
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return &model.InviteListResponse{
		Data:       responses,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

// CancelInvite cancels a pending invite
func (s *inviteService) CancelInvite(ctx context.Context, inviteID, clinicID uuid.UUID) error {
	invite, err := s.inviteRepo.GetByID(ctx, inviteID)
	if err != nil {
		return err
	}

	// Verify clinic ownership
	if invite.ClinicID != clinicID {
		return errors.ErrForbiddenInstance
	}

	// Can only cancel pending invites
	if invite.Status != model.InviteStatusPending {
		return errors.ErrBadRequestInstance.WithMessage("Only pending invites can be canceled")
	}

	invite.Status = model.InviteStatusCanceled
	return s.inviteRepo.Update(ctx, invite)
}

// ResendInvite resends an invite email
func (s *inviteService) ResendInvite(ctx context.Context, inviteID, clinicID uuid.UUID, resenderID uuid.UUID) error {
	invite, err := s.inviteRepo.GetByID(ctx, inviteID)
	if err != nil {
		return err
	}

	// Verify clinic ownership
	if invite.ClinicID != clinicID {
		return errors.ErrForbiddenInstance
	}

	// Can only resend pending invites
	if invite.Status != model.InviteStatusPending {
		return errors.ErrBadRequestInstance.WithMessage("Only pending invites can be resent")
	}

	// Generate new token and extend expiration
	token, err := generateSecureToken()
	if err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}

	invite.Token = token
	invite.ExpiresAt = time.Now().AddDate(0, 0, InviteExpirationDays)

	if err := s.inviteRepo.Update(ctx, invite); err != nil {
		return err
	}

	// Get clinic and resender for email
	clinic, err := s.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return err
	}

	resender, err := s.userRepo.GetByID(ctx, resenderID, clinicID)
	if err != nil {
		return err
	}

	// Send email
	inviteLink := fmt.Sprintf("%s/pt-BR/accept-invite?token=%s", s.frontendURL, token)
	roleLabel := getRoleLabel(invite.Role)

	return s.emailService.SendInvite(ctx, &email_service.InviteEmailData{
		RecipientEmail: invite.Email,
		ClinicName:     clinic.Name,
		InviterName:    resender.Name,
		Role:           roleLabel,
		InviteLink:     inviteLink,
		ExpiresAt:      invite.ExpiresAt,
	})
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// getRoleLabel returns a human-readable role label
func getRoleLabel(role user_model.UserRole) string {
	switch role {
	case user_model.UserRoleAdmin:
		return "Administrador"
	case user_model.UserRoleProfessional:
		return "Profissional"
	case user_model.UserRoleReceptionist:
		return "Recepcionista"
	default:
		return string(role)
	}
}
