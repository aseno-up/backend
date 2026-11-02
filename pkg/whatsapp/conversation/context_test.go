package conversation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanonicalizePhone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"webhook format passes through", "5511999999999", "5511999999999"},
		{"UI format with parens and dash", "(11) 99999-9999", "5511999999999"},
		{"with plus sign", "+5511999999999", "5511999999999"},
		{"10-digit local gets 55 prefix", "1199999999", "551199999999"},
		{"11-digit local gets 55 prefix", "11999999999", "5511999999999"},
		{"empty stays empty", "", ""},
		{"pure letters become empty digits", "abc", ""},
		{"idempotent on already-canonical", "5511999999999", "5511999999999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CanonicalizePhone(tt.input))
		})
	}
}

func TestCanonicalizePhone_Idempotent(t *testing.T) {
	// Applying the function twice must yield the same result.
	inputs := []string{"(11) 99999-9999", "+5511999999999", "5511999999999", "1199999999"}
	for _, in := range inputs {
		once := CanonicalizePhone(in)
		twice := CanonicalizePhone(once)
		assert.Equal(t, once, twice, "CanonicalizePhone must be idempotent for %q", in)
	}
}
