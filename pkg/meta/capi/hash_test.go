package capi

import "testing"

// Meta CAPI requires PII fields (email, phone) to be SHA-256 hashed (lowercase
// hex) after normalization. These tests pin the normalization rules — the
// digest itself is tested via the canonical sha256("abc") vector below.

func TestHashEmail_NormalizesBeforeHashing(t *testing.T) {
	// Different surface forms of the same address must collapse to one digest.
	mixedCase := HashEmail("  Foo.Bar@Example.COM  ")
	canonical := HashSHA256Hex("foo.bar@example.com")

	if mixedCase == "" {
		t.Fatalf("HashEmail returned empty for non-empty input")
	}
	if mixedCase != canonical {
		t.Errorf("HashEmail did not lowercase+trim before hashing\n got: %s\nwant: %s", mixedCase, canonical)
	}
}

func TestHashEmail_EmptyReturnsEmpty(t *testing.T) {
	// Caller (the CAPI payload builder) decides whether to omit the field;
	// the helper must not emit a hash of the empty string.
	if got := HashEmail(""); got != "" {
		t.Errorf("HashEmail(\"\") = %q, want \"\"", got)
	}
	if got := HashEmail("   "); got != "" {
		t.Errorf("HashEmail(whitespace) = %q, want \"\"", got)
	}
}

func TestHashPhone_StripsNonDigits(t *testing.T) {
	formatted := HashPhone("+55 (11) 99999-0000")
	canonical := HashSHA256Hex("5511999990000")

	if formatted == "" {
		t.Fatalf("HashPhone returned empty for non-empty input")
	}
	if formatted != canonical {
		t.Errorf("HashPhone did not normalize to digits before hashing\n got: %s\nwant: %s", formatted, canonical)
	}
}

func TestHashPhone_EmptyOrNoDigitsReturnsEmpty(t *testing.T) {
	cases := []string{"", "   ", "abc-def", "(--)"}
	for _, in := range cases {
		if got := HashPhone(in); got != "" {
			t.Errorf("HashPhone(%q) = %q, want \"\"", in, got)
		}
	}
}

func TestHashSHA256Hex_KnownVector(t *testing.T) {
	// Canonical sha256("abc") test vector — pins the digest contract.
	got := HashSHA256Hex("abc")
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Errorf("HashSHA256Hex(\"abc\") = %q, want %q", got, want)
	}
}

func TestHashSHA256Hex_EmptyReturnsEmpty(t *testing.T) {
	if HashSHA256Hex("") != "" {
		t.Errorf("HashSHA256Hex(\"\") should return \"\" (caller decides omission)")
	}
}
