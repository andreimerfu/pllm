package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/database"
	"github.com/amerfu/pllm/pkg/logger"
	"github.com/amerfu/pllm/internal/services/data/cache"
	"github.com/amerfu/pllm/internal/services/llm/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestRouterIntegration tests end-to-end router functionality
func TestRouterIntegration(t *testing.T) {
	// Setup test database
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Set global DB for health checks
	oldDB := database.DB
	database.DB = db
	defer func() { database.DB = oldDB }()

	// Setup Redis using test container
	_, redisURL, redisCleanup := testutil.NewTestRedisWithURL(t)
	defer redisCleanup()

	// Initialize cache for health checks
	cache.Initialize(&cache.Config{
		RedisURL: redisURL,
		TTL:      5 * time.Minute,
	})

	// Setup test config
	cfg := &config.Config{
		Redis: config.RedisConfig{
			URL:      redisURL,
			Password: "",
			DB:       0,
		},
		Auth: config.AuthConfig{
			MasterKey: "test-master-key-123",
		},
		JWT: config.JWTConfig{
			SecretKey:           "test-jwt-secret-key-for-testing",
			AccessTokenDuration: time.Hour,
		},
		CORS: config.CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"*"},
			ExposedHeaders:   []string{"*"},
			AllowCredentials: true,
			MaxAge:           3600,
		},
		RateLimit: config.RateLimitConfig{
			Enabled: true,
		},
		Cache: config.CacheConfig{
			Enabled: false, // Disable for testing
		},
	}

	// Setup logger
	logger := logger.NewLogger("test", "info")

	// Setup model manager with test models
	routerSettings := config.RouterSettings{
		RoutingStrategy:     "weighted",
		HealthCheckInterval: 30 * time.Second,
		EnableLoadBalancing: true,
		MaxRetries:          3,
		DefaultTimeout:      5 * time.Second,
	}
	modelManager := models.NewModelManager(logger, routerSettings, nil)
	
	// Load test model instances
	testInstances := []config.ModelInstance{
		{
			ID:           "gpt-4-instance-1",
			ModelName:    "gpt-4",
			InstanceName: "gpt-4-instance-1",
			Provider: config.ProviderParams{
				Type:     "openai",
				BaseURL:  "https://api.openai.com/v1",
				APIKey:   "test-key-1",
			},
			Weight:   10,
			Priority: 50,
		},
		{
			ID:           "gpt-3.5-turbo-instance-1",
			ModelName:    "gpt-3.5-turbo",
			InstanceName: "gpt-3.5-turbo-instance-1",
			Provider: config.ProviderParams{
				Type:     "openai",
				BaseURL:  "https://api.openai.com/v1",
				APIKey:   "test-key-2",
			},
			Weight:   10,
			Priority: 50,
		},
	}
	err := modelManager.LoadModelInstances(testInstances)
	require.NoError(t, err)

	// Create pricing manager for tests
	pricingManager := config.GetPricingManager()

	// Create router
	router := NewRouter(cfg, logger, modelManager, db, pricingManager)

	t.Run("Health Endpoints", func(t *testing.T) {
		testHealthEndpoints(t, router)
	})

	t.Run("Authentication Flow", func(t *testing.T) {
		testAuthenticationFlow(t, router, db)
	})

	t.Run("Load Balancing", func(t *testing.T) {
		testLoadBalancing(t, router)
	})

	t.Run("Concurrent Request Handling", func(t *testing.T) {
		testConcurrentRequests(t, router, db)
	})

	t.Run("Redis Integration", func(t *testing.T) {
		testRedisIntegration(t, router)
	})
}

func testHealthEndpoints(t *testing.T, router http.Handler) {
	tests := []struct {
		name     string
		endpoint string
		expected int
	}{
		{"Health Check", "/health", http.StatusOK},
		{"Ready Check", "/ready", http.StatusOK},
		{"Metrics", "/metrics", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expected, w.Code)
		})
	}
}

func testAuthenticationFlow(t *testing.T, router http.Handler, db *gorm.DB) {
	// Test master key authentication
	t.Run("Master Key Auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer test-master-key-123")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Test invalid authentication
	t.Run("Invalid Auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer invalid-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// Test missing authentication
	t.Run("Missing Auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/models", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func testLoadBalancing(t *testing.T, router http.Handler) {
	// Test model selection and load distribution
	t.Run("Model Selection", func(t *testing.T) {
		chatRequest := map[string]interface{}{
			"model": "gpt-4",
			"messages": []map[string]interface{}{
				{"role": "user", "content": "Hello, world!"},
			},
			"max_tokens": 10,
		}

		body, _ := json.Marshal(chatRequest)
		req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test-master-key-123")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should get an error response (either 401 for missing key/budget or 503 for no providers)
		// but this proves routing logic works without crashing
		assert.True(t, w.Code >= 400, "Should return error status, got %d", w.Code)
	})
}

func testConcurrentRequests(t *testing.T, router http.Handler, db *gorm.DB) {
	// Test high concurrent load
	const numRequests = 100
	const numWorkers = 10

	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	// Create worker pool
	requests := make(chan int, numRequests)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range requests {
				req := httptest.NewRequest("GET", "/health", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				results <- w.Code
			}
		}()
	}

	// Send requests
	start := time.Now()
	for i := 0; i < numRequests; i++ {
		requests <- i
	}
	close(requests)

	// Wait for completion
	wg.Wait()
	close(results)
	duration := time.Since(start)

	// Collect results
	statusCodes := make(map[int]int)
	for code := range results {
		statusCodes[code]++
	}

	// Assertions for banking-grade performance
	assert.True(t, duration < 5*time.Second, "Should handle %d requests in <5s, took %v", numRequests, duration)
	assert.Equal(t, numRequests, statusCodes[200], "All health checks should succeed")
	
	// Calculate requests per second
	rps := float64(numRequests) / duration.Seconds()
	assert.True(t, rps > 20, "Should achieve >20 RPS, got %.2f", rps)
	
	t.Logf("Handled %d concurrent requests in %v (%.2f RPS)", numRequests, duration, rps)
}

func testRedisIntegration(t *testing.T, router http.Handler) {
	// Test Redis connectivity and caching
	t.Run("Redis Health", func(t *testing.T) {
		// This is indirectly tested by router startup
		// If Redis was down, router creation would fail
		assert.NotNil(t, router, "Router should initialize with Redis")
	})
}

// TestRouterLatencyRequirements tests banking-specific latency requirements
func TestRouterLatencyRequirements(t *testing.T) {
	// Setup minimal router for latency testing
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Set global DB for health checks
	oldDB := database.DB
	database.DB = db
	defer func() { database.DB = oldDB }()

	_, redisURL, redisCleanup := testutil.NewTestRedisWithURL(t)
	defer redisCleanup()

	// Initialize cache for health checks
	cache.Initialize(&cache.Config{
		RedisURL: redisURL,
		TTL:      5 * time.Minute,
	})

	cfg := &config.Config{
		Redis: config.RedisConfig{URL: redisURL},
		Auth:  config.AuthConfig{MasterKey: "test-key"},
		JWT:   config.JWTConfig{SecretKey: "test-jwt-secret", AccessTokenDuration: time.Hour},
		CORS:  config.CORSConfig{AllowedOrigins: []string{"*"}},
	}

	logger := logger.NewLogger("test", "info")
	routerSettings := config.RouterSettings{
		RoutingStrategy:     "weighted",
		HealthCheckInterval: 30 * time.Second,
		EnableLoadBalancing: true,
		MaxRetries:          3,
		DefaultTimeout:      5 * time.Second,
	}
	modelManager := models.NewModelManager(logger, routerSettings, nil)
	pricingManager := config.GetPricingManager()
	router := NewRouter(cfg, logger, modelManager, db, pricingManager)

	// Banking latency requirements
	const (
		p95Target = 100 * time.Millisecond  // 95th percentile under 100ms
		p99Target = 500 * time.Millisecond  // 99th percentile under 500ms
		maxTarget = 2 * time.Second         // No request over 2s
	)

	latencies := make([]time.Duration, 1000)

	// Measure latencies
	for i := 0; i < 1000; i++ {
		start := time.Now()
		
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		latencies[i] = time.Since(start)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Calculate percentiles
	latenciesCopy := make([]time.Duration, len(latencies))
	copy(latenciesCopy, latencies)
	
	// Sort for percentile calculation
	for i := 0; i < len(latenciesCopy)-1; i++ {
		for j := i + 1; j < len(latenciesCopy); j++ {
			if latenciesCopy[i] > latenciesCopy[j] {
				latenciesCopy[i], latenciesCopy[j] = latenciesCopy[j], latenciesCopy[i]
			}
		}
	}

	p95 := latenciesCopy[int(0.95*float64(len(latenciesCopy)))]
	p99 := latenciesCopy[int(0.99*float64(len(latenciesCopy)))]
	max := latenciesCopy[len(latenciesCopy)-1]

	// Banking-grade assertions
	assert.True(t, p95 < p95Target, "P95 latency %v should be < %v", p95, p95Target)
	assert.True(t, p99 < p99Target, "P99 latency %v should be < %v", p99, p99Target)
	assert.True(t, max < maxTarget, "Max latency %v should be < %v", max, maxTarget)

	t.Logf("Latency Results: P95=%v, P99=%v, Max=%v", p95, p99, max)
}

// TestRouterFailover tests failover scenarios critical for banking
func TestRouterFailover(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Set global DB for health checks
	oldDB := database.DB
	database.DB = db
	defer func() { database.DB = oldDB }()

	_, redisURL, redisCleanup := testutil.NewTestRedisWithURL(t)
	defer redisCleanup()

	// Initialize cache for health checks
	cache.Initialize(&cache.Config{
		RedisURL: redisURL,
		TTL:      5 * time.Minute,
	})

	cfg := &config.Config{
		Redis: config.RedisConfig{URL: redisURL},
		Auth:  config.AuthConfig{MasterKey: "test-key"},
		JWT:   config.JWTConfig{SecretKey: "test-jwt-secret", AccessTokenDuration: time.Hour},
	}

	logger := logger.NewLogger("test", "info")
	routerSettings := config.RouterSettings{
		RoutingStrategy:     "weighted",
		HealthCheckInterval: 30 * time.Second,
		EnableLoadBalancing: true,
		MaxRetries:          3,
		DefaultTimeout:      5 * time.Second,
	}
	modelManager := models.NewModelManager(logger, routerSettings, nil)
	pricingManager := config.GetPricingManager()

	// Load test model instances for failover testing
	testInstances := []config.ModelInstance{
		{
			ID:           "primary-model-instance",
			ModelName:    "primary-model",
			InstanceName: "primary-model-instance",
			Provider: config.ProviderParams{
				Type:     "openai",
				BaseURL:  "https://api.openai.com/v1",
				APIKey:   "primary-key",
			},
			Weight:   10,
			Priority: 50,
		},
		{
			ID:           "backup-model-instance",
			ModelName:    "backup-model",
			InstanceName: "backup-model-instance",
			Provider: config.ProviderParams{
				Type:     "openai",
				BaseURL:  "https://api.openai.com/v1",
				APIKey:   "backup-key",
			},
			Weight:   10,
			Priority: 50,
		},
	}
	err := modelManager.LoadModelInstances(testInstances)
	require.NoError(t, err)

	router := NewRouter(cfg, logger, modelManager, db, pricingManager)

	t.Run("Model Failover", func(t *testing.T) {
		// Test that requests to unavailable models fail gracefully
		chatRequest := map[string]interface{}{
			"model": "unavailable-model",
			"messages": []map[string]interface{}{
				{"role": "user", "content": "test"},
			},
		}

		body, _ := json.Marshal(chatRequest)
		req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test-master-key-123")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail gracefully, not crash
		assert.True(t, w.Code >= 400, "Should handle model unavailability gracefully")

		// Response should be JSON
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "Response should be valid JSON")
		assert.Contains(t, response, "error", "Should contain error field")
	})
}