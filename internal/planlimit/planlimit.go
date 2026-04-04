package planlimit

// IsUnlimited returns true when the limit value represents "no cap" (conventionally -1).
func IsUnlimited(limit int) bool {
	return limit < 0
}
