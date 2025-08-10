package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
	AllowN(ctx context.Context, key string, n int, limit int, window time.Duration) (bool, error)
	Reset(ctx context.Context, key string) error
	GetRemaining(ctx context.Context, key string, limit int, window time.Duration) (int, error)
}

type RedisLimiter struct {
	client *redis.Client
	log    *zap.Logger
}

func NewRedisLimiter(client *redis.Client, log *zap.Logger) *RedisLimiter {
	return &RedisLimiter{
		client: client,
		log:    log,
	}
}

func (r *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return r.AllowN(ctx, key, 1, limit, window)
}

func (r *RedisLimiter) AllowN(ctx context.Context, key string, n int, limit int, window time.Duration) (bool, error) {
	now := time.Now().UnixNano()
	windowStart := now - window.Nanoseconds()
	
	pipe := r.client.Pipeline()
	
	// Remove old entries outside the window
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	
	// Count current entries in window
	count := pipe.ZCount(ctx, key, fmt.Sprintf("%d", windowStart), "+inf")
	
	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("failed to check rate limit: %w", err)
	}
	
	// Check if adding n requests would exceed limit
	currentCount, err := count.Result()
	if err != nil {
		return false, fmt.Errorf("failed to get count: %w", err)
	}
	
	if currentCount+int64(n) > int64(limit) {
		return false, nil
	}
	
	// Add new entries
	members := make([]redis.Z, n)
	for i := 0; i < n; i++ {
		members[i] = redis.Z{
			Score:  float64(now + int64(i)), // Slightly different timestamps
			Member: fmt.Sprintf("%d-%d", now, i),
		}
	}
	
	if err := r.client.ZAdd(ctx, key, members...).Err(); err != nil {
		return false, fmt.Errorf("failed to add rate limit entry: %w", err)
	}
	
	// Set expiry
	r.client.Expire(ctx, key, window)
	
	return true, nil
}

func (r *RedisLimiter) Reset(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisLimiter) GetRemaining(ctx context.Context, key string, limit int, window time.Duration) (int, error) {
	now := time.Now().UnixNano()
	windowStart := now - window.Nanoseconds()
	
	// Remove old entries and count current
	pipe := r.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	count := pipe.ZCount(ctx, key, fmt.Sprintf("%d", windowStart), "+inf")
	
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("failed to get remaining: %w", err)
	}
	
	currentCount, err := count.Result()
	if err != nil {
		return 0, err
	}
	
	remaining := limit - int(currentCount)
	if remaining < 0 {
		remaining = 0
	}
	
	return remaining, nil
}

// InMemoryLimiter for lite mode
type InMemoryLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
	log     *zap.Logger
}

type bucket struct {
	tokens    float64
	lastRefill time.Time
	mu        sync.Mutex
}

func NewInMemoryLimiter(log *zap.Logger) *InMemoryLimiter {
	limiter := &InMemoryLimiter{
		buckets: make(map[string]*bucket),
		log:     log,
	}
	
	// Start cleanup goroutine
	go limiter.cleanup()
	
	return limiter
}

func (l *InMemoryLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return l.AllowN(ctx, key, 1, limit, window)
}

func (l *InMemoryLimiter) AllowN(ctx context.Context, key string, n int, limit int, window time.Duration) (bool, error) {
	l.mu.Lock()
	b, exists := l.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     float64(limit),
			lastRefill: time.Now(),
		}
		l.buckets[key] = b
	}
	l.mu.Unlock()
	
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Token bucket algorithm
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	refillRate := float64(limit) / window.Seconds()
	tokensToAdd := elapsed.Seconds() * refillRate
	
	b.tokens = min(float64(limit), b.tokens+tokensToAdd)
	b.lastRefill = now
	
	if b.tokens >= float64(n) {
		b.tokens -= float64(n)
		return true, nil
	}
	
	return false, nil
}

func (l *InMemoryLimiter) Reset(ctx context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
	return nil
}

func (l *InMemoryLimiter) GetRemaining(ctx context.Context, key string, limit int, window time.Duration) (int, error) {
	l.mu.RLock()
	b, exists := l.buckets[key]
	if !exists {
		l.mu.RUnlock()
		return limit, nil
	}
	l.mu.RUnlock()
	
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Update tokens
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	refillRate := float64(limit) / window.Seconds()
	tokensToAdd := elapsed.Seconds() * refillRate
	
	b.tokens = min(float64(limit), b.tokens+tokensToAdd)
	b.lastRefill = now
	
	return int(b.tokens), nil
}

func (l *InMemoryLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for key, b := range l.buckets {
			b.mu.Lock()
			if now.Sub(b.lastRefill) > 1*time.Hour {
				delete(l.buckets, key)
			}
			b.mu.Unlock()
		}
		l.mu.Unlock()
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// FixedWindowLimiter using Redis
type FixedWindowLimiter struct {
	client *redis.Client
	log    *zap.Logger
}

func NewFixedWindowLimiter(client *redis.Client, log *zap.Logger) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		client: client,
		log:    log,
	}
}

func (f *FixedWindowLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return f.AllowN(ctx, key, 1, limit, window)
}

func (f *FixedWindowLimiter) AllowN(ctx context.Context, key string, n int, limit int, window time.Duration) (bool, error) {
	// Create window key based on current time
	windowKey := f.getWindowKey(key, window)
	
	// Increment counter
	count, err := f.client.IncrBy(ctx, windowKey, int64(n)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to increment counter: %w", err)
	}
	
	// Set expiry on first increment
	if count == int64(n) {
		f.client.Expire(ctx, windowKey, window)
	}
	
	// Check if limit exceeded
	if count > int64(limit) {
		// Rollback the increment
		f.client.DecrBy(ctx, windowKey, int64(n))
		return false, nil
	}
	
	return true, nil
}

func (f *FixedWindowLimiter) Reset(ctx context.Context, key string) error {
	pattern := fmt.Sprintf("%s:*", key)
	keys, err := f.client.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}
	
	if len(keys) > 0 {
		return f.client.Del(ctx, keys...).Err()
	}
	
	return nil
}

func (f *FixedWindowLimiter) GetRemaining(ctx context.Context, key string, limit int, window time.Duration) (int, error) {
	windowKey := f.getWindowKey(key, window)
	
	count, err := f.client.Get(ctx, windowKey).Int()
	if err == redis.Nil {
		return limit, nil
	}
	if err != nil {
		return 0, err
	}
	
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	
	return remaining, nil
}

func (f *FixedWindowLimiter) getWindowKey(key string, window time.Duration) string {
	windowStart := time.Now().Truncate(window).Unix()
	return fmt.Sprintf("%s:%d", key, windowStart)
}