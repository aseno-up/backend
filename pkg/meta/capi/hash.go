// Package capi implements a client for the Meta Conversions API.
//
// PII fields sent to Meta (email, phone, external_id) must arrive as
// lowercase-hex SHA-256 digests of the normalized value. The helpers in this
// file produce those digests and return "" when the input is empty/has no
// usable content, so callers can omit the field instead of sending a hash of
// the empty string (which silently degrades match quality).
package capi

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// HashSHA256Hex returns the lowercase hex SHA-256 of s, or "" when s is empty.
func HashSHA256Hex(s string) string {
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// HashEmail normalizes (trim + lowercase) then SHA-256-hex-hashes the address.
// Returns "" when the input is empty or only whitespace.
func HashEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ""
	}
	return HashSHA256Hex(normalized)
}

// HashPhone strips every non-digit, then SHA-256-hex-hashes the result.
// Meta expects E.164 without the leading "+". Returns "" when the input has
// no digits.
func HashPhone(phone string) string {
	var b strings.Builder
	b.Grow(len(phone))
	for _, r := range phone {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return ""
	}
	return HashSHA256Hex(digits)
}
