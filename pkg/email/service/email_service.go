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
	SendBookingConfirmation(ctx context.Context, data *BookingConfirmationEmailData) error
	SendPortalEmailVerification(ctx context.Context, data *PortalEmailVerificationData) error
	SendLeadIsca(ctx context.Context, data *LeadIscaEmailData) error
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

// BookingConfirmationEmailData contains data for booking confirmation emails
type BookingConfirmationEmailData struct {
	RecipientEmail   string
	RecipientName    string
	ClinicName       string
	ProfessionalName string
	StartTime        time.Time
	EndTime          time.Time
	PortalLink       string // Link to create portal account
	PaymentURL       string // Link to payment page (if payment required)
}

// PortalEmailVerificationData contains data for portal email verification
type PortalEmailVerificationData struct {
	RecipientEmail string
	RecipientName  string
	ClinicName     string
	VerifyLink     string
}

// LeadIscaEmailData contains data for lead isca (bait) emails with spreadsheet attachments
type LeadIscaEmailData struct {
	RecipientEmail string
	RecipientName  string
	Attachments    []EmailAttachment
}

// EmailAttachment represents a file attachment for an email
type EmailAttachment struct {
	Name    string // filename
	Content string // base64-encoded content
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

// SendLeadIsca sends the lead isca email with spreadsheet attachments via SMTP
func (s *smtpEmailService) SendLeadIsca(ctx context.Context, data *LeadIscaEmailData) error {
	subject := "Seu Kit de Planilhas para Clínicas — Aseno"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family:Arial,sans-serif;">
	<div style="max-width:520px;margin:0 auto;padding:32px 24px;">
		<h2 style="color:#0d1b2a;">Olá, %s!</h2>
		<p style="color:#607d94;font-size:15px;line-height:1.6;">
			Obrigado por baixar o <strong>Kit de Planilhas para Clínicas</strong> da Aseno.
		</p>
		<p style="color:#607d94;font-size:15px;line-height:1.6;">
			Em anexo você encontra as 3 planilhas prontas para usar.
		</p>
		<p style="color:#a0aec0;font-size:12px;text-align:center;">
			Enviado por <strong>Aseno</strong> — Software de gestão para clínicas
		</p>
	</div>
</body>
</html>`, data.RecipientName)

	return s.sendEmail(data.RecipientEmail, subject, body)
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

// SendBookingConfirmation sends a booking confirmation email
func (s *smtpEmailService) SendBookingConfirmation(ctx context.Context, data *BookingConfirmationEmailData) error {
	subject := "Agendamento confirmado — " + data.ClinicName

	body, err := s.renderBookingConfirmationTemplate(data)
	if err != nil {
		return fmt.Errorf("failed to render booking confirmation template: %w", err)
	}

	return s.sendEmail(data.RecipientEmail, subject, body)
}

func (s *smtpEmailService) renderBookingConfirmationTemplate(data *BookingConfirmationEmailData) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #0891b2; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; background: #0891b2; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
        .detail { margin: 8px 0; }
        .label { color: #666; font-size: 14px; }
        .value { font-weight: bold; color: #333; font-size: 15px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1 style="margin:0;font-size:22px;">Agendamento Confirmado</h1>
            <p style="margin:4px 0 0;opacity:0.85;font-size:14px;">{{.ClinicName}}</p>
        </div>
        <div class="content">
            <p>Olá, <strong>{{.RecipientName}}</strong>!</p>
            <p>Seu agendamento foi realizado com sucesso. Confira os detalhes:</p>

            <div style="background:#fff;border:1px solid #e5e7eb;border-radius:8px;padding:16px;margin:20px 0;">
                <div class="detail">
                    <span class="label">Profissional:</span><br>
                    <span class="value">{{.ProfessionalName}}</span>
                </div>
                <div class="detail">
                    <span class="label">Data:</span><br>
                    <span class="value">{{.DateFormatted}}</span>
                </div>
                <div class="detail">
                    <span class="label">Horário:</span><br>
                    <span class="value">{{.TimeFormatted}}</span>
                </div>
            </div>

            <p style="color:#607d94;font-size:14px;">
                Aguarde a confirmação da clínica. Você receberá uma notificação quando o agendamento for confirmado.
            </p>

            {{if .PaymentURL}}
            <div style="background:#fffbeb;border:1px solid #fbbf24;border-radius:8px;padding:16px;margin:20px 0;text-align:center;">
                <p style="margin:0 0 12px;font-size:14px;color:#92400e;font-weight:600;">Pagamento pendente</p>
                <p style="margin:0 0 16px;font-size:13px;color:#a16207;">Realize o pagamento para confirmar seu agendamento.</p>
                <a href="{{.PaymentURL}}" style="display:inline-block;background:#0891b2;color:white;padding:12px 30px;text-decoration:none;border-radius:6px;font-weight:600;font-size:14px;">Pagar agora</a>
            </div>
            {{end}}

            {{if .PortalLink}}
            <p style="text-align:center;margin:28px 0 12px;">
                <a href="{{.PortalLink}}" class="button">Gerenciar meus agendamentos</a>
            </p>
            <p style="text-align:center;color:#a0aec0;font-size:12px;">
                Crie sua conta para visualizar e gerenciar seus agendamentos online.
            </p>
            {{end}}
        </div>
        <div class="footer">
            <p>Agendamento online por <strong>Aseno</strong></p>
        </div>
    </div>
</body>
</html>`

	t, err := template.New("booking_confirmation").Parse(tmpl)
	if err != nil {
		return "", err
	}

	templateData := struct {
		RecipientName    string
		ClinicName       string
		ProfessionalName string
		DateFormatted    string
		TimeFormatted    string
		PortalLink       string
	}{
		RecipientName:    data.RecipientName,
		ClinicName:       data.ClinicName,
		ProfessionalName: data.ProfessionalName,
		DateFormatted:    data.StartTime.Format("02/01/2006"),
		TimeFormatted:    data.StartTime.Format("15:04") + " - " + data.EndTime.Format("15:04"),
		PortalLink:       data.PortalLink,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, templateData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// SendPortalEmailVerification sends a portal email verification email
func (s *smtpEmailService) SendPortalEmailVerification(ctx context.Context, data *PortalEmailVerificationData) error {
	subject := "Verifique seu email — " + data.ClinicName

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #0891b2; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; background: #0891b2; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1 style="margin:0;font-size:22px;">Verifique seu Email</h1>
            <p style="margin:4px 0 0;opacity:0.85;font-size:14px;">{{.ClinicName}}</p>
        </div>
        <div class="content">
            <p>Olá!</p>
            <p>Para ativar sua conta no portal de agendamentos, clique no botão abaixo:</p>
            <p style="text-align:center;">
                <a href="{{.VerifyLink}}" class="button">Verificar Email</a>
            </p>
            <p style="color:#666;font-size:13px;">Se você não solicitou esta verificação, ignore este email.</p>
        </div>
        <div class="footer">
            <p>Agendamento online por <strong>Aseno</strong></p>
        </div>
    </div>
</body>
</html>`

	t, err := template.New("portal_verify").Parse(tmpl)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	return s.sendEmail(data.RecipientEmail, subject, buf.String())
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

func (s *noOpEmailService) SendBookingConfirmation(ctx context.Context, data *BookingConfirmationEmailData) error {
	fmt.Printf("[EMAIL] NoOp: booking confirmation to %s at %s with %s\n",
		data.RecipientEmail, data.ClinicName, data.ProfessionalName)
	return nil
}

func (s *noOpEmailService) SendPortalEmailVerification(ctx context.Context, data *PortalEmailVerificationData) error {
	fmt.Printf("[EMAIL] NoOp: portal verification to %s for %s (link: %s)\n",
		data.RecipientEmail, data.ClinicName, data.VerifyLink)
	return nil
}

func (s *noOpEmailService) SendLeadIsca(ctx context.Context, data *LeadIscaEmailData) error {
	fmt.Printf("[EMAIL] NoOp: lead isca email to %s (%s) with %d attachments\n",
		data.RecipientName, data.RecipientEmail, len(data.Attachments))
	return nil
}
