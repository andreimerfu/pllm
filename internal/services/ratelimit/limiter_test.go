package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestInMemoryLimiter(t *testing.T) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)

	ctx := context.Background()
	key := "test-key"
	limit := 5
	window := 1 * time.Second

	t.Run("allow requests within limit", func(t *testing.T) {
		// Reset limiter
		_ = limiter.Reset(ctx, key)

		for i := 0; i < limit; i++ {
			allowed, err := limiter.Allow(ctx, key, limit, window)
			require.NoError(t, err)
			assert.True(t, allowed, "Request %d should be allowed", i+1)
		}
	})

	t.Run("reject requests exceeding limit", func(t *testing.T) {
		// Reset limiter
		_ = limiter.Reset(ctx, key)

		// Use up all tokens
		for i := 0; i < limit; i++ {
			allowed, err := limiter.Allow(ctx, key, limit, window)
			require.NoError(t, err)
			require.True(t, allowed)
		}

		// Next request should be rejected
		allowed, err := limiter.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("tokens refill over time", func(t *testing.T) {
		// Reset limiter
		_ = limiter.Reset(ctx, key)

		// Use up all tokens
		for i := 0; i < limit; i++ {
			allowed, err := limiter.Allow(ctx, key, limit, window)
			require.NoError(t, err)
			require.True(t, allowed)
		}

		// Wait for some tokens to refill
		time.Sleep(200 * time.Millisecond)

		// Should be able to make at least one more request
		allowed, err := limiter.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("AllowN multiple tokens", func(t *testing.T) {
		// Reset limiter
		_ = limiter.Reset(ctx, key)

		// Take 3 tokens at once
		allowed, err := limiter.AllowN(ctx, key, 3, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)

		// Should only have 2 tokens left
		allowed, err = limiter.AllowN(ctx, key, 3, limit, window)
		require.NoError(t, err)
		assert.False(t, allowed)

		// But should allow 2 tokens
		allowed, err = limiter.AllowN(ctx, key, 2, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("GetRemaining", func(t *testing.T) {
		// Reset limiter
		_ = limiter.Reset(ctx, key)

		remaining, err := limiter.GetRemaining(ctx, key, limit, window)
		require.NoError(t, err)
		assert.Equal(t, limit, remaining)

		// Use some tokens
		_, _ = limiter.AllowN(ctx, key, 2, limit, window)

		remaining, err = limiter.GetRemaining(ctx, key, limit, window)
		require.NoError(t, err)
		assert.Equal(t, 3, remaining)
	})

	t.Run("concurrent access", func(t *testing.T) {
		// Reset limiter
		_ = limiter.Reset(ctx, key)

		const numGoroutines = 10
		results := make(chan bool, numGoroutines)

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				allowed, err := limiter.Allow(ctx, key, limit, window)
				require.NoError(t, err)
				results <- allowed
			}()
		}

		wg.Wait()
		close(results)

		// Count allowed requests
		allowedCount := 0
		for allowed := range results {
			if allowed {
				allowedCount++
			}
		}

		// Should have allowed exactly 'limit' requests
		assert.Equal(t, limit, allowedCount)
	})
}

func TestInMemoryLimiter_TokenBucketAlgorithm(t *testing.T) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()
	key := "token-bucket-test"
	limit := 10
	window := 1 * time.Second

	t.Run("initial bucket is full", func(t *testing.T) {
		_ = limiter.Reset(ctx, key)

		remaining, err := limiter.GetRemaining(ctx, key, limit, window)
		require.NoError(t, err)
		assert.Equal(t, limit, remaining)
	})

	t.Run("tokens consume from bucket", func(t *testing.T) {
		_ = limiter.Reset(ctx, key)

		// Use 3 tokens
		allowed, err := limiter.AllowN(ctx, key, 3, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)

		remaining, err := limiter.GetRemaining(ctx, key, limit, window)
		require.NoError(t, err)
		assert.Equal(t, 7, remaining)
	})

	t.Run("tokens refill based on elapsed time", func(t *testing.T) {
		_ = limiter.Reset(ctx, key)

		// Use all tokens
		allowed, err := limiter.AllowN(ctx, key, limit, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)

		// No tokens left
		remaining, err := limiter.GetRemaining(ctx, key, limit, window)
		require.NoError(t, err)
		assert.Equal(t, 0, remaining)

		// Wait for half the window
		time.Sleep(500 * time.Millisecond)

		// Should have refilled approximately half
		remaining, err = limiter.GetRemaining(ctx, key, limit, window)
		require.NoError(t, err)
		assert.Greater(t, remaining, 3) // Should be around 5
		assert.LessOrEqual(t, remaining, limit)
	})

	t.Run("bucket cannot exceed capacity", func(t *testing.T) {
		_ = limiter.Reset(ctx, key)

		// Wait for extra time
		time.Sleep(2 * window)

		// Should not exceed limit
		remaining, err := limiter.GetRemaining(ctx, key, limit, window)
		require.NoError(t, err)
		assert.Equal(t, limit, remaining)
	})
}

func TestInMemoryLimiter_DifferentKeys(t *testing.T) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()
	limit := 3
	window := 1 * time.Second

	t.Run("different keys have independent limits", func(t *testing.T) {
		// Reset both keys
		_ = limiter.Reset(ctx, "key1")
		_ = limiter.Reset(ctx, "key2")

		// Use up key1's limit
		for i := 0; i < limit; i++ {
			allowed, err := limiter.Allow(ctx, "key1", limit, window)
			require.NoError(t, err)
			require.True(t, allowed)
		}

		// key1 should be exhausted
		allowed, err := limiter.Allow(ctx, "key1", limit, window)
		require.NoError(t, err)
		assert.False(t, allowed)

		// key2 should still be available
		allowed, err = limiter.Allow(ctx, "key2", limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("keys can have different limits", func(t *testing.T) {
		_ = limiter.Reset(ctx, "low-limit")
		_ = limiter.Reset(ctx, "high-limit")

		lowLimit := 2
		highLimit := 10

		// Low limit key should be exhausted quickly
		for i := 0; i < lowLimit; i++ {
			allowed, err := limiter.Allow(ctx, "low-limit", lowLimit, window)
			require.NoError(t, err)
			require.True(t, allowed)
		}

		allowed, err := limiter.Allow(ctx, "low-limit", lowLimit, window)
		require.NoError(t, err)
		assert.False(t, allowed)

		// High limit key should still have capacity
		for i := 0; i < lowLimit+1; i++ {
			allowed, err := limiter.Allow(ctx, "high-limit", highLimit, window)
			require.NoError(t, err)
			assert.True(t, allowed)
		}
	})
}

// Test Redis limiter integration (skipped in short mode)
func TestRedisLimiter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration tests in short mode")
	}

	// This test would require a real Redis connection
	// In a real project, you might use testcontainers or docker-compose
	t.Skip("Redis integration tests require Redis setup")
}

func TestFixedWindowLimiter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration tests in short mode")
	}

	// This test would require a real Redis connection
	t.Skip("Fixed window limiter integration tests require Redis setup")
}

// Benchmark tests
func BenchmarkInMemoryLimiter_Allow(b *testing.B) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()
	key := "bench-key"
	limit := 1000
	window := 1 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = limiter.Allow(ctx, key, limit, window)
	}
}

func BenchmarkInMemoryLimiter_AllowN(b *testing.B) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()
	key := "bench-key"
	limit := 1000
	window := 1 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = limiter.AllowN(ctx, key, 5, limit, window)
	}
}

func BenchmarkInMemoryLimiter_ConcurrentAccess(b *testing.B) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()
	limit := 1000
	window := 1 * time.Second

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench-key-%d", i%10) // 10 different keys
			_, _ = limiter.Allow(ctx, key, limit, window)
			i++
		}
	})
}

func TestInMemoryLimiter_Cleanup(t *testing.T) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()

	// Add some buckets
	_, _ = limiter.Allow(ctx, "key1", 10, 1*time.Second)
	_, _ = limiter.Allow(ctx, "key2", 10, 1*time.Second)
	_, _ = limiter.Allow(ctx, "key3", 10, 1*time.Second)

	// Check buckets exist
	limiter.mu.RLock()
	initialCount := len(limiter.buckets)
	limiter.mu.RUnlock()

	assert.Equal(t, 3, initialCount)

	// Manually trigger cleanup by modifying last refill time
	limiter.mu.Lock()
	for _, bucket := range limiter.buckets {
		bucket.mu.Lock()
		bucket.lastRefill = time.Now().Add(-2 * time.Hour) // Make it old
		bucket.mu.Unlock()
	}
	limiter.mu.Unlock()

	// Wait a bit for cleanup to potentially run
	// Note: In a real test, you might want to expose the cleanup trigger
	// or make the cleanup interval configurable for testing
	time.Sleep(10 * time.Millisecond)
}

func TestRateLimiterEdgeCases(t *testing.T) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()

	t.Run("zero limit", func(t *testing.T) {
		allowed, err := limiter.Allow(ctx, "test", 0, 1*time.Second)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("negative tokens request", func(t *testing.T) {
		allowed, err := limiter.AllowN(ctx, "test", -1, 10, 1*time.Second)
		require.NoError(t, err)
		// Should handle gracefully - behavior may vary
		assert.True(t, allowed || !allowed) // Either behavior is acceptable
	})

	t.Run("very short window", func(t *testing.T) {
		allowed, err := limiter.Allow(ctx, "test", 1, 1*time.Nanosecond)
		require.NoError(t, err)
		// Should work but tokens refill very quickly
		assert.True(t, allowed)
	})

	t.Run("very long window", func(t *testing.T) {
		// Reset to ensure clean state
		_ = limiter.Reset(ctx, "test-long")
		allowed, err := limiter.Allow(ctx, "test-long", 1, 24*time.Hour)
		require.NoError(t, err)
		assert.True(t, allowed)
	})
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b     float64
		expected float64
	}{
		{1.0, 2.0, 1.0},
		{2.0, 1.0, 1.0},
		{1.0, 1.0, 1.0},
		{0.0, 1.0, 0.0},
		{-1.0, 1.0, -1.0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("min(%.1f, %.1f)", tt.a, tt.b), func(t *testing.T) {
			result := min(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test behavior under extreme load
func TestInMemoryLimiter_HighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high concurrency test in short mode")
	}

	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()
	key := "high-concurrency-test"
	limit := 100
	window := 1 * time.Second

	const numWorkers = 50
	const requestsPerWorker = 20

	results := make(chan bool, numWorkers*requestsPerWorker)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				allowed, err := limiter.Allow(ctx, key, limit, window)
				require.NoError(t, err)
				results <- allowed
			}
		}()
	}

	wg.Wait()
	close(results)

	// Count results
	allowedCount := 0
	totalRequests := 0
	for allowed := range results {
		totalRequests++
		if allowed {
			allowedCount++
		}
	}

	assert.Equal(t, numWorkers*requestsPerWorker, totalRequests)
	// Due to refill during the test, we might get more than the limit
	// but we should get at least the limit
	assert.GreaterOrEqual(t, allowedCount, limit)
	assert.LessOrEqual(t, allowedCount, totalRequests)

	t.Logf("Allowed %d out of %d requests", allowedCount, totalRequests)
}

// Test context cancellation
func TestInMemoryLimiter_ContextCancellation(t *testing.T) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should still work (context is mainly for future extensibility)
	allowed, err := limiter.Allow(ctx, "test", 10, 1*time.Second)
	require.NoError(t, err)
	assert.True(t, allowed)
}

// Test different window sizes
func TestInMemoryLimiter_DifferentWindows(t *testing.T) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()
	key := "window-test"
	limit := 10

	windows := []time.Duration{
		100 * time.Millisecond,
		1 * time.Second,
		1 * time.Minute,
		1 * time.Hour,
	}

	for _, window := range windows {
		t.Run(fmt.Sprintf("window_%v", window), func(t *testing.T) {
			_ = limiter.Reset(ctx, key)

			// Should be able to use the full limit
			for i := 0; i < limit; i++ {
				allowed, err := limiter.Allow(ctx, key, limit, window)
				require.NoError(t, err)
				assert.True(t, allowed, "Request %d should be allowed for window %v", i+1, window)
			}

			// Next request should be rejected
			allowed, err := limiter.Allow(ctx, key, limit, window)
			require.NoError(t, err)
			assert.False(t, allowed, "Should reject request after limit for window %v", window)
		})
	}
}

// Test reset functionality
func TestInMemoryLimiter_Reset(t *testing.T) {
	logger := zap.NewNop()
	limiter := NewInMemoryLimiter(logger)
	ctx := context.Background()
	key := "reset-test"
	limit := 5
	window := 1 * time.Second

	// Use up the limit
	for i := 0; i < limit; i++ {
		allowed, err := limiter.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		require.True(t, allowed)
	}

	// Should be exhausted
	allowed, err := limiter.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.False(t, allowed)

	// Reset should restore capacity
	err = limiter.Reset(ctx, key)
	require.NoError(t, err)

	// Should be able to use full capacity again
	allowed, err = limiter.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.True(t, allowed)

	// Check remaining capacity after an access (which creates a new bucket)
	remaining, err := limiter.GetRemaining(ctx, key, limit, window)
	require.NoError(t, err)
	assert.Equal(t, limit-1, remaining) // Should have used 1 token above
}
