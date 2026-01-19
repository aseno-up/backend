package redis

import (
	"context"
	"fmt"
	"github.com/aseno/backend/internal/logger"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis client for conversation management
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient creates a new Redis client
func NewRedisClient(redisURL string) (*RedisClient, error) {
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
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Println("[Redis] Connected successfully")
	return &RedisClient{client: client}, nil
}

// GetClient returns the underlying Redis client
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// IsAvailable returns true if Redis is connected and available
func (r *RedisClient) IsAvailable() bool {
	if r.client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	return r.client.Ping(ctx).Err() == nil
}

// Set stores a value in Redis with optional expiration
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value from Redis
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Delete removes a key from Redis
func (r *RedisClient) Delete(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists in Redis
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}
