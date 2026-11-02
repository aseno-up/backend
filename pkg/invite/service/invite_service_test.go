package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aseno/backend/internal/errors"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	email_service "github.com/aseno/backend/pkg/email/service"
	"github.com/aseno/backend/pkg/invite/model"
	"github.com/aseno/backend/pkg/pagination"
	user_model "github.com/aseno/backend/pkg/user/model"
)

// ---------- Mock: InviteRepository ----------

type mockInviteRepo struct {
	mock.Mock
}

func (m *mockInviteRepo) Create(ctx context.Context, invite *model.Invite) error {
	return m.Called(ctx, invite).Error(0)
}
func (m *mockInviteRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Invite, error) {
	args := m.Called(ctx, id)
	var inv *model.Invite
	if args.Get(0) != nil {
		inv = args.Get(0).(*model.Invite)
	}
	return inv, args.Error(1)
}
func (m *mockInviteRepo) GetByToken(ctx context.Context, token string) (*model.Invite, error) {
	args := m.Called(ctx, token)
	var inv *model.Invite
	if args.Get(0) != nil {
		inv = args.Get(0).(*model.Invite)
	}
	return inv, args.Error(1)
}
func (m *mockInviteRepo) GetByClinicID(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*model.Invite, int64, error) {
	args := m.Called(ctx, clinicID, limit, offset)
	return args.Get(0).([]*model.Invite), args.Get(1).(int64), args.Error(2)
}
func (m *mockInviteRepo) GetPendingByEmail(ctx context.Context, email string, clinicID uuid.UUID) (*model.Invite, error) {
	args := m.Called(ctx, email, clinicID)
	var inv *model.Invite
	if args.Get(0) != nil {
		inv = args.Get(0).(*model.Invite)
	}
	return inv, args.Error(1)
}
func (m *mockInviteRepo) Update(ctx context.Context, invite *model.Invite) error {
	return m.Called(ctx, invite).Error(0)
}
func (m *mockInviteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockInviteRepo) ExpireOldInvites(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// ---------- Mock: UserRepository ----------

type mockUserRepo struct {
	mock.Mock
}

func (m *mockUserRepo) Create(ctx context.Context, user *user_model.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *mockUserRepo) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*user_model.User, error) {
	args := m.Called(ctx, id, clinicID)
	var u *user_model.User
	if args.Get(0) != nil {
		u = args.Get(0).(*user_model.User)
	}
	return u, args.Error(1)
}
func (m *mockUserRepo) GetByIDNoTenant(ctx context.Context, id uuid.UUID) (*user_model.User, error) {
	args := m.Called(ctx, id)
	var u *user_model.User
	if args.Get(0) != nil {
		u = args.Get(0).(*user_model.User)
	}
	return u, args.Error(1)
}
func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*user_model.User, error) {
	args := m.Called(ctx, email)
	var u *user_model.User
	if args.Get(0) != nil {
		u = args.Get(0).(*user_model.User)
	}
	return u, args.Error(1)
}
func (m *mockUserRepo) GetByGoogleID(ctx context.Context, googleID string) (*user_model.User, error) {
	args := m.Called(ctx, googleID)
	var u *user_model.User
	if args.Get(0) != nil {
		u = args.Get(0).(*user_model.User)
	}
	return u, args.Error(1)
}
func (m *mockUserRepo) GetByClinicID(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*user_model.User, int64, error) {
	args := m.Called(ctx, clinicID, filter)
	return args.Get(0).([]*user_model.User), args.Get(1).(int64), args.Error(2)
}
func (m *mockUserRepo) Update(ctx context.Context, user *user_model.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *mockUserRepo) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return m.Called(ctx, id, clinicID).Error(0)
}
func (m *mockUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}
func (m *mockUserRepo) UpdateRefreshToken(ctx context.Context, userID uuid.UUID, token string) error {
	return m.Called(ctx, userID, token).Error(0)
}
func (m *mockUserRepo) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	return m.Called(ctx, userID).Error(0)
}
func (m *mockUserRepo) BulkDelete(ctx context.Context, clinicID uuid.UUID, ids []uuid.UUID) error {
	return m.Called(ctx, clinicID, ids).Error(0)
}

// ---------- Mock: ClinicRepository ----------

type mockClinicRepo struct {
	mock.Mock
}

func (m *mockClinicRepo) Create(ctx context.Context, clinic *clinic_model.Clinic) error {
	return m.Called(ctx, clinic).Error(0)
}
func (m *mockClinicRepo) GetByID(ctx context.Context, id uuid.UUID) (*clinic_model.Clinic, error) {
	args := m.Called(ctx, id)
	var c *clinic_model.Clinic
	if args.Get(0) != nil {
		c = args.Get(0).(*clinic_model.Clinic)
	}
	return c, args.Error(1)
}
func (m *mockClinicRepo) GetBySlug(ctx context.Context, slug string) (*clinic_model.Clinic, error) {
	args := m.Called(ctx, slug)
	var c *clinic_model.Clinic
	if args.Get(0) != nil {
		c = args.Get(0).(*clinic_model.Clinic)
	}
	return c, args.Error(1)
}
func (m *mockClinicRepo) GetByEmail(ctx context.Context, email string) (*clinic_model.Clinic, error) {
	args := m.Called(ctx, email)
	var c *clinic_model.Clinic
	if args.Get(0) != nil {
		c = args.Get(0).(*clinic_model.Clinic)
	}
	return c, args.Error(1)
}
func (m *mockClinicRepo) Update(ctx context.Context, clinic *clinic_model.Clinic) error {
	return m.Called(ctx, clinic).Error(0)
}
func (m *mockClinicRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockClinicRepo) List(ctx context.Context, limit, offset int) ([]*clinic_model.Clinic, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*clinic_model.Clinic), args.Get(1).(int64), args.Error(2)
}
func (m *mockClinicRepo) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	args := m.Called(ctx, slug)
	return args.Bool(0), args.Error(1)
}
func (m *mockClinicRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}
func (m *mockClinicRepo) CountByAccountID(ctx context.Context, accountID uuid.UUID) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

// ---------- Mock: EmailService ----------

type mockEmailService struct {
	mock.Mock
}

func (m *mockEmailService) SendInvite(ctx context.Context, data *email_service.InviteEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendWelcome(ctx context.Context, data *email_service.WelcomeEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendPasswordReset(ctx context.Context, data *email_service.ResetPasswordEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendEmailVerification(ctx context.Context, data *email_service.EmailVerificationData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendContactForm(ctx context.Context, data *email_service.ContactFormData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendBookingConfirmation(ctx context.Context, data *email_service.BookingConfirmationEmailData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendPortalEmailVerification(ctx context.Context, data *email_service.PortalEmailVerificationData) error {
	return m.Called(ctx, data).Error(0)
}
func (m *mockEmailService) SendLeadIsca(ctx context.Context, data *email_service.LeadIscaEmailData) error {
	return m.Called(ctx, data).Error(0)
}

// ---------- Tests ----------

// TestCreateInvite_InviterHomeClinicDiffersFromActive reproduces the bug:
// inviter's users.clinic_id is clinic A (home), but they are currently acting
// on clinic B (via clinic_members after SwitchClinic). The invite must still
// succeed — the inviter lookup should not tenant-filter by the active clinic.
func TestCreateInvite_InviterHomeClinicDiffersFromActive(t *testing.T) {
	ctx := context.Background()

	activeClinicID := uuid.New() // clinic B (active, from JWT)
	homeClinicID := uuid.New()   // clinic A (user's users.clinic_id)
	inviterID := uuid.New()

	inviter := &user_model.User{
		ID:       inviterID,
		Name:     "Dr. Hamilton",
		Email:    "hamilton@example.com",
		ClinicID: homeClinicID, // home clinic is NOT the active one
		Role:     user_model.UserRoleOwner,
	}

	activeClinic := &clinic_model.Clinic{
		ID:   activeClinicID,
		Name: "Clinica B",
	}

	inviteRepo := new(mockInviteRepo)
	userRepo := new(mockUserRepo)
	clinicRepo := new(mockClinicRepo)
	emailSvc := new(mockEmailService)

	// No existing user with the invitee email.
	userRepo.On("GetByEmail", ctx, "newuser@example.com").
		Return(nil, errors.ErrUserNotFoundInstance)

	// No pending invite.
	inviteRepo.On("GetPendingByEmail", ctx, "newuser@example.com", activeClinicID).
		Return(nil, nil)

	// Active clinic lookup succeeds.
	clinicRepo.On("GetByID", ctx, activeClinicID).Return(activeClinic, nil)

	// BUG REPRO: GetByID(inviterID, activeClinicID) would return NotFound because
	// the inviter's users.clinic_id is homeClinicID, not activeClinicID.
	// The fix uses GetByIDNoTenant, which only needs the ID.
	userRepo.On("GetByIDNoTenant", ctx, inviterID).Return(inviter, nil)

	// Invite is persisted and email is sent.
	inviteRepo.On("Create", ctx, mock.AnythingOfType("*model.Invite")).Return(nil)
	emailSvc.On("SendInvite", ctx, mock.AnythingOfType("*service.InviteEmailData")).Return(nil)

	svc := &inviteService{
		inviteRepo:   inviteRepo,
		userRepo:     userRepo,
		clinicRepo:   clinicRepo,
		emailService: emailSvc,
		frontendURL:  "https://app.test",
	}

	req := &model.CreateInviteRequest{
		Email: "newuser@example.com",
		Role:  user_model.UserRoleReceptionist,
	}

	resp, err := svc.CreateInvite(ctx, activeClinicID, inviterID, req)

	assert.NoError(t, err, "invite should succeed even when inviter's home clinic differs from the active clinic")
	assert.NotNil(t, resp)
	assert.Equal(t, activeClinicID, resp.ClinicID)
	assert.Equal(t, "newuser@example.com", resp.Email)

	userRepo.AssertCalled(t, "GetByIDNoTenant", ctx, inviterID)
	emailSvc.AssertCalled(t, "SendInvite", ctx, mock.AnythingOfType("*service.InviteEmailData"))
}

// TestResendInvite_ResenderHomeClinicDiffersFromActive covers the same bug on the
// resend path: the resender acting from a non-home clinic must still be found.
func TestResendInvite_ResenderHomeClinicDiffersFromActive(t *testing.T) {
	ctx := context.Background()

	activeClinicID := uuid.New()
	homeClinicID := uuid.New()
	resenderID := uuid.New()
	inviteID := uuid.New()

	resender := &user_model.User{
		ID:       resenderID,
		Name:     "Dr. Hamilton",
		Email:    "hamilton@example.com",
		ClinicID: homeClinicID,
		Role:     user_model.UserRoleOwner,
	}

	activeClinic := &clinic_model.Clinic{ID: activeClinicID, Name: "Clinica B"}

	existingInvite := &model.Invite{
		ID:        inviteID,
		ClinicID:  activeClinicID,
		Email:     "pending@example.com",
		Role:      user_model.UserRoleReceptionist,
		Status:    model.InviteStatusPending,
		InvitedBy: resenderID,
	}

	inviteRepo := new(mockInviteRepo)
	userRepo := new(mockUserRepo)
	clinicRepo := new(mockClinicRepo)
	emailSvc := new(mockEmailService)

	inviteRepo.On("GetByID", ctx, inviteID).Return(existingInvite, nil)
	inviteRepo.On("Update", ctx, mock.AnythingOfType("*model.Invite")).Return(nil)
	clinicRepo.On("GetByID", ctx, activeClinicID).Return(activeClinic, nil)
	userRepo.On("GetByIDNoTenant", ctx, resenderID).Return(resender, nil)
	emailSvc.On("SendInvite", ctx, mock.AnythingOfType("*service.InviteEmailData")).Return(nil)

	svc := &inviteService{
		inviteRepo:   inviteRepo,
		userRepo:     userRepo,
		clinicRepo:   clinicRepo,
		emailService: emailSvc,
		frontendURL:  "https://app.test",
	}

	err := svc.ResendInvite(ctx, inviteID, activeClinicID, resenderID)

	assert.NoError(t, err, "resend should succeed even when resender's home clinic differs from the active clinic")
	userRepo.AssertCalled(t, "GetByIDNoTenant", ctx, resenderID)
}

// TestGetInviteByToken_InviterHomeClinicDiffersFromInviteClinic covers the
// public invite-details endpoint: even if the inviter later switched home
// clinic, the stored InvitedBy must still resolve.
func TestGetInviteByToken_InviterHomeClinicDiffersFromInviteClinic(t *testing.T) {
	ctx := context.Background()

	inviteClinicID := uuid.New()
	inviterHomeClinicID := uuid.New()
	inviterID := uuid.New()

	inviter := &user_model.User{
		ID:       inviterID,
		Name:     "Dr. Hamilton",
		ClinicID: inviterHomeClinicID,
		Role:     user_model.UserRoleOwner,
	}

	clinic := &clinic_model.Clinic{ID: inviteClinicID, Name: "Clinica B"}

	invite := &model.Invite{
		ID:        uuid.New(),
		ClinicID:  inviteClinicID,
		Email:     "pending@example.com",
		Role:      user_model.UserRoleReceptionist,
		Status:    model.InviteStatusPending,
		InvitedBy: inviterID,
		Token:     "tok123",
	}

	inviteRepo := new(mockInviteRepo)
	userRepo := new(mockUserRepo)
	clinicRepo := new(mockClinicRepo)

	inviteRepo.On("GetByToken", ctx, "tok123").Return(invite, nil)
	clinicRepo.On("GetByID", ctx, inviteClinicID).Return(clinic, nil)
	userRepo.On("GetByIDNoTenant", ctx, inviterID).Return(inviter, nil)
	userRepo.On("GetByEmail", ctx, "pending@example.com").
		Return(nil, errors.ErrUserNotFoundInstance)

	svc := &inviteService{
		inviteRepo: inviteRepo,
		userRepo:   userRepo,
		clinicRepo: clinicRepo,
	}

	resp, err := svc.GetInviteByToken(ctx, "tok123")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Dr. Hamilton", resp.InviterName)
	userRepo.AssertCalled(t, "GetByIDNoTenant", ctx, inviterID)
}
