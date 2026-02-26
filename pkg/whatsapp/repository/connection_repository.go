package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/whatsapp/model"
)

// WhatsAppConnectionRepository defines the interface for WhatsApp connection data operations
type WhatsAppConnectionRepository interface {
	Create(ctx context.Context, conn *model.WhatsAppConnection) error
	GetActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.WhatsAppConnection, error)
	GetByInstance(ctx context.Context, instance string) (*model.WhatsAppConnection, error)
	UpdateConnected(ctx context.Context, clinicID uuid.UUID, connected bool) error
	UpsertInstance(ctx context.Context, clinicID uuid.UUID, instance string) error
}

// whatsAppConnectionRepository implements WhatsAppConnectionRepository
type whatsAppConnectionRepository struct {
	db *gorm.DB
}

// NewWhatsAppConnectionRepository creates a new WhatsApp connection repository
func NewWhatsAppConnectionRepository(db *gorm.DB) WhatsAppConnectionRepository {
	return &whatsAppConnectionRepository{db: db}
}

// Create creates a new WhatsApp connection record
func (r *whatsAppConnectionRepository) Create(ctx context.Context, conn *model.WhatsAppConnection) error {
	result := r.db.WithContext(ctx).Create(conn)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetActiveByClinicID retrieves the active WhatsApp connection for a clinic
func (r *whatsAppConnectionRepository) GetActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (*model.WhatsAppConnection, error) {
	var conn model.WhatsAppConnection
	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND is_active = true AND deleted_at IS NULL", clinicID).
		First(&conn)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("WhatsApp connection not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &conn, nil
}

// GetByInstance retrieves a clinic's active WhatsApp connection by instance name
func (r *whatsAppConnectionRepository) GetByInstance(ctx context.Context, instance string) (*model.WhatsAppConnection, error) {
	var conn model.WhatsAppConnection
	result := r.db.WithContext(ctx).
		Where("instance = ? AND deleted_at IS NULL", instance).
		First(&conn)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("WhatsApp connection not found")
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &conn, nil
}

// UpdateConnected updates the connected status of the active connection for a clinic
func (r *whatsAppConnectionRepository) UpdateConnected(ctx context.Context, clinicID uuid.UUID, connected bool) error {
	result := r.db.WithContext(ctx).
		Model(&model.WhatsAppConnection{}).
		Where("clinic_id = ? AND is_active = true AND deleted_at IS NULL", clinicID).
		Update("connected", connected)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// UpsertInstance creates a new connection or updates the instance name for the active connection.
// If no active connection exists, it creates one with is_active = true.
// This supports the MVP constraint (1 active connection per clinic).
func (r *whatsAppConnectionRepository) UpsertInstance(ctx context.Context, clinicID uuid.UUID, instance string) error {
	existing, err := r.GetActiveByClinicID(ctx, clinicID)
	if err != nil {
		// No active connection exists — create one
		conn := &model.WhatsAppConnection{
			ClinicID:  clinicID,
			Instance:  instance,
			Connected: false,
			IsActive:  true,
		}
		return r.Create(ctx, conn)
	}

	// Update the instance name of the existing connection
	result := r.db.WithContext(ctx).
		Model(existing).
		Update("instance", instance)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}
