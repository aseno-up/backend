package repository

import (
	"context"

	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/procedure/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProcedureRepository defines the interface for procedure data access
type ProcedureRepository interface {
	Create(procedure *model.Procedure) error
	GetByID(id uuid.UUID, clinicID uuid.UUID) (*model.Procedure, error)
	GetByIDs(ids []uuid.UUID, clinicID uuid.UUID) ([]*model.Procedure, error)
	Update(procedure *model.Procedure) error
	Delete(id uuid.UUID, clinicID uuid.UUID) error
	BulkDelete(ids []uuid.UUID, clinicID uuid.UUID) error
	Restore(id uuid.UUID, clinicID uuid.UUID) error
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Procedure, int64, error)
	ListByClinicAndSpecialty(ctx context.Context, clinicID uuid.UUID, specialtyID *uuid.UUID) ([]*model.Procedure, error)
}

type procedureRepository struct {
	db *gorm.DB
}

// NewProcedureRepository creates a new procedure repository
func NewProcedureRepository(db *gorm.DB) ProcedureRepository {
	return &procedureRepository{db: db}
}

// Create inserts a new procedure into the database
func (r *procedureRepository) Create(procedure *model.Procedure) error {
	return r.db.Create(procedure).Error
}

// GetByID retrieves a procedure by ID and ClinicID
func (r *procedureRepository) GetByID(id uuid.UUID, clinicID uuid.UUID) (*model.Procedure, error) {
	var procedure model.Procedure
	err := r.db.Where("id = ? AND clinic_id = ?", id, clinicID).First(&procedure).Error
	if err != nil {
		return nil, err
	}
	return &procedure, nil
}

// GetByIDs retrieves multiple procedures by IDs and ClinicID
func (r *procedureRepository) GetByIDs(ids []uuid.UUID, clinicID uuid.UUID) ([]*model.Procedure, error) {
	var procedures []*model.Procedure
	err := r.db.Where("id IN ? AND clinic_id = ?", ids, clinicID).Find(&procedures).Error
	return procedures, err
}

// Update updates an existing procedure
func (r *procedureRepository) Update(procedure *model.Procedure) error {
	// GORM's Save updates all fields. We ensure ClinicID hasn't changed by explicitly scoping.
	return r.db.Where("id = ? AND clinic_id = ?", procedure.ID, procedure.ClinicID).Updates(procedure).Error
}

// Delete marks a procedure as deleted
func (r *procedureRepository) Delete(id uuid.UUID, clinicID uuid.UUID) error {
	return r.db.Where("id = ? AND clinic_id = ?", id, clinicID).Delete(&model.Procedure{}).Error
}

// BulkDelete marks multiple procedures as deleted
func (r *procedureRepository) BulkDelete(ids []uuid.UUID, clinicID uuid.UUID) error {
	return r.db.Where("id IN ? AND clinic_id = ?", ids, clinicID).Delete(&model.Procedure{}).Error
}

// Restore clears deleted_at for a soft-deleted procedure, bringing it back
// to the active catalog. Uses Unscoped because the row would otherwise be
// filtered out by the gorm.DeletedAt auto-filter.
func (r *procedureRepository) Restore(id uuid.UUID, clinicID uuid.UUID) error {
	return r.db.Unscoped().
		Model(&model.Procedure{}).
		Where("id = ? AND clinic_id = ? AND deleted_at IS NOT NULL", id, clinicID).
		Update("deleted_at", nil).Error
}

// List retrieves a paginated list of procedures for a clinic
func (r *procedureRepository) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Procedure, int64, error) {
	var procedures []*model.Procedure
	var total int64

	// "inactive" shows only soft-deleted, "all" shows both active + inactive,
	// and the default (or "active") relies on GORM's automatic gorm.DeletedAt
	// filter to hide soft-deleted rows.
	var query *gorm.DB
	switch filter.Status {
	case "inactive":
		query = r.db.WithContext(ctx).Unscoped().Model(&model.Procedure{}).
			Where("clinic_id = ? AND deleted_at IS NOT NULL", clinicID)
	case "all":
		query = r.db.WithContext(ctx).Unscoped().Model(&model.Procedure{}).
			Where("clinic_id = ?", clinicID)
	default:
		query = r.db.WithContext(ctx).Model(&model.Procedure{}).Where("clinic_id = ?", clinicID)
	}

	// Search
	if filter.Search != "" {
		searchQuery := "%" + filter.Search + "%"
		query = query.Where("name ILIKE ?", searchQuery)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Sorting
	switch filter.SortBy {
	case "name_asc":
		query = query.Order("name ASC")
	case "name_desc":
		query = query.Order("name DESC")
	case "price_asc":
		query = query.Order("price ASC")
	case "price_desc":
		query = query.Order("price DESC")
	case "created_desc":
		query = query.Order("created_at DESC")
	case "created_asc":
		query = query.Order("created_at ASC")
	default:
		query = query.Order("name ASC")
	}

	err = query.Scopes(pagination.Paginate(&filter)).Find(&procedures).Error
	return procedures, total, err
}

// ListByClinicAndSpecialty returns all procedures for a clinic, optionally filtered by specialty
func (r *procedureRepository) ListByClinicAndSpecialty(ctx context.Context, clinicID uuid.UUID, specialtyID *uuid.UUID) ([]*model.Procedure, error) {
	var procedures []*model.Procedure
	query := r.db.WithContext(ctx).Where("clinic_id = ?", clinicID)
	if specialtyID != nil {
		query = query.Where("specialty_id = ?", *specialtyID)
	}
	err := query.Order("name ASC").Find(&procedures).Error
	return procedures, err
}
