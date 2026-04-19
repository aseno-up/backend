package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/sessionplan/model"
)

// PaymentReminderRepository defines persistence operations for monthly payment
// reminders. The worker consults Exists before enqueuing a message and calls
// Create after a successful dispatch, so the same patient/month/channel never
// receives more than one reminder.
type PaymentReminderRepository interface {
	Exists(ctx context.Context, clinicID, sessionPlanID uuid.UUID, enrollmentID *uuid.UUID, referenceMonth string, channel model.PaymentReminderChannel) (bool, error)
	Create(ctx context.Context, reminder *model.SessionPlanPaymentReminder) error
}

type paymentReminderRepository struct {
	db *gorm.DB
}

// NewPaymentReminderRepository creates a new payment reminder repository
func NewPaymentReminderRepository(db *gorm.DB) PaymentReminderRepository {
	return &paymentReminderRepository{db: db}
}

func (r *paymentReminderRepository) Exists(ctx context.Context, clinicID, sessionPlanID uuid.UUID, enrollmentID *uuid.UUID, referenceMonth string, channel model.PaymentReminderChannel) (bool, error) {
	q := r.db.WithContext(ctx).
		Model(&model.SessionPlanPaymentReminder{}).
		Where("clinic_id = ? AND session_plan_id = ? AND reference_month = ? AND channel = ?", clinicID, sessionPlanID, referenceMonth, channel)

	// enrollment_id is nullable: treat NULL and concrete values as distinct to
	// separate individual-plan reminders from group-enrollment reminders on
	// the same plan.
	if enrollmentID == nil {
		q = q.Where("enrollment_id IS NULL")
	} else {
		q = q.Where("enrollment_id = ?", *enrollmentID)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, errors.ErrInternalInstance.WithError(err)
	}
	return count > 0, nil
}

func (r *paymentReminderRepository) Create(ctx context.Context, reminder *model.SessionPlanPaymentReminder) error {
	if err := r.db.WithContext(ctx).Create(reminder).Error; err != nil {
		return errors.ErrInternalInstance.WithError(err)
	}
	return nil
}
