package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/pagination"
	professional_repository "github.com/aseno/backend/pkg/professional/repository"
	"github.com/aseno/backend/pkg/user/model"
	"github.com/aseno/backend/pkg/user/repository"
)

// UserService defines the interface for user business logic
type UserService interface {
	ListUsers(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.UserResponse], error)
	GetUser(ctx context.Context, clinicID, userID uuid.UUID) (*model.UserResponse, error)
	UpdateUser(ctx context.Context, clinicID, userID, currentUserID uuid.UUID, req *model.UpdateUserRequest) (*model.UserResponse, error)
	UpdateRole(ctx context.Context, clinicID, userID, currentUserID uuid.UUID, role model.UserRole) (*model.UserResponse, error)
	DeactivateUser(ctx context.Context, clinicID, userID, currentUserID uuid.UUID) error
	ReactivateUser(ctx context.Context, clinicID, userID uuid.UUID) error
	RemoveUser(ctx context.Context, clinicID, userID, currentUserID uuid.UUID) error
	BulkRemoveUsers(ctx context.Context, clinicID uuid.UUID, currentUserID uuid.UUID, userIDs []uuid.UUID) error

	// Tutorials — tracked per user so the in-app tour only shows to those who haven't seen it.
	GetTutorialsCompleted(ctx context.Context, clinicID, userID uuid.UUID) ([]string, error)
	MarkTutorialCompleted(ctx context.Context, clinicID, userID uuid.UUID, page string) ([]string, error)
	ResetTutorials(ctx context.Context, clinicID, userID uuid.UUID) error
}

// UserListResponse is now deprecated and defined by pagination.Response

// userService implements UserService
type userService struct {
	userRepo         repository.UserRepository
	professionalRepo professional_repository.ProfessionalRepository
}

// NewUserService creates a new user service
func NewUserService(userRepo repository.UserRepository, professionalRepo professional_repository.ProfessionalRepository) UserService {
	return &userService{
		userRepo:         userRepo,
		professionalRepo: professionalRepo,
	}
}

// loadProfessionalID looks up the professional linked to a user and populates user.ProfessionalID.
func (s *userService) loadProfessionalID(ctx context.Context, user *model.User) {
	if s.professionalRepo == nil {
		return
	}
	prof, err := s.professionalRepo.GetByUserID(ctx, user.ID, user.ClinicID)
	if err == nil && prof != nil {
		user.ProfessionalID = &prof.ID
	}
}

// ListUsers returns paginated users for a clinic
func (s *userService) ListUsers(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) (pagination.Response[*model.UserResponse], error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 10
	}

	users, total, err := s.userRepo.GetByClinicID(ctx, clinicID, filter)
	if err != nil {
		return pagination.Response[*model.UserResponse]{}, err
	}

	data := make([]*model.UserResponse, len(users))
	for i, user := range users {
		s.loadProfessionalID(ctx, user)
		data[i] = user.ToResponse()
	}

	totalPages := int((total + int64(filter.Limit) - 1) / int64(filter.Limit))

	return pagination.Response[*model.UserResponse]{
		Data: data,
		Meta: pagination.Meta{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// GetUser returns a user by ID
func (s *userService) GetUser(ctx context.Context, clinicID, userID uuid.UUID) (*model.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return nil, err
	}

	s.loadProfessionalID(ctx, user)
	return user.ToResponse(), nil
}

// UpdateUser updates a user's information
func (s *userService) UpdateUser(ctx context.Context, clinicID, userID, currentUserID uuid.UUID, req *model.UpdateUserRequest) (*model.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}
	if req.IsActive != nil {
		// Cannot deactivate self
		if userID == currentUserID && !*req.IsActive {
			return nil, errors.ErrCannotDeactivateSelfInstance
		}
		// Cannot deactivate owner
		if user.Role == model.UserRoleOwner && !*req.IsActive {
			return nil, errors.ErrCannotRemoveOwnerInstance
		}
		user.IsActive = *req.IsActive
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user.ToResponse(), nil
}

// UpdateRole changes a user's role
func (s *userService) UpdateRole(ctx context.Context, clinicID, userID, currentUserID uuid.UUID, role model.UserRole) (*model.UserResponse, error) {
	// Cannot change own role
	if userID == currentUserID {
		return nil, errors.ErrCannotChangeOwnRoleInstance
	}

	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return nil, err
	}

	// Cannot change owner role
	if user.Role == model.UserRoleOwner {
		return nil, errors.ErrCannotRemoveOwnerInstance.WithMessage("Cannot change owner role")
	}

	// Cannot promote to owner
	if role == model.UserRoleOwner {
		return nil, errors.ErrForbiddenInstance.WithMessage("Cannot promote to owner")
	}

	// When changing to professional role, user must have a linked professional
	if role == model.UserRoleProfessional && s.professionalRepo != nil {
		prof, err := s.professionalRepo.GetByUserID(ctx, userID, clinicID)
		if err != nil || prof == nil {
			return nil, errors.ErrValidationInstance.WithMessage(
				"Para alterar a role para profissional, é necessário vincular um profissional primeiro")
		}
	}

	user.Role = role

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	s.loadProfessionalID(ctx, user)
	return user.ToResponse(), nil
}

// DeactivateUser deactivates a user account
func (s *userService) DeactivateUser(ctx context.Context, clinicID, userID, currentUserID uuid.UUID) error {
	// Cannot deactivate self
	if userID == currentUserID {
		return errors.ErrCannotDeactivateSelfInstance
	}

	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return err
	}

	// Cannot deactivate owner
	if user.Role == model.UserRoleOwner {
		return errors.ErrCannotRemoveOwnerInstance
	}

	user.IsActive = false
	return s.userRepo.Update(ctx, user)
}

// ReactivateUser reactivates a user account
func (s *userService) ReactivateUser(ctx context.Context, clinicID, userID uuid.UUID) error {
	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return err
	}

	user.IsActive = true
	return s.userRepo.Update(ctx, user)
}

// RemoveUser removes a user from the clinic (soft delete)
func (s *userService) RemoveUser(ctx context.Context, clinicID, userID, currentUserID uuid.UUID) error {
	// Cannot remove self
	if userID == currentUserID {
		return errors.ErrCannotDeactivateSelfInstance.WithMessage("Cannot remove your own account")
	}

	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return err
	}

	// Cannot remove owner
	if user.Role == model.UserRoleOwner {
		return errors.ErrCannotRemoveOwnerInstance
	}

	// Deactivate linked professional (keeps history for reports, but removes from schedule)
	if s.professionalRepo != nil {
		prof, err := s.professionalRepo.GetByUserID(ctx, userID, clinicID)
		if err == nil && prof != nil {
			prof.IsActive = false
			_ = s.professionalRepo.Update(ctx, prof)
		}
	}

	return s.userRepo.Delete(ctx, userID, clinicID)
}

// GetTutorialsCompleted returns the list of tutorial pages the user has already finished.
// Always returns a non-nil slice so JSON serialization yields [] instead of null.
func (s *userService) GetTutorialsCompleted(ctx context.Context, clinicID, userID uuid.UUID) ([]string, error) {
	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return nil, err
	}
	if user.TutorialsCompleted == nil {
		return []string{}, nil
	}
	return []string(user.TutorialsCompleted), nil
}

// MarkTutorialCompleted records that the user finished a tutorial page. Idempotent:
// if the page is already in the list, it returns the current list without touching the DB.
func (s *userService) MarkTutorialCompleted(ctx context.Context, clinicID, userID uuid.UUID, page string) ([]string, error) {
	page = strings.TrimSpace(page)
	if page == "" {
		return nil, errors.ErrValidationInstance.WithMessage("tutorial page is required")
	}

	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return nil, err
	}

	for _, p := range user.TutorialsCompleted {
		if p == page {
			return []string(user.TutorialsCompleted), nil
		}
	}

	user.TutorialsCompleted = append(user.TutorialsCompleted, page)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	return []string(user.TutorialsCompleted), nil
}

// ResetTutorials clears the completed list so the user can watch the tour again.
func (s *userService) ResetTutorials(ctx context.Context, clinicID, userID uuid.UUID) error {
	user, err := s.userRepo.GetByID(ctx, userID, clinicID)
	if err != nil {
		return err
	}
	user.TutorialsCompleted = pq.StringArray{}
	return s.userRepo.Update(ctx, user)
}

// BulkRemoveUsers soft deletes multiple users (skip the current user if they accidentally included themselves)
func (s *userService) BulkRemoveUsers(ctx context.Context, clinicID uuid.UUID, currentUserID uuid.UUID, userIDs []uuid.UUID) error {
	var idsToDelete []uuid.UUID
	for _, id := range userIDs {
		if id != currentUserID {
			idsToDelete = append(idsToDelete, id)
		}
	}

	if len(idsToDelete) == 0 {
		return errors.ErrValidationInstance.WithMessage("Não é possível remover a si próprio desta forma")
	}

	// Deactivate linked professionals
	if s.professionalRepo != nil {
		for _, uid := range idsToDelete {
			prof, err := s.professionalRepo.GetByUserID(ctx, uid, clinicID)
			if err == nil && prof != nil {
				prof.IsActive = false
				_ = s.professionalRepo.Update(ctx, prof)
			}
		}
	}

	if err := s.userRepo.BulkDelete(ctx, clinicID, idsToDelete); err != nil {
		return err
	}
	return nil
}
