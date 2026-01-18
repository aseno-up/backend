package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/clinic/model"
	"github.com/aseno/backend/pkg/clinic/repository"
)

// ClinicService defines the interface for clinic operations
type ClinicService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.ClinicResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateClinicRequest) (*model.ClinicResponse, error)
}

// clinicService implements ClinicService
type clinicService struct {
	repo        repository.ClinicRepository
	frontendURL string
}

// NewClinicService creates a new clinic service
func NewClinicService(repo repository.ClinicRepository, frontendURL string) ClinicService {
	return &clinicService{
		repo:        repo,
		frontendURL: strings.TrimSuffix(frontendURL, "/"),
	}
}

// buildBookingURL constructs the public booking URL for a clinic
func (s *clinicService) buildBookingURL(slug string) string {
	return fmt.Sprintf("%s/agendar/%s", s.frontendURL, slug)
}

// GetByID retrieves a clinic by ID
func (s *clinicService) GetByID(ctx context.Context, id uuid.UUID) (*model.ClinicResponse, error) {
	clinic, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := clinic.ToResponse()
	resp.BookingURL = s.buildBookingURL(clinic.Slug)
	return resp, nil
}

// Update updates a clinic's information
func (s *clinicService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateClinicRequest) (*model.ClinicResponse, error) {
	// Get existing clinic
	clinic, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Name != "" {
		clinic.Name = req.Name
	}
	if req.Phone != "" {
		clinic.Phone = req.Phone
	}
	if req.Address != "" {
		clinic.Address = req.Address
	}
	if req.City != "" {
		clinic.City = req.City
	}
	if req.State != "" {
		clinic.State = req.State
	}
	if req.ZipCode != "" {
		clinic.ZipCode = req.ZipCode
	}
	if req.LogoURL != "" {
		clinic.LogoURL = req.LogoURL
	}
	if req.PrimaryColor != "" {
		clinic.PrimaryColor = req.PrimaryColor
	}
	if req.Settings != nil {
		// Validate AI/WhatsApp settings before updating
		if req.Settings.AIMinDelayBetweenMessages != nil {
			delay := *req.Settings.AIMinDelayBetweenMessages
			if delay < 1 || delay > 30 {
				return nil, errors.ErrValidationInstance.WithMessage(
					"ai_min_delay_between_messages must be between 1 and 30")
			}
		}

		if req.Settings.AIMinDelayBetweenPatients != nil {
			delay := *req.Settings.AIMinDelayBetweenPatients
			if delay < 1 || delay > 300 {
				return nil, errors.ErrValidationInstance.WithMessage(
					"ai_min_delay_between_patients must be between 1 and 300")
			}
		}

		if req.Settings.AIBusinessHours != nil {
			if err := req.Settings.AIBusinessHours.Validate(); err != nil {
				return nil, errors.ErrValidationInstance.WithMessage(
					fmt.Sprintf("Invalid business hours: %v", err))
			}
		}

		// Merge settings - only update fields that are provided
		if req.Settings.Timezone != "" {
			clinic.Settings.Timezone = req.Settings.Timezone
		}
		if req.Settings.DefaultAppointmentMinutes > 0 {
			clinic.Settings.DefaultAppointmentMinutes = req.Settings.DefaultAppointmentMinutes
		}
		if req.Settings.ReminderHoursBefore > 0 {
			clinic.Settings.ReminderHoursBefore = req.Settings.ReminderHoursBefore
		}
		// Boolean fields - only update if pointer is not nil
		if req.Settings.AllowOnlineBooking != nil {
			clinic.Settings.AllowOnlineBooking = *req.Settings.AllowOnlineBooking
		}
		if req.Settings.RequireConfirmation != nil {
			clinic.Settings.RequireConfirmation = *req.Settings.RequireConfirmation
		}
		if req.Settings.WhatsAppRemindersEnabled != nil {
			clinic.Settings.WhatsAppRemindersEnabled = *req.Settings.WhatsAppRemindersEnabled
		}
		if req.Settings.EmailRemindersEnabled != nil {
			clinic.Settings.EmailRemindersEnabled = *req.Settings.EmailRemindersEnabled
		}

		// AI/WhatsApp settings
		if req.Settings.AIMinDelayBetweenMessages != nil {
			clinic.Settings.AIMinDelayBetweenMessages = req.Settings.AIMinDelayBetweenMessages
		}
		if req.Settings.AIMinDelayBetweenPatients != nil {
			clinic.Settings.AIMinDelayBetweenPatients = req.Settings.AIMinDelayBetweenPatients
		}
		if req.Settings.AIBusinessHours != nil {
			clinic.Settings.AIBusinessHours = req.Settings.AIBusinessHours
		}
	}

	// Save changes
	if err := s.repo.Update(ctx, clinic); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	resp := clinic.ToResponse()
	resp.BookingURL = s.buildBookingURL(clinic.Slug)
	return resp, nil
}
