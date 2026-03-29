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
	expiresFormatted := data.ExpiresAt.Format("02/01/2006")

	_, _, err := s.client.TransactionalEmailsApi.SendTransacEmail(ctx, lib.SendSmtpEmail{
		Sender: &lib.SendSmtpEmailSender{
			Name:  s.senderName,
			Email: s.senderEmail,
		},
		To: []lib.SendSmtpEmailTo{
			{Email: data.RecipientEmail},
		},
		Subject: fmt.Sprintf("Voce foi convidado para %s — Aseno", data.ClinicName),
		HtmlContent: fmt.Sprintf(`
			<div style="font-family:Arial,sans-serif;max-width:520px;margin:0 auto;padding:32px 24px;">
				<h2 style="color:#0d1b2a;margin-bottom:8px;">Voce foi convidado!</h2>
				<p style="color:#607d94;font-size:15px;line-height:1.6;">
					<strong>%s</strong> convidou voce para fazer parte da equipe da clinica
					<strong>%s</strong> como <strong>%s</strong>.
				</p>
				<p style="text-align:center;margin:32px 0;">
					<a href="%s"
					   style="background:#38b6ff;color:#fff;text-decoration:none;padding:12px 32px;border-radius:8px;font-weight:600;font-size:15px;display:inline-block;">
						Aceitar Convite
					</a>
				</p>
				<p style="color:#a0aec0;font-size:12px;margin-top:32px;">
					Este convite expira em %s. Se voce nao reconhece este convite, ignore este e-mail.
				</p>
			</div>
		`, data.InviterName, data.ClinicName, data.Role, data.InviteLink, expiresFormatted),
	})

	if err != nil {
		return fmt.Errorf("failed to send invite email via Brevo: %w", err)
	}
	return nil
}

// SendWelcome sends a welcome email using Brevo
func (s *brevoEmailService) SendWelcome(ctx context.Context, data *WelcomeEmailData) error {
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
		Subject: fmt.Sprintf("Bem-vindo a %s — Aseno", data.ClinicName),
		HtmlContent: fmt.Sprintf(`
			<div style="font-family:Arial,sans-serif;max-width:520px;margin:0 auto;padding:32px 24px;">
				<h2 style="color:#0d1b2a;margin-bottom:8px;">Bem-vindo, %s!</h2>
				<p style="color:#607d94;font-size:15px;line-height:1.6;">
					Sua conta na clinica <strong>%s</strong> foi criada com sucesso.
				</p>
				<p style="color:#607d94;font-size:15px;line-height:1.6;">
					Acesse o sistema para comecar:
				</p>
				<p style="text-align:center;margin:32px 0;">
					<a href="%s"
					   style="background:#38b6ff;color:#fff;text-decoration:none;padding:12px 32px;border-radius:8px;font-weight:600;font-size:15px;display:inline-block;">
						Acessar o Sistema
					</a>
				</p>
			</div>
		`, data.RecipientName, data.ClinicName, data.LoginLink),
	})

	if err != nil {
		return fmt.Errorf("failed to send welcome email via Brevo: %w", err)
	}
	return nil
}

// SendEmailVerification sends an email verification email using Brevo
func (s *brevoEmailService) SendEmailVerification(ctx context.Context, data *EmailVerificationData) error {
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
		Subject: "Confirme seu e-mail — Aseno",
		HtmlContent: fmt.Sprintf(`
			<div style="font-family:Arial,sans-serif;max-width:520px;margin:0 auto;padding:32px 24px;">
				<h2 style="color:#0d1b2a;margin-bottom:8px;">Confirme seu e-mail</h2>
				<p style="color:#607d94;font-size:15px;line-height:1.6;">
					Olá, <strong>%s</strong>! Obrigado por se cadastrar no Aseno.
				</p>
				<p style="color:#607d94;font-size:15px;line-height:1.6;">
					Clique no botão abaixo para confirmar seu endereço de e-mail e ativar sua conta:
				</p>
				<p style="text-align:center;margin:32px 0;">
					<a href="%s"
					   style="background:#38b6ff;color:#fff;text-decoration:none;padding:12px 32px;border-radius:8px;font-weight:600;font-size:15px;display:inline-block;">
						Confirmar e-mail
					</a>
				</p>
				<p style="color:#607d94;font-size:13px;line-height:1.6;">
					Se o botão não funcionar, copie e cole este link no navegador:<br/>
					<a href="%s" style="color:#38b6ff;word-break:break-all;">%s</a>
				</p>
				<p style="color:#a0aec0;font-size:12px;margin-top:32px;">
					Este link expira em 24 horas. Se você não criou uma conta no Aseno, ignore este e-mail.
				</p>
			</div>
		`, data.RecipientName, data.VerificationLink, data.VerificationLink, data.VerificationLink),
	})

	if err != nil {
		return fmt.Errorf("failed to send email verification via Brevo: %w", err)
	}
	return nil
}

// SendContactForm sends a contact form submission to the support email using Brevo
func (s *brevoEmailService) SendContactForm(ctx context.Context, data *ContactFormData) error {
	_, _, err := s.client.TransactionalEmailsApi.SendTransacEmail(ctx, lib.SendSmtpEmail{
		Sender: &lib.SendSmtpEmailSender{
			Name:  data.Name,
			Email: data.Email,
		},
		To: []lib.SendSmtpEmailTo{
			{
				Email: s.senderEmail,
				Name:  s.senderName,
			},
		},
		ReplyTo: &lib.SendSmtpEmailReplyTo{
			Email: data.Email,
			Name:  data.Name,
		},
		Subject: fmt.Sprintf("Contato via site — %s", data.Name),
		HtmlContent: fmt.Sprintf(`
			<h2>Nova mensagem de contato</h2>
			<p><strong>Nome:</strong> %s</p>
			<p><strong>Email:</strong> %s</p>
			<p><strong>Mensagem:</strong></p>
			<p style="background:#f4f8fb;padding:15px;border-radius:8px;">%s</p>
		`, data.Name, data.Email, data.Message),
	})

	if err != nil {
		return fmt.Errorf("failed to send contact form email via Brevo: %w", err)
	}
	return nil
}

// SendBookingConfirmation sends a booking confirmation email using Brevo
func (s *brevoEmailService) SendBookingConfirmation(ctx context.Context, data *BookingConfirmationEmailData) error {
	dateFormatted := data.StartTime.Format("02/01/2006")
	timeFormatted := data.StartTime.Format("15:04") + " - " + data.EndTime.Format("15:04")

	paymentSection := ""
	if data.PaymentURL != "" {
		paymentSection = fmt.Sprintf(`
			<div style="background:#fffbeb;border:1px solid #fbbf24;border-radius:8px;padding:16px;margin:20px 0;text-align:center;">
				<p style="margin:0 0 12px;font-size:14px;color:#92400e;font-weight:600;">Pagamento pendente</p>
				<p style="margin:0 0 16px;font-size:13px;color:#a16207;">Realize o pagamento para confirmar seu agendamento.</p>
				<a href="%s" style="display:inline-block;background:#0891b2;color:white;padding:12px 30px;text-decoration:none;border-radius:6px;font-weight:600;font-size:14px;">Pagar agora</a>
			</div>
		`, data.PaymentURL)
	}

	portalSection := ""
	if data.PortalLink != "" {
		portalSection = fmt.Sprintf(`
			<p style="text-align:center;margin:28px 0 12px;">
				<a href="%s"
				   style="background:#0891b2;color:#fff;text-decoration:none;padding:12px 32px;border-radius:8px;font-weight:600;font-size:15px;display:inline-block;">
					Gerenciar meus agendamentos
				</a>
			</p>
			<p style="text-align:center;color:#a0aec0;font-size:12px;">
				Crie sua conta para visualizar e gerenciar seus agendamentos online.
			</p>
		`, data.PortalLink)
	}

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
		Subject: fmt.Sprintf("Agendamento confirmado — %s", data.ClinicName),
		HtmlContent: fmt.Sprintf(`
			<div style="font-family:Arial,sans-serif;max-width:520px;margin:0 auto;padding:32px 24px;">
				<h2 style="color:#0d1b2a;margin-bottom:8px;">Agendamento Confirmado</h2>
				<p style="color:#607d94;font-size:15px;line-height:1.6;">
					Olá, <strong>%s</strong>! Seu agendamento na <strong>%s</strong> foi realizado com sucesso.
				</p>
				<div style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:8px;padding:16px;margin:20px 0;">
					<p style="margin:4px 0;color:#334155;font-size:14px;"><strong>Profissional:</strong> %s</p>
					<p style="margin:4px 0;color:#334155;font-size:14px;"><strong>Data:</strong> %s</p>
					<p style="margin:4px 0;color:#334155;font-size:14px;"><strong>Horário:</strong> %s</p>
				</div>
				<p style="color:#607d94;font-size:14px;line-height:1.6;">
					Aguarde a confirmação da clínica. Você receberá uma notificação quando o agendamento for confirmado.
				</p>
				%s
				%s
				<p style="color:#a0aec0;font-size:12px;margin-top:32px;text-align:center;">
					Agendamento online por <strong>aseno</strong>
				</p>
			</div>
		`, data.RecipientName, data.ClinicName, data.ProfessionalName, dateFormatted, timeFormatted, paymentSection, portalSection),
	})

	if err != nil {
		return fmt.Errorf("failed to send booking confirmation email via Brevo: %w", err)
	}
	return nil
}

// SendPortalEmailVerification sends a portal email verification email using Brevo
func (s *brevoEmailService) SendPortalEmailVerification(ctx context.Context, data *PortalEmailVerificationData) error {
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
		Subject: fmt.Sprintf("Verifique seu email — %s", data.ClinicName),
		HtmlContent: fmt.Sprintf(`
			<div style="font-family:Arial,sans-serif;max-width:520px;margin:0 auto;padding:32px 24px;">
				<h2 style="color:#0d1b2a;margin-bottom:8px;">Verifique seu Email</h2>
				<p style="color:#607d94;font-size:15px;line-height:1.6;">
					Para ativar sua conta no portal de agendamentos da <strong>%s</strong>, clique no botão abaixo:
				</p>
				<p style="text-align:center;margin:32px 0;">
					<a href="%s"
					   style="background:#0891b2;color:#fff;text-decoration:none;padding:12px 32px;border-radius:8px;font-weight:600;font-size:15px;display:inline-block;">
						Verificar Email
					</a>
				</p>
				<p style="color:#a0aec0;font-size:12px;margin-top:32px;">
					Se você não solicitou esta verificação, ignore este email.
				</p>
				<p style="color:#a0aec0;font-size:12px;text-align:center;margin-top:24px;">
					Agendamento online por <strong>Aseno</strong>
				</p>
			</div>
		`, data.ClinicName, data.VerifyLink),
	})

	if err != nil {
		return fmt.Errorf("failed to send portal verification email via Brevo: %w", err)
	}
	return nil
}
