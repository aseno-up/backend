package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	auth_model "github.com/aseno/backend/pkg/auth/model"
	auth_service "github.com/aseno/backend/pkg/auth/service"
	email_service "github.com/aseno/backend/pkg/email/service"
	"github.com/aseno/backend/pkg/onboarding/model"
	"github.com/aseno/backend/pkg/onboarding/repository"
)

// DefaultTokenTTL is how long a magic link stays usable from the moment of
// e-mail dispatch. Long enough to survive a delayed open, short enough that a
// leaked link doesn't become an indefinite signup vector.
const DefaultTokenTTL = 7 * 24 * time.Hour

// CompleteRequest is the payload posted from /onboarding/[token]/page.tsx
// after the visitor fills name + password + clinic name.
type CompleteRequest struct {
	Name       string
	Password   string
	ClinicName string
	Phone      string
}

// OnboardingService coordinates the magic-link flow:
//   - GenerateForLead: store a one-time token + ship the e-mail with the link.
//   - Validate: front-end calls this when the visitor opens the link to learn
//     which e-mail the form belongs to (and to detect expired/consumed links).
//   - Complete: visitor submits name + password + clinic name; we reuse the
//     existing auth.Register flow with EmailVerified=true (the click already
//     proved e-mail ownership) and mark the token consumed.
type OnboardingService interface {
	GenerateForLead(ctx context.Context, email, name, whatsapp, origem string) (string, *model.OnboardingToken, error)
	Validate(ctx context.Context, token string) (*model.OnboardingToken, error)
	Complete(ctx context.Context, token string, req CompleteRequest) (*auth_model.AuthResponse, error)
}

type onboardingService struct {
	repo        repository.OnboardingTokenRepository
	authSvc     auth_service.AuthService
	emailSvc    email_service.EmailService
	frontendURL string
	tokenTTL    time.Duration
	now         func() time.Time
}

// NewOnboardingService wires the dependencies. `frontendURL` is the absolute
// base for the link (e.g. "https://aseno.com.br"); the language segment and
// /onboarding/<token> are appended downstream. `tokenTTL` controls how long
// the e-mailed link stays usable — pass 0 to fall back to DefaultTokenTTL.
func NewOnboardingService(
	repo repository.OnboardingTokenRepository,
	authSvc auth_service.AuthService,
	emailSvc email_service.EmailService,
	frontendURL string,
	tokenTTL time.Duration,
) OnboardingService {
	ttl := tokenTTL
	if ttl <= 0 {
		ttl = DefaultTokenTTL
	}
	return &onboardingService{
		repo:        repo,
		authSvc:     authSvc,
		emailSvc:    emailSvc,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		tokenTTL:    ttl,
		now:         time.Now,
	}
}

func (s *onboardingService) GenerateForLead(
	ctx context.Context,
	email, name, whatsapp, origem string,
) (string, *model.OnboardingToken, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", nil, errors.ErrBadRequestInstance.WithMessage("email é obrigatório")
	}

	tokenString, err := generateToken()
	if err != nil {
		return "", nil, errors.ErrInternalInstance.WithError(err)
	}
	now := s.now()
	t := &model.OnboardingToken{
		ID:        uuid.New(),
		Token:     tokenString,
		Email:     email,
		Name:      strings.TrimSpace(name),
		WhatsApp:  strings.TrimSpace(whatsapp),
		Origem:    strings.TrimSpace(origem),
		ExpiresAt: now.Add(s.tokenTTL),
		CreatedAt: now,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return "", nil, err
	}

	if s.emailSvc != nil {
		link := s.buildLink(tokenString)
		_ = s.emailSvc.SendOnboardingLink(ctx, &email_service.OnboardingLinkEmailData{
			RecipientEmail: t.Email,
			RecipientName:  t.Name,
			OnboardingLink: link,
			ExpiresAt:      t.ExpiresAt,
		})
	}

	return tokenString, t, nil
}

func (s *onboardingService) Validate(ctx context.Context, token string) (*model.OnboardingToken, error) {
	t, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if !t.IsValid(s.now()) {
		return nil, errors.ErrConflictInstance.WithMessage("link expirado ou já utilizado")
	}
	return t, nil
}

func (s *onboardingService) Complete(
	ctx context.Context,
	token string,
	req CompleteRequest,
) (*auth_model.AuthResponse, error) {
	t, err := s.Validate(ctx, token)
	if err != nil {
		return nil, err
	}

	clinicName := strings.TrimSpace(req.ClinicName)
	if clinicName == "" {
		clinicName = strings.TrimSpace(req.Name)
	}
	if clinicName == "" {
		clinicName = deriveClinicNameFromEmail(t.Email)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.ErrBadRequestInstance.WithMessage("nome é obrigatório")
	}
	if len(req.Password) < 8 {
		return nil, errors.ErrBadRequestInstance.WithMessage("senha precisa ter pelo menos 8 caracteres")
	}

	phone := strings.TrimSpace(req.Phone)
	if phone == "" {
		phone = t.WhatsApp
	}

	resp, err := s.authSvc.Register(ctx, &auth_model.RegisterRequest{
		ClinicName:    clinicName,
		Email:         t.Email,
		Password:      req.Password,
		Name:          name,
		Phone:         phone,
		Plan:          "trial",
		EmailVerified: true,
	})
	if err != nil {
		return nil, err
	}

	if err := s.repo.MarkUsed(ctx, t.ID, s.now()); err != nil {
		// Account is already created; the token-consume failure is best-effort
		// — log and continue so the user isn't stuck on a screen error.
		_ = err
	}

	return resp, nil
}

func (s *onboardingService) buildLink(token string) string {
	base := s.frontendURL
	if base == "" {
		base = "https://aseno.com.br"
	}
	return base + "/pt-BR/onboarding/" + token
}

// generateToken returns a URL-safe 32-byte hex string (64 chars), big enough
// to make guessing impractical and short enough to fit a single line in the
// e-mail body.
func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// deriveClinicNameFromEmail produces a reasonable default when the visitor
// skipped the clinic-name field — usually the local part of the e-mail
// title-cased. The user can rename it from settings later.
func deriveClinicNameFromEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return "Minha Clínica"
	}
	local := email[:at]
	cleaned := strings.NewReplacer(".", " ", "_", " ", "-", " ", "+", " ").Replace(local)
	parts := strings.Fields(cleaned)
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	out := strings.Join(parts, " ")
	if out == "" {
		return "Minha Clínica"
	}
	return out
}
