package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOffer_IsValid(t *testing.T) {
	now := time.Now()
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	var limit5 = 5
	var uses0 = 0
	var uses5 = 5

	tests := []struct {
		name     string
		offer    Offer
		expected bool
	}{
		{
			name: "Active offer with no dates and no limits should be valid",
			offer: Offer{
				IsActive: true,
			},
			expected: true,
		},
		{
			name: "Inactive offer should be invalid",
			offer: Offer{
				IsActive: false,
			},
			expected: false,
		},
		{
			name: "Offer valid from past and until future should be valid",
			offer: Offer{
				IsActive:   true,
				ValidFrom:  &past,
				ValidUntil: &future,
			},
			expected: true,
		},
		{
			name: "Offer valid from future should be invalid",
			offer: Offer{
				IsActive:  true,
				ValidFrom: &future,
			},
			expected: false,
		},
		{
			name: "Offer valid until past should be invalid",
			offer: Offer{
				IsActive:   true,
				ValidUntil: &past,
			},
			expected: false,
		},
		{
			name: "Offer with uses below max should be valid",
			offer: Offer{
				IsActive:    true,
				MaxUses:     &limit5,
				CurrentUses: uses0,
			},
			expected: true,
		},
		{
			name: "Offer with uses equal to max should be invalid",
			offer: Offer{
				IsActive:    true,
				MaxUses:     &limit5,
				CurrentUses: uses5,
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.offer.IsValid())
		})
	}
}
