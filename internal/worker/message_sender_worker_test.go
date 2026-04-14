package worker

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/config"
	"github.com/aseno/backend/internal/queue"
	whatsapp_model "github.com/aseno/backend/pkg/whatsapp/model"
)

type stubWhatsAppSender struct {
	sent int
}

func (s *stubWhatsAppSender) SendTextMessage(_ context.Context, _, _, _ string) (*whatsapp_model.SendMessageResponse, error) {
	s.sent++
	return &whatsapp_model.SendMessageResponse{}, nil
}

func newSenderJob(clinicID uuid.UUID) *queue.MessageJob {
	return &queue.MessageJob{
		ID:          uuid.New().String(),
		ClinicID:    clinicID.String(),
		Phone:       "+5511988887777",
		Instance:    "instance-1",
		Message:     "hi",
		MessageType: "reminder_24h",
		MaxAttempts: 3,
	}
}

func TestMessageSenderWorker_InactiveSubscription_DoesNotSend(t *testing.T) {
	clinicID := uuid.New()
	sender := &stubWhatsAppSender{}
	checker := &stubSubscriptionChecker{activeByClinic: map[uuid.UUID]bool{clinicID: false}}

	w := NewMessageSenderWorker(&config.WorkerConfig{}, nil, sender, nil, nil)
	w.SetSubscriptionChecker(checker)

	if err := w.processMessage(newSenderJob(clinicID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sent != 0 {
		t.Errorf("sender should not have been called for inactive subscription, got %d", sender.sent)
	}
}

func TestMessageSenderWorker_ActiveSubscription_Sends(t *testing.T) {
	clinicID := uuid.New()
	sender := &stubWhatsAppSender{}
	checker := &stubSubscriptionChecker{activeByClinic: map[uuid.UUID]bool{clinicID: true}}

	w := NewMessageSenderWorker(&config.WorkerConfig{}, nil, sender, nil, nil)
	w.SetSubscriptionChecker(checker)

	if err := w.processMessage(newSenderJob(clinicID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sent != 1 {
		t.Errorf("sender should have been called once for active subscription, got %d", sender.sent)
	}
}

func TestMessageSenderWorker_NoChecker_Sends(t *testing.T) {
	clinicID := uuid.New()
	sender := &stubWhatsAppSender{}

	w := NewMessageSenderWorker(&config.WorkerConfig{}, nil, sender, nil, nil)

	if err := w.processMessage(newSenderJob(clinicID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sent != 1 {
		t.Errorf("sender should send when no checker wired (backward compat), got %d", sender.sent)
	}
}
