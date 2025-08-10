package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	client *redis.Client
	ctx    = context.Background()
)

type Config struct {
	RedisURL string
	Password string
	DB       int
	TTL      time.Duration
	MaxSize  int
}

type Cache interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
	Exists(key string) bool
	Clear() error
}

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

func Initialize(cfg *Config) error {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("failed to parse redis URL: %w", err)
	}
	
	if cfg.Password != "" {
		opt.Password = cfg.Password
	}
	if cfg.DB != 0 {
		opt.DB = cfg.DB
	}
	
	client = redis.NewClient(opt)
	
	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	
	return nil
}

func NewRedisCache(ttl time.Duration) *RedisCache {
	return &RedisCache{
		client: client,
		ttl:    ttl,
	}
}

func (c *RedisCache) Get(key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (c *RedisCache) Set(key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.ttl
	}
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Delete(key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) Exists(key string) bool {
	exists, _ := c.client.Exists(ctx, key).Result()
	return exists > 0
}

func (c *RedisCache) Clear() error {
	return c.client.FlushDB(ctx).Err()
}

func (c *RedisCache) GetJSON(key string, dest interface{}) error {
	data, err := c.Get(key)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	return json.Unmarshal(data, dest)
}

func (c *RedisCache) SetJSON(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(key, data, ttl)
}

func GenerateCacheKey(prefix string, params map[string]interface{}) string {
	data, _ := json.Marshal(params)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%s:%s", prefix, hex.EncodeToString(hash[:]))
}

func GeneratePromptCacheKey(provider, model, prompt string, params map[string]interface{}) string {
	combined := map[string]interface{}{
		"provider": provider,
		"model":    model,
		"prompt":   prompt,
		"params":   params,
	}
	return GenerateCacheKey("prompt", combined)
}

func Close() error {
	if client != nil {
		return client.Close()
	}
	return nil
}

func GetClient() *redis.Client {
	return client
}

func IsHealthy() bool {
	if client == nil {
		return false
	}
	
	if err := client.Ping(ctx).Err(); err != nil {
		return false
	}
	
	return true
}

type CacheStats struct {
	Hits   int64   `json:"hits"`
	Misses int64   `json:"misses"`
	HitRate float64 `json:"hit_rate"`
	Size   int64   `json:"size"`
	Keys   int64   `json:"keys"`
}

func GetStats() (*CacheStats, error) {
	if client == nil {
		return nil, fmt.Errorf("cache not initialized")
	}
	
	// TODO: Parse Redis INFO stats
	// info := client.Info(ctx, "stats")
	// This is simplified, actual implementation would parse the INFO response
	
	keys, _ := client.DBSize(ctx).Result()
	
	return &CacheStats{
		Keys: keys,
	}, nil
}

type InMemoryCache struct {
	data map[string]cacheItem
	ttl  time.Duration
}

type cacheItem struct {
	value     []byte
	expiresAt time.Time
}

func NewInMemoryCache(ttl time.Duration) *InMemoryCache {
	cache := &InMemoryCache{
		data: make(map[string]cacheItem),
		ttl:  ttl,
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

func (c *InMemoryCache) Get(key string) ([]byte, error) {
	item, exists := c.data[key]
	if !exists {
		return nil, nil
	}
	
	if time.Now().After(item.expiresAt) {
		delete(c.data, key)
		return nil, nil
	}
	
	return item.value, nil
}

func (c *InMemoryCache) Set(key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.ttl
	}
	
	c.data[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	
	return nil
}

func (c *InMemoryCache) Delete(key string) error {
	delete(c.data, key)
	return nil
}

func (c *InMemoryCache) Exists(key string) bool {
	_, exists := c.data[key]
	return exists
}

func (c *InMemoryCache) Clear() error {
	c.data = make(map[string]cacheItem)
	return nil
}

func (c *InMemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		for key, item := range c.data {
			if now.After(item.expiresAt) {
				delete(c.data, key)
			}
		}
	}
}