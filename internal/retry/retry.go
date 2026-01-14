package retry

import (
	"math"
	"math/rand"
	"time"
)

// Strategy defines the retry behavior configuration
type Strategy struct {
	// InitialDelay is the base delay for the first retry (e.g., 1 second)
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries (e.g., 5 minutes)
	MaxDelay time.Duration

	// Multiplier is the factor to multiply delay by each retry (typically 2.0)
	Multiplier float64

	// JitterFactor adds randomness to prevent thundering herd (0.0 to 1.0)
	// 0.5 means delay can vary by ±50%
	JitterFactor float64

	// MaxAttempts is the maximum number of retry attempts (0 = unlimited)
	MaxAttempts int
}

// DefaultStrategy returns a sensible default retry strategy
func DefaultStrategy() *Strategy {
	return &Strategy{
		InitialDelay: 2 * time.Second,
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
		JitterFactor: 0.3,
		MaxAttempts:  5,
	}
}

// WhatsAppStrategy returns a strategy optimized for WhatsApp rate limits
func WhatsAppStrategy() *Strategy {
	return &Strategy{
		InitialDelay: 5 * time.Second,
		MaxDelay:     10 * time.Minute,
		Multiplier:   2.0,
		JitterFactor: 0.25,
		MaxAttempts:  5,
	}
}

// NextDelay calculates the next retry delay based on the attempt number
// attempt is 0-indexed (0 = first retry after initial failure)
func (s *Strategy) NextDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate base delay: initialDelay * multiplier^attempt
	delay := float64(s.InitialDelay) * math.Pow(s.Multiplier, float64(attempt))

	// Cap at max delay
	if delay > float64(s.MaxDelay) {
		delay = float64(s.MaxDelay)
	}

	// Apply jitter
	if s.JitterFactor > 0 {
		jitter := delay * s.JitterFactor
		// Random value between -jitter and +jitter
		delay = delay + (rand.Float64()*2-1)*jitter
	}

	// Ensure we don't go below 0
	if delay < 0 {
		delay = float64(s.InitialDelay)
	}

	return time.Duration(delay)
}

// ShouldRetry returns true if another retry attempt should be made
func (s *Strategy) ShouldRetry(currentAttempt int) bool {
	if s.MaxAttempts == 0 {
		return true // Unlimited retries
	}
	return currentAttempt < s.MaxAttempts
}

// NextRetryAt calculates the timestamp for the next retry
func (s *Strategy) NextRetryAt(attempt int) time.Time {
	return time.Now().Add(s.NextDelay(attempt))
}

// RetryInfo holds information about a retry operation
type RetryInfo struct {
	Attempt      int           `json:"attempt"`
	MaxAttempts  int           `json:"max_attempts"`
	NextRetryAt  time.Time     `json:"next_retry_at,omitempty"`
	LastError    string        `json:"last_error,omitempty"`
	TotalElapsed time.Duration `json:"total_elapsed,omitempty"`
}

// ShouldRetry returns true if the retry info indicates another attempt should be made
func (r *RetryInfo) ShouldRetry() bool {
	if r.MaxAttempts == 0 {
		return true
	}
	return r.Attempt < r.MaxAttempts
}

// IsReady returns true if it's time to retry (NextRetryAt has passed)
func (r *RetryInfo) IsReady() bool {
	if r.NextRetryAt.IsZero() {
		return true
	}
	return time.Now().After(r.NextRetryAt)
}
