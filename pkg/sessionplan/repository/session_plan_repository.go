package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/sessionplan/model"
)

// SessionPlanRepository defines the interface for session plan repository
type SessionPlanRepository interface {
	Create(ctx context.Context, plan *model.SessionPlan) error
	GetByID(ctx context.Context, clinicID, id uuid.UUID) (*model.SessionPlan, error)
	GetByClinic(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.SessionPlan, int64, error)
	GetByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.SessionPlan, error)
	GetByProfessional(ctx context.Context, clinicID, professionalID uuid.UUID) ([]*model.SessionPlan, error)
	GetActivePlansForGeneration(ctx context.Context, generateBefore time.Time) ([]*model.SessionPlan, error)
	GetAllActive(ctx context.Context, clinicID uuid.UUID) ([]*model.SessionPlan, error)
	Update(ctx context.Context, plan *model.SessionPlan) error
	UpdateStatus(ctx context.Context, clinicID, id uuid.UUID, status model.PlanStatus) error
	IncrementSessionsDone(ctx context.Context, clinicID, id uuid.UUID) error
	UpdateLastGenerated(ctx context.Context, clinicID, id uuid.UUID, lastGenerated time.Time) error
	Delete(ctx context.Context, clinicID, id uuid.UUID) error
	BulkCancel(ctx context.Context, clinicID uuid.UUID, ids []uuid.UUID) error
}

// sessionPlanRepository implements SessionPlanRepository
type sessionPlanRepository struct {
	db *gorm.DB
}

// NewSessionPlanRepository creates a new session plan repository
func NewSessionPlanRepository(db *gorm.DB) SessionPlanRepository {
	return &sessionPlanRepository{db: db}
}

// Create creates a new session plan
func (r *sessionPlanRepository) Create(ctx context.Context, plan *model.SessionPlan) error {
	if err := r.db.WithContext(ctx).Create(plan).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// GetByID retrieves a session plan by ID
func (r *sessionPlanRepository) GetByID(ctx context.Context, clinicID, id uuid.UUID) (*model.SessionPlan, error) {
	var plan model.SessionPlan
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		First(&plan).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFoundInstance.WithMessage("Session plan not found")
		}
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Load patient and professional names
	r.loadRelations(ctx, &plan)

	return &plan, nil
}

// GetByClinic retrieves all session plans for a clinic with pagination
func (r *sessionPlanRepository) GetByClinic(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.SessionPlan, int64, error) {
	var plans []*model.SessionPlan
	var total int64

	query := r.db.WithContext(ctx).Model(&model.SessionPlan{}).Where("clinic_id = ?", clinicID)

	if filter.Status != "" && filter.Status != "all" {
		query = query.Where("status = ?", filter.Status)
	}

	if filter.Search != "" {
		searchQuery := "%" + filter.Search + "%"
		query = query.Where("title ILIKE ?", searchQuery)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Sorting
	switch filter.SortBy {
	case "created_desc":
		query = query.Order("created_at DESC")
	case "created_asc":
		query = query.Order("created_at ASC")
	case "title_asc":
		query = query.Order("title ASC")
	case "title_desc":
		query = query.Order("title DESC")
	default:
		query = query.Order("created_at DESC")
	}

	if err := query.Scopes(pagination.Paginate(&filter)).Find(&plans).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Load relations for each plan
	for _, plan := range plans {
		r.loadRelations(ctx, plan)
	}

	return plans, total, nil
}

// GetByPatient retrieves all session plans for a patient
func (r *sessionPlanRepository) GetByPatient(ctx context.Context, clinicID, patientID uuid.UUID) ([]*model.SessionPlan, error) {
	var plans []*model.SessionPlan
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND patient_id = ?", clinicID, patientID).
		Order("created_at DESC").
		Find(&plans).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	for _, plan := range plans {
		r.loadRelations(ctx, plan)
	}

	return plans, nil
}

// GetByProfessional retrieves all session plans for a professional
func (r *sessionPlanRepository) GetByProfessional(ctx context.Context, clinicID, professionalID uuid.UUID) ([]*model.SessionPlan, error) {
	var plans []*model.SessionPlan
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND professional_id = ?", clinicID, professionalID).
		Order("created_at DESC").
		Find(&plans).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	for _, plan := range plans {
		r.loadRelations(ctx, plan)
	}

	return plans, nil
}

// GetActivePlansForGeneration retrieves all active plans that need appointment generation
func (r *sessionPlanRepository) GetActivePlansForGeneration(ctx context.Context, generateBefore time.Time) ([]*model.SessionPlan, error) {
	var plans []*model.SessionPlan
	err := r.db.WithContext(ctx).
		Where("status = ?", model.PlanStatusActive).
		Where("sessions_done < total_sessions").
		Where("generate_until <= ? OR last_generated_at IS NULL OR last_generated_at < ?", generateBefore, generateBefore.AddDate(0, 0, -7)).
		Find(&plans).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return plans, nil
}

// Update updates a session plan
func (r *sessionPlanRepository) Update(ctx context.Context, plan *model.SessionPlan) error {
	if err := r.db.WithContext(ctx).Save(plan).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}

// UpdateStatus updates the status of a session plan
func (r *sessionPlanRepository) UpdateStatus(ctx context.Context, clinicID, id uuid.UUID, status model.PlanStatus) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	now := time.Now()
	switch status {
	case model.PlanStatusPaused:
		updates["paused_at"] = now
	case model.PlanStatusCompleted:
		updates["completed_at"] = now
	case model.PlanStatusCancelled:
		updates["cancelled_at"] = now
	case model.PlanStatusActive:
		updates["paused_at"] = nil
	}

	result := r.db.WithContext(ctx).
		Model(&model.SessionPlan{}).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Updates(updates)

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.ErrNotFoundInstance.WithMessage("Session plan not found")
	}

	return nil
}

// IncrementSessionsDone increments the sessions_done counter
func (r *sessionPlanRepository) IncrementSessionsDone(ctx context.Context, clinicID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.SessionPlan{}).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Update("sessions_done", gorm.Expr("sessions_done + 1"))

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}

	// Check if plan is now complete (only for determinate plans)
	var plan model.SessionPlan
	if err := r.db.WithContext(ctx).Where("clinic_id = ? AND id = ?", clinicID, id).First(&plan).Error; err == nil {
		if !plan.IsIndeterminate() && plan.SessionsDone >= plan.TotalSessions {
			now := time.Now()
			r.db.WithContext(ctx).
				Model(&model.SessionPlan{}).
				Where("clinic_id = ? AND id = ?", clinicID, id).
				Updates(map[string]any{
					"status":       model.PlanStatusCompleted,
					"completed_at": now,
				})
		}
	}

	return nil
}

// UpdateLastGenerated updates the last_generated_at timestamp
func (r *sessionPlanRepository) UpdateLastGenerated(ctx context.Context, clinicID, id uuid.UUID, lastGenerated time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.SessionPlan{}).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Update("last_generated_at", lastGenerated).Error
}

// Delete soft deletes a session plan
func (r *sessionPlanRepository) Delete(ctx context.Context, clinicID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND id = ?", clinicID, id).
		Delete(&model.SessionPlan{})

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.ErrNotFoundInstance.WithMessage("Session plan not found")
	}

	return nil
}

// loadRelations loads patient and professional names
func (r *sessionPlanRepository) loadRelations(ctx context.Context, plan *model.SessionPlan) {
	// Load patient name
	var patientName string
	r.db.WithContext(ctx).
		Table("patients").
		Select("name").
		Where("id = ?", plan.PatientID).
		Scan(&patientName)
	plan.PatientName = patientName

	// Load professional name
	var professionalName string
	r.db.WithContext(ctx).
		Table("professionals").
		Select("name").
		Where("id = ?", plan.ProfessionalID).
		Scan(&professionalName)
	plan.ProfessionalName = professionalName

	// Load procedure name
	if plan.ProcedureID != nil {
		var procedureName string
		r.db.WithContext(ctx).
			Table("procedures").
			Select("name").
			Where("id = ?", plan.ProcedureID).
			Scan(&procedureName)
		plan.ProcedureName = procedureName
	}
}

// GetAllActive retrieves all active session plans for a clinic
func (r *sessionPlanRepository) GetAllActive(ctx context.Context, clinicID uuid.UUID) ([]*model.SessionPlan, error) {
	var plans []*model.SessionPlan
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND status = ?", clinicID, model.PlanStatusActive).
		Order("created_at DESC").
		Find(&plans).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	for _, plan := range plans {
		r.loadRelations(ctx, plan)
	}

	return plans, nil
}

// BulkCancel cancels multiple session plans
func (r *sessionPlanRepository) BulkCancel(ctx context.Context, clinicID uuid.UUID, ids []uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Model(&model.SessionPlan{}).
		Where("clinic_id = ? AND id IN ? AND status NOT IN ?", clinicID, ids, []model.PlanStatus{model.PlanStatusCancelled, model.PlanStatusCompleted}).
		Update("status", model.PlanStatusCancelled).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}
