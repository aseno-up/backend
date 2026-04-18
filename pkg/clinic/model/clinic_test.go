package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBusinessHoursConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *BusinessHoursConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config is valid",
			config:  nil,
			wantErr: false,
		},
		{
			name: "valid config",
			config: &BusinessHoursConfig{
				Monday: &DaySchedule{
					Enabled:   true,
					StartTime: "08:00",
					EndTime:   "18:00",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid start time format",
			config: &BusinessHoursConfig{
				Monday: &DaySchedule{
					Enabled:   true,
					StartTime: "8:00",
					EndTime:   "18:00",
				},
			},
			wantErr: true,
			errMsg:  "Monday start_time has invalid format",
		},
		{
			name: "invalid end time format",
			config: &BusinessHoursConfig{
				Tuesday: &DaySchedule{
					Enabled:   true,
					StartTime: "08:00",
					EndTime:   "18:0",
				},
			},
			wantErr: true,
			errMsg:  "Tuesday end_time has invalid format",
		},
		{
			name: "start time after end time",
			config: &BusinessHoursConfig{
				Wednesday: &DaySchedule{
					Enabled:   true,
					StartTime: "18:00",
					EndTime:   "08:00",
				},
			},
			wantErr: true,
			errMsg:  "Wednesday start_time must be before end_time",
		},
		{
			name: "start time equals end time",
			config: &BusinessHoursConfig{
				Thursday: &DaySchedule{
					Enabled:   true,
					StartTime: "12:00",
					EndTime:   "12:00",
				},
			},
			wantErr: true,
			errMsg:  "Thursday start_time must be before end_time",
		},
		{
			name: "disabled day is valid even with invalid times",
			config: &BusinessHoursConfig{
				Friday: &DaySchedule{
					Enabled:   false,
					StartTime: "invalid",
					EndTime:   "invalid",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClinic_IsWithinBusinessHours(t *testing.T) {
	// Helper para criar DaySchedule
	makeSchedule := func(enabled bool, start, end string) *DaySchedule {
		return &DaySchedule{
			Enabled:   enabled,
			StartTime: start,
			EndTime:   end,
		}
	}

	tests := []struct {
		name     string
		clinic   *Clinic
		testTime time.Time
		want     bool
	}{
		{
			name: "default hours - Monday 10:00",
			clinic: &Clinic{
				Settings: ClinicSettings{
					Timezone:        "UTC",
					AIBusinessHours: nil, // usar padrão
				},
			},
			testTime: time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC), // Monday
			want:     true,
		},
		{
			name: "default hours - Sunday (disabled)",
			clinic: &Clinic{
				Settings: ClinicSettings{
					Timezone:        "UTC",
					AIBusinessHours: nil,
				},
			},
			testTime: time.Date(2024, 1, 7, 10, 0, 0, 0, time.UTC), // Sunday
			want:     false,
		},
		{
			name: "default hours - Monday 07:00 (before start)",
			clinic: &Clinic{
				Settings: ClinicSettings{
					Timezone:        "UTC",
					AIBusinessHours: nil,
				},
			},
			testTime: time.Date(2024, 1, 8, 7, 0, 0, 0, time.UTC), // Monday 07:00
			want:     false,
		},
		{
			name: "default hours - Monday 19:00 (after end)",
			clinic: &Clinic{
				Settings: ClinicSettings{
					Timezone:        "UTC",
					AIBusinessHours: nil,
				},
			},
			testTime: time.Date(2024, 1, 8, 19, 0, 0, 0, time.UTC), // Monday 19:00
			want:     false,
		},
		{
			name: "custom hours - Monday enabled 09:00-17:00, testing 10:00",
			clinic: &Clinic{
				Settings: ClinicSettings{
					Timezone: "UTC",
					AIBusinessHours: &BusinessHoursConfig{
						Monday: makeSchedule(true, "09:00", "17:00"),
					},
				},
			},
			testTime: time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC), // Monday 10:00
			want:     true,
		},
		{
			name: "custom hours - Monday disabled",
			clinic: &Clinic{
				Settings: ClinicSettings{
					Timezone: "UTC",
					AIBusinessHours: &BusinessHoursConfig{
						Monday: makeSchedule(false, "09:00", "17:00"),
					},
				},
			},
			testTime: time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC), // Monday 10:00
			want:     false,
		},
		{
			name: "custom hours - Tuesday enabled, testing before start",
			clinic: &Clinic{
				Settings: ClinicSettings{
					Timezone: "UTC",
					AIBusinessHours: &BusinessHoursConfig{
						Tuesday: makeSchedule(true, "10:00", "16:00"),
					},
				},
			},
			testTime: time.Date(2024, 1, 9, 9, 30, 0, 0, time.UTC), // Tuesday 09:30
			want:     false,
		},
		{
			name: "custom hours - Tuesday enabled, testing after end",
			clinic: &Clinic{
				Settings: ClinicSettings{
					Timezone: "UTC",
					AIBusinessHours: &BusinessHoursConfig{
						Tuesday: makeSchedule(true, "10:00", "16:00"),
					},
				},
			},
			testTime: time.Date(2024, 1, 9, 16, 30, 0, 0, time.UTC), // Tuesday 16:30
			want:     false,
		},
		{
			name: "custom hours - Wednesday nil schedule",
			clinic: &Clinic{
				Settings: ClinicSettings{
					Timezone: "UTC",
					AIBusinessHours: &BusinessHoursConfig{
						Wednesday: nil,
					},
				},
			},
			testTime: time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC), // Wednesday 12:00
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.clinic.IsWithinBusinessHours(tt.testTime)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClinic_GetMinDelayBetweenMessages(t *testing.T) {
	tests := []struct {
		name         string
		clinic       *Clinic
		defaultDelay int
		want         int
	}{
		{
			name: "use clinic delay when set",
			clinic: &Clinic{
				Settings: ClinicSettings{
					AIMinDelayBetweenMessages: intPtr(5),
				},
			},
			defaultDelay: 3,
			want:         5,
		},
		{
			name: "use default when clinic delay is nil",
			clinic: &Clinic{
				Settings: ClinicSettings{
					AIMinDelayBetweenMessages: nil,
				},
			},
			defaultDelay: 3,
			want:         3,
		},
		{
			name: "use default when clinic delay is zero",
			clinic: &Clinic{
				Settings: ClinicSettings{
					AIMinDelayBetweenMessages: intPtr(0),
				},
			},
			defaultDelay: 3,
			want:         3,
		},
		{
			name: "use default when clinic delay is negative",
			clinic: &Clinic{
				Settings: ClinicSettings{
					AIMinDelayBetweenMessages: intPtr(-5),
				},
			},
			defaultDelay: 3,
			want:         3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.clinic.GetMinDelayBetweenMessages(tt.defaultDelay)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClinic_GetMinDelayBetweenPatients(t *testing.T) {
	tests := []struct {
		name         string
		clinic       *Clinic
		defaultDelay int
		want         int
	}{
		{
			name: "use clinic delay when set",
			clinic: &Clinic{
				Settings: ClinicSettings{
					AIMinDelayBetweenPatients: intPtr(120),
				},
			},
			defaultDelay: 60,
			want:         120,
		},
		{
			name: "use default when clinic delay is nil",
			clinic: &Clinic{
				Settings: ClinicSettings{
					AIMinDelayBetweenPatients: nil,
				},
			},
			defaultDelay: 60,
			want:         60,
		},
		{
			name: "use default when clinic delay is zero",
			clinic: &Clinic{
				Settings: ClinicSettings{
					AIMinDelayBetweenPatients: intPtr(0),
				},
			},
			defaultDelay: 60,
			want:         60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.clinic.GetMinDelayBetweenPatients(tt.defaultDelay)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func TestClinicSettings_IsAITool_EnabledByDefault(t *testing.T) {
	s := ClinicSettings{}
	assert.True(t, s.IsAIToolScheduleEnabled(), "nil pointer must default to true for backwards compat")
	assert.True(t, s.IsAIToolConfirmEnabled())
	assert.True(t, s.IsAIToolCancelEnabled())
	assert.True(t, s.IsAIToolRescheduleEnabled())
}

func TestClinicSettings_IsAITool_RespectsExplicitValues(t *testing.T) {
	tests := []struct {
		name   string
		ptr    *bool
		wantOn bool
	}{
		{"nil defaults to true", nil, true},
		{"true stays true", boolPtr(true), true},
		{"false stays false", boolPtr(false), false},
	}
	for _, tt := range tests {
		t.Run("schedule: "+tt.name, func(t *testing.T) {
			s := ClinicSettings{AIToolSchedule: tt.ptr}
			assert.Equal(t, tt.wantOn, s.IsAIToolScheduleEnabled())
		})
		t.Run("confirm: "+tt.name, func(t *testing.T) {
			s := ClinicSettings{AIToolConfirm: tt.ptr}
			assert.Equal(t, tt.wantOn, s.IsAIToolConfirmEnabled())
		})
		t.Run("cancel: "+tt.name, func(t *testing.T) {
			s := ClinicSettings{AIToolCancel: tt.ptr}
			assert.Equal(t, tt.wantOn, s.IsAIToolCancelEnabled())
		})
		t.Run("reschedule: "+tt.name, func(t *testing.T) {
			s := ClinicSettings{AIToolReschedule: tt.ptr}
			assert.Equal(t, tt.wantOn, s.IsAIToolRescheduleEnabled())
		})
	}
}

func TestClinicSettings_GetHumanTakeoverPauseDuration(t *testing.T) {
	tests := []struct {
		name  string
		hours *int
		want  time.Duration
	}{
		{"nil uses default 4h", nil, 4 * time.Hour},
		{"explicit 1h", intPtr(1), 1 * time.Hour},
		{"explicit 24h", intPtr(24), 24 * time.Hour},
		{"zero disables (returns 0)", intPtr(0), 0},
		{"negative disables (returns 0)", intPtr(-1), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ClinicSettings{AIHumanTakeoverPauseHours: tt.hours}
			assert.Equal(t, tt.want, s.GetHumanTakeoverPauseDuration())
		})
	}
}

func TestClinicSettings_GetHumanTakeoverPauseDuration_NilReceiver(t *testing.T) {
	var s *ClinicSettings
	assert.Equal(t, 4*time.Hour, s.GetHumanTakeoverPauseDuration(),
		"nil settings pointer must still return the default")
}

func TestClinicSettings_IsAITool_EachFlagIsIndependent(t *testing.T) {
	s := ClinicSettings{
		AIToolSchedule:   boolPtr(false),
		AIToolReschedule: boolPtr(false),
	}
	assert.False(t, s.IsAIToolScheduleEnabled())
	assert.False(t, s.IsAIToolRescheduleEnabled())
	assert.True(t, s.IsAIToolConfirmEnabled(), "confirm unchanged (nil) must stay true")
	assert.True(t, s.IsAIToolCancelEnabled(), "cancel unchanged (nil) must stay true")
}
