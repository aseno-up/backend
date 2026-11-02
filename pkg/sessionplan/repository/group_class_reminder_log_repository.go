package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/sessionplan/model"
)

// GroupClassReminderLogRepository persists the dedup record for class
// reminders. The worker calls HasBeenSent before enqueueing and MarkSent
// after; the unique index on (clinic, plan, patient, date, type) is the
// safety net so concurrent workers can't both send the same reminder.
type GroupClassReminderLogRepository interface {
	HasBeenSent(ctx context.Context, clinicID, planID, patientID uuid.UUID, sessionDate, reminderType string) (bool, error)
	MarkSent(ctx context.Context, clinicID, planID, patientID uuid.UUID, sessionDate, reminderType string) error
}

type groupClassReminderLogRepository struct {
	db *gorm.DB
}

// NewGroupClassReminderLogRepository creates a new repository.
func NewGroupClassReminderLogRepository(db *gorm.DB) GroupClassReminderLogRepository {
	return &groupClassReminderLogRepository{db: db}
}

func (r *groupClassReminderLogRepository) HasBeenSent(
	ctx context.Context,
	clinicID, planID, patientID uuid.UUID,
	sessionDate, reminderType string,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.GroupClassReminderLog{}).
		Where("clinic_id = ? AND session_plan_id = ? AND patient_id = ? AND session_date = ? AND reminder_type = ?",
			clinicID, planID, patientID, sessionDate, reminderType).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MarkSent inserts a row. A unique-violation collision (because another
// worker tick raced ahead) is treated as success — the goal is "exactly
// once" enqueue, and a duplicate log row would mean the reminder went out
// twice anyway. We swallow the duplicate error so the worker doesn't
// resurface the same candidate forever.
func (r *groupClassReminderLogRepository) MarkSent(
	ctx context.Context,
	clinicID, planID, patientID uuid.UUID,
	sessionDate, reminderType string,
) error {
	row := &model.GroupClassReminderLog{
		ClinicID:      clinicID,
		SessionPlanID: planID,
		PatientID:     patientID,
		SessionDate:   sessionDate,
		ReminderType:  reminderType,
	}
	err := r.db.WithContext(ctx).Create(row).Error
	if err != nil && isUniqueViolation(err) {
		return nil
	}
	return err
}

// isUniqueViolation matches the Postgres SQLSTATE 23505 surfaced by
// pq/pgx through gorm. We only need a substring check — this is a single
// constraint on a small table and we never want to misclassify a real
// error as a dedup hit.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "idx_group_class_reminder_log_unique")
}
