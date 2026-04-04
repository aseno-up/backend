package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/patient/model"
)

// GetUnlinkedProvisionalPatients returns self-registered patients that have not been merged yet.
func (r *patientRepository) GetUnlinkedProvisionalPatients(ctx context.Context, clinicID uuid.UUID) ([]*model.Patient, error) {
	var patients []*model.Patient

	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND source = ? AND merged_into_patient_id IS NULL AND deleted_at IS NULL", clinicID, model.PatientSourceSelfRegistered).
		Order("created_at DESC").
		Find(&patients)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}

	return patients, nil
}

// SearchPatientsExcludingProvisional searches patients by name, CPF, email or phone within the clinic.
// Returns all matching patients (including SELF_REGISTERED). Excludes soft-deleted.
// Returns max `limit` results.
func (r *patientRepository) SearchPatientsExcludingProvisional(ctx context.Context, clinicID uuid.UUID, query string, limit int) ([]*model.Patient, error) {
	if limit <= 0 {
		limit = 10
	}

	var patients []*model.Patient

	searchPattern := "%" + query + "%"

	result := r.db.WithContext(ctx).
		Where("clinic_id = ? AND deleted_at IS NULL AND merged_into_patient_id IS NULL", clinicID).
		Where("name ILIKE ? OR document ILIKE ? OR email ILIKE ? OR phone ILIKE ?", searchPattern, searchPattern, searchPattern, searchPattern).
		Order("name ASC").
		Limit(limit).
		Find(&patients)

	if result.Error != nil {
		return nil, errors.ErrInternalInstance.WithError(result.Error)
	}

	return patients, nil
}

// MergePatientsBulk merges multiple provisional (self-registered) patients into a target patient
// in a single transaction. Returns total number of rows reassigned across all tables.
func (r *patientRepository) MergePatientsBulk(ctx context.Context, clinicID, targetID uuid.UUID, provisionalIDs []uuid.UUID, actorID uuid.UUID) (int64, error) {
	if len(provisionalIDs) == 0 {
		return 0, errors.ErrValidationInstance.WithMessage("provisional_ids cannot be empty")
	}

	var totalReassigned int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Validate all provisional_ids belong to the clinic and are SELF_REGISTERED + not yet merged
		var provisionalCount int64
		if err := tx.Model(&model.Patient{}).
			Where("id IN ? AND clinic_id = ? AND source = ? AND merged_into_patient_id IS NULL AND deleted_at IS NULL",
				provisionalIDs, clinicID, model.PatientSourceSelfRegistered).
			Count(&provisionalCount).Error; err != nil {
			return errors.ErrInternalInstance.WithError(err)
		}

		if int(provisionalCount) != len(provisionalIDs) {
			// Distinguish 409 (already merged) from 404 (not found)
			var alreadyMergedCount int64
			tx.Model(&model.Patient{}).Unscoped().
				Where("id IN ? AND clinic_id = ? AND merged_into_patient_id IS NOT NULL", provisionalIDs, clinicID).
				Count(&alreadyMergedCount)

			if alreadyMergedCount > 0 {
				return errors.ErrConflictInstance.WithMessage("Um ou mais pacientes provisórios já foram mesclados em outro paciente")
			}
			return errors.ErrNotFoundInstance.WithMessage("Um ou mais pacientes provisórios não encontrados nesta clínica")
		}

		// 2. Validate target belongs to the clinic and is NOT a provisional patient
		var targetPatient model.Patient
		if err := tx.Where("id = ? AND clinic_id = ? AND deleted_at IS NULL", targetID, clinicID).
			First(&targetPatient).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.ErrNotFoundInstance.WithMessage("Paciente alvo não encontrado nesta clínica")
			}
			return errors.ErrInternalInstance.WithError(err)
		}

		// Target cannot already be merged into someone else
		if targetPatient.MergedIntoPatientID != nil {
			return errors.ErrConflictInstance.WithMessage("Paciente alvo já foi mesclado em outro paciente")
		}

		// 3. Reassign patient_id in ALL tables that reference patients(id)
		tablesToUpdate := []string{
			"appointments",
			"session_notes",
			"clinic_payments",
			"patient_records",
			"conversation_contexts",
			"message_logs",
			"scheduling_intents",
			"session_plans",
			"waitlist_entries",
			"whatsapp_messages",
		}

		for _, table := range tablesToUpdate {
			query := fmt.Sprintf("UPDATE %s SET patient_id = ? WHERE patient_id IN ? AND clinic_id = ?", table)
			result := tx.Exec(query, targetID, provisionalIDs, clinicID)
			if result.Error != nil {
				return errors.ErrInternalInstance.WithError(result.Error)
			}
			totalReassigned += result.RowsAffected
		}

		// 4. Transfer portal_accounts: point them at the target patient
		if err := tx.Exec(
			"UPDATE portal_accounts SET patient_id = ? WHERE patient_id IN ? AND clinic_id = ?",
			targetID, provisionalIDs, clinicID,
		).Error; err != nil {
			return errors.ErrInternalInstance.WithError(err)
		}

		// 5. Soft-delete provisional patients and mark them as merged
		if err := tx.Model(&model.Patient{}).
			Where("id IN ? AND clinic_id = ?", provisionalIDs, clinicID).
			Updates(map[string]interface{}{
				"merged_into_patient_id": targetID,
				"deleted_at":             gorm.Expr("NOW()"),
			}).Error; err != nil {
			return errors.ErrInternalInstance.WithError(err)
		}

		// 6. Audit log
		newData, _ := json.Marshal(map[string]interface{}{
			"provisional_ids":    provisionalIDs,
			"target_patient_id":  targetID,
			"rows_reassigned":    totalReassigned,
		})

		if err := tx.Exec(
			`INSERT INTO audit_logs (id, clinic_id, user_id, action, entity_type, entity_id, new_data, created_at)
			 VALUES (?, ?, ?, 'UPDATE', 'PATIENT', ?, ?::jsonb, NOW())`,
			uuid.New(), clinicID, actorID, targetID.String(), string(newData),
		).Error; err != nil {
			fmt.Printf("Failed to create audit log: %v\n", err)
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return totalReassigned, nil
}
