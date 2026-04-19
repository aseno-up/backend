package worker

import (
	"context"

	email_service "github.com/aseno/backend/pkg/email/service"
)

// emailServiceAdapter bridges the pkg/email EmailService to the narrow sender
// interface the payment reminder task consumes. The two structs are
// intentionally identical in shape so the adapter is a memcpy.
type emailServiceAdapter struct {
	svc email_service.EmailService
}

// NewPaymentReminderEmailAdapter wraps an EmailService so the worker can use
// it via its minimal interface.
func NewPaymentReminderEmailAdapter(svc email_service.EmailService) paymentReminderEmailSender {
	return &emailServiceAdapter{svc: svc}
}

func (a *emailServiceAdapter) SendPaymentReminder(ctx context.Context, data *PaymentReminderEmailData) error {
	return a.svc.SendPaymentReminder(ctx, &email_service.PaymentReminderEmailData{
		RecipientEmail: data.RecipientEmail,
		RecipientName:  data.RecipientName,
		ClinicName:     data.ClinicName,
		PlanTitle:      data.PlanTitle,
		DueDate:        data.DueDate,
		Amount:         data.Amount,
	})
}
