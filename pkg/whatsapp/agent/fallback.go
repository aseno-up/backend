package agent

import (
	"context"
	"fmt"
	"github.com/aseno/backend/internal/logger"
	"sync"
	"time"
)

// FallbackHandler handles graceful degradation when AI agent fails
type FallbackHandler struct {
	mu                    sync.RWMutex
	consecutiveFailures   int
	lastFailureTime       time.Time
	circuitBreakerOpen    bool
	circuitBreakerTimeout time.Duration
	maxConsecutiveFailures int
}

// NewFallbackHandler creates a new fallback handler
func NewFallbackHandler(maxConsecutiveFailures int, circuitBreakerTimeout time.Duration) *FallbackHandler {
	return &FallbackHandler{
		circuitBreakerTimeout:  circuitBreakerTimeout,
		maxConsecutiveFailures: maxConsecutiveFailures,
	}
}

// ShouldUseFallback checks if fallback should be used instead of AI agent
func (f *FallbackHandler) ShouldUseFallback() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// If circuit breaker is open, check if timeout has elapsed
	if f.circuitBreakerOpen {
		if time.Since(f.lastFailureTime) > f.circuitBreakerTimeout {
			// Try to close circuit breaker
			return false
		}
		return true
	}

	return false
}

// RecordSuccess records a successful AI agent call
func (f *FallbackHandler) RecordSuccess() {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Reset failure count on success
	if f.consecutiveFailures > 0 {
		logger.Printf("[Fallback Handler] AI agent recovered, resetting failure count (was %d)", f.consecutiveFailures)
	}

	f.consecutiveFailures = 0

	// Close circuit breaker if it was open
	if f.circuitBreakerOpen {
		logger.Println("[Fallback Handler] Circuit breaker closed, AI agent back online")
		f.circuitBreakerOpen = false
	}
}

// RecordFailure records an AI agent failure
func (f *FallbackHandler) RecordFailure(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.consecutiveFailures++
	f.lastFailureTime = time.Now()

	logger.Printf("[Fallback Handler] AI agent failure #%d: %v", f.consecutiveFailures, err)

	// Open circuit breaker if threshold reached
	if f.consecutiveFailures >= f.maxConsecutiveFailures && !f.circuitBreakerOpen {
		f.circuitBreakerOpen = true
		logger.Printf("[Fallback Handler] Circuit breaker OPENED after %d consecutive failures. Falling back to traditional handler for %v",
			f.maxConsecutiveFailures, f.circuitBreakerTimeout)
	}
}

// GetStatus returns the current status of the fallback handler
func (f *FallbackHandler) GetStatus() *FallbackStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return &FallbackStatus{
		ConsecutiveFailures:    f.consecutiveFailures,
		LastFailureTime:        f.lastFailureTime,
		CircuitBreakerOpen:     f.circuitBreakerOpen,
		TimeSinceLastFailure:   time.Since(f.lastFailureTime),
		TimeUntilCircuitClose:  f.calculateTimeUntilCircuitClose(),
	}
}

func (f *FallbackHandler) calculateTimeUntilCircuitClose() time.Duration {
	if !f.circuitBreakerOpen {
		return 0
	}

	elapsed := time.Since(f.lastFailureTime)
	remaining := f.circuitBreakerTimeout - elapsed

	if remaining < 0 {
		return 0
	}

	return remaining
}

// FallbackStatus represents the current fallback status
type FallbackStatus struct {
	ConsecutiveFailures   int
	LastFailureTime       time.Time
	CircuitBreakerOpen    bool
	TimeSinceLastFailure  time.Duration
	TimeUntilCircuitClose time.Duration
}

// FallbackConfig holds configuration for fallback behavior
type FallbackConfig struct {
	MaxConsecutiveFailures int           // Open circuit breaker after this many failures
	CircuitBreakerTimeout  time.Duration // How long to keep circuit breaker open
	RetryDelay             time.Duration // Delay before retrying failed request
	MaxRetries             int           // Max retries before giving up
}

// DefaultFallbackConfig returns sensible defaults
func DefaultFallbackConfig() *FallbackConfig {
	return &FallbackConfig{
		MaxConsecutiveFailures: 5,
		CircuitBreakerTimeout:  5 * time.Minute,
		RetryDelay:             1 * time.Second,
		MaxRetries:             3,
	}
}

// WithRetry wraps an AI agent call with retry logic
func WithRetry(ctx context.Context, cfg *FallbackConfig, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cfg.RetryDelay):
			}

			logger.Printf("[Fallback] Retrying AI agent (attempt %d/%d)", attempt+1, cfg.MaxRetries+1)
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on context errors
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Don't retry on certain errors (add logic here)
		if !isRetryableError(err) {
			return err
		}
	}

	return fmt.Errorf("failed after %d retries: %w", cfg.MaxRetries, lastErr)
}

// isRetryableError determines if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Retry on rate limits, timeouts, and temporary failures
	retryableErrors := []string{
		"429",
		"rate limit",
		"timeout",
		"temporary",
		"unavailable",
		"resource exhausted",
	}

	for _, retryable := range retryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// FallbackResponse represents a fallback response when AI fails
type FallbackResponse struct {
	Message string
	Reason  string
}

// GetFallbackMessage returns an appropriate fallback message based on the error
func GetFallbackMessage(err error, intent string) *FallbackResponse {
	// Generic fallback messages based on intent
	switch intent {
	case "schedule":
		return &FallbackResponse{
			Message: "Desculpe, estou com dificuldades no momento. Por favor, entre em contato conosco pelo telefone para agendar sua consulta.",
			Reason:  "AI agent unavailable for scheduling",
		}
	case "confirm":
		return &FallbackResponse{
			Message: "Desculpe, não consegui processar sua confirmação. Por favor, entre em contato conosco.",
			Reason:  "AI agent unavailable for confirmation",
		}
	case "cancel":
		return &FallbackResponse{
			Message: "Desculpe, não consegui processar seu cancelamento. Por favor, entre em contato conosco.",
			Reason:  "AI agent unavailable for cancellation",
		}
	default:
		return &FallbackResponse{
			Message: "Desculpe, estou com dificuldades no momento. Por favor, tente novamente em alguns minutos ou entre em contato conosco.",
			Reason:  "AI agent unavailable",
		}
	}
}
