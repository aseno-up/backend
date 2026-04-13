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
	Update(ctx context.Context, conn *model.WhatsAppConnection) error
	UpdateConnected(ctx context.Context, clinicID uuid.UUID, connected bool) error
	Deactivate(ctx context.Context, clinicID uuid.UUID) error
	HasAnyByClinicID(ctx context.Context, clinicID uuid.UUID) bool
	UpsertInstance(ctx context.Context, clinicID uuid.UUID, instance string) error
	CountActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (int64, error)
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

// Update updates an existing WhatsApp connection record
func (r *whatsAppConnectionRepository) Update(ctx context.Context, conn *model.WhatsAppConnection) error {
	result := r.db.WithContext(ctx).Save(conn)
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

// HasAnyByClinicID returns true if the clinic has ever had a WhatsApp connection (active or not).
func (r *whatsAppConnectionRepository) HasAnyByClinicID(ctx context.Context, clinicID uuid.UUID) bool {
	var count int64
	r.db.WithContext(ctx).
		Model(&model.WhatsAppConnection{}).
		Where("clinic_id = ?", clinicID).
		Count(&count)
	return count > 0
}

// Deactivate sets is_active=false and connected=false for the active connection of a clinic.
func (r *whatsAppConnectionRepository) Deactivate(ctx context.Context, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.WhatsAppConnection{}).
		Where("clinic_id = ? AND is_active = true AND deleted_at IS NULL", clinicID).
		Updates(map[string]interface{}{"is_active": false, "connected": false})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// UpsertInstance creates a new connection or updates the instance name for the active connection.
// If an inactive row exists for this clinic (user disconnected and is reconnecting),
// it is reactivated in place — required because `instance` has a unique index that
// would reject a fresh INSERT with the same generated name.
func (r *whatsAppConnectionRepository) UpsertInstance(ctx context.Context, clinicID uuid.UUID, instance string) error {
	existing, err := r.GetActiveByClinicID(ctx, clinicID)
	if err == nil {
		result := r.db.WithContext(ctx).
			Model(existing).
			Update("instance", &instance)
		if result.Error != nil {
			return errors.ErrInternalInstance.WithError(result.Error)
		}
		return nil
	}

	var inactive model.WhatsAppConnection
	res := r.db.WithContext(ctx).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID).
		Order("updated_at DESC").
		First(&inactive)
	if res.Error == nil {
		upd := r.db.WithContext(ctx).
			Model(&inactive).
			Updates(map[string]interface{}{
				"instance":  &instance,
				"is_active": true,
				"connected": false,
			})
		if upd.Error != nil {
			return errors.ErrInternalInstance.WithError(upd.Error)
		}
		return nil
	}

	conn := &model.WhatsAppConnection{
		ClinicID:  clinicID,
		Instance:  &instance,
		Connected: false,
		IsActive:  true,
	}
	return r.Create(ctx, conn)
}

// CountActiveByClinicID counts active WhatsApp connections belonging to a single clinic
func (r *whatsAppConnectionRepository) CountActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("whatsapp_connections").
		Where("clinic_id = ? AND is_active = true AND deleted_at IS NULL", clinicID).
		Count(&count).Error
	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return count, nil
}
