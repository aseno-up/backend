package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/aseno/backend/internal/config"
	whatsapp_model "github.com/aseno/backend/pkg/whatsapp/model"
	"github.com/aseno/backend/pkg/whatsapp/service"
)

// stubWhatsAppSvc for worker test
type stubWhatsAppSvc struct {
	service.WhatsAppService
	processDelayedAIFunc func(ctx context.Context, msg *whatsapp_model.ScheduledMessage) error
	sendTextMessageFunc  func(ctx context.Context, instance, phone, message string) (*whatsapp_model.SendMessageResponse, error)
	aiCalled             bool
	sendCalled           bool
}

func (s *stubWhatsAppSvc) ProcessDelayedAI(ctx context.Context, msg *whatsapp_model.ScheduledMessage) error {
	s.aiCalled = true
	if s.processDelayedAIFunc != nil {
		return s.processDelayedAIFunc(ctx, msg)
	}
	return nil
}

func (s *stubWhatsAppSvc) SendTextMessage(ctx context.Context, instance, phone, message string) (*whatsapp_model.SendMessageResponse, error) {
	s.sendCalled = true
	if s.sendTextMessageFunc != nil {
		return s.sendTextMessageFunc(ctx, instance, phone, message)
	}
	return &whatsapp_model.SendMessageResponse{}, nil
}

// stubScheduledMsgRepo for worker test
type stubScheduledMsgRepo struct {
	markedAsSent   bool
	markedAsFailed bool
}

func (r *stubScheduledMsgRepo) Create(ctx context.Context, msg *whatsapp_model.ScheduledMessage) error {
	return nil
}
func (r *stubScheduledMsgRepo) GetPendingMessages(ctx context.Context, limit int) ([]*whatsapp_model.ScheduledMessage, error) {
	return nil, nil
}
func (r *stubScheduledMsgRepo) GetLastScheduledForClinic(ctx context.Context, clinicID uuid.UUID) (*whatsapp_model.ScheduledMessage, error) {
	return nil, nil
}
func (r *stubScheduledMsgRepo) MarkAsSent(ctx context.Context, id uuid.UUID) error {
	r.markedAsSent = true
	return nil
}
func (r *stubScheduledMsgRepo) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	r.markedAsFailed = true
	return nil
}
func (r *stubScheduledMsgRepo) DeletePendingByPhoneAndType(ctx context.Context, clinicID uuid.UUID, phone string, messageType string) error {
	return nil
}

func TestScheduledMessageWorker_ProcessSingleMessage(t *testing.T) {
	t.Run("Processes AI Processing intent", func(t *testing.T) {
		svc := &stubWhatsAppSvc{}
		repo := &stubScheduledMsgRepo{}
		worker := NewScheduledMessageWorker(&config.WorkerConfig{}, repo, nil, svc)

		msg := &whatsapp_model.ScheduledMessage{
			ID:          uuid.New(),
			Phone:       "5511999999999",
			MessageType: "ai_processing",
			ScheduledAt: time.Now(),
		}

		worker.processSingleMessage(context.Background(), msg)

		assert.True(t, svc.aiCalled)
		assert.False(t, svc.sendCalled)
		assert.True(t, repo.markedAsSent)
		assert.False(t, repo.markedAsFailed)
	})

	t.Run("Processes Normal Outbound message", func(t *testing.T) {
		svc := &stubWhatsAppSvc{}
		repo := &stubScheduledMsgRepo{}
		worker := NewScheduledMessageWorker(&config.WorkerConfig{}, repo, nil, svc)

		msg := &whatsapp_model.ScheduledMessage{
			ID:          uuid.New(),
			Phone:       "5511999999999",
			MessageType: "outbound",
			ScheduledAt: time.Now(),
		}

		worker.processSingleMessage(context.Background(), msg)

		assert.False(t, svc.aiCalled)
		assert.True(t, svc.sendCalled)
		assert.True(t, repo.markedAsSent)
		assert.False(t, repo.markedAsFailed)
	})
}

func TestScheduledMessageWorker_SubscriptionGate(t *testing.T) {
	t.Run("Inactive subscription drops messages without calling whatsapp", func(t *testing.T) {
		clinicID := uuid.New()
		svc := &stubWhatsAppSvc{}
		repo := &stubScheduledMsgRepo{}
		w := NewScheduledMessageWorker(&config.WorkerConfig{}, repo, nil, svc)
		w.SetSubscriptionChecker(&stubSubscriptionChecker{
			activeByClinic: map[uuid.UUID]bool{clinicID: false},
		})

		msgs := []*whatsapp_model.ScheduledMessage{{
			ID:          uuid.New(),
			ClinicID:    clinicID,
			Phone:       "5511999999999",
			MessageType: "outbound",
			ScheduledAt: time.Now(),
		}}

		w.processPatientMessages(context.Background(), msgs)

		assert.False(t, svc.aiCalled)
		assert.False(t, svc.sendCalled)
		// Marked as sent so they stop being returned by GetPendingMessages.
		assert.True(t, repo.markedAsSent)
		assert.True(t, repo.markedAsFailed)
	})

	t.Run("Active subscription processes normally", func(t *testing.T) {
		clinicID := uuid.New()
		svc := &stubWhatsAppSvc{}
		repo := &stubScheduledMsgRepo{}
		w := NewScheduledMessageWorker(&config.WorkerConfig{}, repo, nil, svc)
		w.SetSubscriptionChecker(&stubSubscriptionChecker{
			activeByClinic: map[uuid.UUID]bool{clinicID: true},
		})

		msgs := []*whatsapp_model.ScheduledMessage{{
			ID:          uuid.New(),
			ClinicID:    clinicID,
			Phone:       "5511999999999",
			MessageType: "outbound",
			ScheduledAt: time.Now(),
		}}

		w.processPatientMessages(context.Background(), msgs)

		assert.True(t, svc.sendCalled)
		assert.True(t, repo.markedAsSent)
	})
}
