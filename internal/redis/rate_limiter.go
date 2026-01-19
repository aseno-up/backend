package redis

import (
	"context"
	"fmt"
	"github.com/aseno/backend/internal/logger"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter manages rate limiting using Redis
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter creates a new Redis-based rate limiter
func NewRateLimiter(redisURL string) (*RateLimiter, error) {
	// Parse Redis URL
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Printf("[Redis] Warning: Failed to connect to Redis: %v (rate limiting will be disabled)", err)
		// Don't return error - we want to continue without Redis
		return &RateLimiter{client: nil}, nil
	}

	logger.Printf("[Redis] Connected successfully")
	return &RateLimiter{client: client}, nil
}

// CheckRateLimit checks if enough time has passed since the last message to a DIFFERENT patient
// Returns the duration to wait before sending, or 0 if ready to send immediately
// The delay is only applied when sending to a different patient than the last one
func (r *RateLimiter) CheckRateLimit(ctx context.Context, clinicID string, phone string, minDelay time.Duration) time.Duration {
	if r.client == nil {
		// Redis not available, no rate limiting
		return 0
	}

	normalizedPhone := normalizePhone(phone)
	key := fmt.Sprintf("whatsapp:rate_limit:clinic:%s", clinicID)

	// Get last sent info (format: "phone:timestamp_ms")
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// No previous message, can send immediately
		return 0
	}
	if err != nil {
		logger.Printf("[Redis] Error checking rate limit: %v", err)
		return 0 // On error, allow sending
	}

	// Parse last phone and timestamp
	lastPhone, lastSentMs, err := parsePhoneAndTimestamp(val)
	if err != nil {
		logger.Printf("[Redis] Error parsing rate limit data: %v", err)
		return 0
	}

	// If sending to the SAME patient, no delay needed
	if lastPhone == normalizedPhone {
		logger.Printf("[RateLimiter] Same patient (%s), no delay needed", phone)
		return 0
	}

	// Sending to DIFFERENT patient - check if enough time has passed
	elapsed := time.Since(lastSentMs)
	if elapsed >= minDelay {
		logger.Printf("[RateLimiter] Different patient, but enough time passed (%v >= %v)", elapsed, minDelay)
		return 0 // Enough time has passed
	}

	// Need to wait before sending to different patient
	waitTime := minDelay - elapsed
	logger.Printf("[RateLimiter] Different patient (last: %s, new: %s), need to wait %v", lastPhone, phone, waitTime)
	return waitTime
}

// RecordMessageSent records that a message was sent to this phone for this clinic
func (r *RateLimiter) RecordMessageSent(ctx context.Context, clinicID string, phone string, ttl time.Duration) error {
	if r.client == nil {
		// Redis not available, skip recording
		return nil
	}

	key := fmt.Sprintf("whatsapp:rate_limit:clinic:%s", clinicID)
	normalizedPhone := normalizePhone(phone)
	now := time.Now().UnixMilli()

	// Store format: "phone:timestamp_ms"
	value := fmt.Sprintf("%s:%d", normalizedPhone, now)

	return r.client.Set(ctx, key, value, ttl).Err()
}

// Close closes the Redis connection
func (r *RateLimiter) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// IsAvailable returns true if Redis is connected and available
func (r *RateLimiter) IsAvailable() bool {
	return r.client != nil
}

// Helper functions

func normalizePhone(phone string) string {
	// Remove common separators and whitespace
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	phone = strings.ReplaceAll(phone, "+", "")
	return phone
}

func parseUnixMillis(val string) (time.Time, error) {
	var ms int64
	if _, err := fmt.Sscanf(val, "%d", &ms); err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms), nil
}

func parsePhoneAndTimestamp(val string) (phone string, timestamp time.Time, err error) {
	// Expected format: "phone:timestamp_ms"
	parts := strings.Split(val, ":")
	if len(parts) != 2 {
		return "", time.Time{}, fmt.Errorf("invalid format, expected 'phone:timestamp', got: %s", val)
	}

	phone = parts[0]
	timestamp, err = parseUnixMillis(parts[1])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return phone, timestamp, nil
}
