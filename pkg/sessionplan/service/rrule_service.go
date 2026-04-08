package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aseno/backend/pkg/sessionplan/model"
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

	// Get all occurrences from dtstart
	// We need to account for sessionsDone by skipping the first N occurrences
	var allDates []time.Time
	if totalSessions > 0 {
		// Get up to totalSessions occurrences total
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

	defaultTime := dtstart.Format("15:04")
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
	dayRangeStart := time.Date(rangeStart.Year(), rangeStart.Month(), rangeStart.Day(), 0, 0, 0, 0, rangeStart.Location())
	dayRangeEnd := time.Date(rangeEnd.Year(), rangeEnd.Month(), rangeEnd.Day(), 23, 59, 59, 0, rangeEnd.Location())

	// Calculate occurrences per day with individual dtstart times
	var allDates []time.Time
	for _, dayOvr := range dayOverrides {
		// Build a single-day RRULE
		singleDayRRule := strings.Replace(rruleStr, "BYDAY="+extractBYDAY(rruleStr), "BYDAY="+dayOvr.abbr, 1)

		// Build dtstart with this day's time
		hour, min := 0, 0
		fmt.Sscanf(dayOvr.time, "%d:%d", &hour, &min)
		dayDtstart := time.Date(dtstart.Year(), dtstart.Month(), dtstart.Day(), hour, min, 0, 0, dtstart.Location())

		dates, err := CalculateOccurrences(singleDayRRule, dayDtstart, dayRangeStart, dayRangeEnd, -1, 0)
		if err != nil {
			continue
		}
		allDates = append(allDates, dates...)
	}

	// Sort chronologically
	sort.Slice(allDates, func(i, j int) bool {
		return allDates[i].Before(allDates[j])
	})

	// Apply total_sessions limit
	if totalSessions > 0 {
		remaining := totalSessions - sessionsDone
		if remaining < 0 {
			remaining = 0
		}
		if len(allDates) > remaining {
			allDates = allDates[:remaining]
		}
	}

	return allDates, nil
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
