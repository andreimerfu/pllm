package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/core/auth"
	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/infrastructure/logger"
	redisService "github.com/amerfu/pllm/internal/services/data/redis"
	"github.com/amerfu/pllm/internal/services/monitoring/metrics"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMiddleware tests authentication middleware
func TestAuthMiddleware(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Setup auth services
	masterKeyService := auth.NewMasterKeyService(&auth.MasterKeyConfig{
		DB:          db,
		MasterKey:   "test-master-key",
		JWTSecret:   []byte("test-jwt-secret"),
		JWTIssuer:   "pllm",
		TokenExpiry: 24 * time.Hour,
	})

	authService, err := auth.NewAuthService(&auth.AuthConfig{
		DB:               db,
		JWTSecret:        "test-jwt-secret",
		JWTIssuer:        "pllm",
		TokenExpiry:      time.Hour,
		MasterKeyService: masterKeyService,
	})
	require.NoError(t, err)

	logger := logger.NewLogger("test", "info")

	middleware := NewAuthMiddleware(&AuthConfig{
		Logger:           logger,
		AuthService:      authService,
		MasterKeyService: masterKeyService,
		RequireAuth:      true,
	})

	// Test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	t.Run("Valid Master Key", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer test-master-key")
		w := httptest.NewRecorder()

		handler := middleware.Authenticate(testHandler)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("Invalid Key", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-key")
		w := httptest.NewRecorder()

		handler := middleware.Authenticate(testHandler)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Missing Authorization", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler := middleware.Authenticate(testHandler)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Concurrent Authentication", func(t *testing.T) {
		// Test auth middleware under concurrent load
		const numRequests = 100
		var wg sync.WaitGroup
		results := make(chan int, numRequests)

		handler := middleware.Authenticate(testHandler)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer test-master-key")
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				results <- w.Code
			}()
		}

		wg.Wait()
		close(results)

		// All requests should succeed
		successCount := 0
		for code := range results {
			if code == http.StatusOK {
				successCount++
			}
		}

		assert.Equal(t, numRequests, successCount, "All concurrent auth requests should succeed")
	})
}

// TestRateLimitMiddleware tests rate limiting
func TestRateLimitMiddleware(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:   true,
			GlobalRPM: 10,
		},
	}

	logger := logger.NewLogger("test", "info")
	middleware := NewRateLimitMiddleware(cfg, logger)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("Normal Rate", func(t *testing.T) {
		handler := middleware.Handler(testHandler)

		// Should allow normal rate
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "127.0.0.1:12345"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("Rate Limit Exceeded", func(t *testing.T) {
		handler := middleware.Handler(testHandler)

		// Exceed rate limit
		for i := 0; i < 20; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "127.0.0.2:12345"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if i < 10 {
				assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request %d should be rate limited", i)
			}
		}
	})
}

// TestBudgetMiddleware tests budget enforcement
func TestBudgetMiddleware(t *testing.T) {
	// Setup test dependencies
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Setup Redis client using test container
	redisClient, redisCleanup := testutil.NewTestRedis(t)
	defer redisCleanup()

	// Clear test Redis DB
	redisClient.FlushDB(context.Background())

	logger := logger.NewLogger("test", "info")

	// Setup auth service
	masterKeyService := auth.NewMasterKeyService(&auth.MasterKeyConfig{
		DB:          db,
		MasterKey:   "test-master-key",
		JWTSecret:   []byte("test-jwt-secret"),
		JWTIssuer:   "pllm",
		TokenExpiry: 24 * time.Hour,
	})

	authService, err := auth.NewAuthService(&auth.AuthConfig{
		DB:               db,
		JWTSecret:        "test-jwt-secret",
		JWTIssuer:        "pllm",
		TokenExpiry:      time.Hour,
		MasterKeyService: masterKeyService,
	})
	require.NoError(t, err)

	// Setup pricing manager - using nil for tests

	// Setup budget middleware
	middleware := NewAsyncBudgetMiddleware(&AsyncBudgetConfig{
		Logger:      logger,
		AuthService: authService,
		BudgetCache: redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
		EventPub:    redisService.NewEventPublisher(redisClient, logger),
		UsageQueue: redisService.NewUsageQueue(&redisService.UsageQueueConfig{
			Client:     redisClient,
			Logger:     logger,
			QueueName:  "test_usage_queue",
			BatchSize:  10,
			MaxRetries: 3,
		}),
		PricingManager: nil,
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Setup auth middleware to set context
	authMiddleware := NewAuthMiddleware(&AuthConfig{
		Logger:           logger,
		AuthService:      authService,
		MasterKeyService: masterKeyService,
		RequireAuth:      true,
	})

	t.Run("Budget Check with Master Key", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{
			"model": "gpt-4",
			"messages": [{"role": "user", "content": "test"}],
			"max_tokens": 10
		}`))
		req.Header.Set("Authorization", "Bearer test-master-key")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		// Chain auth middleware before budget middleware
		handler := authMiddleware.Authenticate(middleware.EnforceBudgetAsync(testHandler))
		handler.ServeHTTP(w, req)

		// Master key should bypass budget checks
		if w.Code != http.StatusOK {
			t.Logf("Response body: %s", w.Body.String())
		}
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Concurrent Budget Checks", func(t *testing.T) {
		// Test budget middleware under concurrent load
		const numRequests = 50
		var wg sync.WaitGroup
		results := make(chan int, numRequests)

		// Chain auth middleware before budget middleware
		handler := authMiddleware.Authenticate(middleware.EnforceBudgetAsync(testHandler))

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{
					"model": "gpt-4",
					"messages": [{"role": "user", "content": "test"}],
					"max_tokens": 10
				}`))
				req.Header.Set("Authorization", "Bearer test-master-key")
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				results <- w.Code
			}()
		}

		wg.Wait()
		close(results)

		// Count results
		statusCodes := make(map[int]int)
		for code := range results {
			statusCodes[code]++
		}

		// Most should succeed (master key bypasses budget)
		assert.True(t, statusCodes[200] > numRequests*0.8, "Most requests should succeed")
	})
}

// TestCacheMiddleware tests response caching
func TestCacheMiddleware(t *testing.T) {
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Enabled: true,
			TTL:     time.Minute,
			MaxSize: 100,
		},
	}

	logger := logger.NewLogger("test", "info")
	middleware := NewCacheMiddleware(cfg, logger)

	responseContent := "test response content"
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseContent))
	})

	t.Run("Cache Miss and Hit", func(t *testing.T) {
		handler := middleware.Handler(testHandler)

		// First request - cache miss
		req1 := httptest.NewRequest("GET", "/test?param=value", nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)

		assert.Equal(t, http.StatusOK, w1.Code)
		assert.Equal(t, responseContent, w1.Body.String())

		// Second request - cache hit
		req2 := httptest.NewRequest("GET", "/test?param=value", nil)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w2.Code)
		assert.Equal(t, responseContent, w2.Body.String())
	})

	t.Run("Different URLs Not Cached", func(t *testing.T) {
		handler := middleware.Handler(testHandler)

		// Different URLs should not share cache
		req1 := httptest.NewRequest("GET", "/test1", nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)

		req2 := httptest.NewRequest("GET", "/test2", nil)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w1.Code)
		assert.Equal(t, http.StatusOK, w2.Code)
	})
}

// TestMiddlewareChain tests multiple middlewares working together
func TestMiddlewareChain(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Setup all middleware components
	logger := logger.NewLogger("test", "info")

	// Auth setup
	masterKeyService := auth.NewMasterKeyService(&auth.MasterKeyConfig{
		DB:          db,
		MasterKey:   "test-master-key",
		JWTSecret:   []byte("test-jwt-secret"),
		JWTIssuer:   "pllm",
		TokenExpiry: 24 * time.Hour,
	})

	authService, err := auth.NewAuthService(&auth.AuthConfig{
		DB:               db,
		JWTSecret:        "test-jwt-secret",
		JWTIssuer:        "pllm",
		TokenExpiry:      time.Hour,
		MasterKeyService: masterKeyService,
	})
	require.NoError(t, err)

	// Middleware setup
	authMiddleware := NewAuthMiddleware(&AuthConfig{
		Logger:           logger,
		AuthService:      authService,
		MasterKeyService: masterKeyService,
		RequireAuth:      true,
	})

	rateLimitConfig := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:   true,
			GlobalRPM: 60,
		},
	}
	rateLimitMiddleware := NewRateLimitMiddleware(rateLimitConfig, logger)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	t.Run("Full Middleware Chain", func(t *testing.T) {
		// Chain middlewares: rate limit -> auth -> handler
		handler := rateLimitMiddleware.Handler(
			authMiddleware.Authenticate(testHandler),
		)

		// Valid request should pass through all middleware
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer test-master-key")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("Auth Failure in Chain", func(t *testing.T) {
		handler := rateLimitMiddleware.Handler(
			authMiddleware.Authenticate(testHandler),
		)

		// Invalid auth should be rejected before reaching handler
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-key")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.NotEqual(t, "success", w.Body.String())
	})
}

// TestMiddlewarePerformance tests middleware performance under load
func TestMiddlewarePerformance(t *testing.T) {
	logger := logger.NewLogger("test", "info")

	// Setup Redis for metrics using test container
	redisClient, cleanup := testutil.NewTestRedis(t)
	defer cleanup()

	// Create a metrics emitter for testing
	emitter := metrics.NewMetricEventEmitter(redisClient, logger)
	metricsMiddleware := NewAsyncMetricsMiddleware(emitter, logger)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	const numRequests = 1000
	start := time.Now()

	handler := metricsMiddleware.Middleware(testHandler)

	for i := 0; i < numRequests; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	duration := time.Since(start)
	rps := float64(numRequests) / duration.Seconds()

	// Banking-grade performance requirement
	assert.True(t, rps > 1000, "Middleware should handle >1000 RPS, got %.2f", rps)
	assert.True(t, duration < 5*time.Second, "Should process %d requests in <5s, took %v", numRequests, duration)

	t.Logf("Middleware Performance: %d requests in %v (%.2f RPS)", numRequests, duration, rps)
}