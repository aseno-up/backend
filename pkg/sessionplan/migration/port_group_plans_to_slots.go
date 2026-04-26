// Package migration contains one-shot data migrations for session plans.
package migration

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/sessionplan/model"
)

// weekdayAbbr maps integer weekday (0=Sun..6=Sat) to RRULE BYDAY abbreviation
// used as the key in session_plans.day_times JSON.
var weekdayAbbr = map[int]string{
	0: "SU",
	1: "MO",
	2: "TU",
	3: "WE",
	4: "TH",
	5: "FR",
	6: "SA",
}

// PortGroupPlansToSlots backfills SessionPlanSlot rows for legacy group plans
// that were created before the slot model existed. For each group plan without
// any slots, it expands (week_days × day_times[abbr]) into one row per
// (weekday, start_time) tuple.
//
// Idempotent: only runs for plans that have zero existing slots. Safe to call
// on every server start.
func PortGroupPlansToSlots(ctx context.Context, db *gorm.DB) error {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}

	var plans []model.SessionPlan
	err = db.WithContext(ctx).
		Where("type = ? AND deleted_at IS NULL", model.PlanTypeGroup).
		Where("id NOT IN (SELECT session_plan_id FROM session_plan_slots WHERE deleted_at IS NULL)").
		Find(&plans).Error
	if err != nil {
		return fmt.Errorf("port group plans: list: %w", err)
	}

	if len(plans) == 0 {
		return nil
	}

	for _, plan := range plans {
		defaultTime := "08:00"
		if plan.DtStart != nil {
			defaultTime = plan.DtStart.In(loc).Format("15:04")
		} else if !plan.StartDate.IsZero() {
			defaultTime = plan.StartDate.In(loc).Format("15:04")
		}

		capacity := 1
		if plan.Capacity != nil && *plan.Capacity > 0 {
			capacity = *plan.Capacity
		}

		// If no week_days but has day_times, derive week_days from day_times keys.
		weekDays := make([]int, 0, len(plan.WeekDays))
		for _, w := range plan.WeekDays {
			weekDays = append(weekDays, int(w))
		}
		if len(weekDays) == 0 && len(plan.DayTimes) > 0 {
			for k := range plan.DayTimes {
				for n, abbr := range weekdayAbbr {
					if abbr == k {
						weekDays = append(weekDays, n)
					}
				}
			}
		}
		if len(weekDays) == 0 {
			// Nothing to port for this plan — skip rather than fail the whole batch.
			continue
		}

		rows := make([]model.SessionPlanSlot, 0, len(weekDays))
		for _, wd := range weekDays {
			startTime := defaultTime
			if t, ok := plan.DayTimes[weekdayAbbr[wd]]; ok && t != "" {
				startTime = t
			}
			rows = append(rows, model.SessionPlanSlot{
				SessionPlanID:  plan.ID,
				Weekday:        wd,
				StartTime:      startTime,
				Capacity:       capacity,
				ProfessionalID: plan.ProfessionalID,
				IsActive:       true,
			})
		}

		if err := db.WithContext(ctx).Create(&rows).Error; err != nil {
			return fmt.Errorf("port group plans: insert slots for plan %s: %w", plan.ID, err)
		}
	}

	return nil
}
