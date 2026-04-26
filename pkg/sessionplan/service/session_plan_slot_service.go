package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	internal_errors "github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/sessionplan/repository"
)

// SessionPlanSlotService manages slot CRUD for group session plans.
type SessionPlanSlotService interface {
	List(ctx context.Context, clinicID, planID uuid.UUID) ([]*model.SessionPlanSlot, error)
	Create(ctx context.Context, clinicID, planID uuid.UUID, req *model.CreateSessionPlanSlotRequest) (*model.SessionPlanSlot, error)
	Update(ctx context.Context, clinicID, slotID uuid.UUID, req *model.UpdateSessionPlanSlotRequest) (*model.SessionPlanSlot, error)
	Delete(ctx context.Context, clinicID, slotID uuid.UUID) error
}

type sessionPlanSlotService struct {
	planRepo repository.SessionPlanRepository
	slotRepo repository.SessionPlanSlotRepository
}

// NewSessionPlanSlotService creates a new slot service.
func NewSessionPlanSlotService(
	planRepo repository.SessionPlanRepository,
	slotRepo repository.SessionPlanSlotRepository,
) SessionPlanSlotService {
	return &sessionPlanSlotService{planRepo: planRepo, slotRepo: slotRepo}
}

func (s *sessionPlanSlotService) List(ctx context.Context, clinicID, planID uuid.UUID) ([]*model.SessionPlanSlot, error) {
	if _, err := s.planRepo.GetByID(ctx, clinicID, planID); err != nil {
		return nil, err
	}
	return s.slotRepo.ListByPlan(ctx, planID)
}

func (s *sessionPlanSlotService) Create(ctx context.Context, clinicID, planID uuid.UUID, req *model.CreateSessionPlanSlotRequest) (*model.SessionPlanSlot, error) {
	plan, err := s.planRepo.GetByID(ctx, clinicID, planID)
	if err != nil {
		return nil, err
	}
	if plan.Type != model.PlanTypeGroup {
		return nil, internal_errors.ErrValidationInstance.WithMessage("Slots are only supported on group session plans")
	}
	if err := validateStartTime(req.StartTime); err != nil {
		return nil, err
	}
	if req.Weekday < 0 || req.Weekday > 6 {
		return nil, internal_errors.ErrValidationInstance.WithMessage("weekday must be between 0 and 6")
	}
	if req.Capacity < 1 {
		return nil, internal_errors.ErrValidationInstance.WithMessage("capacity must be at least 1")
	}

	slot := &model.SessionPlanSlot{
		SessionPlanID:  planID,
		Weekday:        req.Weekday,
		StartTime:      req.StartTime,
		Capacity:       req.Capacity,
		ProfessionalID: req.ProfessionalID,
		IsActive:       true,
	}
	if err := s.slotRepo.Create(ctx, slot); err != nil {
		return nil, mapSlotDBError(err)
	}
	return slot, nil
}

func (s *sessionPlanSlotService) Update(ctx context.Context, clinicID, slotID uuid.UUID, req *model.UpdateSessionPlanSlotRequest) (*model.SessionPlanSlot, error) {
	slot, err := s.slotRepo.GetByIDScoped(ctx, slotID, clinicID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, internal_errors.ErrNotFoundInstance.WithMessage("slot not found")
		}
		return nil, err
	}
	if req.Weekday != nil {
		if *req.Weekday < 0 || *req.Weekday > 6 {
			return nil, internal_errors.ErrValidationInstance.WithMessage("weekday must be between 0 and 6")
		}
		slot.Weekday = *req.Weekday
	}
	if req.StartTime != nil {
		if err := validateStartTime(*req.StartTime); err != nil {
			return nil, err
		}
		slot.StartTime = *req.StartTime
	}
	if req.Capacity != nil {
		if *req.Capacity < 1 {
			return nil, internal_errors.ErrValidationInstance.WithMessage("capacity must be at least 1")
		}
		slot.Capacity = *req.Capacity
	}
	if req.ProfessionalID != nil {
		// Empty UUID means "clear the professional override".
		if *req.ProfessionalID == uuid.Nil {
			slot.ProfessionalID = nil
		} else {
			id := *req.ProfessionalID
			slot.ProfessionalID = &id
		}
	}
	if req.IsActive != nil {
		slot.IsActive = *req.IsActive
	}
	if err := s.slotRepo.Update(ctx, slot); err != nil {
		return nil, mapSlotDBError(err)
	}
	return slot, nil
}

func (s *sessionPlanSlotService) Delete(ctx context.Context, clinicID, slotID uuid.UUID) error {
	if _, err := s.slotRepo.GetByIDScoped(ctx, slotID, clinicID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return internal_errors.ErrNotFoundInstance.WithMessage("slot not found")
		}
		return err
	}
	return s.slotRepo.Delete(ctx, slotID, clinicID)
}

// validateStartTime requires HH:MM format. Hour 00-23, minute 00-59.
func validateStartTime(t string) error {
	if len(t) != 5 || t[2] != ':' {
		return internal_errors.ErrValidationInstance.WithMessage("start_time must be HH:MM")
	}
	h := t[:2]
	m := t[3:]
	if !isDigits(h) || !isDigits(m) {
		return internal_errors.ErrValidationInstance.WithMessage("start_time must contain digits only")
	}
	hh := (int(h[0])-'0')*10 + int(h[1]-'0')
	mm := (int(m[0])-'0')*10 + int(m[1]-'0')
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return internal_errors.ErrValidationInstance.WithMessage("start_time out of range")
	}
	return nil
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// mapSlotDBError converts a Postgres unique violation on the slot composite
// index into a user-friendly validation error.
func mapSlotDBError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "idx_session_plan_slot_unique") || strings.Contains(msg, "duplicate key") {
		return internal_errors.ErrValidationInstance.WithMessage("Já existe um slot para este dia e horário")
	}
	return err
}
