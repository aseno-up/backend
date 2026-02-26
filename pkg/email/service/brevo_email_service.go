package service

import (
	"context"
	"fmt"

	"github.com/getbrevo/brevo-go/lib"
)

// brevoEmailService implements EmailService using the Brevo (formerly Sendinblue) API
type brevoEmailService struct {
	client      *lib.APIClient
	senderName  string
	senderEmail string
}

// NewBrevoEmailService creates a new Brevo email service
func NewBrevoEmailService(apiKey, senderEmail, senderName string) (EmailService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("brevo API key is required")
	}

	if senderEmail == "" {
		return nil, fmt.Errorf("sender email address is required")
	}

	if senderName == "" {
		senderName = "Aseno" // Default sender name
	}

	cfg := lib.NewConfiguration()
	cfg.AddDefaultHeader("api-key", apiKey)
	client := lib.NewAPIClient(cfg)

	return &brevoEmailService{
			client:      client,
			senderName:  senderName,
			senderEmail: senderEmail,
		},
		nil
}

// SendPasswordReset sends a password reset email using Brevo
func (s *brevoEmailService) SendPasswordReset(ctx context.Context, data *ResetPasswordEmailData) error {
	_, _, err := s.client.TransactionalEmailsApi.SendTransacEmail(ctx, lib.SendSmtpEmail{
		Sender: &lib.SendSmtpEmailSender{
			Name:  s.senderName,
			Email: s.senderEmail,
		},
		To: []lib.SendSmtpEmailTo{
			{
				Email: data.RecipientEmail,
				Name:  data.RecipientName,
			},
		},
		Subject: "Redefinição de senha — Aseno",
		HtmlContent: fmt.Sprintf(`
			<p>Olá,</p>
            <p>Recebemos uma solicitação para redefinir sua senha.</p>
            <p>Clique no botão abaixo para criar uma nova senha:</p>
            <p><a href="%s">Redefinir Senha</a></p>
            <p><small>Este link expira em 15 minutos.</small></p>
            <p><small>Se você não solicitou isso, ignore este e-mail.</small></p>
		`,
			data.ResetLink),
	})

	if err != nil {
		return fmt.Errorf("failed to send password reset email via Brevo: %w", err)
	}
	return nil
}

// SendInvite sends an invitation email using Brevo
func (s *brevoEmailService) SendInvite(ctx context.Context, data *InviteEmailData) error {
	// For now, we'll just log this. Implementation can be added later.
	fmt.Printf("[BREVO] Would send invite to %s for %s (link: %s)\n",
		data.RecipientEmail,
		data.ClinicName,
		data.InviteLink)
	return nil
}

// SendWelcome sends a welcome email using Brevo
func (s *brevoEmailService) SendWelcome(ctx context.Context, data *WelcomeEmailData) error {
	// For now, we'll just log this. Implementation can be added later.
	fmt.Printf("[BREVO] Would send welcome to %s (%s) for %s\n",
		data.RecipientEmail,
		data.RecipientName,
		data.ClinicName)
	return nil
}

// SendEmailVerification sends an email verification email using Brevo
func (s *brevoEmailService) SendEmailVerification(ctx context.Context, data *EmailVerificationData) error {
	// For now, we'll just log this. Implementation can be added later.
	fmt.Printf("[BREVO] Would send email verification to %s (%s) (link: %s)\n",
		data.RecipientEmail,
		data.RecipientName,
		data.VerificationLink)
	return nil
}
