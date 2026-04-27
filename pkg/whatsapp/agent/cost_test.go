package agent

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

const usdToBrlForTests = 5.0

func approxEqual(t *testing.T, expected, got float64, tolerance float64, msg string) {
	t.Helper()
	if math.Abs(expected-got) > tolerance {
		t.Errorf("%s: expected ~%v, got %v (delta=%v, tol=%v)", msg, expected, got, math.Abs(expected-got), tolerance)
	}
}

// The previous EstimateTokenCost only knew Gemini prices, so OpenAI/Claude
// fell into a default branch that wildly underestimated cost. Production
// runs openai/gpt-4o-mini as primary, so getting that one wrong silently
// invalidates every R$ figure on the admin dashboard.
//
// All prices below are USD per 1M tokens, sourced from OpenRouter's listing
// for the model in question. usdToBrl is intentionally fixed (not an FX
// lookup) to keep cost estimates deterministic and reproducible.

func TestCostSplit_GPT4oMini_AsymmetricInputOutput(t *testing.T) {
	// gpt-4o-mini: $0.15 / 1M input, $0.60 / 1M output
	got := EstimateTokenCostSplit(1_000_000, 1_000_000, "openai/gpt-4o-mini")
	expected := (0.15 + 0.60) * usdToBrlForTests
	approxEqual(t, expected, got, 0.0001, "1M in + 1M out at gpt-4o-mini rates")
}

func TestCostSplit_GPT4oMini_OutputCostsMore(t *testing.T) {
	// 1k output should cost 4x more than 1k input for gpt-4o-mini.
	in := EstimateTokenCostSplit(1000, 0, "openai/gpt-4o-mini")
	out := EstimateTokenCostSplit(0, 1000, "openai/gpt-4o-mini")
	assert.InDelta(t, 4.0, out/in, 0.001, "output rate is 4x input rate for gpt-4o-mini")
}

func TestCostSplit_Gemini2FlashRates(t *testing.T) {
	// gemini-2.0-flash-001: $0.10 / 1M input, $0.40 / 1M output
	got := EstimateTokenCostSplit(1_000_000, 1_000_000, "google/gemini-2.0-flash-001")
	expected := (0.10 + 0.40) * usdToBrlForTests
	approxEqual(t, expected, got, 0.0001, "1M in + 1M out at gemini-2.0-flash rates")
}

func TestCostSplit_GeminiLiteIsCheaper(t *testing.T) {
	flash := EstimateTokenCostSplit(1000, 1000, "google/gemini-2.0-flash-001")
	lite := EstimateTokenCostSplit(1000, 1000, "google/gemini-2.0-flash-lite-001")
	assert.Less(t, lite, flash, "lite must be strictly cheaper than the standard flash model")
}

func TestCostSplit_ClaudeSonnetMoreExpensiveThanGPT(t *testing.T) {
	// Claude 3.5 Sonnet costs 20x more than gpt-4o-mini per token at parity.
	gpt := EstimateTokenCostSplit(10_000, 5_000, "openai/gpt-4o-mini")
	claude := EstimateTokenCostSplit(10_000, 5_000, "anthropic/claude-3.5-sonnet")
	assert.Greater(t, claude, gpt*10, "Claude Sonnet should be much more expensive than gpt-4o-mini")
}

func TestCostSplit_ZeroTokensIsZero(t *testing.T) {
	assert.Equal(t, 0.0, EstimateTokenCostSplit(0, 0, "openai/gpt-4o-mini"))
	assert.Equal(t, 0.0, EstimateTokenCostSplit(0, 0, "unknown/model"))
}

func TestCostSplit_UnknownModelStillReturnsNonzero(t *testing.T) {
	// We must not return 0 for an unknown model — that would silently hide
	// real spend in the admin dashboard. A reasonable conservative rate is
	// applied (matching gpt-4o-mini, our primary).
	cost := EstimateTokenCostSplit(1000, 1000, "some/new-model-we-have-not-mapped")
	assert.Greater(t, cost, 0.0, "unknown models must produce a non-zero estimate, not silently zero")
}

func TestCostSplit_BackwardsCompatibleEstimateTokenCost(t *testing.T) {
	// EstimateTokenCost is the legacy single-number signature. It must keep
	// returning a sensible (non-zero) number for backwards compatibility,
	// using a chat-typical 80/20 input/output split.
	got := EstimateTokenCost(1000, "openai/gpt-4o-mini")
	expected := EstimateTokenCostSplit(800, 200, "openai/gpt-4o-mini")
	assert.InDelta(t, expected, got, 0.0001, "legacy fn must match an 80/20 split through the new fn")
}
