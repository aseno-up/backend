package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/calendar/model"
)

// GoogleCalendarClient defines the interface for Google Calendar operations
type GoogleCalendarClient interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*model.GoogleOAuthTokens, error)
	RefreshAccessToken(ctx context.Context, refreshToken string) (*model.GoogleOAuthTokens, error)
	GetUserEmail(ctx context.Context, accessToken string) (string, error)
	ListCalendars(ctx context.Context, accessToken string) ([]model.GoogleCalendar, error)
	CreateEvent(ctx context.Context, accessToken, calendarID string, event *model.GoogleEvent) (*model.GoogleEvent, error)
	UpdateEvent(ctx context.Context, accessToken, calendarID, eventID string, event *model.GoogleEvent) (*model.GoogleEvent, error)
	DeleteEvent(ctx context.Context, accessToken, calendarID, eventID string) error
	GetEvent(ctx context.Context, accessToken, calendarID, eventID string) (*model.GoogleEvent, error)
	ListEvents(ctx context.Context, accessToken, calendarID string, timeMin, timeMax time.Time, pageToken string) (*model.EventListResponse, error)
}

// IntegrationRepository defines the interface for calendar integration persistence
type IntegrationRepository interface {
	Create(ctx context.Context, integration *model.CalendarIntegration) error
	GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.CalendarIntegration, error)
	Update(ctx context.Context, integration *model.CalendarIntegration) error
	Delete(ctx context.Context, clinicID uuid.UUID) error
	UpdateTokens(ctx context.Context, clinicID uuid.UUID, accessToken, refreshToken string, expiry time.Time) error
	UpdateLastSync(ctx context.Context, clinicID uuid.UUID, lastSyncAt time.Time, syncError string) error
}

// SyncRepository defines the interface for calendar sync persistence
type SyncRepository interface {
	Create(ctx context.Context, sync *model.CalendarSync) error
	GetByAppointmentID(ctx context.Context, appointmentID uuid.UUID) (*model.CalendarSync, error)
	GetByGoogleEventID(ctx context.Context, googleEventID string) (*model.CalendarSync, error)
	Update(ctx context.Context, sync *model.CalendarSync) error
	Delete(ctx context.Context, appointmentID uuid.UUID) error
	GetPendingSyncs(ctx context.Context, clinicID uuid.UUID, limit int) ([]*model.CalendarSync, error)
}

// CalendarService defines the interface for calendar operations
type CalendarService interface {
	// OAuth
	GetAuthURL(clinicID uuid.UUID) string
	HandleCallback(ctx context.Context, clinicID uuid.UUID, code string) (*model.CalendarIntegration, error)

	// Integration management
	GetIntegration(ctx context.Context, clinicID uuid.UUID) (*model.CalendarIntegration, error)
	ListCalendars(ctx context.Context, clinicID uuid.UUID) ([]model.GoogleCalendar, error)
	SetCalendar(ctx context.Context, clinicID uuid.UUID, calendarID string) error
	DisableIntegration(ctx context.Context, clinicID uuid.UUID) error
	UpdateSyncSettings(ctx context.Context, clinicID uuid.UUID, enabled bool, direction model.SyncDirection) error

	// Sync operations
	SyncAppointmentToGoogle(ctx context.Context, clinicID uuid.UUID, appointment *AppointmentData) error
	DeleteAppointmentFromGoogle(ctx context.Context, clinicID uuid.UUID, appointmentID uuid.UUID) error
	UpdateAppointmentInGoogle(ctx context.Context, clinicID uuid.UUID, appointment *AppointmentData) error
}

// AppointmentData represents appointment data for syncing
type AppointmentData struct {
	ID               uuid.UUID
	Title            string
	PatientName      string
	PatientEmail     string
	ProfessionalName string
	StartTime        time.Time
	EndTime          time.Time
	Notes            string
	Location         string
}

// calendarService implements CalendarService
type calendarService struct {
	client          GoogleCalendarClient
	integrationRepo IntegrationRepository
	syncRepo        SyncRepository
}

// NewCalendarService creates a new calendar service
func NewCalendarService(
	client GoogleCalendarClient,
	integrationRepo IntegrationRepository,
	syncRepo SyncRepository,
) CalendarService {
	return &calendarService{
		client:          client,
		integrationRepo: integrationRepo,
		syncRepo:        syncRepo,
	}
}

// GetAuthURL returns the Google OAuth authorization URL
func (s *calendarService) GetAuthURL(clinicID uuid.UUID) string {
	return s.client.GetAuthURL(clinicID.String())
}

// HandleCallback handles the OAuth callback and saves the integration
func (s *calendarService) HandleCallback(ctx context.Context, clinicID uuid.UUID, code string) (*model.CalendarIntegration, error) {
	// Exchange code for tokens
	tokens, err := s.client.ExchangeCode(ctx, code)
	if err != nil {
		return nil, errors.ErrCalendarConnectionInstance.WithError(err)
	}

	// Get user email
	email, err := s.client.GetUserEmail(ctx, tokens.AccessToken)
	if err != nil {
		return nil, errors.ErrCalendarConnectionInstance.WithError(err)
	}

	// Check if integration already exists
	existing, _ := s.integrationRepo.GetByClinicID(ctx, clinicID)
	if existing != nil {
		// Update existing integration
		existing.AccessToken = tokens.AccessToken
		existing.RefreshToken = tokens.RefreshToken
		existing.TokenExpiry = tokens.Expiry
		existing.Email = email
		existing.SyncEnabled = true

		if err := s.integrationRepo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// Create new integration
	integration := &model.CalendarIntegration{
		ClinicID:      clinicID,
		Provider:      "google",
		AccessToken:   tokens.AccessToken,
		RefreshToken:  tokens.RefreshToken,
		TokenExpiry:   tokens.Expiry,
		Email:         email,
		SyncEnabled:   true,
		SyncDirection: model.SyncDirectionToGoogle,
	}

	if err := s.integrationRepo.Create(ctx, integration); err != nil {
		return nil, err
	}

	return integration, nil
}

// GetIntegration retrieves the calendar integration for a clinic
func (s *calendarService) GetIntegration(ctx context.Context, clinicID uuid.UUID) (*model.CalendarIntegration, error) {
	return s.integrationRepo.GetByClinicID(ctx, clinicID)
}

// ListCalendars lists available calendars for the clinic
func (s *calendarService) ListCalendars(ctx context.Context, clinicID uuid.UUID) ([]model.GoogleCalendar, error) {
	integration, err := s.integrationRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.getValidAccessToken(ctx, integration)
	if err != nil {
		return nil, err
	}

	return s.client.ListCalendars(ctx, accessToken)
}

// SetCalendar sets the calendar to sync with
func (s *calendarService) SetCalendar(ctx context.Context, clinicID uuid.UUID, calendarID string) error {
	integration, err := s.integrationRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return err
	}

	integration.CalendarID = calendarID
	return s.integrationRepo.Update(ctx, integration)
}

// DisableIntegration disables the calendar integration
func (s *calendarService) DisableIntegration(ctx context.Context, clinicID uuid.UUID) error {
	return s.integrationRepo.Delete(ctx, clinicID)
}

// UpdateSyncSettings updates sync settings
func (s *calendarService) UpdateSyncSettings(ctx context.Context, clinicID uuid.UUID, enabled bool, direction model.SyncDirection) error {
	integration, err := s.integrationRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return err
	}

	integration.SyncEnabled = enabled
	integration.SyncDirection = direction
	return s.integrationRepo.Update(ctx, integration)
}

// SyncAppointmentToGoogle creates an event in Google Calendar for an appointment
func (s *calendarService) SyncAppointmentToGoogle(ctx context.Context, clinicID uuid.UUID, appointment *AppointmentData) error {
	integration, err := s.integrationRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return err
	}

	if !integration.SyncEnabled || integration.CalendarID == "" {
		return nil // Sync not configured, skip silently
	}

	accessToken, err := s.getValidAccessToken(ctx, integration)
	if err != nil {
		return err
	}

	// Check if already synced
	existingSync, _ := s.syncRepo.GetByAppointmentID(ctx, appointment.ID)
	if existingSync != nil && existingSync.GoogleEventID != "" {
		// Update existing event
		return s.UpdateAppointmentInGoogle(ctx, clinicID, appointment)
	}

	// Create Google Calendar event
	event := s.appointmentToGoogleEvent(appointment)
	createdEvent, err := s.client.CreateEvent(ctx, accessToken, integration.CalendarID, event)
	if err != nil {
		// Save sync error
		s.saveSyncError(ctx, clinicID, appointment.ID, err)
		return errors.ErrCalendarSyncFailedInstance.WithError(err)
	}

	// Save sync record
	sync := &model.CalendarSync{
		ClinicID:      clinicID,
		AppointmentID: appointment.ID,
		GoogleEventID: createdEvent.ID,
		CalendarID:    integration.CalendarID,
		SyncStatus:    model.SyncStatusSynced,
	}
	now := time.Now()
	sync.LastSyncAt = &now

	if existingSync != nil {
		existingSync.GoogleEventID = createdEvent.ID
		existingSync.SyncStatus = model.SyncStatusSynced
		existingSync.LastSyncAt = &now
		existingSync.LastSyncError = ""
		return s.syncRepo.Update(ctx, existingSync)
	}

	return s.syncRepo.Create(ctx, sync)
}

// UpdateAppointmentInGoogle updates an existing event in Google Calendar
func (s *calendarService) UpdateAppointmentInGoogle(ctx context.Context, clinicID uuid.UUID, appointment *AppointmentData) error {
	integration, err := s.integrationRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return err
	}

	if !integration.SyncEnabled || integration.CalendarID == "" {
		return nil
	}

	sync, err := s.syncRepo.GetByAppointmentID(ctx, appointment.ID)
	if err != nil || sync.GoogleEventID == "" {
		// No existing sync, create new
		return s.SyncAppointmentToGoogle(ctx, clinicID, appointment)
	}

	accessToken, err := s.getValidAccessToken(ctx, integration)
	if err != nil {
		return err
	}

	event := s.appointmentToGoogleEvent(appointment)
	_, err = s.client.UpdateEvent(ctx, accessToken, integration.CalendarID, sync.GoogleEventID, event)
	if err != nil {
		s.saveSyncError(ctx, clinicID, appointment.ID, err)
		return errors.ErrCalendarSyncFailedInstance.WithError(err)
	}

	now := time.Now()
	sync.SyncStatus = model.SyncStatusSynced
	sync.LastSyncAt = &now
	sync.LastSyncError = ""
	return s.syncRepo.Update(ctx, sync)
}

// DeleteAppointmentFromGoogle deletes an event from Google Calendar
func (s *calendarService) DeleteAppointmentFromGoogle(ctx context.Context, clinicID uuid.UUID, appointmentID uuid.UUID) error {
	integration, err := s.integrationRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil // No integration, nothing to delete
	}

	if !integration.SyncEnabled || integration.CalendarID == "" {
		return nil
	}

	sync, err := s.syncRepo.GetByAppointmentID(ctx, appointmentID)
	if err != nil || sync.GoogleEventID == "" {
		return nil // No sync record, nothing to delete
	}

	accessToken, err := s.getValidAccessToken(ctx, integration)
	if err != nil {
		return err
	}

	if err := s.client.DeleteEvent(ctx, accessToken, integration.CalendarID, sync.GoogleEventID); err != nil {
		// Log but don't fail - event might already be deleted
		s.saveSyncError(ctx, clinicID, appointmentID, err)
	}

	sync.SyncStatus = model.SyncStatusDeleted
	return s.syncRepo.Update(ctx, sync)
}

// getValidAccessToken returns a valid access token, refreshing if necessary
func (s *calendarService) getValidAccessToken(ctx context.Context, integration *model.CalendarIntegration) (string, error) {
	if time.Now().Before(integration.TokenExpiry.Add(-5 * time.Minute)) {
		return integration.AccessToken, nil
	}

	// Token expired or about to expire, refresh it
	tokens, err := s.client.RefreshAccessToken(ctx, integration.RefreshToken)
	if err != nil {
		return "", errors.ErrCalendarConnectionInstance.WithError(err)
	}

	// Update stored tokens
	if err := s.integrationRepo.UpdateTokens(ctx, integration.ClinicID, tokens.AccessToken, tokens.RefreshToken, tokens.Expiry); err != nil {
		return "", err
	}

	return tokens.AccessToken, nil
}

// appointmentToGoogleEvent converts appointment data to a Google Calendar event
func (s *calendarService) appointmentToGoogleEvent(appointment *AppointmentData) *model.GoogleEvent {
	description := fmt.Sprintf("Paciente: %s\n", appointment.PatientName)
	if appointment.ProfessionalName != "" {
		description += fmt.Sprintf("Profissional: %s\n", appointment.ProfessionalName)
	}
	if appointment.Notes != "" {
		description += fmt.Sprintf("\nObservacoes: %s", appointment.Notes)
	}

	event := &model.GoogleEvent{
		Summary:     appointment.Title,
		Description: description,
		Location:    appointment.Location,
		Start: &model.GoogleEventTime{
			DateTime: appointment.StartTime.Format(time.RFC3339),
		},
		End: &model.GoogleEventTime{
			DateTime: appointment.EndTime.Format(time.RFC3339),
		},
	}

	// Add patient as attendee if email is provided
	if appointment.PatientEmail != "" {
		event.Attendees = []model.GoogleAttendee{
			{
				Email:       appointment.PatientEmail,
				DisplayName: appointment.PatientName,
			},
		}
	}

	return event
}

// saveSyncError saves a sync error to the sync record
func (s *calendarService) saveSyncError(ctx context.Context, clinicID, appointmentID uuid.UUID, err error) {
	sync, getErr := s.syncRepo.GetByAppointmentID(ctx, appointmentID)
	if getErr != nil {
		// Create new sync record with error
		sync = &model.CalendarSync{
			ClinicID:      clinicID,
			AppointmentID: appointmentID,
			SyncStatus:    model.SyncStatusFailed,
			LastSyncError: err.Error(),
		}
		s.syncRepo.Create(ctx, sync)
		return
	}

	sync.SyncStatus = model.SyncStatusFailed
	sync.LastSyncError = err.Error()
	s.syncRepo.Update(ctx, sync)
}
