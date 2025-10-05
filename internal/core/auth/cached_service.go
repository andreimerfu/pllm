package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
)

// CachedAuthService wraps AuthService with caching capabilities
type CachedAuthService struct {
	authService *AuthService
	cache       *simpleCache
	logger      *zap.Logger
}

// CachedTokenClaims extends TokenClaims with permission cache
type CachedTokenClaims struct {
	*TokenClaims
	Permissions []string `json:"permissions"`
}

// simpleCache is an in-memory cache with TTL
type simpleCache struct {
	data   map[string]cacheItem
	mutex  sync.RWMutex
	logger *zap.Logger
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

// NewCachedAuthService creates a new cached auth service
func NewCachedAuthService(authService *AuthService, logger *zap.Logger) *CachedAuthService {
	cache := &simpleCache{
		data:   make(map[string]cacheItem),
		logger: logger,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return &CachedAuthService{
		authService: authService,
		cache:       cache,
		logger:      logger,
	}
}

// ValidateKeyCached validates an API key with caching
func (c *CachedAuthService) ValidateKeyCached(ctx context.Context, keyValue string) (*models.Key, error) {
	cacheKey := fmt.Sprintf("key:%s", models.HashKey(keyValue))

	// Try cache first
	if cached, found := c.cache.get(cacheKey); found {
		if keyData, ok := cached.(*models.Key); ok {
			c.logger.Debug("Key validation cache hit", zap.String("cache_key", cacheKey))
			return keyData, nil
		}
	}

	// Cache miss - validate through auth service
	key, err := c.authService.ValidateKey(ctx, keyValue)
	if err != nil {
		return nil, err
	}

	// Cache the result for 5 minutes
	c.cache.set(cacheKey, key, 5*time.Minute)
	c.logger.Debug("Key validation cached", zap.String("cache_key", cacheKey))

	return key, nil
}

// ValidateTokenCached validates a JWT token with caching and includes permissions
func (c *CachedAuthService) ValidateTokenCached(ctx context.Context, tokenString string) (*CachedTokenClaims, error) {
	cacheKey := fmt.Sprintf("token:%s", models.HashKey(tokenString))

	// Try cache first
	if cached, found := c.cache.get(cacheKey); found {
		if tokenData, ok := cached.(*CachedTokenClaims); ok {
			c.logger.Debug("Token validation cache hit", zap.String("cache_key", cacheKey))
			return tokenData, nil
		}
	}

	// Cache miss - validate through auth service
	claims, err := c.authService.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Get user permissions
	permissions, err := c.authService.GetUserPermissions(ctx, claims.UserID)
	if err != nil {
		c.logger.Warn("Failed to get user permissions", zap.Error(err))
		permissions = []string{} // Default to empty permissions
	}

	// Create cached claims with permissions
	cachedClaims := &CachedTokenClaims{
		TokenClaims: claims,
		Permissions: permissions,
	}

	// Cache the result for 10 minutes
	c.cache.set(cacheKey, cachedClaims, 10*time.Minute)
	c.logger.Debug("Token validation cached", zap.String("cache_key", cacheKey))

	return cachedClaims, nil
}

// Cache implementation methods
func (sc *simpleCache) get(key string) (interface{}, bool) {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	item, exists := sc.data[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		delete(sc.data, key)
		return nil, false
	}

	return item.value, true
}

func (sc *simpleCache) set(key string, value interface{}, ttl time.Duration) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.data[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

func (sc *simpleCache) delete(key string) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	delete(sc.data, key)
}

func (sc *simpleCache) clear() {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.data = make(map[string]cacheItem)
}

func (sc *simpleCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sc.mutex.Lock()
		now := time.Now()
		for key, item := range sc.data {
			if now.After(item.expiresAt) {
				delete(sc.data, key)
			}
		}
		sc.mutex.Unlock()
	}
}

// InvalidateKeyCache removes a key from cache
func (c *CachedAuthService) InvalidateKeyCache(keyValue string) {
	cacheKey := fmt.Sprintf("key:%s", models.HashKey(keyValue))
	c.cache.delete(cacheKey)
	c.logger.Debug("Key cache invalidated", zap.String("cache_key", cacheKey))
}

// InvalidateTokenCache removes a token from cache
func (c *CachedAuthService) InvalidateTokenCache(tokenString string) {
	cacheKey := fmt.Sprintf("token:%s", models.HashKey(tokenString))
	c.cache.delete(cacheKey)
	c.logger.Debug("Token cache invalidated", zap.String("cache_key", cacheKey))
}

// InvalidateUserCache removes all cached entries for a user
func (c *CachedAuthService) InvalidateUserCache(userID uuid.UUID) {
	c.cache.mutex.Lock()
	defer c.cache.mutex.Unlock()

	prefix := fmt.Sprintf("user:%s:", userID.String())
	for key := range c.cache.data {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			delete(c.cache.data, key)
		}
	}
	c.logger.Debug("User cache invalidated", zap.String("user_id", userID.String()))
}

// ClearCache removes all cached entries
func (c *CachedAuthService) ClearCache() {
	c.cache.clear()
	c.logger.Info("All auth cache cleared")
}

// GetCacheStats returns cache statistics
func (c *CachedAuthService) GetCacheStats() map[string]interface{} {
	c.cache.mutex.RLock()
	defer c.cache.mutex.RUnlock()

	return map[string]interface{}{
		"entries": len(c.cache.data),
	}
}
