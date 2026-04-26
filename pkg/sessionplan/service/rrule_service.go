package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/google/uuid"
	rrule "github.com/teambition/rrule-go"
)

// weekdayAbbrMap maps integer day of week to RRULE BYDAY abbreviation
var weekdayAbbrMap = map[int]string{
	0: "SU",
	1: "MO",
	2: "TU",
	3: "WE",
	4: "TH",
	5: "FR",
	6: "SA",
}

// FrequencyToRRule converts the legacy Frequency enum + WeekDays to an RRULE string.
func FrequencyToRRule(freq model.Frequency, weekDays []int64) string {
	switch freq {
	case model.FrequencyDaily:
		return "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR"
	case model.FrequencyWeekly:
		return "FREQ=WEEKLY;BYDAY=" + weekDaysToBYDAY(weekDays, []int{1})
	case model.FrequencyBiweekly:
		return "FREQ=WEEKLY;BYDAY=" + weekDaysToBYDAY(weekDays, []int{1, 4})
	case model.FrequencyTriweekly:
		return "FREQ=WEEKLY;BYDAY=" + weekDaysToBYDAY(weekDays, []int{1, 3, 5})
	case model.FrequencyBimonthly:
		return "FREQ=WEEKLY;INTERVAL=2;BYDAY=" + weekDaysToBYDAY(weekDays, []int{1})
	case model.FrequencyMonthly:
		return "FREQ=MONTHLY"
	case model.FrequencyCustom:
		return "FREQ=WEEKLY;BYDAY=" + weekDaysToBYDAY(weekDays, nil)
	default:
		return "FREQ=WEEKLY;BYDAY=MO"
	}
}

// weekDaysToBYDAY converts an int64 array of weekdays to a BYDAY string.
// If weekDays is empty, uses defaults.
func weekDaysToBYDAY(weekDays []int64, defaults []int) string {
	days := weekDays
	if len(days) == 0 && len(defaults) > 0 {
		for _, d := range defaults {
			days = append(days, int64(d))
		}
	}
	if len(days) == 0 {
		return "MO"
	}

	parts := make([]string, 0, len(days))
	for _, d := range days {
		if abbr, ok := weekdayAbbrMap[int(d)]; ok {
			parts = append(parts, abbr)
		}
	}
	return strings.Join(parts, ",")
}

// ValidateRRule validates an RRULE string by attempting to parse it.
func ValidateRRule(rruleStr string) error {
	if rruleStr == "" {
		return fmt.Errorf("rrule string is empty")
	}
	_, err := rrule.StrToROption("RRULE:" + rruleStr)
	if err != nil {
		return fmt.Errorf("invalid rrule: %w", err)
	}
	return nil
}

// CalculateOccurrences returns the dates of sessions within the given range.
// dtstart is the first session datetime.
// totalSessions limits the total occurrences from the start (-1 = no limit).
// sessionsDone is subtracted from totalSessions to determine remaining.
func CalculateOccurrences(rruleStr string, dtstart time.Time, rangeStart, rangeEnd time.Time, totalSessions, sessionsDone int) ([]time.Time, error) {
	opt, err := rrule.StrToROption("RRULE:" + rruleStr)
	if err != nil {
		return nil, fmt.Errorf("invalid rrule %q: %w", rruleStr, err)
	}

	opt.Dtstart = dtstart
	rule, err := rrule.NewRRule(*opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create rrule: %w", err)
	}

	// Get all occurrences from dtstart to rangeEnd, then cap at totalSessions.
	// The caller (CalendarFacade) handles suppression of already-materialized dates,
	// so sessionsDone is not used here — the totalSessions cap is sufficient.
	var allDates []time.Time
	if totalSessions > 0 {
		allDates = rule.Between(dtstart.Add(-time.Second), rangeEnd.Add(time.Second), true)
		if len(allDates) > totalSessions {
			allDates = allDates[:totalSessions]
		}
	} else {
		// Indeterminate: get all in range
		allDates = rule.Between(dtstart.Add(-time.Second), rangeEnd.Add(time.Second), true)
	}

	// Filter to only dates within the requested range
	var result []time.Time
	for _, d := range allDates {
		if !d.Before(rangeStart) && !d.After(rangeEnd) {
			result = append(result, d)
		}
	}

	return result, nil
}

// CalculateOccurrencesWithDayTimes calculates occurrences using per-day time overrides.
// For each weekday in the RRULE, it uses the corresponding time from dayTimes if present,
// otherwise falls back to dtstart's time. Results are sorted chronologically.
// dtstart is still respected as the lower bound — no occurrences are generated before it.
func CalculateOccurrencesWithDayTimes(rruleStr string, dtstart time.Time, dayTimes map[string]string, rangeStart, rangeEnd time.Time, totalSessions, sessionsDone int) ([]time.Time, error) {
	if len(dayTimes) == 0 {
		return CalculateOccurrences(rruleStr, dtstart, rangeStart, rangeEnd, totalSessions, sessionsDone)
	}

	// Parse the RRULE to extract BYDAY values
	opt, err := rrule.StrToROption("RRULE:" + rruleStr)
	if err != nil {
		return nil, fmt.Errorf("invalid rrule %q: %w", rruleStr, err)
	}

	// Extract BYDAY from the rrule string directly (avoids unexported field issues)
	byday := extractBYDAY(rruleStr)
	if byday == "" {
		return CalculateOccurrences(rruleStr, dtstart, rangeStart, rangeEnd, totalSessions, sessionsDone)
	}

	type dayOccurrence struct {
		abbr string
		time string // HH:MM
	}

	// day_times are always in clinic local time (America/Sao_Paulo).
	// dtstart from the DB may be in UTC, so we must build overrides in local tz.
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	if loc == nil {
		loc = time.UTC
	}
	dtstartLocal := dtstart.In(loc)

	defaultTime := dtstartLocal.Format("15:04")
	var dayOverrides []dayOccurrence
	for _, abbr := range strings.Split(byday, ",") {
		abbr = strings.TrimSpace(abbr)
		t := defaultTime
		if override, ok := dayTimes[abbr]; ok {
			t = override
		}
		dayOverrides = append(dayOverrides, dayOccurrence{abbr: abbr, time: t})
	}

	_ = opt // parsed only for validation

	// Use day-level boundaries so time overrides don't get excluded by the hour-level range
	dayRangeEnd := time.Date(rangeEnd.Year(), rangeEnd.Month(), rangeEnd.Day(), 23, 59, 59, 0, rangeEnd.Location())

	// Calculate occurrences per day with individual dtstart times.
	// We generate from dayDtstart (plan start) to dayRangeEnd so that the
	// totalSessions cap counts ALL occurrences from the beginning of the plan,
	// not just those in the visible calendar range.
	var allDates []time.Time
	for _, dayOvr := range dayOverrides {
		// Build a single-day RRULE
		singleDayRRule := strings.Replace(rruleStr, "BYDAY="+extractBYDAY(rruleStr), "BYDAY="+dayOvr.abbr, 1)

		// Build dtstart with this day's time in local timezone
		hour, min := 0, 0
		fmt.Sscanf(dayOvr.time, "%d:%d", &hour, &min)
		dayDtstart := time.Date(dtstartLocal.Year(), dtstartLocal.Month(), dtstartLocal.Day(), hour, min, 0, 0, loc)

		// Use dayDtstart as rangeStart (not the calendar rangeStart) so we get
		// ALL occurrences from plan start. This is needed to correctly count
		// occurrences against the totalSessions limit.
		dates, err := CalculateOccurrences(singleDayRRule, dayDtstart, dayDtstart, dayRangeEnd, -1, 0)
		if err != nil {
			continue
		}
		allDates = append(allDates, dates...)
	}

	// Sort chronologically
	sort.Slice(allDates, func(i, j int) bool {
		return allDates[i].Before(allDates[j])
	})

	// Apply total_sessions limit: cap at the first N sessions from plan start.
	// The caller (CalendarFacade) handles suppression of already-materialized dates.
	if totalSessions > 0 {
		if len(allDates) > totalSessions {
			allDates = allDates[:totalSessions]
		}
	}

	// Filter to only dates within the requested range using day-level boundaries
	// so that time overrides don't get excluded by hour-level comparisons.
	dayRangeStart := time.Date(rangeStart.Year(), rangeStart.Month(), rangeStart.Day(), 0, 0, 0, 0, rangeStart.Location())
	dayFilterEnd := time.Date(rangeEnd.Year(), rangeEnd.Month(), rangeEnd.Day(), 23, 59, 59, 0, rangeEnd.Location())
	var result []time.Time
	for _, d := range allDates {
		if !d.Before(dayRangeStart) && !d.After(dayFilterEnd) {
			result = append(result, d)
		}
	}

	return result, nil
}

// SlotOccurrence is one rendered occurrence of a SessionPlanSlot.
type SlotOccurrence struct {
	SlotID    uuid.UUID
	StartTime time.Time
}

// CalculateOccurrencesFromSlots emits one occurrence per active slot per
// matching weekday in [rangeStart, rangeEnd]. Each slot uses its own
// (weekday, start_time). Multiple slots on the same weekday produce multiple
// occurrences for that day. Results are sorted chronologically.
//
// totalSessions caps the total returned occurrences (across all slots) when
// the plan is finite (>0). For indeterminate plans (-1 or 0) no cap is applied.
// dtstart is respected as the lower bound — no occurrences before it.
func CalculateOccurrencesFromSlots(
	slots []model.SessionPlanSlot,
	dtstart, rangeStart, rangeEnd time.Time,
	totalSessions, sessionsDone int,
) ([]SlotOccurrence, error) {
	if len(slots) == 0 {
		return nil, nil
	}

	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	dtstartLocal := dtstart.In(loc)

	// Use day-level boundaries so the lower-bound cut doesn't drop occurrences
	// based on time-of-day comparisons.
	dayRangeEnd := time.Date(rangeEnd.Year(), rangeEnd.Month(), rangeEnd.Day(), 23, 59, 59, 0, rangeEnd.Location())

	var all []SlotOccurrence
	for _, slot := range slots {
		if !slot.IsActive {
			continue
		}
		abbr, ok := weekdayAbbrMap[slot.Weekday]
		if !ok {
			continue
		}
		hour, min := 0, 0
		fmt.Sscanf(slot.StartTime, "%d:%d", &hour, &min)
		slotDtstart := time.Date(
			dtstartLocal.Year(), dtstartLocal.Month(), dtstartLocal.Day(),
			hour, min, 0, 0, loc,
		)
		// If the plan starts on a date earlier than the first matching weekday,
		// the rrule library will compute the first valid occurrence on or after dtstart.
		rruleStr := "FREQ=WEEKLY;BYDAY=" + abbr
		dates, err := CalculateOccurrences(rruleStr, slotDtstart, slotDtstart, dayRangeEnd, -1, 0)
		if err != nil {
			continue
		}
		for _, d := range dates {
			all = append(all, SlotOccurrence{SlotID: slot.ID, StartTime: d})
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].StartTime.Before(all[j].StartTime)
	})

	if totalSessions > 0 && len(all) > totalSessions {
		all = all[:totalSessions]
	}

	dayRangeStart := time.Date(rangeStart.Year(), rangeStart.Month(), rangeStart.Day(), 0, 0, 0, 0, rangeStart.Location())
	dayFilterEnd := time.Date(rangeEnd.Year(), rangeEnd.Month(), rangeEnd.Day(), 23, 59, 59, 0, rangeEnd.Location())
	out := make([]SlotOccurrence, 0, len(all))
	for _, occ := range all {
		if occ.StartTime.Before(dayRangeStart) || occ.StartTime.After(dayFilterEnd) {
			continue
		}
		out = append(out, occ)
	}
	return out, nil
}

// extractBYDAY extracts the BYDAY value from an RRULE string.
func extractBYDAY(rruleStr string) string {
	for _, part := range strings.Split(rruleStr, ";") {
		if strings.HasPrefix(part, "BYDAY=") {
			return strings.TrimPrefix(part, "BYDAY=")
		}
	}
	return ""
}

// BuildDtStart combines a date string (YYYY-MM-DD) and time string (HH:MM) into a time.Time.
// Parses in America/Sao_Paulo timezone so the weekday matches the user's local date.
func BuildDtStart(dateStr, timeStr string) (time.Time, error) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		// Fallback to UTC if timezone data is unavailable
		loc = time.UTC
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+timeStr, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date/time: %w", err)
	}
	return t, nil
}
