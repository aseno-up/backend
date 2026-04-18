package phone

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanonicalize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"already canonical (13 digits)", "5511999999999", "5511999999999"},
		{"brazilian UI with parens and dash", "(11) 99999-9999", "5511999999999"},
		{"E.164 with plus", "+5511999999999", "5511999999999"},
		{"UI with country code spaces", "+55 11 99999-9999", "5511999999999"},
		{"11-digit local gets 55 prefix", "11999999999", "5511999999999"},
		{"10-digit local gets 55 prefix", "1199999999", "551199999999"},
		{"empty stays empty", "", ""},
		{"only letters → empty", "abc", ""},
		{"12 digits passes through (already has 55 + 10)", "551199999999", "551199999999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Canonicalize(tt.input))
		})
	}
}

func TestCanonicalize_Idempotent(t *testing.T) {
	inputs := []string{"(11) 99999-9999", "+55 11 99999-9999", "5511999999999", "11999999999", "abc"}
	for _, in := range inputs {
		once := Canonicalize(in)
		twice := Canonicalize(once)
		assert.Equal(t, once, twice, "Canonicalize must be idempotent for %q", in)
	}
}

func TestLocalDigits(t *testing.T) {
	assert.Equal(t, "11999999999", LocalDigits("5511999999999"))
	assert.Equal(t, "1199999999", LocalDigits("551199999999"))
	assert.Equal(t, "11999999999", LocalDigits("11999999999"), "no prefix → returned as-is")
	assert.Equal(t, "", LocalDigits(""))
}
