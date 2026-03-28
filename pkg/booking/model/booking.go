package model

import (
	"time"

	"github.com/google/uuid"

	professionalModel "github.com/aseno/backend/pkg/professional/model"
)

// ClinicBookingInfo contains public clinic data for booking page
type ClinicBookingInfo struct {
	Name                      string `json:"name"`
	Slug                      string `json:"slug"`
	LogoURL                   string `json:"logo_url"`
	PrimaryColor              string `json:"primary_color"`
	Phone                     string `json:"phone"`
	Address                   string `json:"address"`
	City                      string `json:"city"`
	State                     string `json:"state"`
	DefaultAppointmentMinutes int    `json:"default_appointment_minutes"`
	AboutUs                   string `json:"about_us"`
	ProfessionalCount         int    `json:"professional_count"`
	SpecialtyCount            int    `json:"specialty_count"`
}

// SpecialtyBookingInfo contains public specialty data for booking
type SpecialtyBookingInfo struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ProcedureBookingInfo contains public procedure data for booking
type ProcedureBookingInfo struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	SpecialtyID     *uuid.UUID `json:"specialty_id,omitempty"`
	Price           float64    `json:"price"`
	DurationMinutes int        `json:"duration_minutes"`
}

// ProfessionalBookingInfo contains public professional data for booking
type ProfessionalBookingInfo struct {
	ID                   uuid.UUID                      `json:"id"`
	Name                 string                         `json:"name"`
	SpecialtyIDs         []uuid.UUID                    `json:"specialty_ids"`
	SpecialtyNames       []string                       `json:"specialty_names"`
	AvatarURL            string                         `json:"avatar_url"`
	Bio                  string                         `json:"bio"`
	Color                string                         `json:"color"`
	HasAvailableSchedule bool                           `json:"has_available_schedule"`
	WorkingHours         professionalModel.WorkingHours `json:"working_hours"`
}

// AvailableSlot represents an available time slot for booking
type AvailableSlot struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// AvailableSlotsResponse contains the list of available slots for a date
type AvailableSlotsResponse struct {
	Date            string           `json:"date"`
	ProfessionalID  uuid.UUID        `json:"professional_id"`
	DurationMinutes int              `json:"duration_minutes"`
	Slots           []*AvailableSlot `json:"slots"`
}

// CreateBookingRequest represents the request to create a booking
type CreateBookingRequest struct {
	ProfessionalID uuid.UUID  `json:"professional_id" binding:"required"`
	StartTime      time.Time  `json:"start_time" binding:"required"`
	ProcedureID    *uuid.UUID `json:"procedure_id"`
	PatientName    string     `json:"patient_name" binding:"required,min=2,max=100"`
	PatientPhone   string     `json:"patient_phone" binding:"required"`
	PatientEmail   string     `json:"patient_email" binding:"omitempty,email"`
	PatientNotes   string     `json:"patient_notes"`
}

// BookingResponse represents the response after creating a booking
type BookingResponse struct {
	ID           uuid.UUID `json:"id"`
	Professional string    `json:"professional"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
}
