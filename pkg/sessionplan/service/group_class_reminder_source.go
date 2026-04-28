package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/worker"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	enrollment_repository "github.com/aseno/backend/pkg/enrollment/repository"
	patient_model "github.com/aseno/backend/pkg/patient/model"
	patient_repository "github.com/aseno/backend/pkg/patient/repository"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/repository"
	whatsapp_repository "github.com/aseno/backend/pkg/whatsapp/repository"
)

// GroupClassReminderSource walks active group session plans × slots × active
// enrollments × patients to produce ClassReminderCandidate rows for the
// reminder worker. Computes occurrences in Go (not SQL) because the
// weekday + HH:MM → next datetime math is awkward in Postgres and the
// volume is small enough (clinic count × ~10 plans × ~50 slots × ~30
// enrollments per plan).
type GroupClassReminderSource struct {
	clinicRepo          clinic_repository.ClinicRepository
	sessionPlanRepo     repository.SessionPlanRepository
	slotRepo            repository.SessionPlanSlotRepository
	enrollmentRepo      enrollment_repository.EnrollmentRepository
	patientRepo         patient_repository.PatientRepository
	whatsappConnRepo    whatsapp_repository.WhatsAppConnectionRepository
	clinicPageSize      int
}

// NewGroupClassReminderSource wires the source. clinicPageSize controls how
// many clinics are scanned per page — defaults to 200 when 0.
func NewGroupClassReminderSource(
	clinicRepo clinic_repository.ClinicRepository,
	sessionPlanRepo repository.SessionPlanRepository,
	slotRepo repository.SessionPlanSlotRepository,
	enrollmentRepo enrollment_repository.EnrollmentRepository,
	patientRepo patient_repository.PatientRepository,
	whatsappConnRepo whatsapp_repository.WhatsAppConnectionRepository,
) *GroupClassReminderSource {
	return &GroupClassReminderSource{
		clinicRepo:       clinicRepo,
		sessionPlanRepo:  sessionPlanRepo,
		slotRepo:         slotRepo,
		enrollmentRepo:   enrollmentRepo,
		patientRepo:      patientRepo,
		whatsappConnRepo: whatsappConnRepo,
		clinicPageSize:   200,
	}
}

// Compile-time interface check.
var _ worker.ClassReminderSource = (*GroupClassReminderSource)(nil)

// UpcomingClasses returns one candidate per (clinic, plan, slot, enrolled
// patient) whose next occurrence falls inside the [windowStart, windowEnd]
// range. The worker further filters by reminder flags / subscription /
// dedup log / existing attendance — the source is purely "what classes
// are coming up and who is enrolled".
func (s *GroupClassReminderSource) UpcomingClasses(ctx context.Context, windowStart, windowEnd time.Time) ([]worker.ClassReminderCandidate, error) {
	loc := saoPaulo()
	now := time.Now().In(loc)

	page := s.clinicPageSize
	if page <= 0 {
		page = 200
	}

	out := []worker.ClassReminderCandidate{}
	offset := 0

	// Caches scoped to this call — patient/instance lookups across
	// candidates of the same clinic don't trigger N redundant queries.
	patientCache := map[uuid.UUID]*patient_model.Patient{}

	for {
		clinics, total, err := s.clinicRepo.List(ctx, page, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to list clinics: %w", err)
		}
		if len(clinics) == 0 {
			break
		}

		for _, clinic := range clinics {
			// Master gate: skip clinics that turned reminders off entirely.
			// Per-threshold gates are checked in the worker (it knows the
			// threshold name), not here.
			if !clinic.Settings.WhatsAppRemindersEnabled {
				continue
			}

			// Lookup the active WhatsApp instance once per clinic. A clinic
			// without an active connected instance can't send anything;
			// skip rather than enqueue jobs that will fail downstream.
			conn, err := s.whatsappConnRepo.GetActiveByClinicID(ctx, clinic.ID)
			if err != nil || conn == nil || !conn.Connected || conn.Instance == nil || *conn.Instance == "" {
				continue
			}

			candidates, err := s.collectForClinic(ctx, clinic, *conn.Instance, now, windowStart, windowEnd, patientCache)
			if err != nil {
				// Per-clinic errors are logged but don't kill the whole
				// run — one bad clinic shouldn't block reminders for
				// every other clinic.
				continue
			}
			out = append(out, candidates...)
		}

		offset += len(clinics)
		if int64(offset) >= total {
			break
		}
	}

	return out, nil
}

func (s *GroupClassReminderSource) collectForClinic(
	ctx context.Context,
	clinic *clinic_model.Clinic,
	whatsappInstance string,
	now, windowStart, windowEnd time.Time,
	patientCache map[uuid.UUID]*patient_model.Patient,
) ([]worker.ClassReminderCandidate, error) {
	plans, err := s.sessionPlanRepo.GetAllActive(ctx, clinic.ID)
	if err != nil {
		return nil, err
	}

	out := []worker.ClassReminderCandidate{}
	for _, plan := range plans {
		if plan.Type != model.PlanTypeGroup {
			continue
		}

		slots, err := s.slotRepo.ListActiveByPlan(ctx, plan.ID)
		if err != nil {
			continue
		}
		if len(slots) == 0 {
			continue
		}

		enrollments, err := s.enrollmentRepo.GetActiveByPlan(ctx, clinic.ID, plan.ID)
		if err != nil {
			continue
		}
		if len(enrollments) == 0 {
			continue
		}

		for _, slot := range slots {
			occ := nextOccurrenceInWindow(now, slot.Weekday, slot.StartTime, windowStart, windowEnd)
			if occ.IsZero() {
				continue
			}

			for _, enrollment := range enrollments {
				if !enrollment.HasWeekDay(slot.Weekday) {
					continue
				}

				patient, ok := patientCache[enrollment.PatientID]
				if !ok {
					p, err := s.patientRepo.GetByID(ctx, enrollment.PatientID, clinic.ID)
					if err != nil || p == nil {
						patientCache[enrollment.PatientID] = nil
						continue
					}
					patient = p
					patientCache[enrollment.PatientID] = patient
				}
				if patient == nil {
					continue
				}

				phone := patient.WhatsApp
				if phone == "" {
					phone = patient.Phone
				}
				if phone == "" {
					continue
				}

				out = append(out, worker.ClassReminderCandidate{
					ClinicID:         clinic.ID,
					ClinicSettings:   clinic.Settings,
					PlanID:           plan.ID,
					PlanTitle:        plan.Title,
					PatientID:        patient.ID,
					PatientName:      patient.Name,
					PatientPhone:     phone,
					WhatsAppInstance: whatsappInstance,
					SessionStart:     occ,
				})
			}
		}
	}
	return out, nil
}

// nextOccurrenceInWindow returns the next datetime matching weekday +
// HH:MM that falls inside [windowStart, windowEnd]. Returns zero time
// when no occurrence fits the window (caller skips). Today counts only
// if the time-of-day hasn't already passed.
func nextOccurrenceInWindow(now time.Time, weekday int, startTime string, windowStart, windowEnd time.Time) time.Time {
	if len(startTime) != 5 || startTime[2] != ':' {
		return time.Time{}
	}
	var h, m int
	if _, err := fmt.Sscanf(startTime, "%d:%d", &h, &m); err != nil {
		return time.Time{}
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}
	}

	loc := now.Location()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, loc)
	dayDiff := (weekday - int(now.Weekday()) + 7) % 7
	if dayDiff == 0 && !candidate.After(now) {
		dayDiff = 7
	}
	occ := candidate.AddDate(0, 0, dayDiff)

	if occ.Before(windowStart) || occ.After(windowEnd) {
		return time.Time{}
	}
	return occ
}

func saoPaulo() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.UTC
	}
	return loc
}
