package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/internal/config"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
)

// stubClassReminderSource returns precomputed candidates regardless of the
// requested window. Real implementation walks active group plans + slots +
// active enrollments — this stub keeps the worker tests focused on
// dedup/gating logic, not occurrence math.
type stubClassReminderSource struct {
	candidates []ClassReminderCandidate
	err        error
	calls      int
}

func (s *stubClassReminderSource) UpcomingClasses(_ context.Context, _, _ time.Time) ([]ClassReminderCandidate, error) {
	s.calls++
	return s.candidates, s.err
}

// stubClassReminderLog is the dedup table. Records inserted via MarkSent are
// consulted by HasBeenSent so a second worker tick is a no-op (idempotent
// reminders, preventing the "patient gets the same reminder 4x in 30 min"
// failure mode).
type stubClassReminderLog struct {
	sent map[string]bool
	err  error
}

func (s *stubClassReminderLog) HasBeenSent(_ context.Context, clinicID, planID, patientID uuid.UUID, sessionDate, reminderType string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if s.sent == nil {
		return false, nil
	}
	return s.sent[reminderLogKey(clinicID, planID, patientID, sessionDate, reminderType)], nil
}

func (s *stubClassReminderLog) MarkSent(_ context.Context, clinicID, planID, patientID uuid.UUID, sessionDate, reminderType string) error {
	if s.err != nil {
		return s.err
	}
	if s.sent == nil {
		s.sent = map[string]bool{}
	}
	s.sent[reminderLogKey(clinicID, planID, patientID, sessionDate, reminderType)] = true
	return nil
}

func reminderLogKey(clinicID, planID, patientID uuid.UUID, sessionDate, reminderType string) string {
	return clinicID.String() + "|" + planID.String() + "|" + patientID.String() + "|" + sessionDate + "|" + reminderType
}

// stubClassAttendanceChecker reports whether a (clinic, plan, patient, date)
// already has a group_session_attendance row. Any status counts — if the
// patient already engaged with this class (confirmed via AI, marked
// present/absent by operator), the worker should not re-prompt them.
type stubClassAttendanceChecker struct {
	exists map[string]bool
}

func (s *stubClassAttendanceChecker) HasAttendanceRow(_ context.Context, clinicID, planID, patientID uuid.UUID, sessionDate string) (bool, error) {
	if s.exists == nil {
		return false, nil
	}
	return s.exists[reminderLogKey(clinicID, planID, patientID, sessionDate, "")], nil
}

func sampleCandidate(clinicID uuid.UUID, settings clinic_model.ClinicSettings, startsIn time.Duration) ClassReminderCandidate {
	return ClassReminderCandidate{
		ClinicID:         clinicID,
		ClinicSettings:   settings,
		PlanID:           uuid.New(),
		PlanTitle:        "Pilates",
		PatientID:        uuid.New(),
		PatientName:      "Maria",
		PatientPhone:     "+5511988887777",
		WhatsAppInstance: "instance-1",
		SessionStart:     time.Now().Add(startsIn),
	}
}

func newClassReminderConfig() *config.WorkerConfig {
	return &config.WorkerConfig{
		MessageRetryMax: 3,
	}
}

// REMINDERS_ENABLED master flag is the kill switch — without it set to true
// at the clinic level, no class reminders go out regardless of per-threshold
// flags. Mirrors the master gate used for appointment reminders.
func enabledRemindersSettings() clinic_model.ClinicSettings {
	return clinic_model.ClinicSettings{WhatsAppRemindersEnabled: true}
}

func TestGroupClassReminderWorker_EnqueuesUpcoming24hClass(t *testing.T) {
	clinicID := uuid.New()
	src := &stubClassReminderSource{
		candidates: []ClassReminderCandidate{sampleCandidate(clinicID, enabledRemindersSettings(), 24*time.Hour)},
	}
	q := &stubQueue{}
	logRepo := &stubClassReminderLog{}
	att := &stubClassAttendanceChecker{}

	w := NewGroupClassReminderWorker(newClassReminderConfig(), src, logRepo, att, q)
	count, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name:     "reminder_24h",
		Type:     "reminder_24h",
		Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	require.NoError(t, err)
	assert.Equal(t, 1, count, "an upcoming class with active settings must enqueue exactly one reminder")
	require.Len(t, q.published, 1)
	assert.Equal(t, "reminder_24h", q.published[0].MessageType)
	assert.Empty(t, q.published[0].AppointmentID, "class reminders are not tied to a materialized appointment — group plans never materialize, so AppointmentID stays empty")
}

func TestGroupClassReminderWorker_DedupsViaLog(t *testing.T) {
	clinicID := uuid.New()
	c := sampleCandidate(clinicID, enabledRemindersSettings(), 24*time.Hour)
	src := &stubClassReminderSource{candidates: []ClassReminderCandidate{c}}
	q := &stubQueue{}
	logRepo := &stubClassReminderLog{}
	att := &stubClassAttendanceChecker{}

	w := NewGroupClassReminderWorker(newClassReminderConfig(), src, logRepo, att, q)

	// First tick — sends.
	_, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_24h", Type: "reminder_24h", Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	require.NoError(t, err)
	require.Len(t, q.published, 1, "first tick must enqueue")

	// Second tick — log already has the row; must not re-enqueue.
	count, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_24h", Type: "reminder_24h", Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	require.NoError(t, err)
	assert.Equal(t, 0, count, "second tick must dedup via the reminder log so the patient doesn't get the same reminder twice")
	assert.Len(t, q.published, 1, "queue length stays at 1 after the dedup")
}

func TestGroupClassReminderWorker_SkipsIfAttendanceAlreadyExists(t *testing.T) {
	// If the AI already wrote a confirmation row, OR the operator pre-marked
	// the class — don't ask the patient again. The worker must not be the
	// reason a confirmed patient gets another "confirma sua aula?" message.
	clinicID := uuid.New()
	c := sampleCandidate(clinicID, enabledRemindersSettings(), 24*time.Hour)
	src := &stubClassReminderSource{candidates: []ClassReminderCandidate{c}}
	q := &stubQueue{}
	logRepo := &stubClassReminderLog{}

	att := &stubClassAttendanceChecker{
		exists: map[string]bool{
			// session date string format: YYYY-MM-DD of c.SessionStart
			reminderLogKey(c.ClinicID, c.PlanID, c.PatientID, c.SessionStart.Format("2006-01-02"), ""): true,
		},
	}

	w := NewGroupClassReminderWorker(newClassReminderConfig(), src, logRepo, att, q)
	count, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_24h", Type: "reminder_24h", Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, q.published)
}

func TestGroupClassReminderWorker_SkipsIfSubscriptionInactive(t *testing.T) {
	clinicID := uuid.New()
	src := &stubClassReminderSource{
		candidates: []ClassReminderCandidate{sampleCandidate(clinicID, enabledRemindersSettings(), 24*time.Hour)},
	}
	q := &stubQueue{}
	logRepo := &stubClassReminderLog{}
	att := &stubClassAttendanceChecker{}
	checker := &stubSubscriptionChecker{activeByClinic: map[uuid.UUID]bool{clinicID: false}}

	w := NewGroupClassReminderWorker(newClassReminderConfig(), src, logRepo, att, q)
	w.SetSubscriptionChecker(checker)

	count, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_24h", Type: "reminder_24h", Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	require.NoError(t, err)
	assert.Equal(t, 0, count, "inactive subscription must skip — same gate the appointment reminder uses")
	assert.Empty(t, q.published)
}

func TestGroupClassReminderWorker_SkipsIfMasterReminderFlagOff(t *testing.T) {
	// WhatsAppRemindersEnabled=false at the clinic level kills ALL reminders
	// (appointments AND classes). Per-threshold flags can't override this.
	clinicID := uuid.New()
	c := sampleCandidate(clinicID, clinic_model.ClinicSettings{WhatsAppRemindersEnabled: false}, 24*time.Hour)
	src := &stubClassReminderSource{candidates: []ClassReminderCandidate{c}}
	q := &stubQueue{}

	w := NewGroupClassReminderWorker(newClassReminderConfig(), src, &stubClassReminderLog{}, &stubClassAttendanceChecker{}, q)
	count, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_24h", Type: "reminder_24h", Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, q.published, "master WhatsAppRemindersEnabled=false must block every threshold")
}

func TestGroupClassReminderWorker_PerThresholdFlagsAreIndependent(t *testing.T) {
	// Disable 24h but leave 2h on. Worker must skip the 24h threshold and
	// fire only the 2h one. The flags are independent peers.
	clinicID := uuid.New()
	settings := clinic_model.ClinicSettings{
		WhatsAppRemindersEnabled:     true,
		GroupClassReminder24hEnabled: boolPtrLocal(false),
		GroupClassReminder2hEnabled:  boolPtrLocal(true),
	}

	w := NewGroupClassReminderWorker(newClassReminderConfig(),
		&stubClassReminderSource{candidates: []ClassReminderCandidate{sampleCandidate(clinicID, settings, 24*time.Hour)}},
		&stubClassReminderLog{},
		&stubClassAttendanceChecker{},
		&stubQueue{})

	count24, _ := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_24h", Type: "reminder_24h", Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	assert.Equal(t, 0, count24, "GroupClassReminder24hEnabled=false must block the 24h threshold")

	w2 := NewGroupClassReminderWorker(newClassReminderConfig(),
		&stubClassReminderSource{candidates: []ClassReminderCandidate{sampleCandidate(clinicID, settings, 2*time.Hour)}},
		&stubClassReminderLog{},
		&stubClassAttendanceChecker{},
		&stubQueue{})
	count2, _ := w2.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_2h", Type: "reminder_2h", Duration: 2 * time.Hour,
	}, map[uuid.UUID]bool{})
	assert.Equal(t, 1, count2, "GroupClassReminder2hEnabled=true must allow the 2h threshold to fire")
}

func TestGroupClassReminderWorker_SkipsIfPatientHasNoPhone(t *testing.T) {
	clinicID := uuid.New()
	c := sampleCandidate(clinicID, enabledRemindersSettings(), 24*time.Hour)
	c.PatientPhone = ""
	src := &stubClassReminderSource{candidates: []ClassReminderCandidate{c}}

	w := NewGroupClassReminderWorker(newClassReminderConfig(), src, &stubClassReminderLog{}, &stubClassAttendanceChecker{}, &stubQueue{})
	count, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_24h", Type: "reminder_24h", Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	require.NoError(t, err)
	assert.Equal(t, 0, count, "patient without WhatsApp phone must be skipped silently — never crash the worker on missing data")
}

func TestGroupClassReminderWorker_MessageContainsPlanAndDateTime(t *testing.T) {
	clinicID := uuid.New()
	c := sampleCandidate(clinicID, enabledRemindersSettings(), 24*time.Hour)
	c.PlanTitle = "Pilates"
	src := &stubClassReminderSource{candidates: []ClassReminderCandidate{c}}
	q := &stubQueue{}

	w := NewGroupClassReminderWorker(newClassReminderConfig(), src, &stubClassReminderLog{}, &stubClassAttendanceChecker{}, q)
	_, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_24h", Type: "reminder_24h", Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	require.NoError(t, err)
	require.Len(t, q.published, 1)
	msg := q.published[0].Message
	assert.True(t, strings.Contains(msg, "Pilates"), "message must include the plan title so the patient knows which class — got %q", msg)
	expectedTime := c.SessionStart.In(saoPaulo()).Format("15:04")
	assert.Contains(t, msg, expectedTime, "message must include the class HH:MM in São Paulo timezone")
}

func TestGroupClassReminderWorker_PropagatesSourceError(t *testing.T) {
	src := &stubClassReminderSource{err: errors.New("db down")}
	w := NewGroupClassReminderWorker(newClassReminderConfig(), src, &stubClassReminderLog{}, &stubClassAttendanceChecker{}, &stubQueue{})
	_, err := w.processThreshold(context.Background(), config.ReminderThreshold{
		Name: "reminder_24h", Type: "reminder_24h", Duration: 24 * time.Hour,
	}, map[uuid.UUID]bool{})
	assert.Error(t, err, "source errors must surface so the operator notices the worker can't read upcoming classes")
}

func boolPtrLocal(b bool) *bool { return &b }

func saoPaulo() *time.Location {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	return loc
}
