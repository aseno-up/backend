package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/professional/model"
)

// ProfessionalProcedureRepository defines data operations for professional-procedure commissions
type ProfessionalProcedureRepository interface {
	BulkUpsert(ctx context.Context, clinicID, professionalID uuid.UUID, items []model.ProfessionalProcedure) error
	ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID) ([]model.ProfessionalProcedure, error)
	ListByProcedure(ctx context.Context, procedureID, clinicID uuid.UUID) ([]model.ProfessionalProcedure, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	DeleteByProfessionalAndProcedure(ctx context.Context, professionalID, procedureID, clinicID uuid.UUID) error
}

type professionalProcedureRepository struct {
	db *gorm.DB
}

func NewProfessionalProcedureRepository(db *gorm.DB) ProfessionalProcedureRepository {
	return &professionalProcedureRepository{db: db}
}

// BulkUpsert inserts or updates commission settings for a professional's procedures.
// It uses ON CONFLICT (professional_id, procedure_id) to upsert.
func (r *professionalProcedureRepository) BulkUpsert(ctx context.Context, clinicID, professionalID uuid.UUID, items []model.ProfessionalProcedure) error {
	if len(items) == 0 {
		return nil
	}

	// Ensure all items have correct clinic and professional IDs
	for i := range items {
		items[i].ClinicID = clinicID
		items[i].ProfessionalID = professionalID
	}

	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "professional_id"}, {Name: "procedure_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"commission_type", "commission_value", "is_active", "updated_at"}),
			Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "professional_procedures.deleted_at IS NULL"}}},
		}).
		Create(&items)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// ListByProfessional returns all procedure commissions for a professional
func (r *professionalProcedureRepository) ListByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID) ([]model.ProfessionalProcedure, error) {
	var results []model.ProfessionalProcedure
	err := r.db.WithContext(ctx).
		Where("professional_id = ? AND clinic_id = ? AND deleted_at IS NULL", professionalID, clinicID).
		Order("created_at ASC").
		Find(&results).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return results, nil
}

// ListByProcedure returns all professional commissions for a procedure
func (r *professionalProcedureRepository) ListByProcedure(ctx context.Context, procedureID, clinicID uuid.UUID) ([]model.ProfessionalProcedure, error) {
	var results []model.ProfessionalProcedure
	err := r.db.WithContext(ctx).
		Where("procedure_id = ? AND clinic_id = ? AND deleted_at IS NULL", procedureID, clinicID).
		Find(&results).Error
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	return results, nil
}

// Delete soft-deletes a professional-procedure commission by ID
func (r *professionalProcedureRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.ProfessionalProcedure{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFoundInstance
	}
	return nil
}

// DeleteByProfessionalAndProcedure soft-deletes by the composite key
func (r *professionalProcedureRepository) DeleteByProfessionalAndProcedure(ctx context.Context, professionalID, procedureID, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("professional_id = ? AND procedure_id = ? AND clinic_id = ?", professionalID, procedureID, clinicID).
		Delete(&model.ProfessionalProcedure{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFoundInstance
	}
	return nil
}
