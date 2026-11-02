// Package phone centralizes phone number canonicalization so repositories and
// services compare/store phones consistently, regardless of whether they came
// from WhatsApp webhooks (already digits with country code) or the UI
// (arbitrary formats like "(11) 99999-9999", "+55 11 99999-9999", etc.).
package phone

// Canonicalize strips every non-digit character and adds the Brazil country
// code (55) when the remaining digits have the 10- or 11-digit local-number
// length. Idempotent — applying it twice yields the same string.
//
// Expected outputs for common inputs:
//
//	"5511999999999"       → "5511999999999"   (already canonical)
//	"(11) 99999-9999"     → "5511999999999"
//	"+55 11 99999-9999"   → "5511999999999"
//	"11999999999"         → "5511999999999"
//	"1199999999"          → "551199999999"    (10-digit local)
//	""                    → ""
//	"abc"                 → ""
func Canonicalize(phone string) string {
	var b []byte
	for i := 0; i < len(phone); i++ {
		c := phone[i]
		if c >= '0' && c <= '9' {
			b = append(b, c)
		}
	}
	digits := string(b)
	if len(digits) == 10 || len(digits) == 11 {
		return "55" + digits
	}
	return digits
}

// LocalDigits returns the digits-only portion without the country code when
// present. Useful when searching legacy records that may have been stored
// without the "55" prefix.
//
//	"5511999999999" → "11999999999"
//	"11999999999"   → "11999999999"
func LocalDigits(canonical string) string {
	if len(canonical) == 13 && canonical[:2] == "55" {
		return canonical[2:]
	}
	if len(canonical) == 12 && canonical[:2] == "55" {
		return canonical[2:]
	}
	return canonical
}
