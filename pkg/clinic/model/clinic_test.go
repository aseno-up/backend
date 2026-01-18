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
					Timezone:        "America/Sao_Paulo",
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
					Timezone:        "America/Sao_Paulo",
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
					Timezone:        "America/Sao_Paulo",
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
					Timezone:        "America/Sao_Paulo",
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
					Timezone: "America/Sao_Paulo",
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
					Timezone: "America/Sao_Paulo",
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
					Timezone: "America/Sao_Paulo",
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
					Timezone: "America/Sao_Paulo",
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
					Timezone: "America/Sao_Paulo",
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
