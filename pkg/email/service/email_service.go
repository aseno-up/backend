package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"time"
)

// EmailService defines the interface for sending emails
type EmailService interface {
	SendInvite(ctx context.Context, data *InviteEmailData) error
	SendWelcome(ctx context.Context, data *WelcomeEmailData) error
	SendPasswordReset(ctx context.Context, data *ResetPasswordEmailData) error
	SendEmailVerification(ctx context.Context, data *EmailVerificationData) error
	SendContactForm(ctx context.Context, data *ContactFormData) error
}

// InviteEmailData contains data for invite emails
type InviteEmailData struct {
	RecipientEmail string
	ClinicName     string
	InviterName    string
	Role           string
	InviteLink     string
	ExpiresAt      time.Time
}

// WelcomeEmailData contains data for welcome emails
type WelcomeEmailData struct {
	RecipientEmail string
	RecipientName  string
	ClinicName     string
	LoginLink      string
}

// ResetPasswordEmailData contains data for password reset emails
type ResetPasswordEmailData struct {
	RecipientEmail string
	RecipientName  string
	ResetLink      string
}

// EmailVerificationData contains data for email verification emails
type EmailVerificationData struct {
	RecipientEmail   string
	RecipientName    string
	VerificationLink string
}

// ContactFormData contains data for contact form submissions
type ContactFormData struct {
	Name    string
	Email   string
	Message string
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	From        string
	FrontendURL string
}

// smtpEmailService implements EmailService using SMTP
type smtpEmailService struct {
	config SMTPConfig
}

// NewSMTPEmailService creates a new SMTP email service
func NewSMTPEmailService(config SMTPConfig) EmailService {
	return &smtpEmailService{config: config}
}

// SendInvite sends an invitation email
func (s *smtpEmailService) SendInvite(ctx context.Context, data *InviteEmailData) error {
	subject := fmt.Sprintf("Convite para %s", data.ClinicName)

	body, err := s.renderInviteTemplate(data)
	if err != nil {
		return fmt.Errorf("failed to render invite template: %w", err)
	}

	return s.sendEmail(data.RecipientEmail, subject, body)
}

// SendWelcome sends a welcome email
func (s *smtpEmailService) SendWelcome(ctx context.Context, data *WelcomeEmailData) error {
	subject := fmt.Sprintf("Bem-vindo(a) a %s", data.ClinicName)

	body, err := s.renderWelcomeTemplate(data)
	if err != nil {
		return fmt.Errorf("failed to render welcome template: %w", err)
	}

	return s.sendEmail(data.RecipientEmail, subject, body)
}

// SendPasswordReset sends a password reset email
func (s *smtpEmailService) SendPasswordReset(ctx context.Context, data *ResetPasswordEmailData) error {
	subject := "Redefinição de senha — Aseno"

	body, err := s.renderPasswordResetTemplate(data)
	if err != nil {
		return fmt.Errorf("failed to render password reset template: %w", err)
	}

	return s.sendEmail(data.RecipientEmail, subject, body)
}

// SendEmailVerification sends an email verification email
func (s *smtpEmailService) SendEmailVerification(ctx context.Context, data *EmailVerificationData) error {
	subject := "Verifique seu email — Aseno"

	body, err := s.renderEmailVerificationTemplate(data)
	if err != nil {
		return fmt.Errorf("failed to render email verification template: %w", err)
	}

	return s.sendEmail(data.RecipientEmail, subject, body)
}

func (s *smtpEmailService) sendEmail(to, subject, body string) error {
	headers := make(map[string]string)
	headers["From"] = s.config.From
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	auth := smtp.PlainAuth("", s.config.User, s.config.Password, s.config.Host)
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Use TLS for secure connection
	tlsConfig := &tls.Config{
		ServerName: s.config.Host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		// Fallback to STARTTLS
		return smtp.SendMail(addr, auth, s.config.From, []string{to}, []byte(message))
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return err
	}

	if err := client.Mail(s.config.From); err != nil {
		return err
	}

	if err := client.Rcpt(to); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

func (s *smtpEmailService) renderInviteTemplate(data *InviteEmailData) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4F46E5; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; background: #4F46E5; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
        .role { display: inline-block; background: #E0E7FF; color: #4F46E5; padding: 4px 12px; border-radius: 4px; font-weight: bold; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Voce foi convidado!</h1>
        </div>
        <div class="content">
            <p>Ola,</p>
            <p><strong>{{.InviterName}}</strong> convidou voce para fazer parte da equipe <strong>{{.ClinicName}}</strong>.</p>
            <p>Funcao: <span class="role">{{.Role}}</span></p>
            <p>Clique no botao abaixo para aceitar o convite e criar sua conta:</p>
            <p style="text-align: center;">
                <a href="{{.InviteLink}}" class="button">Aceitar Convite</a>
            </p>
            <p><small>Este convite expira em {{.ExpiresAt.Format "02/01/2006 as 15:04"}}.</small></p>
            <p><small>Se voce nao reconhece este convite, pode ignorar este email.</small></p>
        </div>
        <div class="footer">
            <p>Este email foi enviado automaticamente. Por favor, nao responda.</p>
        </div>
    </div>
</body>
</html>
`
	t, err := template.New("invite").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (s *smtpEmailService) renderWelcomeTemplate(data *WelcomeEmailData) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4F46E5; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; background: #4F46E5; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Bem-vindo(a)!</h1>
        </div>
        <div class="content">
            <p>Ola {{.RecipientName}},</p>
            <p>Sua conta na <strong>{{.ClinicName}}</strong> foi criada com sucesso!</p>
            <p>Clique no botao abaixo para acessar o sistema:</p>
            <p style="text-align: center;">
                <a href="{{.LoginLink}}" class="button">Acessar Sistema</a>
            </p>
        </div>
        <div class="footer">
            <p>Este email foi enviado automaticamente. Por favor, nao responda.</p>
        </div>
    </div>
</body>
</html>
`
	t, err := template.New("welcome").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (s *smtpEmailService) renderPasswordResetTemplate(data *ResetPasswordEmailData) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4F46E5; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; background: #4F46E5; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Redefinicao de Senha</h1>
        </div>
        <div class="content">
            <p>Ola,</p>
            <p>Recebemos uma solicitacao para redefinir sua senha.</p>
            <p>Clique no botao abaixo para criar uma nova senha:</p>
            <p style="text-align: center;">
                <a href="{{.ResetLink}}" class="button">Redefinir Senha</a>
            </p>
            <p><small>Este link expira em 15 minutos.</small></p>
            <p><small>Se voce nao solicitou isso, ignore este e-mail.</small></p>
        </div>
        <div class="footer">
            <p>Este email foi enviado automaticamente. Por favor, nao responda.</p>
        </div>
    </div>
</body>
</html>
`
	t, err := template.New("reset_password").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (s *smtpEmailService) renderEmailVerificationTemplate(data *EmailVerificationData) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4F46E5; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; background: #4F46E5; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Verifique seu Email</h1>
        </div>
        <div class="content">
            <p>Ola {{.RecipientName}},</p>
            <p>Obrigado por se cadastrar! Para completar seu registro, por favor verifique seu endereco de email.</p>
            <p>Clique no botao abaixo para verificar:</p>
            <p style="text-align: center;">
                <a href="{{.VerificationLink}}" class="button">Verificar Email</a>
            </p>
            <p><small>Este link expira em 24 horas.</small></p>
            <p><small>Se voce nao criou uma conta, ignore este email.</small></p>
        </div>
        <div class="footer">
            <p>Este email foi enviado automaticamente. Por favor, nao responda.</p>
        </div>
    </div>
</body>
</html>
`
	t, err := template.New("email_verification").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// SendContactForm sends a contact form submission to the support email via SMTP
func (s *smtpEmailService) SendContactForm(ctx context.Context, data *ContactFormData) error {
	subject := fmt.Sprintf("Contato via site — %s", data.Name)
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px;">
        <h2>Nova mensagem de contato</h2>
        <p><strong>Nome:</strong> %s</p>
        <p><strong>Email:</strong> %s</p>
        <p><strong>Mensagem:</strong></p>
        <p style="background: #f9fafb; padding: 15px; border-radius: 8px;">%s</p>
    </div>
</body>
</html>`, data.Name, data.Email, data.Message)

	return s.sendEmail(s.config.From, subject, body)
}

// NoOpEmailService is a no-operation email service for development
type noOpEmailService struct{}

// NewNoOpEmailService creates a no-op email service that logs instead of sending
func NewNoOpEmailService() EmailService {
	return &noOpEmailService{}
}

func (s *noOpEmailService) SendInvite(ctx context.Context, data *InviteEmailData) error {
	fmt.Printf("[EMAIL] NoOp: invite email to %s for clinic %s (role: %s)\n",
		data.RecipientEmail, data.ClinicName, data.Role)
	return nil
}

func (s *noOpEmailService) SendWelcome(ctx context.Context, data *WelcomeEmailData) error {
	fmt.Printf("[EMAIL] NoOp: welcome email to %s for clinic %s\n",
		data.RecipientEmail, data.ClinicName)
	return nil
}

func (s *noOpEmailService) SendPasswordReset(ctx context.Context, data *ResetPasswordEmailData) error {
	fmt.Printf("[EMAIL] NoOp: password reset email to %s\n", data.RecipientEmail)
	return nil
}

func (s *noOpEmailService) SendEmailVerification(ctx context.Context, data *EmailVerificationData) error {
	fmt.Printf("[EMAIL] NoOp: verification email to %s\n", data.RecipientEmail)
	return nil
}

func (s *noOpEmailService) SendContactForm(ctx context.Context, data *ContactFormData) error {
	fmt.Printf("[EMAIL] NoOp: contact form from %s (%s)\n", data.Name, data.Email)
	return nil
}
