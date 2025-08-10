package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/cache"
	"go.uber.org/zap"
)

type CacheMiddleware struct {
	cache   cache.Cache
	config  *config.CacheConfig
	log     *zap.Logger
	enabled bool
}

type CachedResponse struct {
	Body       json.RawMessage    `json:"body"`
	Headers    map[string]string  `json:"headers"`
	StatusCode int                `json:"status_code"`
	CachedAt   time.Time          `json:"cached_at"`
	Model      string             `json:"model"`
	Provider   string             `json:"provider"`
}

type responseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	written    bool
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.body.Write(b)
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

func NewCacheMiddleware(cfg *config.Config, log *zap.Logger) *CacheMiddleware {
	var cacheImpl cache.Cache
	
	if cfg.Cache.Enabled {
		if cache.IsHealthy() {
			// Use Redis cache if available
			cacheImpl = cache.NewRedisCache(cfg.Cache.TTL)
			log.Info("Using Redis cache for LLM responses")
		} else {
			// Use in-memory cache as fallback
			cacheImpl = cache.NewInMemoryCache(cfg.Cache.TTL)
			log.Info("Using in-memory cache for LLM responses")
		}
	}
	
	return &CacheMiddleware{
		cache:   cacheImpl,
		config:  &cfg.Cache,
		log:     log,
		enabled: cfg.Cache.Enabled && cacheImpl != nil,
	}
}

func (m *CacheMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip caching if disabled
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}
		
		// Only cache GET requests and specific POST endpoints
		if !m.shouldCache(r) {
			next.ServeHTTP(w, r)
			return
		}
		
		// Generate cache key
		cacheKey, err := m.generateCacheKey(r)
		if err != nil {
			m.log.Error("Failed to generate cache key", zap.Error(err))
			next.ServeHTTP(w, r)
			return
		}
		
		// Check cache
		cachedData, err := m.cache.Get(cacheKey)
		if err == nil && cachedData != nil {
			// Cache hit
			var cached CachedResponse
			if err := json.Unmarshal(cachedData, &cached); err == nil {
				m.serveCachedResponse(w, &cached)
				m.log.Debug("Cache hit", 
					zap.String("key", cacheKey),
					zap.String("model", cached.Model),
					zap.Time("cached_at", cached.CachedAt))
				return
			}
		}
		
		// Cache miss - capture response
		captureWriter := &responseWriter{
			ResponseWriter: w,
			body:          &bytes.Buffer{},
			statusCode:    http.StatusOK,
		}
		
		next.ServeHTTP(captureWriter, r)
		
		// Write captured response to actual writer
		for k, v := range captureWriter.Header() {
			w.Header()[k] = v
		}
		if captureWriter.written && captureWriter.statusCode != 0 {
			w.WriteHeader(captureWriter.statusCode)
		}
		bodyBytes := captureWriter.body.Bytes()
		w.Write(bodyBytes)
		
		// Cache successful responses
		if m.shouldCacheResponse(captureWriter.statusCode, bodyBytes) {
			go m.cacheResponse(cacheKey, captureWriter, r)
		}
	})
}

func (m *CacheMiddleware) shouldCache(r *http.Request) bool {
	// Cache GET requests to models endpoint
	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/models") {
		return true
	}
	
	// Cache POST requests to completion endpoints (if not streaming)
	if r.Method == http.MethodPost {
		path := r.URL.Path
		if strings.Contains(path, "/chat/completions") || 
		   strings.Contains(path, "/completions") ||
		   strings.Contains(path, "/embeddings") {
			// Check if streaming is requested
			var body map[string]interface{}
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				return false
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				return false
			}
			
			// Don't cache streaming responses
			if stream, ok := body["stream"].(bool); ok && stream {
				return false
			}
			
			// Don't cache if temperature > 0 (non-deterministic)
			if temp, ok := body["temperature"].(float64); ok && temp > 0 {
				return false
			}
			
			return true
		}
	}
	
	return false
}

func (m *CacheMiddleware) shouldCacheResponse(statusCode int, body []byte) bool {
	// Only cache successful responses
	if statusCode != http.StatusOK {
		return false
	}
	
	// Don't cache empty responses
	if len(body) == 0 {
		return false
	}
	
	// Don't cache error responses
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err == nil {
		if _, hasError := response["error"]; hasError {
			return false
		}
	}
	
	return true
}

func (m *CacheMiddleware) generateCacheKey(r *http.Request) (string, error) {
	// Create a unique key based on request properties
	keyData := map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
		"query":  r.URL.Query().Encode(),
	}
	
	// Include body for POST requests
	if r.Method == http.MethodPost {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		
		// Parse body to normalize it
		var body map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &body); err == nil {
			// Remove non-deterministic fields
			delete(body, "user")
			delete(body, "timestamp")
			keyData["body"] = body
		} else {
			keyData["body"] = string(bodyBytes)
		}
	}
	
	// Include API key or user ID if available
	if apiKey := r.Header.Get("Authorization"); apiKey != "" {
		// Hash the API key for privacy
		h := sha256.New()
		h.Write([]byte(apiKey))
		keyData["auth"] = hex.EncodeToString(h.Sum(nil))[:16]
	}
	
	// Generate hash of key data
	jsonData, err := json.Marshal(keyData)
	if err != nil {
		return "", err
	}
	
	h := sha256.New()
	h.Write(jsonData)
	return fmt.Sprintf("llm:cache:%s", hex.EncodeToString(h.Sum(nil))), nil
}

func (m *CacheMiddleware) cacheResponse(key string, rw *responseWriter, r *http.Request) {
	// Extract model and provider from response if available
	var responseBody map[string]interface{}
	model := ""
	provider := ""
	
	if err := json.Unmarshal(rw.body.Bytes(), &responseBody); err == nil {
		if m, ok := responseBody["model"].(string); ok {
			model = m
		}
		if obj, ok := responseBody["object"].(string); ok {
			provider = obj
		}
	}
	
	// Prepare cached response
	cached := CachedResponse{
		Body:       json.RawMessage(rw.body.Bytes()),
		Headers:    make(map[string]string),
		StatusCode: rw.statusCode,
		CachedAt:   time.Now(),
		Model:      model,
		Provider:   provider,
	}
	
	// Copy relevant headers
	for k, v := range rw.Header() {
		if len(v) > 0 && m.shouldCacheHeader(k) {
			cached.Headers[k] = v[0]
		}
	}
	
	// Marshal and cache
	data, err := json.Marshal(cached)
	if err != nil {
		m.log.Error("Failed to marshal cached response", zap.Error(err))
		return
	}
	
	// Use configured TTL or default
	ttl := m.config.TTL
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	
	if err := m.cache.Set(key, data, ttl); err != nil {
		m.log.Error("Failed to cache response", 
			zap.String("key", key),
			zap.Error(err))
	} else {
		m.log.Debug("Response cached", 
			zap.String("key", key),
			zap.String("model", model),
			zap.Duration("ttl", ttl))
	}
}

func (m *CacheMiddleware) serveCachedResponse(w http.ResponseWriter, cached *CachedResponse) {
	// Set headers
	for k, v := range cached.Headers {
		w.Header().Set(k, v)
	}
	
	// Add cache headers
	w.Header().Set("X-Cache", "HIT")
	w.Header().Set("X-Cache-Time", cached.CachedAt.Format(time.RFC3339))
	age := time.Since(cached.CachedAt).Seconds()
	w.Header().Set("Age", fmt.Sprintf("%.0f", age))
	
	// Write status and body
	w.WriteHeader(cached.StatusCode)
	w.Write(cached.Body)
}

func (m *CacheMiddleware) shouldCacheHeader(name string) bool {
	// Headers to cache
	cacheHeaders := []string{
		"Content-Type",
		"Content-Length",
		"X-Request-ID",
		"X-Model",
		"X-Provider",
	}
	
	name = strings.ToLower(name)
	for _, h := range cacheHeaders {
		if strings.ToLower(h) == name {
			return true
		}
	}
	
	return false
}

// InvalidateCache invalidates cache entries matching a pattern
func (m *CacheMiddleware) InvalidateCache(pattern string) error {
	if !m.enabled {
		return nil
	}
	
	// For Redis cache, we could use SCAN to find and delete keys
	// For in-memory cache, we'd need to implement pattern matching
	// For now, we'll just clear all cache
	return m.cache.Clear()
}

// GetCacheStats returns cache statistics
func (m *CacheMiddleware) GetCacheStats() map[string]interface{} {
	if !m.enabled {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	
	stats := map[string]interface{}{
		"enabled": true,
		"type":    "unknown",
	}
	
	// Determine cache type
	if cache.IsHealthy() {
		stats["type"] = "redis"
		if cacheStats, err := cache.GetStats(); err == nil {
			stats["keys"] = cacheStats.Keys
			stats["hits"] = cacheStats.Hits
			stats["misses"] = cacheStats.Misses
			stats["hit_rate"] = cacheStats.HitRate
		}
	} else {
		stats["type"] = "in-memory"
	}
	
	return stats
}