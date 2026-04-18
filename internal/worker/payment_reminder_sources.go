package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
	sessionplan_repository "github.com/aseno/backend/pkg/sessionplan/repository"
)

// gormClinicSource lists every clinic the worker might need to process. The
// worker only scans clinics that have "Lembretes" enabled, but the filter is
// applied in-memory because the setting lives inside a jsonb column and the
// data volume is small (one row per tenant).
type gormClinicSource struct {
	db *gorm.DB
}

// NewGormClinicSource adapts the shared DB into the narrow interface the
// payment reminder task expects.
func NewGormClinicSource(db *gorm.DB) paymentReminderClinicSource {
	return &gormClinicSource{db: db}
}

func (s *gormClinicSource) ListActiveClinics(ctx context.Context) ([]*clinic_model.Clinic, error) {
	var clinics []*clinic_model.Clinic
	err := s.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Find(&clinics).Error
	if err != nil {
		return nil, err
	}
	return clinics, nil
}

// gormCandidateSource builds the list of candidates (individual monthly plans
// + group monthly enrollments) via SQL. The query intentionally filters only
// on columns that are indexed or selective — active status, monthly payment
// mode, billing_day not null — so it stays cheap even as the tables grow.
type gormCandidateSource struct {
	db *gorm.DB
}

// NewGormCandidateSource builds a candidate source backed by GORM.
func NewGormCandidateSource(db *gorm.DB) paymentReminderCandidateSource {
	return &gormCandidateSource{db: db}
}

// candidateRow mirrors the SELECT columns so GORM can scan straight into it.
type candidateRow struct {
	ClinicID         uuid.UUID  `gorm:"column:clinic_id"`
	SessionPlanID    uuid.UUID  `gorm:"column:session_plan_id"`
	EnrollmentID     *uuid.UUID `gorm:"column:enrollment_id"`
	PatientID        uuid.UUID  `gorm:"column:patient_id"`
	PatientName      string     `gorm:"column:patient_name"`
	PatientWhatsApp  string     `gorm:"column:patient_whatsapp"`
	PatientPhone     string     `gorm:"column:patient_phone"`
	PatientEmail     string     `gorm:"column:patient_email"`
	PlanTitle        string     `gorm:"column:plan_title"`
	BillingDay       int        `gorm:"column:billing_day"`
	Amount           float64    `gorm:"column:amount"`
	WhatsAppInstance string     `gorm:"column:whatsapp_instance"`
	EnrolledAt       time.Time  `gorm:"column:enrolled_at"`
}

func (s *gormCandidateSource) ListPaymentReminderCandidates(ctx context.Context, clinicID uuid.UUID) ([]*PaymentReminderCandidate, error) {
	// The UNION combines two candidate sources:
	//   * individual monthly session plans (enrollment_id = NULL)
	//   * active enrollments in group monthly plans
	// Both are joined to patients and to the clinic's active WhatsApp
	// instance. Patients without a phone/whatsapp are kept in the result so
	// the in-memory layer can still try email — filtering them out here
	// would couple channel policy to the repository.
	query := `
		WITH wa AS (
			SELECT instance FROM whatsapp_connections
			WHERE clinic_id = ? AND is_active = true AND connected = true AND deleted_at IS NULL
			LIMIT 1
		)
		SELECT
			sp.clinic_id                             AS clinic_id,
			sp.id                                    AS session_plan_id,
			NULL::uuid                               AS enrollment_id,
			sp.patient_id                            AS patient_id,
			p.name                                   AS patient_name,
			p.whatsapp                               AS patient_whatsapp,
			p.phone                                  AS patient_phone,
			p.email                                  AS patient_email,
			sp.title                                 AS plan_title,
			sp.billing_day                           AS billing_day,
			COALESCE(sp.price_override, sp.session_price, 0) AS amount,
			COALESCE((SELECT instance FROM wa), '')  AS whatsapp_instance,
			sp.created_at                            AS enrolled_at
		FROM session_plans sp
		INNER JOIN patients p ON p.id = sp.patient_id AND p.deleted_at IS NULL
		WHERE sp.clinic_id = ?
		  AND sp.deleted_at IS NULL
		  AND sp.status = 'active'
		  AND sp.payment_mode = 'monthly'
		  AND sp.type = 'individual'
		  AND sp.billing_day IS NOT NULL
		UNION ALL
		SELECT
			spe.clinic_id                            AS clinic_id,
			spe.session_plan_id                      AS session_plan_id,
			spe.id                                   AS enrollment_id,
			spe.patient_id                           AS patient_id,
			p.name                                   AS patient_name,
			p.whatsapp                               AS patient_whatsapp,
			p.phone                                  AS patient_phone,
			p.email                                  AS patient_email,
			sp.title                                 AS plan_title,
			spe.billing_day                          AS billing_day,
			COALESCE(spe.monthly_amount, 0)          AS amount,
			COALESCE((SELECT instance FROM wa), '')  AS whatsapp_instance,
			spe.joined_at                            AS enrolled_at
		FROM session_plan_enrollments spe
		INNER JOIN session_plans sp ON sp.id = spe.session_plan_id AND sp.deleted_at IS NULL
		INNER JOIN patients p ON p.id = spe.patient_id AND p.deleted_at IS NULL
		WHERE spe.clinic_id = ?
		  AND spe.deleted_at IS NULL
		  AND spe.status = 'active'
		  AND spe.payment_mode = 'monthly'
		  AND spe.billing_day IS NOT NULL
	`

	var rows []candidateRow
	if err := s.db.WithContext(ctx).Raw(query, clinicID, clinicID, clinicID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]*PaymentReminderCandidate, 0, len(rows))
	for _, r := range rows {
		phone := r.PatientWhatsApp
		if phone == "" {
			phone = r.PatientPhone
		}
		out = append(out, &PaymentReminderCandidate{
			ClinicID:         r.ClinicID,
			SessionPlanID:    r.SessionPlanID,
			EnrollmentID:     r.EnrollmentID,
			PatientID:        r.PatientID,
			PatientName:      r.PatientName,
			PatientWhatsApp:  phone,
			PatientEmail:     r.PatientEmail,
			PlanTitle:        r.PlanTitle,
			BillingDay:       r.BillingDay,
			Amount:           r.Amount,
			WhatsAppInstance: r.WhatsAppInstance,
			EnrolledAt:       r.EnrolledAt,
		})
	}
	return out, nil
}

// gormTracker wires the "has this been paid / has this been reminded"
// lookups to real tables. The reminder repository already exposes both
// Exists and Create.
type gormTracker struct {
	db           *gorm.DB
	reminderRepo sessionplan_repository.PaymentReminderRepository
}

// NewGormTracker adapts the reminder repository into the task's tracker
// interface and adds the "is there a session_plan_payment row for this
// reference month?" lookup that the repository doesn't expose.
func NewGormTracker(db *gorm.DB, reminderRepo sessionplan_repository.PaymentReminderRepository) paymentReminderTracker {
	return &gormTracker{db: db, reminderRepo: reminderRepo}
}

func (g *gormTracker) PaymentExists(ctx context.Context, clinicID, sessionPlanID uuid.UUID, enrollmentID *uuid.UUID, referenceMonth string) (bool, error) {
	q := g.db.WithContext(ctx).
		Model(&sessionplan_model.SessionPlanPayment{}).
		Where("clinic_id = ? AND session_plan_id = ? AND reference_month = ? AND deleted_at IS NULL", clinicID, sessionPlanID, referenceMonth)

	if enrollmentID == nil {
		q = q.Where("enrollment_id IS NULL")
	} else {
		q = q.Where("enrollment_id = ?", *enrollmentID)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (g *gormTracker) ReminderExists(ctx context.Context, clinicID, sessionPlanID uuid.UUID, enrollmentID *uuid.UUID, referenceMonth string, channel sessionplan_model.PaymentReminderChannel) (bool, error) {
	return g.reminderRepo.Exists(ctx, clinicID, sessionPlanID, enrollmentID, referenceMonth, channel)
}

func (g *gormTracker) RecordReminder(ctx context.Context, reminder *sessionplan_model.SessionPlanPaymentReminder) error {
	return g.reminderRepo.Create(ctx, reminder)
}
