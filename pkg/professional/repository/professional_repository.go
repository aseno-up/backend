package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/professional/model"
)

// ProfessionalRepository defines the interface for professional data operations
type ProfessionalRepository interface {
	Create(ctx context.Context, professional *model.Professional) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Professional, error)
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Professional, int64, error)
	Update(ctx context.Context, professional *model.Professional) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	BulkDelete(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error
	GetByUserID(ctx context.Context, userID, clinicID uuid.UUID) (*model.Professional, error)
	CountActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (int64, error)
	FirstActiveIDByClinic(ctx context.Context, clinicID uuid.UUID) (*uuid.UUID, error)
	Restore(ctx context.Context, id, clinicID uuid.UUID) error
}

// professionalRepository implements ProfessionalRepository
type professionalRepository struct {
	db *gorm.DB
}

// NewProfessionalRepository creates a new professional repository
func NewProfessionalRepository(db *gorm.DB) ProfessionalRepository {
	return &professionalRepository{db: db}
}

// Create creates a new professional
func (r *professionalRepository) Create(ctx context.Context, professional *model.Professional) error {
	result := r.db.WithContext(ctx).Create(professional)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a professional by ID within a clinic
func (r *professionalRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Professional, error) {
	var professional model.Professional
	result := r.db.WithContext(ctx).
		Preload("Specialties").
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).
		First(&professional)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrProfessionalNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &professional, nil
}

// List retrieves professionals belonging to a clinic using standard pagination filters
func (r *professionalRepository) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Professional, int64, error) {
	var professionals []*model.Professional
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Professional{}).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID)

	// Default is active-only so dropdowns and pickers never leak inactive
	// professionals into places users shouldn't assign them. Callers that
	// genuinely need the full list (financial repasse, /profissionais with
	// the "Todos" tab) must pass status=all explicitly.
	switch filter.Status {
	case "inactive":
		query = query.Where("is_active = ?", false)
	case "all":
		// no filter on is_active
	default:
		query = query.Where("is_active = ?", true)
	}

	// Search string
	if filter.Search != "" {
		searchTerm := "%" + filter.Search + "%"
		query = query.Where(
			"name ILIKE ? OR email ILIKE ? OR phone ILIKE ?",
			searchTerm, searchTerm, searchTerm,
		)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Sorting
	switch filter.SortBy {
	case "name_desc":
		query = query.Order("name DESC")
	case "created_asc":
		query = query.Order("created_at ASC")
	case "created_desc":
		query = query.Order("created_at DESC")
	default: // "name_asc"
		query = query.Order("name ASC")
	}

	// Get paginated results
	result := query.
		Scopes(pagination.Paginate(&filter)).
		Find(&professionals)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return professionals, total, nil
}

// Update updates a professional. Specialty associations are managed separately
// via SpecialtyRepository.SetProfessionalSpecialties, so we omit them here to
// keep GORM from cascading an upsert into professional_specialties.
func (r *professionalRepository) Update(ctx context.Context, professional *model.Professional) error {
	result := r.db.WithContext(ctx).Omit("Specialties").Save(professional)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete sets the professional as inactive (is_active = false). We intentionally
// do not hard or soft delete the row — professionals are referenced from many
// other tables (appointments, session_plans, professional_procedures, ...) and
// toggling is_active preserves every historical reference while hiding the
// professional from pickers. Reactivate later via Restore.
func (r *professionalRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.Professional{}).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Update("is_active", false)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrProfessionalNotFoundInstance
	}
	return nil
}

// Restore flips is_active back to true so the professional becomes usable
// again in pickers and filters.
func (r *professionalRepository) Restore(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.Professional{}).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Update("is_active", true)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrProfessionalNotFoundInstance
	}
	return nil
}

// BulkDelete soft deletes multiple professionals belonging to a clinic
func (r *professionalRepository) BulkDelete(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id IN ? AND clinic_id = ?", ids, clinicID).
		Delete(&model.Professional{})

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// CountActiveByClinicID counts active professionals belonging to a single clinic
func (r *professionalRepository) CountActiveByClinicID(ctx context.Context, clinicID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Professional{}).
		Where("clinic_id = ? AND is_active = true AND deleted_at IS NULL", clinicID).
		Count(&count).Error
	if err != nil {
		return 0, errors.ErrInternalInstance.WithError(err)
	}
	return count, nil
}

// FirstActiveIDByClinic returns the ID of any active professional in the clinic,
// used as a fallback when a session plan was created without a fixed professional
// ("any professional" mode) and needs a concrete assignee to materialize.
func (r *professionalRepository) FirstActiveIDByClinic(ctx context.Context, clinicID uuid.UUID) (*uuid.UUID, error) {
	var prof model.Professional
	err := r.db.WithContext(ctx).
		Select("id").
		Where("clinic_id = ? AND is_active = true AND deleted_at IS NULL", clinicID).
		Order("created_at ASC").
		First(&prof).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return &prof.ID, nil
}

// GetByUserID retrieves a professional by their linked user ID
func (r *professionalRepository) GetByUserID(ctx context.Context, userID, clinicID uuid.UUID) (*model.Professional, error) {
	var professional model.Professional
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND clinic_id = ? AND deleted_at IS NULL", userID, clinicID).
		First(&professional)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrProfessionalNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &professional, nil
}
