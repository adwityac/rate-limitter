// internal/limitter/limiter.go
package limitter

import (
	"context"
	"fmt"
	"time"
)

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed    bool
	Remaining  int
	ResetTime  time.Time
	RetryAfter time.Duration
}

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	IsAllowed(ctx context.Context, key string, limit int, window time.Duration) (*RateLimitResult, error)
}

// Config holds rate limiter configuration
type Config struct {
	DefaultLimit  int
	DefaultWindow time.Duration
}

// RedisRateLimiter implements rate limiting using Redis
type RedisRateLimiter struct {
	client RedisClient
	config *Config
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(client RedisClient, config *Config) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		config: config,
	}
}

// IsAllowed checks if a request is allowed based on rate limits
func (r *RedisRateLimiter) IsAllowed(ctx context.Context, key string, limit int, window time.Duration) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-window)
	
	// Use sliding window log approach
	pipe := r.client.Pipeline()
	
	// Remove old entries
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%.0f", float64(windowStart.UnixNano())))
	
	// Add current request
	pipe.ZAdd(ctx, key, float64(now.UnixNano()), fmt.Sprintf("%.0f", float64(now.UnixNano())))
	
	// Count current requests in window
	countCmd := pipe.ZCard(ctx, key)
	
	// Set expiration for cleanup
	r.client.Expire(ctx, key, window+time.Minute)
	
	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("redis pipeline error: %w", err)
	}
	
	// Get count result
	count, err := countCmd.Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get count: %w", err)
	}
	
	// Calculate remaining requests
	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}
	
	// Calculate reset time (next window)
	resetTime := now.Add(window)
	
	// Calculate retry after
	retryAfter := time.Duration(0)
	if count > int64(limit) {
		retryAfter = window
	}
	
	return &RateLimitResult{
		Allowed:    count <= int64(limit),
		Remaining:  remaining,
		ResetTime:  resetTime,
		RetryAfter: retryAfter,
	}, nil
}

// Redis client interfaces
type RedisClient interface {
	Get(ctx context.Context, key string) StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) StatusCmd
	Incr(ctx context.Context, key string) IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) BoolCmd
	Del(ctx context.Context, keys ...string) IntCmd
	Close() error
	TTL(ctx context.Context, key string) DurationCmd
	Ping(ctx context.Context) StatusCmd
	HealthCheck(ctx context.Context) error
	Pipeline() Pipeline
	ZRemRangeByScore(ctx context.Context, key string, min, max string) IntCmd
	ZCard(ctx context.Context, key string) IntCmd
	ZRange(ctx context.Context, key string, start, stop int64, args ...interface{}) StringSliceCmd
	ZAdd(ctx context.Context, key string, score float64, member interface{}) IntCmd
	ZCount(ctx context.Context, key string, min, max string) IntCmd
}

// Pipeline interface - FIXED: Added missing ZAdd method
type Pipeline interface {
	ZRemRangeByScore(ctx context.Context, key string, min, max string) IntCmd
	ZCard(ctx context.Context, key string) IntCmd
	ZRange(ctx context.Context, key string, start, stop int64, args ...interface{}) StringSliceCmd
	ZAdd(ctx context.Context, key string, score float64, member interface{}) IntCmd  // <-- This was missing!
	Exec(ctx context.Context) ([]Cmd, error)
}

// Command interfaces
type StringCmd interface {
	Result() (string, error)
	Err() error
	Val() string
}

type StatusCmd interface {
	Result() (string, error)
	Err() error
	Val() string
}

type IntCmd interface {
	Result() (int64, error)
	Err() error
	Val() int64
}

type BoolCmd interface {
	Result() (bool, error)
	Err() error
	Val() bool
}

type DurationCmd interface {
	Result() (time.Duration, error)
	Err() error
	Val() time.Duration
}

type StringSliceCmd interface {
	Result() ([]string, error)
	Err() error
	Val() []string
}

type Cmd interface {
	Err() error
}
