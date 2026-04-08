package service

import (
	"testing"
	"time"

	"github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrequencyToRRule(t *testing.T) {
	tests := []struct {
		name     string
		freq     model.Frequency
		weekDays []int64
		expected string
	}{
		{
			name:     "daily maps to weekdays MO-FR",
			freq:     model.FrequencyDaily,
			weekDays: nil,
			expected: "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR",
		},
		{
			name:     "weekly with default weekday",
			freq:     model.FrequencyWeekly,
			weekDays: nil,
			expected: "FREQ=WEEKLY;BYDAY=MO",
		},
		{
			name:     "weekly with custom weekday",
			freq:     model.FrequencyWeekly,
			weekDays: []int64{3}, // Wednesday
			expected: "FREQ=WEEKLY;BYDAY=WE",
		},
		{
			name:     "biweekly with defaults",
			freq:     model.FrequencyBiweekly,
			weekDays: nil,
			expected: "FREQ=WEEKLY;BYDAY=MO,TH",
		},
		{
			name:     "biweekly with custom days",
			freq:     model.FrequencyBiweekly,
			weekDays: []int64{2, 5}, // Tuesday, Friday
			expected: "FREQ=WEEKLY;BYDAY=TU,FR",
		},
		{
			name:     "triweekly with defaults",
			freq:     model.FrequencyTriweekly,
			weekDays: nil,
			expected: "FREQ=WEEKLY;BYDAY=MO,WE,FR",
		},
		{
			name:     "bimonthly uses INTERVAL=2",
			freq:     model.FrequencyBimonthly,
			weekDays: nil,
			expected: "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO",
		},
		{
			name:     "monthly",
			freq:     model.FrequencyMonthly,
			weekDays: nil,
			expected: "FREQ=MONTHLY",
		},
		{
			name:     "custom with specified days",
			freq:     model.FrequencyCustom,
			weekDays: []int64{1, 3, 5}, // MO, WE, FR
			expected: "FREQ=WEEKLY;BYDAY=MO,WE,FR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FrequencyToRRule(tt.freq, tt.weekDays)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateRRule(t *testing.T) {
	tests := []struct {
		name    string
		rrule   string
		wantErr bool
	}{
		{"valid weekly", "FREQ=WEEKLY;BYDAY=MO,WE", false},
		{"valid monthly", "FREQ=MONTHLY", false},
		{"valid daily with interval", "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO", false},
		{"valid daily all weekdays", "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR", false},
		{"empty string", "", true},
		{"invalid frequency", "FREQ=INVALID", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRRule(tt.rrule)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCalculateOccurrences(t *testing.T) {
	// Use a fixed reference date: Monday, 2026-01-05
	dtstart := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

	t.Run("weekly MO produces one occurrence per week", func(t *testing.T) {
		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 0, 28) // 4 weeks

		dates, err := CalculateOccurrences("FREQ=WEEKLY;BYDAY=MO", dtstart, rangeStart, rangeEnd, -1, 0)
		require.NoError(t, err)
		assert.Len(t, dates, 5) // 5 Mondays in 4 weeks (inclusive)

		for _, d := range dates {
			assert.Equal(t, time.Monday, d.Weekday())
			assert.Equal(t, 9, d.Hour())
		}
	})

	t.Run("biweekly MO,TH produces two per week", func(t *testing.T) {
		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 0, 13) // ~2 weeks

		dates, err := CalculateOccurrences("FREQ=WEEKLY;BYDAY=MO,TH", dtstart, rangeStart, rangeEnd, -1, 0)
		require.NoError(t, err)

		// 2 weeks: MO 5, TH 8, MO 12 = at least 3 dates
		assert.GreaterOrEqual(t, len(dates), 3)
	})

	t.Run("total_sessions limits output", func(t *testing.T) {
		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(1, 0, 0) // 1 year range

		dates, err := CalculateOccurrences("FREQ=WEEKLY;BYDAY=MO", dtstart, rangeStart, rangeEnd, 5, 0)
		require.NoError(t, err)
		assert.Len(t, dates, 5)
	})

	t.Run("range filters results correctly", func(t *testing.T) {
		// Start range 2 weeks in
		rangeStart := dtstart.AddDate(0, 0, 14)
		rangeEnd := dtstart.AddDate(0, 0, 28)

		dates, err := CalculateOccurrences("FREQ=WEEKLY;BYDAY=MO", dtstart, rangeStart, rangeEnd, -1, 0)
		require.NoError(t, err)

		for _, d := range dates {
			assert.False(t, d.Before(rangeStart), "date should not be before rangeStart")
			assert.False(t, d.After(rangeEnd), "date should not be after rangeEnd")
		}
	})

	t.Run("monthly produces one per month", func(t *testing.T) {
		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 3, 0) // 3 months

		dates, err := CalculateOccurrences("FREQ=MONTHLY", dtstart, rangeStart, rangeEnd, -1, 0)
		require.NoError(t, err)
		assert.Len(t, dates, 4) // Jan, Feb, Mar, Apr
	})

	t.Run("bimonthly (INTERVAL=2 weekly) produces every other week", func(t *testing.T) {
		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 0, 42) // 6 weeks

		dates, err := CalculateOccurrences("FREQ=WEEKLY;INTERVAL=2;BYDAY=MO", dtstart, rangeStart, rangeEnd, -1, 0)
		require.NoError(t, err)
		// Every other Monday in 6 weeks: week 0, 2, 4, 6 = 4 dates
		assert.GreaterOrEqual(t, len(dates), 3)
	})

	t.Run("preserves time component from dtstart", func(t *testing.T) {
		dtstartWithTime := time.Date(2026, 1, 5, 14, 30, 0, 0, time.UTC)
		rangeStart := dtstartWithTime
		rangeEnd := dtstartWithTime.AddDate(0, 0, 7)

		dates, err := CalculateOccurrences("FREQ=WEEKLY;BYDAY=MO", dtstartWithTime, rangeStart, rangeEnd, -1, 0)
		require.NoError(t, err)
		require.NotEmpty(t, dates)

		for _, d := range dates {
			assert.Equal(t, 14, d.Hour())
			assert.Equal(t, 30, d.Minute())
		}
	})
}

func TestCalculateOccurrencesWithDayTimes(t *testing.T) {
	// Monday, 2026-01-05 at 09:00
	dtstart := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

	t.Run("different times per weekday", func(t *testing.T) {
		// MO at 08:00, WE at 14:00
		dayTimes := map[string]string{
			"MO": "08:00",
			"WE": "14:00",
		}
		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 0, 6) // 6 days (Mon-Sun, avoids next Monday)

		dates, err := CalculateOccurrencesWithDayTimes(
			"FREQ=WEEKLY;BYDAY=MO,WE", dtstart, dayTimes,
			rangeStart, rangeEnd, -1, 0,
		)
		require.NoError(t, err)
		require.Len(t, dates, 2) // MO + WE

		// Monday should be at 08:00
		assert.Equal(t, time.Monday, dates[0].Weekday())
		assert.Equal(t, 8, dates[0].Hour())
		assert.Equal(t, 0, dates[0].Minute())

		// Wednesday should be at 14:00
		assert.Equal(t, time.Wednesday, dates[1].Weekday())
		assert.Equal(t, 14, dates[1].Hour())
		assert.Equal(t, 0, dates[1].Minute())
	})

	t.Run("falls back to dtstart time when day not in map", func(t *testing.T) {
		// Only MO has override, WE uses dtstart time (09:00)
		dayTimes := map[string]string{
			"MO": "07:30",
		}
		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 0, 6) // 6 days to avoid next Monday

		dates, err := CalculateOccurrencesWithDayTimes(
			"FREQ=WEEKLY;BYDAY=MO,WE", dtstart, dayTimes,
			rangeStart, rangeEnd, -1, 0,
		)
		require.NoError(t, err)
		require.Len(t, dates, 2)

		// MO at 07:30 (override)
		assert.Equal(t, time.Monday, dates[0].Weekday())
		assert.Equal(t, 7, dates[0].Hour())
		assert.Equal(t, 30, dates[0].Minute())

		// WE at 09:00 (fallback to dtstart)
		assert.Equal(t, time.Wednesday, dates[1].Weekday())
		assert.Equal(t, 9, dates[1].Hour())
		assert.Equal(t, 0, dates[1].Minute())
	})

	t.Run("empty day_times uses dtstart for all", func(t *testing.T) {
		dates, err := CalculateOccurrencesWithDayTimes(
			"FREQ=WEEKLY;BYDAY=MO,WE", dtstart, nil,
			dtstart, dtstart.AddDate(0, 0, 6), -1, 0,
		)
		require.NoError(t, err)
		require.Len(t, dates, 2)

		for _, d := range dates {
			assert.Equal(t, 9, d.Hour())
			assert.Equal(t, 0, d.Minute())
		}
	})

	t.Run("respects total_sessions limit across days", func(t *testing.T) {
		dayTimes := map[string]string{
			"MO": "08:00",
			"WE": "14:00",
			"FR": "10:00",
		}
		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 0, 28) // 4 weeks

		dates, err := CalculateOccurrencesWithDayTimes(
			"FREQ=WEEKLY;BYDAY=MO,WE,FR", dtstart, dayTimes,
			rangeStart, rangeEnd, 5, 0,
		)
		require.NoError(t, err)
		assert.Len(t, dates, 5, "should limit to total_sessions=5")
	})

	t.Run("does not generate before dtstart", func(t *testing.T) {
		// dtstart is Monday Jan 5, but range starts a week earlier
		dayTimes := map[string]string{
			"MO": "08:00",
			"WE": "14:00",
		}
		rangeStart := dtstart.AddDate(0, 0, -7)
		rangeEnd := dtstart.AddDate(0, 0, 7)

		dates, err := CalculateOccurrencesWithDayTimes(
			"FREQ=WEEKLY;BYDAY=MO,WE", dtstart, dayTimes,
			rangeStart, rangeEnd, -1, 0,
		)
		require.NoError(t, err)

		for _, d := range dates {
			assert.False(t, d.Before(dtstart.Add(-24*time.Hour)),
				"should not generate occurrences before dtstart date: got %s", d)
		}
	})

	t.Run("results are sorted chronologically", func(t *testing.T) {
		dayTimes := map[string]string{
			"MO": "16:00",
			"WE": "08:00",
			"FR": "12:00",
		}
		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 0, 14)

		dates, err := CalculateOccurrencesWithDayTimes(
			"FREQ=WEEKLY;BYDAY=MO,WE,FR", dtstart, dayTimes,
			rangeStart, rangeEnd, -1, 0,
		)
		require.NoError(t, err)
		require.True(t, len(dates) > 1)

		for i := 1; i < len(dates); i++ {
			assert.True(t, dates[i].After(dates[i-1]),
				"dates[%d]=%s should be after dates[%d]=%s", i, dates[i], i-1, dates[i-1])
		}
	})
}

func TestBuildDtStart(t *testing.T) {
	t.Run("valid date and time parsed in BRT", func(t *testing.T) {
		dt, err := BuildDtStart("2026-01-05", "09:30")
		require.NoError(t, err)
		assert.Equal(t, 2026, dt.Year())
		assert.Equal(t, time.January, dt.Month())
		assert.Equal(t, 5, dt.Day())
		assert.Equal(t, 9, dt.Hour())
		assert.Equal(t, 30, dt.Minute())

		// Must be parsed in America/Sao_Paulo, not UTC
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		if loc != nil {
			assert.Equal(t, loc.String(), dt.Location().String())
		}
	})

	t.Run("weekday is correct for local timezone", func(t *testing.T) {
		// April 8, 2026 is Wednesday regardless of timezone
		dt, err := BuildDtStart("2026-04-08", "09:00")
		require.NoError(t, err)
		assert.Equal(t, time.Wednesday, dt.Weekday(),
			"April 8 2026 must be Wednesday in local timezone")
	})

	t.Run("invalid date", func(t *testing.T) {
		_, err := BuildDtStart("invalid", "09:30")
		assert.Error(t, err)
	})

	t.Run("invalid time", func(t *testing.T) {
		_, err := BuildDtStart("2026-01-05", "invalid")
		assert.Error(t, err)
	})
}

func TestCalculateOccurrences_StartDateMatchesUserChoice(t *testing.T) {
	// Bug: user picks Wednesday April 8 as start date with weekly frequency,
	// but first session appears on Monday instead.
	// The first occurrence MUST be on the same weekday as the start date.

	t.Run("weekly session starts on exact start date weekday", func(t *testing.T) {
		// April 8, 2026 is a Wednesday
		dtstart := time.Date(2026, 4, 8, 9, 0, 0, 0, time.UTC)
		assert.Equal(t, time.Wednesday, dtstart.Weekday(), "sanity check: April 8 2026 is Wednesday")

		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 0, 28) // 4 weeks

		rrule := "FREQ=WEEKLY;BYDAY=WE"
		dates, err := CalculateOccurrences(rrule, dtstart, rangeStart, rangeEnd, -1, 0)
		require.NoError(t, err)
		require.NotEmpty(t, dates)

		// First occurrence must be on the start date itself
		assert.Equal(t, dtstart.Format("2006-01-02"), dates[0].Format("2006-01-02"),
			"first occurrence must be on the user's chosen start date")

		// All occurrences must be on Wednesday
		for i, d := range dates {
			assert.Equal(t, time.Wednesday, d.Weekday(),
				"occurrence %d should be on Wednesday, got %s", i, d.Weekday())
		}
	})

	t.Run("weekly Tuesday start produces Tuesday sessions", func(t *testing.T) {
		// April 7, 2026 is a Tuesday
		dtstart := time.Date(2026, 4, 7, 14, 0, 0, 0, time.UTC)
		assert.Equal(t, time.Tuesday, dtstart.Weekday())

		rangeStart := dtstart
		rangeEnd := dtstart.AddDate(0, 0, 14)

		rrule := "FREQ=WEEKLY;BYDAY=TU"
		dates, err := CalculateOccurrences(rrule, dtstart, rangeStart, rangeEnd, -1, 0)
		require.NoError(t, err)
		require.NotEmpty(t, dates)

		assert.Equal(t, dtstart.Format("2006-01-02"), dates[0].Format("2006-01-02"))
		for _, d := range dates {
			assert.Equal(t, time.Tuesday, d.Weekday())
		}
	})
}

func TestIndeterminatePlanNeverCompletes(t *testing.T) {
	// Bug: indeterminate plans (total_sessions <= 0) were being marked as completed.
	// This test verifies the model correctly identifies indeterminate plans.

	t.Run("total_sessions -1 is indeterminate", func(t *testing.T) {
		plan := &model.SessionPlan{TotalSessions: -1, SessionsDone: 0}
		assert.True(t, plan.IsIndeterminate())
		assert.Equal(t, -1, plan.RemainingSessions())
		assert.Equal(t, float64(0), plan.Progress())
	})

	t.Run("total_sessions 0 is indeterminate", func(t *testing.T) {
		plan := &model.SessionPlan{TotalSessions: 0, SessionsDone: 0}
		assert.True(t, plan.IsIndeterminate())
		assert.Equal(t, -1, plan.RemainingSessions())
		assert.Equal(t, float64(0), plan.Progress())
	})

	t.Run("indeterminate plan with many sessions done is still not complete", func(t *testing.T) {
		plan := &model.SessionPlan{TotalSessions: -1, SessionsDone: 100, Status: model.PlanStatusActive}
		assert.True(t, plan.IsIndeterminate())
		assert.True(t, plan.IsActive())
		// SessionsDone > TotalSessions but plan must NOT be considered complete
		assert.False(t, plan.IsCompleted())
	})

	t.Run("determinate plan with sessions_done >= total is complete candidate", func(t *testing.T) {
		plan := &model.SessionPlan{TotalSessions: 10, SessionsDone: 10}
		assert.False(t, plan.IsIndeterminate())
		assert.True(t, plan.SessionsDone >= plan.TotalSessions)
	})
}
