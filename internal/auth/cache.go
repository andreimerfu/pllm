package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Cache provides caching functionality for auth-related data
type Cache struct {
	redis  *redis.Client
	logger *zap.Logger
	ttl    time.Duration
}

// CacheEntry represents a cached auth entry
type CacheEntry struct {
	Data      interface{} `json:"data"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// NewCache creates a new auth cache instance
func NewCache(redisClient *redis.Client, logger *zap.Logger, ttl time.Duration) *Cache {
	return &Cache{
		redis:  redisClient,
		logger: logger,
		ttl:    ttl,
	}
}

// Set stores data in the cache with the specified key
func (c *Cache) Set(ctx context.Context, key string, value interface{}) error {
	entry := CacheEntry{
		Data:      value,
		ExpiresAt: time.Now().Add(c.ttl),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	cacheKey := c.buildKey(key)
	if err := c.redis.Set(ctx, cacheKey, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache entry: %w", err)
	}

	c.logger.Debug("Cache entry set", zap.String("key", key))
	return nil
}

// Get retrieves data from the cache
func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
	cacheKey := c.buildKey(key)
	data, err := c.redis.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	if err != nil {
		return fmt.Errorf("failed to get cache entry: %w", err)
	}

	var entry CacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return fmt.Errorf("failed to unmarshal cache entry: %w", err)
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		c.Delete(ctx, key)
		return ErrCacheMiss
	}

	// Marshal entry data back to JSON and unmarshal into destination
	entryData, err := json.Marshal(entry.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal entry data: %w", err)
	}

	if err := json.Unmarshal(entryData, dest); err != nil {
		return fmt.Errorf("failed to unmarshal into destination: %w", err)
	}

	c.logger.Debug("Cache entry retrieved", zap.String("key", key))
	return nil
}

// Delete removes an entry from the cache
func (c *Cache) Delete(ctx context.Context, key string) error {
	cacheKey := c.buildKey(key)
	if err := c.redis.Del(ctx, cacheKey).Err(); err != nil {
		return fmt.Errorf("failed to delete cache entry: %w", err)
	}

	c.logger.Debug("Cache entry deleted", zap.String("key", key))
	return nil
}

// Exists checks if a key exists in the cache
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	cacheKey := c.buildKey(key)
	count, err := c.redis.Exists(ctx, cacheKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check cache entry existence: %w", err)
	}
	return count > 0, nil
}

// Clear removes all auth-related cache entries
func (c *Cache) Clear(ctx context.Context) error {
	pattern := c.buildKey("*")
	keys, err := c.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get cache keys: %w", err)
	}

	if len(keys) > 0 {
		if err := c.redis.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete cache entries: %w", err)
		}
	}

	c.logger.Info("Auth cache cleared", zap.Int("keys_deleted", len(keys)))
	return nil
}

// buildKey creates a cache key with the auth prefix
func (c *Cache) buildKey(key string) string {
	// Create a hash of the key to ensure consistent key format
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])[:16] // Use first 16 chars of hash
	
	return fmt.Sprintf("auth:%s:%s", hashStr, key)
}

// CacheUserPermissions caches user permissions
func (c *Cache) CacheUserPermissions(ctx context.Context, userID uint, permissions []Permission) error {
	key := fmt.Sprintf("user_permissions:%d", userID)
	return c.Set(ctx, key, permissions)
}

// GetUserPermissions retrieves cached user permissions
func (c *Cache) GetUserPermissions(ctx context.Context, userID uint) ([]Permission, error) {
	key := fmt.Sprintf("user_permissions:%d", userID)
	var permissions []Permission
	if err := c.Get(ctx, key, &permissions); err != nil {
		return nil, err
	}
	return permissions, nil
}

// CacheUserGroups caches user group memberships
func (c *Cache) CacheUserGroups(ctx context.Context, userID uint, groups []string) error {
	key := fmt.Sprintf("user_groups:%d", userID)
	return c.Set(ctx, key, groups)
}

// GetUserGroups retrieves cached user groups
func (c *Cache) GetUserGroups(ctx context.Context, userID uint) ([]string, error) {
	key := fmt.Sprintf("user_groups:%d", userID)
	var groups []string
	if err := c.Get(ctx, key, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

// InvalidateUserCache removes all cached data for a specific user
func (c *Cache) InvalidateUserCache(ctx context.Context, userID uint) error {
	keys := []string{
		fmt.Sprintf("user_permissions:%d", userID),
		fmt.Sprintf("user_groups:%d", userID),
	}

	for _, key := range keys {
		if err := c.Delete(ctx, key); err != nil {
			c.logger.Warn("Failed to delete cache entry", 
				zap.String("key", key), 
				zap.Error(err))
		}
	}

	c.logger.Debug("User cache invalidated", zap.Uint("user_id", userID))
	return nil
}

// Error definitions
var (
	ErrCacheMiss = fmt.Errorf("cache miss")
)