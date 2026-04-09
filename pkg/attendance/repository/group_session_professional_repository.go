package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/attendance/model"
)

// GroupSessionProfessionalRepository defines data operations for group session professionals
type GroupSessionProfessionalRepository interface {
	Upsert(ctx context.Context, record *model.GroupSessionProfessional) error
	GetByPlanAndDate(ctx context.Context, clinicID, sessionPlanID uuid.UUID, date string) (*model.GroupSessionProfessional, error)
}

type groupSessionProfessionalRepository struct {
	db *gorm.DB
}

// NewGroupSessionProfessionalRepository creates a new repository
func NewGroupSessionProfessionalRepository(db *gorm.DB) GroupSessionProfessionalRepository {
	return &groupSessionProfessionalRepository{db: db}
}

// Upsert creates or updates the professional assignment for a group session date
func (r *groupSessionProfessionalRepository) Upsert(ctx context.Context, record *model.GroupSessionProfessional) error {
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "session_plan_id"}, {Name: "session_date"}},
			DoUpdates: clause.AssignmentColumns([]string{"professional_id", "updated_at"}),
		}).
		Create(record).Error
	if err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// GetByPlanAndDate retrieves the professional assigned to a specific group session date
func (r *groupSessionProfessionalRepository) GetByPlanAndDate(ctx context.Context, clinicID, sessionPlanID uuid.UUID, date string) (*model.GroupSessionProfessional, error) {
	var record model.GroupSessionProfessional
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND session_plan_id = ? AND session_date = ?", clinicID, sessionPlanID, date).
		First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	r.loadRelations(ctx, &record)
	return &record, nil
}

// loadRelations loads professional name
func (r *groupSessionProfessionalRepository) loadRelations(ctx context.Context, record *model.GroupSessionProfessional) {
	var name string
	r.db.WithContext(ctx).
		Table("professionals").
		Select("name").
		Where("id = ?", record.ProfessionalID).
		Scan(&name)
	record.ProfessionalName = name
}
