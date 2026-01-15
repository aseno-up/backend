package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/whatsapp/model"
)

// MessageLogService provides helper methods for creating message logs
type MessageLogService interface {
	// LogCreated logs when a message is created
	LogCreated(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error

	// LogQueued logs when a message is queued for sending
	LogQueued(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error

	// LogSent logs when a message is successfully sent
	LogSent(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error

	// LogDelivered logs when a message is delivered
	LogDelivered(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error

	// LogRead logs when a message is read
	LogRead(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error

	// LogReplied logs when a reply is received
	LogReplied(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error

	// LogFailed logs when a message fails to send
	LogFailed(ctx context.Context, msg *model.Message, errorMsg string, payload model.MessageLogPayload) error

	// LogRetried logs when a message is retried
	LogRetried(ctx context.Context, msg *model.Message, attempt int, payload model.MessageLogPayload) error

	// LogProcessed logs when a response is processed
	LogProcessed(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error

	// GetByMessageID retrieves all logs for a message
	GetByMessageID(ctx context.Context, messageID uuid.UUID) ([]*model.MessageLog, error)

	// GetByAppointmentID retrieves all logs for an appointment
	GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) ([]*model.MessageLog, error)
}

// messageLogService implements MessageLogService
type messageLogService struct {
	repo MessageLogRepository
}

// NewMessageLogService creates a new message log service
func NewMessageLogService(repo MessageLogRepository) MessageLogService {
	return &messageLogService{repo: repo}
}

// createLog is a helper that creates a log entry
func (s *messageLogService) createLog(ctx context.Context, msg *model.Message, event model.MessageLogEvent, errorMsg *string, payload model.MessageLogPayload) error {
	log := &model.MessageLog{
		ClinicID:      msg.ClinicID,
		MessageID:     msg.ID,
		PatientID:     msg.PatientID,
		AppointmentID: msg.AppointmentID,
		Event:         event,
		Channel:       model.MessageLogChannelWhatsApp,
		Direction:     msg.Direction,
		Payload:       payload,
		Error:         errorMsg,
		CreatedAt:     time.Now(),
	}
	return s.repo.Create(ctx, log)
}

// LogCreated logs when a message is created
func (s *messageLogService) LogCreated(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error {
	return s.createLog(ctx, msg, model.MessageLogEventCreated, nil, payload)
}

// LogQueued logs when a message is queued for sending
func (s *messageLogService) LogQueued(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error {
	return s.createLog(ctx, msg, model.MessageLogEventQueued, nil, payload)
}

// LogSent logs when a message is successfully sent
func (s *messageLogService) LogSent(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error {
	return s.createLog(ctx, msg, model.MessageLogEventSent, nil, payload)
}

// LogDelivered logs when a message is delivered
func (s *messageLogService) LogDelivered(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error {
	return s.createLog(ctx, msg, model.MessageLogEventDelivered, nil, payload)
}

// LogRead logs when a message is read
func (s *messageLogService) LogRead(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error {
	return s.createLog(ctx, msg, model.MessageLogEventRead, nil, payload)
}

// LogReplied logs when a reply is received
func (s *messageLogService) LogReplied(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error {
	return s.createLog(ctx, msg, model.MessageLogEventReplied, nil, payload)
}

// LogFailed logs when a message fails to send
func (s *messageLogService) LogFailed(ctx context.Context, msg *model.Message, errorMsg string, payload model.MessageLogPayload) error {
	return s.createLog(ctx, msg, model.MessageLogEventFailed, &errorMsg, payload)
}

// LogRetried logs when a message is retried
func (s *messageLogService) LogRetried(ctx context.Context, msg *model.Message, attempt int, payload model.MessageLogPayload) error {
	if payload == nil {
		payload = make(model.MessageLogPayload)
	}
	payload["attempt"] = attempt
	return s.createLog(ctx, msg, model.MessageLogEventRetried, nil, payload)
}

// LogProcessed logs when a response is processed
func (s *messageLogService) LogProcessed(ctx context.Context, msg *model.Message, payload model.MessageLogPayload) error {
	return s.createLog(ctx, msg, model.MessageLogEventProcessed, nil, payload)
}

// GetByMessageID retrieves all logs for a message
func (s *messageLogService) GetByMessageID(ctx context.Context, messageID uuid.UUID) ([]*model.MessageLog, error) {
	return s.repo.GetByMessageID(ctx, messageID)
}

// GetByAppointmentID retrieves all logs for an appointment
func (s *messageLogService) GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) ([]*model.MessageLog, error) {
	return s.repo.GetByAppointmentID(ctx, appointmentID)
}
