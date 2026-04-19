package worker

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupCandidateTestDB builds an in-memory SQLite database with the minimal
// schema the candidate source query touches. Columns match the production
// model exactly so the query works without alteration.
func setupCandidateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	stmts := []string{
		`CREATE TABLE patients (
			id text PRIMARY KEY,
			clinic_id text,
			name text,
			whatsapp text,
			phone text,
			email text,
			deleted_at datetime
		)`,
		`CREATE TABLE session_plans (
			id text PRIMARY KEY,
			clinic_id text,
			patient_id text,
			title text,
			status text,
			type text,
			payment_mode text,
			billing_day integer,
			session_price real,
			price_override real,
			created_at datetime,
			deleted_at datetime
		)`,
		`CREATE TABLE session_plan_enrollments (
			id text PRIMARY KEY,
			clinic_id text,
			session_plan_id text,
			patient_id text,
			status text,
			payment_mode text,
			billing_day integer,
			monthly_amount real,
			joined_at datetime,
			left_at datetime,
			deleted_at datetime
		)`,
		`CREATE TABLE whatsapp_connections (
			id text PRIMARY KEY,
			clinic_id text,
			instance text,
			is_active integer,
			connected integer,
			deleted_at datetime
		)`,
	}
	for _, s := range stmts {
		require.NoError(t, db.Exec(s).Error)
	}
	return db
}

type testPatient struct {
	id       uuid.UUID
	name     string
	whatsapp string
	email    string
}

func insertPatient(t *testing.T, db *gorm.DB, clinicID uuid.UUID, p testPatient) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO patients (id, clinic_id, name, whatsapp, phone, email) VALUES (?, ?, ?, ?, '', ?)`,
		p.id.String(), clinicID.String(), p.name, p.whatsapp, p.email,
	).Error)
}

type testPlan struct {
	id          uuid.UUID
	patientID   uuid.UUID
	title       string
	status      string // active | cancelled | paused | completed
	typ         string // individual | group
	paymentMode string // monthly | per_session | upfront
	billingDay  *int
	price       float64
	createdAt   time.Time
}

func insertPlan(t *testing.T, db *gorm.DB, clinicID uuid.UUID, p testPlan) {
	t.Helper()
	var bd any
	if p.billingDay != nil {
		bd = *p.billingDay
	}
	require.NoError(t, db.Exec(
		`INSERT INTO session_plans (id, clinic_id, patient_id, title, status, type, payment_mode, billing_day, session_price, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.id.String(), clinicID.String(), p.patientID.String(), p.title, p.status, p.typ, p.paymentMode, bd, p.price, p.createdAt,
	).Error)
}

type testEnrollment struct {
	id            uuid.UUID
	sessionPlanID uuid.UUID
	patientID     uuid.UUID
	status        string // active | cancelled | suspended
	paymentMode   string
	billingDay    *int
	monthlyAmount float64
	joinedAt      time.Time
	leftAt        *time.Time
}

func insertEnrollment(t *testing.T, db *gorm.DB, clinicID uuid.UUID, e testEnrollment) {
	t.Helper()
	var bd any
	if e.billingDay != nil {
		bd = *e.billingDay
	}
	var la any
	if e.leftAt != nil {
		la = *e.leftAt
	}
	require.NoError(t, db.Exec(
		`INSERT INTO session_plan_enrollments (id, clinic_id, session_plan_id, patient_id, status, payment_mode, billing_day, monthly_amount, joined_at, left_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.id.String(), clinicID.String(), e.sessionPlanID.String(), e.patientID.String(), e.status, e.paymentMode, bd, e.monthlyAmount, e.joinedAt, la,
	).Error)
}

func intPtr(v int) *int { return &v }

// --- Tests ---

func TestListCandidates_IndividualActivePlan_Returned(t *testing.T) {
	db := setupCandidateTestDB(t)
	clinicID := uuid.New()

	p := testPatient{id: uuid.New(), name: "Ana", whatsapp: "5575999990000", email: "ana@example.com"}
	insertPatient(t, db, clinicID, p)
	insertPlan(t, db, clinicID, testPlan{
		id: uuid.New(), patientID: p.id, title: "Plano Individual",
		status: "active", typ: "individual", paymentMode: "monthly",
		billingDay: intPtr(10), price: 200, createdAt: time.Now().AddDate(0, -2, 0),
	})

	src := NewGormCandidateSource(db)
	got, err := src.ListPaymentReminderCandidates(context.Background(), clinicID)
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, "Ana", got[0].PatientName)
	assert.Nil(t, got[0].EnrollmentID)
	assert.Equal(t, 10, got[0].BillingDay)
	assert.InDelta(t, 200.0, got[0].Amount, 0.001)
}

func TestListCandidates_IndividualCancelledPlan_NotReturned(t *testing.T) {
	db := setupCandidateTestDB(t)
	clinicID := uuid.New()

	p := testPatient{id: uuid.New(), name: "Bia", whatsapp: "5575999990001"}
	insertPatient(t, db, clinicID, p)
	insertPlan(t, db, clinicID, testPlan{
		id: uuid.New(), patientID: p.id, title: "Plano Cancelado",
		status: "cancelled", typ: "individual", paymentMode: "monthly",
		billingDay: intPtr(10), price: 200, createdAt: time.Now().AddDate(0, -2, 0),
	})

	src := NewGormCandidateSource(db)
	got, err := src.ListPaymentReminderCandidates(context.Background(), clinicID)
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

func TestListCandidates_OtherClinic_NotReturned(t *testing.T) {
	db := setupCandidateTestDB(t)
	clinicA := uuid.New()
	clinicB := uuid.New()

	p := testPatient{id: uuid.New(), name: "Carol", whatsapp: "5575999990002"}
	insertPatient(t, db, clinicB, p)
	insertPlan(t, db, clinicB, testPlan{
		id: uuid.New(), patientID: p.id, title: "Outro tenant",
		status: "active", typ: "individual", paymentMode: "monthly",
		billingDay: intPtr(10), price: 200, createdAt: time.Now(),
	})

	src := NewGormCandidateSource(db)
	got, err := src.ListPaymentReminderCandidates(context.Background(), clinicA)
	require.NoError(t, err)
	assert.Len(t, got, 0, "candidate from another clinic must not leak across tenants")
}

func TestListCandidates_EnrollmentActive_ParentActive_Returned(t *testing.T) {
	db := setupCandidateTestDB(t)
	clinicID := uuid.New()

	p := testPatient{id: uuid.New(), name: "Dora", whatsapp: "5575999990003"}
	insertPatient(t, db, clinicID, p)
	planID := uuid.New()
	insertPlan(t, db, clinicID, testPlan{
		id: planID, patientID: uuid.Nil, title: "Turma Pilates",
		status: "active", typ: "group", paymentMode: "monthly",
		billingDay: nil, price: 0, createdAt: time.Now().AddDate(0, -3, 0),
	})
	insertEnrollment(t, db, clinicID, testEnrollment{
		id: uuid.New(), sessionPlanID: planID, patientID: p.id,
		status: "active", paymentMode: "monthly", billingDay: intPtr(15),
		monthlyAmount: 300, joinedAt: time.Now().AddDate(0, -2, 0),
	})

	src := NewGormCandidateSource(db)
	got, err := src.ListPaymentReminderCandidates(context.Background(), clinicID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.NotNil(t, got[0].EnrollmentID)
	assert.Equal(t, 15, got[0].BillingDay)
	assert.InDelta(t, 300.0, got[0].Amount, 0.001)
}

// BUG fix: an enrollment must not be reminded if the parent plan was cancelled.
// Currently the SQL joins session_plans without checking status, so a cancelled
// group plan still leaks its active enrollments into the candidate list.
func TestListCandidates_EnrollmentActive_ParentCancelled_NotReturned(t *testing.T) {
	db := setupCandidateTestDB(t)
	clinicID := uuid.New()

	p := testPatient{id: uuid.New(), name: "Elis", whatsapp: "5575999990004"}
	insertPatient(t, db, clinicID, p)
	planID := uuid.New()
	insertPlan(t, db, clinicID, testPlan{
		id: planID, patientID: uuid.Nil, title: "Turma Cancelada",
		status: "cancelled", typ: "group", paymentMode: "monthly",
		billingDay: nil, price: 0, createdAt: time.Now().AddDate(0, -3, 0),
	})
	insertEnrollment(t, db, clinicID, testEnrollment{
		id: uuid.New(), sessionPlanID: planID, patientID: p.id,
		status: "active", paymentMode: "monthly", billingDay: intPtr(15),
		monthlyAmount: 300, joinedAt: time.Now().AddDate(0, -2, 0),
	})

	src := NewGormCandidateSource(db)
	got, err := src.ListPaymentReminderCandidates(context.Background(), clinicID)
	require.NoError(t, err)
	assert.Len(t, got, 0, "enrollments whose parent plan is cancelled must not be reminded")
}

// BUG fix: an enrollment with left_at set means the patient left the group.
// They should not receive a payment reminder even if status was not flipped.
func TestListCandidates_EnrollmentLeftAtSet_NotReturned(t *testing.T) {
	db := setupCandidateTestDB(t)
	clinicID := uuid.New()

	p := testPatient{id: uuid.New(), name: "Fran", whatsapp: "5575999990005"}
	insertPatient(t, db, clinicID, p)
	planID := uuid.New()
	insertPlan(t, db, clinicID, testPlan{
		id: planID, patientID: uuid.Nil, title: "Turma Ativa",
		status: "active", typ: "group", paymentMode: "monthly",
		createdAt: time.Now().AddDate(0, -3, 0),
	})
	leftAt := time.Now().AddDate(0, 0, -7)
	insertEnrollment(t, db, clinicID, testEnrollment{
		id: uuid.New(), sessionPlanID: planID, patientID: p.id,
		status: "active", paymentMode: "monthly", billingDay: intPtr(15),
		monthlyAmount: 300, joinedAt: time.Now().AddDate(0, -2, 0),
		leftAt: &leftAt,
	})

	src := NewGormCandidateSource(db)
	got, err := src.ListPaymentReminderCandidates(context.Background(), clinicID)
	require.NoError(t, err)
	assert.Len(t, got, 0, "enrollment with left_at set must not be reminded")
}

// Parent plan paused has the same effect as cancelled — no billing during pause.
func TestListCandidates_EnrollmentActive_ParentPaused_NotReturned(t *testing.T) {
	db := setupCandidateTestDB(t)
	clinicID := uuid.New()

	p := testPatient{id: uuid.New(), name: "Gabi", whatsapp: "5575999990006"}
	insertPatient(t, db, clinicID, p)
	planID := uuid.New()
	insertPlan(t, db, clinicID, testPlan{
		id: planID, patientID: uuid.Nil, title: "Turma Pausada",
		status: "paused", typ: "group", paymentMode: "monthly",
		createdAt: time.Now().AddDate(0, -3, 0),
	})
	insertEnrollment(t, db, clinicID, testEnrollment{
		id: uuid.New(), sessionPlanID: planID, patientID: p.id,
		status: "active", paymentMode: "monthly", billingDay: intPtr(15),
		monthlyAmount: 300, joinedAt: time.Now().AddDate(0, -2, 0),
	})

	src := NewGormCandidateSource(db)
	got, err := src.ListPaymentReminderCandidates(context.Background(), clinicID)
	require.NoError(t, err)
	assert.Len(t, got, 0, "enrollments whose parent plan is paused must not be reminded")
}

func TestListCandidates_WhatsAppInstanceBoundToClinic(t *testing.T) {
	db := setupCandidateTestDB(t)
	clinicID := uuid.New()

	require.NoError(t, db.Exec(
		`INSERT INTO whatsapp_connections (id, clinic_id, instance, is_active, connected) VALUES (?, ?, ?, 1, 1)`,
		uuid.New().String(), clinicID.String(), "wa-instance-1",
	).Error)

	p := testPatient{id: uuid.New(), name: "Hana", whatsapp: "5575999990007"}
	insertPatient(t, db, clinicID, p)
	insertPlan(t, db, clinicID, testPlan{
		id: uuid.New(), patientID: p.id, title: "Individual",
		status: "active", typ: "individual", paymentMode: "monthly",
		billingDay: intPtr(10), price: 200, createdAt: time.Now().AddDate(0, -2, 0),
	})

	src := NewGormCandidateSource(db)
	got, err := src.ListPaymentReminderCandidates(context.Background(), clinicID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "wa-instance-1", got[0].WhatsAppInstance)
}
