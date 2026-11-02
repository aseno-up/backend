package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/enrollment/model"
	"github.com/aseno/backend/pkg/enrollment/repository"
	pn_model "github.com/aseno/backend/pkg/partnernetwork/model"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
	sessionplan_repository "github.com/aseno/backend/pkg/sessionplan/repository"
)

// SlotProvider is a narrow interface used by the enrollment service to look up
// session plan slots when validating slot-based enrollments. Implemented by
// pkg/sessionplan/repository.SessionPlanSlotRepository.
type SlotProvider interface {
	GetByIDScoped(ctx context.Context, id, clinicID uuid.UUID) (*sessionplan_model.SessionPlanSlot, error)
}

// PartnerPlanProvider is a narrow interface used by the enrollment service to
// fetch partner plan limits. Implemented by pkg/partnernetwork/repository.PartnerPlanRepository.
type PartnerPlanProvider interface {
	GetByIDScoped(id, clinicID uuid.UUID) (*pn_model.PartnerPlan, error)
}

// EnrollmentService defines the interface for enrollment operations
type EnrollmentService interface {
	Enroll(ctx context.Context, clinicID, sessionPlanID uuid.UUID, req *model.EnrollRequest) (*model.SessionPlanEnrollment, error)
	Update(ctx context.Context, clinicID, enrollmentID uuid.UUID, req *model.EnrollUpdateRequest) (*model.SessionPlanEnrollment, error)
	Suspend(ctx context.Context, clinicID, enrollmentID uuid.UUID) error
	Reactivate(ctx context.Context, clinicID, enrollmentID uuid.UUID) error
	Cancel(ctx context.Context, clinicID, enrollmentID uuid.UUID) error
	ListByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanEnrollment, error)
	GetByID(ctx context.Context, clinicID, enrollmentID uuid.UUID) (*model.SessionPlanEnrollment, error)
	CountActiveByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (int64, error)

	// SetSlotProvider wires the session_plan_slots lookup used for slot-based
	// enrollments. When nil, slot_ids in EnrollRequest are rejected.
	SetSlotProvider(p SlotProvider)
	// SetPartnerPlanProvider wires the partner plan lookup used to validate
	// per-week frequency limits. When nil, partner_plan_id is accepted but
	// no frequency check runs.
	SetPartnerPlanProvider(p PartnerPlanProvider)
}

type enrollmentService struct {
	repo            repository.EnrollmentRepository
	planRepo        sessionplan_repository.SessionPlanRepository
	slotProvider    SlotProvider
	partnerPlanProv PartnerPlanProvider
}

// NewEnrollmentService creates a new enrollment service
func NewEnrollmentService(
	repo repository.EnrollmentRepository,
	planRepo sessionplan_repository.SessionPlanRepository,
) EnrollmentService {
	return &enrollmentService{
		repo:     repo,
		planRepo: planRepo,
	}
}

func (s *enrollmentService) SetSlotProvider(p SlotProvider) {
	s.slotProvider = p
}

func (s *enrollmentService) SetPartnerPlanProvider(p PartnerPlanProvider) {
	s.partnerPlanProv = p
}

// Enroll adds a patient to a group session plan
func (s *enrollmentService) Enroll(ctx context.Context, clinicID, sessionPlanID uuid.UUID, req *model.EnrollRequest) (*model.SessionPlanEnrollment, error) {
	// Validate the plan exists and is a group plan
	plan, err := s.planRepo.GetByID(ctx, clinicID, sessionPlanID)
	if err != nil {
		return nil, err
	}

	if plan.Type != "group" {
		return nil, errors.ErrValidationInstance.WithMessage("Can only enroll patients in group plans")
	}

	if !plan.IsActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Plan is not active")
	}

	// slot-based vs legacy week_days resolution
	usingSlots := len(req.SlotIDs) > 0
	var slots []*sessionplan_model.SessionPlanSlot

	if usingSlots {
		if s.slotProvider == nil {
			return nil, errors.ErrValidationInstance.WithMessage("slot-based enrollment is not supported in this configuration")
		}
		// Deduplicate slot ids while preserving order.
		seenSlot := make(map[uuid.UUID]bool, len(req.SlotIDs))
		for _, sid := range req.SlotIDs {
			if seenSlot[sid] {
				continue
			}
			seenSlot[sid] = true
			slot, err := s.slotProvider.GetByIDScoped(ctx, sid, clinicID)
			if err != nil {
				return nil, errors.ErrValidationInstance.WithMessage("invalid slot_id: slot not found in this clinic")
			}
			if slot.SessionPlanID != sessionPlanID {
				return nil, errors.ErrValidationInstance.WithMessage("slot does not belong to the requested session plan")
			}
			if !slot.IsActive {
				return nil, errors.ErrValidationInstance.WithMessage("slot is inactive and cannot accept new enrollments")
			}
			slots = append(slots, slot)
		}
	}

	// Resolve weekDays — derived from slots when slot-based, else taken from request.
	var weekDays pq.Int64Array
	if usingSlots {
		weekDays = uniqueWeekDaysFromSlots(slots)
	} else {
		if len(req.WeekDays) == 0 {
			return nil, errors.ErrValidationInstance.WithMessage("week_days is required for group enrollments")
		}
		weekDays, err = normalizeWeekDays(req.WeekDays)
		if err != nil {
			return nil, err
		}
	}

	// Partner plan: validate existence and per-week limits.
	if req.PartnerPlanID != nil && s.partnerPlanProv != nil {
		pp, err := s.partnerPlanProv.GetByIDScoped(*req.PartnerPlanID, clinicID)
		if err != nil {
			return nil, errors.ErrValidationInstance.WithMessage("invalid partner_plan_id: plan not found in this clinic")
		}
		if !pp.IsActive {
			return nil, errors.ErrValidationInstance.WithMessage("partner plan is inactive")
		}
		if pp.MaxSessionsPerWeek != nil {
			used := len(req.SlotIDs)
			if !usingSlots {
				used = len(weekDays)
			}
			if used > *pp.MaxSessionsPerWeek {
				return nil, errors.ErrValidationInstance.WithMessage("enrollment exceeds the partner plan's max sessions per week")
			}
		}
	}

	// Capacity check.
	if usingSlots {
		// Per-slot capacity.
		for _, slot := range slots {
			count, err := s.repo.CountActiveBySlot(ctx, clinicID, sessionPlanID, slot.ID)
			if err != nil {
				return nil, err
			}
			if int(count) >= slot.Capacity {
				return nil, errors.ErrValidationInstance.WithMessage("slot has reached its capacity")
			}
		}
	} else if plan.Capacity != nil {
		// Legacy per-day capacity: for each requested day, count active enrollments on that day.
		for _, day := range weekDays {
			count, err := s.repo.CountActiveByPlanAndWeekDay(ctx, clinicID, sessionPlanID, int(day))
			if err != nil {
				return nil, err
			}
			if int(count) >= *plan.Capacity {
				return nil, errors.ErrValidationInstance.WithMessage("Plan has reached maximum capacity for the selected day")
			}
		}
	}

	// Check if patient is already enrolled
	existing, err := s.repo.GetByPatientAndPlan(ctx, clinicID, req.PatientID, sessionPlanID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.IsActive() {
		return nil, errors.ErrValidationInstance.WithMessage("Patient is already enrolled in this plan")
	}

	// If patient was previously cancelled, reactivate
	if existing != nil && existing.Status == model.EnrollmentStatusCancelled {
		// Same billing_day guard as below so reactivation into monthly also
		// requires the field.
		effectiveMode := req.PaymentMode
		if effectiveMode == "" {
			effectiveMode = existing.PaymentMode
		}
		if effectiveMode == model.PaymentModeMonthly {
			if req.BillingDay == nil {
				return nil, errors.ErrValidationInstance.WithMessage("billing_day is required for monthly enrollments")
			}
			if *req.BillingDay < 1 || *req.BillingDay > 31 {
				return nil, errors.ErrValidationInstance.WithMessage("billing_day must be between 1 and 31")
			}
		}
		existing.Status = model.EnrollmentStatusActive
		existing.LeftAt = nil
		existing.WeekDays = weekDays
		existing.PartnerPlanID = req.PartnerPlanID
		if req.TotalSessions != nil {
			existing.TotalSessions = req.TotalSessions
		}
		if req.PaymentMode != "" {
			existing.PaymentMode = req.PaymentMode
		}
		existing.MonthlyAmount = req.MonthlyAmount
		existing.BillingDay = req.BillingDay
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		if usingSlots {
			if err := s.repo.SetEnrollmentSlots(ctx, existing.ID, dedupSlotIDs(req.SlotIDs)); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}

	// Default payment mode
	paymentMode := req.PaymentMode
	if paymentMode == "" {
		paymentMode = model.PaymentModePerSession
	}

	// Monthly enrollments require a billing day so the payment reminder
	// worker can compute the next due date. We validate it here (instead of
	// relying solely on the frontend) because enrollments can be created
	// from other callers too (e.g. import scripts).
	if paymentMode == model.PaymentModeMonthly {
		if req.BillingDay == nil {
			return nil, errors.ErrValidationInstance.WithMessage("billing_day is required for monthly enrollments")
		}
		if *req.BillingDay < 1 || *req.BillingDay > 31 {
			return nil, errors.ErrValidationInstance.WithMessage("billing_day must be between 1 and 31")
		}
	}

	enrollment := &model.SessionPlanEnrollment{
		ClinicID:      clinicID,
		SessionPlanID: sessionPlanID,
		PatientID:     req.PatientID,
		Status:        model.EnrollmentStatusActive,
		WeekDays:      weekDays,
		PartnerPlanID: req.PartnerPlanID,
		TotalSessions: req.TotalSessions,
		SessionsDone:  0,
		PaymentMode:   paymentMode,
		MonthlyAmount: req.MonthlyAmount,
		BillingDay:    req.BillingDay,
		JoinedAt:      time.Now(),
	}

	if err := s.repo.Create(ctx, enrollment); err != nil {
		return nil, err
	}

	if usingSlots {
		if err := s.repo.SetEnrollmentSlots(ctx, enrollment.ID, dedupSlotIDs(req.SlotIDs)); err != nil {
			return nil, err
		}
	}

	return s.repo.GetByID(ctx, clinicID, enrollment.ID)
}

// Update applies a partial change to an existing enrollment. Patient cannot
// change. Slots/weekdays/partner_plan/payment fields are validated similarly
// to Enroll, but capacity checks exclude the current enrollment's existing
// slots so an "edit in place" doesn't bump itself over the limit.
func (s *enrollmentService) Update(ctx context.Context, clinicID, enrollmentID uuid.UUID, req *model.EnrollUpdateRequest) (*model.SessionPlanEnrollment, error) {
	existing, err := s.repo.GetByID(ctx, clinicID, enrollmentID)
	if err != nil {
		return nil, err
	}

	plan, err := s.planRepo.GetByID(ctx, clinicID, existing.SessionPlanID)
	if err != nil {
		return nil, err
	}

	// Resolve target slots / weekdays.
	wantsSlotChange := req.SlotIDs != nil
	wantsWeekDayChange := req.WeekDays != nil

	currentSlotIDSet := make(map[uuid.UUID]bool, len(existing.Slots))
	for _, s := range existing.Slots {
		currentSlotIDSet[s.SessionPlanSlotID] = true
	}

	var newSlots []*sessionplan_model.SessionPlanSlot
	var newWeekDays pq.Int64Array

	if wantsSlotChange {
		if s.slotProvider == nil {
			return nil, errors.ErrValidationInstance.WithMessage("slot-based enrollment is not supported in this configuration")
		}
		seen := make(map[uuid.UUID]bool, len(*req.SlotIDs))
		for _, sid := range *req.SlotIDs {
			if seen[sid] {
				continue
			}
			seen[sid] = true
			slot, err := s.slotProvider.GetByIDScoped(ctx, sid, clinicID)
			if err != nil {
				return nil, errors.ErrValidationInstance.WithMessage("invalid slot_id: slot not found in this clinic")
			}
			if slot.SessionPlanID != existing.SessionPlanID {
				return nil, errors.ErrValidationInstance.WithMessage("slot does not belong to the requested session plan")
			}
			if !slot.IsActive {
				return nil, errors.ErrValidationInstance.WithMessage("slot is inactive and cannot accept new enrollments")
			}
			newSlots = append(newSlots, slot)
		}
		newWeekDays = uniqueWeekDaysFromSlots(newSlots)
	} else if wantsWeekDayChange {
		if len(*req.WeekDays) == 0 {
			return nil, errors.ErrValidationInstance.WithMessage("week_days is required for group enrollments")
		}
		newWeekDays, err = normalizeWeekDays(*req.WeekDays)
		if err != nil {
			return nil, err
		}
	}

	// Resolve effective partner plan id after this update.
	effectivePartnerPlanID := existing.PartnerPlanID
	if req.ClearPartnerPlan {
		effectivePartnerPlanID = nil
	} else if req.PartnerPlanID != nil {
		effectivePartnerPlanID = req.PartnerPlanID
	}

	if effectivePartnerPlanID != nil && s.partnerPlanProv != nil {
		pp, err := s.partnerPlanProv.GetByIDScoped(*effectivePartnerPlanID, clinicID)
		if err != nil {
			return nil, errors.ErrValidationInstance.WithMessage("invalid partner_plan_id: plan not found in this clinic")
		}
		if !pp.IsActive {
			return nil, errors.ErrValidationInstance.WithMessage("partner plan is inactive")
		}
		if pp.MaxSessionsPerWeek != nil {
			used := 0
			if wantsSlotChange {
				used = len(newSlots)
			} else if wantsWeekDayChange {
				used = len(newWeekDays)
			} else if len(existing.Slots) > 0 {
				used = len(existing.Slots)
			} else {
				used = len(existing.WeekDays)
			}
			if used > *pp.MaxSessionsPerWeek {
				return nil, errors.ErrValidationInstance.WithMessage("enrollment exceeds the partner plan's max sessions per week")
			}
		}
	}

	// Capacity check, excluding self.
	if wantsSlotChange {
		for _, slot := range newSlots {
			if currentSlotIDSet[slot.ID] {
				continue // already on this slot, no extra capacity needed
			}
			count, err := s.repo.CountActiveBySlot(ctx, clinicID, existing.SessionPlanID, slot.ID)
			if err != nil {
				return nil, err
			}
			if int(count) >= slot.Capacity {
				return nil, errors.ErrValidationInstance.WithMessage("slot has reached its capacity")
			}
		}
	} else if wantsWeekDayChange && plan.Capacity != nil {
		currentDaySet := make(map[int]bool, len(existing.WeekDays))
		for _, d := range existing.WeekDays {
			currentDaySet[int(d)] = true
		}
		for _, day := range newWeekDays {
			if currentDaySet[int(day)] {
				continue
			}
			count, err := s.repo.CountActiveByPlanAndWeekDay(ctx, clinicID, existing.SessionPlanID, int(day))
			if err != nil {
				return nil, err
			}
			if int(count) >= *plan.Capacity {
				return nil, errors.ErrValidationInstance.WithMessage("Plan has reached maximum capacity for the selected day")
			}
		}
	}

	// Apply changes.
	if wantsSlotChange || wantsWeekDayChange {
		existing.WeekDays = newWeekDays
	}
	if req.ClearPartnerPlan || req.PartnerPlanID != nil {
		existing.PartnerPlanID = effectivePartnerPlanID
	}
	if req.TotalSessions != nil {
		existing.TotalSessions = req.TotalSessions
	}
	if req.PaymentMode != nil {
		// Monthly requires a billing day — validate consistently with Enroll.
		if *req.PaymentMode == model.PaymentModeMonthly {
			day := existing.BillingDay
			if req.BillingDay != nil {
				day = req.BillingDay
			}
			if day == nil {
				return nil, errors.ErrValidationInstance.WithMessage("billing_day is required for monthly enrollments")
			}
			if *day < 1 || *day > 31 {
				return nil, errors.ErrValidationInstance.WithMessage("billing_day must be between 1 and 31")
			}
		}
		existing.PaymentMode = *req.PaymentMode
	}
	if req.MonthlyAmount != nil {
		existing.MonthlyAmount = req.MonthlyAmount
	}
	if req.BillingDay != nil {
		if *req.BillingDay < 1 || *req.BillingDay > 31 {
			return nil, errors.ErrValidationInstance.WithMessage("billing_day must be between 1 and 31")
		}
		existing.BillingDay = req.BillingDay
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	if wantsSlotChange {
		ids := make([]uuid.UUID, len(newSlots))
		for i, sl := range newSlots {
			ids[i] = sl.ID
		}
		if err := s.repo.SetEnrollmentSlots(ctx, existing.ID, ids); err != nil {
			return nil, err
		}
	}

	return s.repo.GetByID(ctx, clinicID, existing.ID)
}

// uniqueWeekDaysFromSlots derives a deduplicated, ordered list of weekdays
// (0..6) from the given slots. Used to populate enrollment.WeekDays when the
// request was slot-based, so calendar/attendance code that still keys by
// weekday keeps working without explicit migration.
func uniqueWeekDaysFromSlots(slots []*sessionplan_model.SessionPlanSlot) pq.Int64Array {
	seen := make(map[int]bool, len(slots))
	out := make(pq.Int64Array, 0, len(slots))
	for _, s := range slots {
		if seen[s.Weekday] {
			continue
		}
		seen[s.Weekday] = true
		out = append(out, int64(s.Weekday))
	}
	return out
}

// dedupSlotIDs returns the input list with duplicates removed, preserving order.
func dedupSlotIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// normalizeWeekDays validates each weekday is in [0,6] and returns a deduplicated
// pq.Int64Array ready for persistence.
func normalizeWeekDays(days []int) (pq.Int64Array, error) {
	if len(days) == 0 {
		return nil, nil
	}
	seen := make(map[int]bool, len(days))
	out := make(pq.Int64Array, 0, len(days))
	for _, d := range days {
		if d < 0 || d > 6 {
			return nil, errors.ErrValidationInstance.WithMessage("week_days must contain values between 0 and 6")
		}
		if seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, int64(d))
	}
	return out, nil
}

// Suspend suspends an enrollment
func (s *enrollmentService) Suspend(ctx context.Context, clinicID, enrollmentID uuid.UUID) error {
	enrollment, err := s.repo.GetByID(ctx, clinicID, enrollmentID)
	if err != nil {
		return err
	}

	if enrollment.Status != model.EnrollmentStatusActive {
		return errors.ErrValidationInstance.WithMessage("Only active enrollments can be suspended")
	}

	return s.repo.UpdateStatus(ctx, clinicID, enrollmentID, model.EnrollmentStatusSuspended)
}

// Reactivate reactivates a suspended enrollment
func (s *enrollmentService) Reactivate(ctx context.Context, clinicID, enrollmentID uuid.UUID) error {
	enrollment, err := s.repo.GetByID(ctx, clinicID, enrollmentID)
	if err != nil {
		return err
	}

	if enrollment.Status != model.EnrollmentStatusSuspended {
		return errors.ErrValidationInstance.WithMessage("Only suspended enrollments can be reactivated")
	}

	return s.repo.UpdateStatus(ctx, clinicID, enrollmentID, model.EnrollmentStatusActive)
}

// Cancel cancels an enrollment
func (s *enrollmentService) Cancel(ctx context.Context, clinicID, enrollmentID uuid.UUID) error {
	enrollment, err := s.repo.GetByID(ctx, clinicID, enrollmentID)
	if err != nil {
		return err
	}

	if enrollment.Status == model.EnrollmentStatusCancelled {
		return errors.ErrValidationInstance.WithMessage("Enrollment is already cancelled")
	}

	return s.repo.UpdateStatus(ctx, clinicID, enrollmentID, model.EnrollmentStatusCancelled)
}

// ListByPlan retrieves all enrollments for a plan
func (s *enrollmentService) ListByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) ([]*model.SessionPlanEnrollment, error) {
	return s.repo.GetByPlan(ctx, clinicID, sessionPlanID)
}

// GetByID retrieves an enrollment by ID
func (s *enrollmentService) GetByID(ctx context.Context, clinicID, enrollmentID uuid.UUID) (*model.SessionPlanEnrollment, error) {
	return s.repo.GetByID(ctx, clinicID, enrollmentID)
}

// CountActiveByPlan counts active enrollments for a plan
func (s *enrollmentService) CountActiveByPlan(ctx context.Context, clinicID, sessionPlanID uuid.UUID) (int64, error) {
	return s.repo.CountActiveByPlan(ctx, clinicID, sessionPlanID)
}
