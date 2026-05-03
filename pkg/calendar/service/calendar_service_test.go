package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/internal/errors"
)

func TestClassifyCalendarLimitError(t *testing.T) {
	cases := []struct {
		name            string
		limit           int
		count           int64
		hasExisting     bool
		wantErr         bool
		wantType        string
		wantMsgContains string
	}{
		{
			name:        "existing integration always allowed (reconnect even when plan limit is zero)",
			limit:       0,
			count:       0,
			hasExisting: true,
		},
		{
			name:        "existing integration always allowed when allotment is full",
			limit:       1,
			count:       1,
			hasExisting: true,
		},
		{
			name:            "plan does not include feature surfaces a plan-tier message",
			limit:           0,
			count:           0,
			hasExisting:     false,
			wantErr:         true,
			wantType:        errors.ErrUpgradeRequired,
			wantMsgContains: "não inclui",
		},
		{
			name:            "allotment exhausted on a paid plan surfaces a limit-reached message",
			limit:           1,
			count:           1,
			hasExisting:     false,
			wantErr:         true,
			wantType:        errors.ErrUpgradeRequired,
			wantMsgContains: "limite",
		},
		{
			name:        "capacity available on a paid plan is allowed",
			limit:       1,
			count:       0,
			hasExisting: false,
		},
		{
			name:        "capacity available on a higher tier is allowed",
			limit:       3,
			count:       1,
			hasExisting: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyCalendarLimitError(tc.limit, tc.count, tc.hasExisting)

			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			appErr, ok := err.(*errors.AppError)
			require.True(t, ok, "expected *errors.AppError, got %T", err)
			assert.Equal(t, tc.wantType, appErr.Type)
			assert.Contains(
				t,
				strings.ToLower(appErr.Message),
				tc.wantMsgContains,
				"message did not contain expected substring",
			)
		})
	}

	t.Run("plan-tier and limit-reached messages are distinct", func(t *testing.T) {
		planErr := classifyCalendarLimitError(0, 0, false)
		limitErr := classifyCalendarLimitError(1, 1, false)
		require.Error(t, planErr)
		require.Error(t, limitErr)
		planAppErr := planErr.(*errors.AppError)
		limitAppErr := limitErr.(*errors.AppError)
		assert.NotEqual(
			t,
			planAppErr.Message,
			limitAppErr.Message,
			"plan-tier and limit-reached messages must differ so the user can tell what to fix",
		)
	})
}
