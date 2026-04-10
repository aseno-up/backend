package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/patient/model"
)

// PatientWhatsAppConnectionRepository defines the interface for patient-whatsapp connection data operations
type PatientWhatsAppConnectionRepository interface {
	GetByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientWhatsAppConnection, error)
	Upsert(ctx context.Context, conn *model.PatientWhatsAppConnection) error
	DeleteByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) error
}

// patientWhatsAppConnectionRepository implements PatientWhatsAppConnectionRepository
type patientWhatsAppConnectionRepository struct {
	db *gorm.DB
}

// NewPatientWhatsAppConnectionRepository creates a new repository
func NewPatientWhatsAppConnectionRepository(db *gorm.DB) PatientWhatsAppConnectionRepository {
	return &patientWhatsAppConnectionRepository{db: db}
}

// GetByPatientID returns the connection config for a patient
func (r *patientWhatsAppConnectionRepository) GetByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientWhatsAppConnection, error) {
	var conn model.PatientWhatsAppConnection
	result := r.db.WithContext(ctx).
		Where("patient_id = ? AND clinic_id = ?", patientID, clinicID).
		First(&conn)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &conn, nil
}

// Upsert creates or updates a patient-whatsapp connection
func (r *patientWhatsAppConnectionRepository) Upsert(ctx context.Context, conn *model.PatientWhatsAppConnection) error {
	var existing model.PatientWhatsAppConnection
	result := r.db.WithContext(ctx).
		Where("patient_id = ? AND clinic_id = ?", conn.PatientID, conn.ClinicID).
		First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		if err := r.db.WithContext(ctx).Create(conn).Error; err != nil {
			return errors.ErrInternalInstance.WithError(err)
		}
		return nil
	}
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}

	// Update existing — Select forces GORM to include zero-value booleans (false)
	conn.ID = existing.ID
	result = r.db.WithContext(ctx).
		Select("WhatsAppConnectionID", "AISchedulingEnabled", "RemindersEnabled").
		Save(conn)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// DeleteByPatientID deletes the connection config for a patient
func (r *patientWhatsAppConnectionRepository) DeleteByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("patient_id = ? AND clinic_id = ?", patientID, clinicID).
		Delete(&model.PatientWhatsAppConnection{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
