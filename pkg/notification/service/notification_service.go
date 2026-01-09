package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/notification/model"
	"github.com/aseno/backend/pkg/notification/repository"
)

// NotificationService defines the interface for notification operations
type NotificationService interface {
	// User-facing operations
	List(ctx context.Context, clinicID, userID uuid.UUID, filter *model.NotificationFilter) ([]*model.NotificationResponse, error)
	CountUnread(ctx context.Context, clinicID, userID uuid.UUID) (int64, error)
	MarkAsRead(ctx context.Context, id, clinicID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, clinicID, userID uuid.UUID) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error

	// System operations (for creating notifications from other services)
	Create(ctx context.Context, notification *model.Notification) error
	NotifyAppointmentConfirmed(ctx context.Context, clinicID uuid.UUID, patientName string, professionalName string, startTime time.Time) error
	NotifyAppointmentCancelled(ctx context.Context, clinicID uuid.UUID, patientName string, professionalName string, startTime time.Time, reason string) error
	NotifyWhatsAppConnected(ctx context.Context, clinicID uuid.UUID) error
	NotifyWhatsAppDisconnected(ctx context.Context, clinicID uuid.UUID) error
	NotifyTrialEnding(ctx context.Context, clinicID uuid.UUID, daysLeft int) error
	NotifyNewPatient(ctx context.Context, clinicID uuid.UUID, patientName string) error
}

type notificationService struct {
	repo repository.NotificationRepository
}

// NewNotificationService creates a new notification service
func NewNotificationService(repo repository.NotificationRepository) NotificationService {
	return &notificationService{repo: repo}
}

// List returns notifications for a user
func (s *notificationService) List(ctx context.Context, clinicID, userID uuid.UUID, filter *model.NotificationFilter) ([]*model.NotificationResponse, error) {
	notifications, err := s.repo.List(ctx, clinicID, userID, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]*model.NotificationResponse, len(notifications))
	for i, n := range notifications {
		responses[i] = n.ToResponse()
	}
	return responses, nil
}

// CountUnread returns the count of unread notifications
func (s *notificationService) CountUnread(ctx context.Context, clinicID, userID uuid.UUID) (int64, error) {
	return s.repo.CountUnread(ctx, clinicID, userID)
}

// MarkAsRead marks a notification as read
func (s *notificationService) MarkAsRead(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.MarkAsRead(ctx, id, clinicID)
}

// MarkAllAsRead marks all notifications as read for a user
func (s *notificationService) MarkAllAsRead(ctx context.Context, clinicID, userID uuid.UUID) error {
	return s.repo.MarkAllAsRead(ctx, clinicID, userID)
}

// Delete deletes a notification
func (s *notificationService) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.Delete(ctx, id, clinicID)
}

// Create creates a new notification
func (s *notificationService) Create(ctx context.Context, notification *model.Notification) error {
	return s.repo.Create(ctx, notification)
}

// NotifyAppointmentConfirmed creates a notification for a confirmed appointment
func (s *notificationService) NotifyAppointmentConfirmed(ctx context.Context, clinicID uuid.UUID, patientName string, professionalName string, startTime time.Time) error {
	notification := &model.Notification{
		ClinicID: clinicID,
		Type:     model.NotificationTypeAppointmentConfirmed,
		Title:    "Agendamento Confirmado",
		Message:  patientName + " confirmou o agendamento com " + professionalName + " para " + startTime.Format("02/01 as 15:04"),
		Data: model.JSON{
			"patient_name":      patientName,
			"professional_name": professionalName,
			"start_time":        startTime,
		},
	}
	return s.repo.Create(ctx, notification)
}

// NotifyAppointmentCancelled creates a notification for a cancelled appointment
func (s *notificationService) NotifyAppointmentCancelled(ctx context.Context, clinicID uuid.UUID, patientName string, professionalName string, startTime time.Time, reason string) error {
	message := patientName + " cancelou o agendamento com " + professionalName + " de " + startTime.Format("02/01 as 15:04")
	if reason != "" {
		message += ". Motivo: " + reason
	}

	notification := &model.Notification{
		ClinicID: clinicID,
		Type:     model.NotificationTypeAppointmentCancelled,
		Title:    "Agendamento Cancelado",
		Message:  message,
		Data: model.JSON{
			"patient_name":      patientName,
			"professional_name": professionalName,
			"start_time":        startTime,
			"reason":            reason,
		},
	}
	return s.repo.Create(ctx, notification)
}

// NotifyWhatsAppConnected creates a notification for WhatsApp connection
func (s *notificationService) NotifyWhatsAppConnected(ctx context.Context, clinicID uuid.UUID) error {
	notification := &model.Notification{
		ClinicID: clinicID,
		Type:     model.NotificationTypeWhatsAppConnected,
		Title:    "WhatsApp Conectado",
		Message:  "Seu WhatsApp foi conectado com sucesso. Os lembretes automaticos estao ativos.",
	}
	return s.repo.Create(ctx, notification)
}

// NotifyWhatsAppDisconnected creates a notification for WhatsApp disconnection
func (s *notificationService) NotifyWhatsAppDisconnected(ctx context.Context, clinicID uuid.UUID) error {
	notification := &model.Notification{
		ClinicID: clinicID,
		Type:     model.NotificationTypeWhatsAppDisconnected,
		Title:    "WhatsApp Desconectado",
		Message:  "Seu WhatsApp foi desconectado. Reconecte para continuar enviando lembretes automaticos.",
	}
	return s.repo.Create(ctx, notification)
}

// NotifyTrialEnding creates a notification for trial ending
func (s *notificationService) NotifyTrialEnding(ctx context.Context, clinicID uuid.UUID, daysLeft int) error {
	var message string
	if daysLeft <= 0 {
		message = "Seu periodo de teste expirou. Assine um plano para continuar usando o Aseno."
	} else if daysLeft == 1 {
		message = "Seu periodo de teste termina amanha. Assine um plano para continuar usando o Aseno."
	} else {
		message = fmt.Sprintf("Seu periodo de teste termina em %d dias. Assine um plano para continuar.", daysLeft)
	}

	notifType := model.NotificationTypeTrialEnding
	if daysLeft <= 0 {
		notifType = model.NotificationTypeTrialExpired
	}

	notification := &model.Notification{
		ClinicID: clinicID,
		Type:     notifType,
		Title:    "Periodo de Teste",
		Message:  message,
		Data: model.JSON{
			"days_left": daysLeft,
		},
	}
	return s.repo.Create(ctx, notification)
}

// NotifyNewPatient creates a notification for a new patient
func (s *notificationService) NotifyNewPatient(ctx context.Context, clinicID uuid.UUID, patientName string) error {
	notification := &model.Notification{
		ClinicID: clinicID,
		Type:     model.NotificationTypeNewPatient,
		Title:    "Novo Paciente",
		Message:  patientName + " foi adicionado a sua lista de pacientes.",
		Data: model.JSON{
			"patient_name": patientName,
		},
	}
	return s.repo.Create(ctx, notification)
}
