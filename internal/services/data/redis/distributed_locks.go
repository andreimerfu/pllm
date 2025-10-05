package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DistributedLock provides Redis-based distributed locking
type DistributedLock struct {
	client *redis.Client
	logger *zap.Logger
	key    string
	value  string
	ttl    time.Duration
}

// LockManager manages distributed locks
type LockManager struct {
	client *redis.Client
	logger *zap.Logger
}

// NewLockManager creates a new lock manager
func NewLockManager(client *redis.Client, logger *zap.Logger) *LockManager {
	return &LockManager{
		client: client,
		logger: logger,
	}
}

// AcquireLock attempts to acquire a distributed lock
func (lm *LockManager) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (*DistributedLock, error) {
	value, err := generateLockValue()
	if err != nil {
		return nil, fmt.Errorf("failed to generate lock value: %w", err)
	}

	key := fmt.Sprintf("lock:%s", lockKey)

	// Try to set the lock with NX (only if not exists) and EX (with expiration)
	success, err := lm.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !success {
		return nil, fmt.Errorf("lock already held: %s", lockKey)
	}

	lock := &DistributedLock{
		client: lm.client,
		logger: lm.logger,
		key:    key,
		value:  value,
		ttl:    ttl,
	}

	lm.logger.Debug("Lock acquired",
		zap.String("lock_key", lockKey),
		zap.String("redis_key", key),
		zap.Duration("ttl", ttl))

	return lock, nil
}

// TryLockWithRetry attempts to acquire a lock with retries
func (lm *LockManager) TryLockWithRetry(ctx context.Context, lockKey string, ttl time.Duration, maxRetries int, retryDelay time.Duration) (*DistributedLock, error) {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		lock, err := lm.AcquireLock(ctx, lockKey, ttl)
		if err == nil {
			return lock, nil
		}

		lastErr = err

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
				// Continue to next retry
			}
		}
	}

	return nil, fmt.Errorf("failed to acquire lock after %d retries: %w", maxRetries, lastErr)
}

// Release releases the distributed lock
func (dl *DistributedLock) Release(ctx context.Context) error {
	// Lua script to ensure we only delete the lock if we own it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := dl.client.Eval(ctx, script, []string{dl.key}, dl.value).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result.(int64) == 0 {
		dl.logger.Warn("Lock was not owned by this instance",
			zap.String("key", dl.key))
		return fmt.Errorf("lock not owned by this instance")
	}

	dl.logger.Debug("Lock released", zap.String("key", dl.key))
	return nil
}

// Extend extends the lock TTL
func (dl *DistributedLock) Extend(ctx context.Context, additionalTTL time.Duration) error {
	// Lua script to extend TTL only if we own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	newTTLSeconds := int64((dl.ttl + additionalTTL).Seconds())
	result, err := dl.client.Eval(ctx, script, []string{dl.key}, dl.value, newTTLSeconds).Result()
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	if result.(int64) == 0 {
		return fmt.Errorf("lock not owned by this instance or expired")
	}

	dl.ttl += additionalTTL
	dl.logger.Debug("Lock extended",
		zap.String("key", dl.key),
		zap.Duration("new_ttl", dl.ttl))

	return nil
}

// WithLock executes a function while holding a distributed lock
func (lm *LockManager) WithLock(ctx context.Context, lockKey string, ttl time.Duration, fn func() error) error {
	lock, err := lm.AcquireLock(ctx, lockKey, ttl)
	if err != nil {
		return fmt.Errorf("failed to acquire lock for operation: %w", err)
	}

	defer func() {
		if releaseErr := lock.Release(context.Background()); releaseErr != nil {
			lm.logger.Error("Failed to release lock",
				zap.String("lock_key", lockKey),
				zap.Error(releaseErr))
		}
	}()

	return fn()
}

// WithLockRetry executes a function while holding a lock with retries
func (lm *LockManager) WithLockRetry(ctx context.Context, lockKey string, ttl time.Duration, maxRetries int, retryDelay time.Duration, fn func() error) error {
	lock, err := lm.TryLockWithRetry(ctx, lockKey, ttl, maxRetries, retryDelay)
	if err != nil {
		return fmt.Errorf("failed to acquire lock for operation: %w", err)
	}

	defer func() {
		if releaseErr := lock.Release(context.Background()); releaseErr != nil {
			lm.logger.Error("Failed to release lock",
				zap.String("lock_key", lockKey),
				zap.Error(releaseErr))
		}
	}()

	return fn()
}

// generateLockValue generates a unique value for the lock
func generateLockValue() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// IsLockHeld checks if a lock is currently held
func (lm *LockManager) IsLockHeld(ctx context.Context, lockKey string) (bool, error) {
	key := fmt.Sprintf("lock:%s", lockKey)
	exists, err := lm.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check lock existence: %w", err)
	}
	return exists > 0, nil
}

// ForcedRelease forcefully releases a lock (use with caution)
func (lm *LockManager) ForcedRelease(ctx context.Context, lockKey string) error {
	key := fmt.Sprintf("lock:%s", lockKey)
	err := lm.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to force release lock: %w", err)
	}

	lm.logger.Warn("Lock forcefully released", zap.String("lock_key", lockKey))
	return nil
}
