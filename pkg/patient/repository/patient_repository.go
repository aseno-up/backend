package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/patient/model"
)

// PatientRepository defines the interface for patient data operations
type PatientRepository interface {
	Create(ctx context.Context, patient *model.Patient) error
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Patient, error)
	List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Patient, int64, error)
	Update(ctx context.Context, patient *model.Patient) error
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	BulkDelete(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error
	GetByPhone(ctx context.Context, phone string, clinicID uuid.UUID) (*model.Patient, error)
	GetByDocument(ctx context.Context, document string, clinicID uuid.UUID) (*model.Patient, error)
	// Risk assessment methods
	GetAppointmentStats(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientAppointmentStats, error)
	UpdateRiskScore(ctx context.Context, patientID, clinicID uuid.UUID, risk model.NoShowRisk, stats *model.PatientAppointmentStats) error
	GetAllByClinicID(ctx context.Context, clinicID uuid.UUID) ([]*model.Patient, error)
	ListBySource(ctx context.Context, clinicID uuid.UUID, source model.PatientSource, filter pagination.Filter) ([]*model.Patient, int64, error)
	MergePatients(ctx context.Context, clinicID, targetID, sourceID uuid.UUID) error
}

// patientRepository implements PatientRepository
type patientRepository struct {
	db *gorm.DB
}

// NewPatientRepository creates a new patient repository
func NewPatientRepository(db *gorm.DB) PatientRepository {
	return &patientRepository{db: db}
}

// Create creates a new patient
func (r *patientRepository) Create(ctx context.Context, patient *model.Patient) error {
	result := r.db.WithContext(ctx).Create(patient)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByID retrieves a patient by ID within a clinic
func (r *patientRepository) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.Patient, error) {
	var patient model.Patient
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", id, clinicID).
		First(&patient)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPatientNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &patient, nil
}

// List retrieves and filters patients belonging to a clinic
func (r *patientRepository) List(ctx context.Context, clinicID uuid.UUID, filter pagination.Filter) ([]*model.Patient, int64, error) {
	var patients []*model.Patient
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Patient{}).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID)

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("name ILIKE ? OR phone ILIKE ? OR email ILIKE ? OR whatsapp ILIKE ? OR document ILIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	switch filter.Status {
	case "complete":
		query = query.Where("email != '' AND phone != '' AND document != ''")
	case "incomplete":
		query = query.Where("email = '' OR phone = '' OR document = ''")
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(err)
	}

	// Determine sort order
	order := "name ASC"
	switch filter.SortBy {
	case "name_desc":
		order = "name DESC"
	case "created_asc":
		order = "created_at ASC"
	case "created_desc":
		order = "created_at DESC"
	case "risk_desc":
		order = "no_show_risk DESC"
	}

	// Get paginated results
	result := query.
		Order(order).
		Scopes(pagination.Paginate(&filter)).
		Find(&patients)

	if result.Error != nil {
		return nil, 0, errors.ErrInternalInstance.WithError(result.Error)
	}

	return patients, total, nil
}

// Update updates a patient
func (r *patientRepository) Update(ctx context.Context, patient *model.Patient) error {
	result := r.db.WithContext(ctx).Save(patient)
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// Delete soft-deletes a patient
func (r *patientRepository) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND clinic_id = ?", id, clinicID).
		Delete(&model.Patient{})
	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrPatientNotFoundInstance
	}
	return nil
}

// BulkDelete soft deletes multiple patients belonging to a clinic
func (r *patientRepository) BulkDelete(ctx context.Context, ids []uuid.UUID, clinicID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id IN ? AND clinic_id = ?", ids, clinicID).
		Delete(&model.Patient{})

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	return nil
}

// GetByPhone retrieves a patient by phone number within a clinic
func (r *patientRepository) GetByPhone(ctx context.Context, phone string, clinicID uuid.UUID) (*model.Patient, error) {
	var patient model.Patient
	result := r.db.WithContext(ctx).
		Where("(phone = ? OR whatsapp = ?) AND clinic_id = ? AND deleted_at IS NULL", phone, phone, clinicID).
		First(&patient)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPatientNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &patient, nil
}

// GetByDocument retrieves a patient by document (CPF)
func (r *patientRepository) GetByDocument(ctx context.Context, document string, clinicID uuid.UUID) (*model.Patient, error) {
	var patient model.Patient
	result := r.db.WithContext(ctx).
		Where("document = ? AND clinic_id = ? AND deleted_at IS NULL", document, clinicID).
		First(&patient)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.ErrPatientNotFoundInstance
		}
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}
	return &patient, nil
}

// GetAppointmentStats retrieves appointment statistics for risk calculation
func (r *patientRepository) GetAppointmentStats(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientAppointmentStats, error) {
	var stats model.PatientAppointmentStats

	err := r.db.WithContext(ctx).Table("appointments").
		Select(`
			COUNT(*) as total_appointments,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_count,
			COUNT(CASE WHEN status = 'no_show' THEN 1 END) as no_show_count
		`).
		Where("patient_id = ? AND clinic_id = ? AND deleted_at IS NULL", patientID, clinicID).
		Scan(&stats).Error

	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return &stats, nil
}

// UpdateRiskScore updates the patient's risk score and statistics
func (r *patientRepository) UpdateRiskScore(ctx context.Context, patientID, clinicID uuid.UUID, risk model.NoShowRisk, stats *model.PatientAppointmentStats) error {
	attendanceRate := 0.0
	attended := stats.CompletedCount
	total := stats.CompletedCount + stats.NoShowCount
	if total > 0 {
		attendanceRate = (float64(attended) / float64(total)) * 100
	}

	result := r.db.WithContext(ctx).
		Model(&model.Patient{}).
		Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", patientID, clinicID).
		Updates(map[string]interface{}{
			"no_show_risk":       risk,
			"total_appointments": stats.TotalAppointments,
			"no_show_count":      stats.NoShowCount,
			"attendance_rate":    attendanceRate,
			"risk_updated_at":    gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		return errors.ErrInternalInstance.WithError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrPatientNotFoundInstance
	}
	return nil
}

// GetAllByClinicID retrieves all patients for a clinic (for batch recalculation)
func (r *patientRepository) GetAllByClinicID(ctx context.Context, clinicID uuid.UUID) ([]*model.Patient, error) {
	var patients []*model.Patient

	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND deleted_at IS NULL", clinicID).
		Find(&patients)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}

	return patients, nil
}

// ListBySource returns patients filtered by source (e.g. SELF_REGISTERED)
func (r *patientRepository) ListBySource(ctx context.Context, clinicID uuid.UUID, source model.PatientSource, filter pagination.Filter) ([]*model.Patient, int64, error) {
	var patients []*model.Patient
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Patient{}).Where("clinic_id = ? AND source = ?", clinicID, source)

	if filter.Search != "" {
		searchQuery := "%" + filter.Search + "%"
		query = query.Where("(name ILIKE ? OR phone ILIKE ? OR email ILIKE ?)", searchQuery, searchQuery, searchQuery)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = query.Order("created_at DESC")
	if err := query.Scopes(pagination.Paginate(&filter)).Find(&patients).Error; err != nil {
		return nil, 0, err
	}

	return patients, total, nil
}

// MergePatients transfers all appointments from sourceID to targetID and soft-deletes sourceID
func (r *patientRepository) MergePatients(ctx context.Context, clinicID, targetID, sourceID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Verify both patients belong to the clinic
		var count int64
		tx.Model(&model.Patient{}).Where("id IN ? AND clinic_id = ? AND deleted_at IS NULL", []uuid.UUID{targetID, sourceID}, clinicID).Count(&count)
		if count != 2 {
			return errors.ErrNotFoundInstance.WithMessage("Paciente não encontrado")
		}

		// Transfer appointments
		if err := tx.Exec("UPDATE appointments SET patient_id = ? WHERE patient_id = ? AND clinic_id = ?", targetID, sourceID, clinicID).Error; err != nil {
			return err
		}

		// Copy missing contact data from source to target
		var source, target model.Patient
		tx.Where("id = ? AND clinic_id = ?", sourceID, clinicID).First(&source)
		tx.Where("id = ? AND clinic_id = ?", targetID, clinicID).First(&target)

		updates := map[string]interface{}{}
		if target.Email == "" && source.Email != "" {
			updates["email"] = source.Email
		}
		if target.Phone == "" && source.Phone != "" {
			updates["phone"] = source.Phone
		}
		if target.WhatsApp == "" && source.WhatsApp != "" {
			updates["whatsapp"] = source.WhatsApp
		}
		if len(updates) > 0 {
			tx.Model(&model.Patient{}).Where("id = ? AND clinic_id = ?", targetID, clinicID).Updates(updates)
		}

		// Soft-delete source patient
		if err := tx.Where("id = ? AND clinic_id = ?", sourceID, clinicID).Delete(&model.Patient{}).Error; err != nil {
			return err
		}

		return nil
	})
}
