package repository

import (
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
	List(clinicID uuid.UUID, page, perPage int) ([]*model.Procedure, int64, error)
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

// List retrieves a paginated list of procedures for a clinic
func (r *procedureRepository) List(clinicID uuid.UUID, page, perPage int) ([]*model.Procedure, int64, error) {
	var procedures []*model.Procedure
	var total int64

	query := r.db.Model(&model.Procedure{}).Where("clinic_id = ?", clinicID)

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	err = query.Order("name ASC").Offset(offset).Limit(perPage).Find(&procedures).Error

	return procedures, total, err
}
