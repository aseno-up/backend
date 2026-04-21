package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/appointment/model"
)

// AppointmentReceiptTokenRepository exposes persistence for the opaque tokens
// used to share the public receipt link (`/recibo/:token`).
type AppointmentReceiptTokenRepository interface {
	// GetOrCreateByAppointment returns an existing token for the appointment
	// or creates a new one. The operation is idempotent so the "Share receipt"
	// button can be clicked repeatedly without generating orphaned rows.
	GetOrCreateByAppointment(ctx context.Context, clinicID, appointmentID uuid.UUID) (*model.AppointmentReceiptToken, error)
	// GetByToken is used by the public endpoint and is intentionally scoped by
	// token only — clinic_id comes back on the row and is used downstream.
	GetByToken(ctx context.Context, token uuid.UUID) (*model.AppointmentReceiptToken, error)
}

type appointmentReceiptTokenRepository struct {
	db *gorm.DB
}

func NewAppointmentReceiptTokenRepository(db *gorm.DB) AppointmentReceiptTokenRepository {
	return &appointmentReceiptTokenRepository{db: db}
}

func (r *appointmentReceiptTokenRepository) GetOrCreateByAppointment(ctx context.Context, clinicID, appointmentID uuid.UUID) (*model.AppointmentReceiptToken, error) {
	var existing model.AppointmentReceiptToken
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND appointment_id = ?", clinicID, appointmentID).
		First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	token := &model.AppointmentReceiptToken{
		ClinicID:      clinicID,
		AppointmentID: appointmentID,
		Token:         uuid.New(),
	}
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		// Race: if another request created the row in parallel, re-read.
		var again model.AppointmentReceiptToken
		if fallbackErr := r.db.WithContext(ctx).
			Where("clinic_id = ? AND appointment_id = ?", clinicID, appointmentID).
			First(&again).Error; fallbackErr == nil {
			return &again, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return token, nil
}

func (r *appointmentReceiptTokenRepository) GetByToken(ctx context.Context, token uuid.UUID) (*model.AppointmentReceiptToken, error) {
	var row model.AppointmentReceiptToken
	err := r.db.WithContext(ctx).
		Where("token = ?", token).
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &row, nil
}
