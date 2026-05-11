package repository

import (
	"strings"
	"testing"
)

// Regression: AcquireCodeLock must call a pg_advisory_xact_lock signature
// that actually exists in PostgreSQL. The two-argument form is
// (integer, integer), NOT (bigint, bigint). Passing two bigint values
// (e.g. via hashtextextended + ?::bigint) makes the call fail at runtime
// with: "function pg_advisory_xact_lock(bigint, bigint) does not exist
// (SQLSTATE 42883)", which broke POST /api/v1/sales-packages in prod.
//
// To stay safe, the production query uses the single-argument form
// pg_advisory_xact_lock(bigint), built from hashtextextended over a
// composite "clinic:year" string. This test guards that contract.
func TestAcquireCodeLockSQL_UsesValidSignature(t *testing.T) {
	sql := acquireCodeLockSQL

	if !strings.Contains(sql, "pg_advisory_xact_lock(") {
		t.Fatalf("expected pg_advisory_xact_lock call, got: %q", sql)
	}

	// Must NOT use the broken two-bigint pattern.
	if strings.Contains(sql, "::bigint)") && strings.Contains(sql, "hashtextextended") &&
		strings.Count(sql, ",") >= 2 {
		// Heuristic: original broken SQL had hashtextextended(?, 0), ?::bigint
		// — two comma-separated args at the outer call. Flag that shape.
		outer := extractOuterArgs(sql, "pg_advisory_xact_lock(")
		if strings.Count(outer, ",") >= 1 {
			t.Fatalf("pg_advisory_xact_lock must use the single-argument (bigint) form; "+
				"the two-argument form requires (integer, integer), not bigint. got: %q", sql)
		}
	}
}

// extractOuterArgs returns the substring inside the parentheses of the
// first occurrence of fn(, balancing nested parens. Returns "" if not found.
func extractOuterArgs(sql, fn string) string {
	i := strings.Index(sql, fn)
	if i < 0 {
		return ""
	}
	start := i + len(fn)
	depth := 1
	for j := start; j < len(sql); j++ {
		switch sql[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return sql[start:j]
			}
		}
	}
	return ""
}
