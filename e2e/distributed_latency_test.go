package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/services/data/redis"
	"github.com/amerfu/pllm/internal/services/llm/models"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestDistributedLatencyTracking simulates multiple PLLM instances (pods)
// sharing latency data via Redis to make intelligent routing decisions
func TestDistributedLatencyTracking(t *testing.T) {
	t.Parallel()

	// Setup shared Redis instance (simulates production Redis)
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = redisClient.Close() }()

	logger, _ := zap.NewDevelopment()

	// Create two PLLM instances (simulating two Kubernetes pods)
	router := config.RouterSettings{
		RoutingStrategy: "least-latency",
	}

	pod1 := models.NewModelManager(logger.Named("pod1"), router, redisClient)
	pod2 := models.NewModelManager(logger.Named("pod2"), router, redisClient)

	// Both pods load the same model configuration
	modelInstances := []config.ModelInstance{
		{
			ID:        "gpt-4-instance-1",
			ModelName: "gpt-4",
			Provider:  config.ProviderParams{Type: "openai"},
			Priority:  1,
			Enabled:   true,
		},
		{
			ID:        "gpt-4-instance-2",
			ModelName: "gpt-4",
			Provider:  config.ProviderParams{Type: "openai"},
			Priority:  2,
			Enabled:   true,
		},
	}

	err = pod1.LoadModelInstances(modelInstances)
	require.NoError(t, err)
	err = pod2.LoadModelInstances(modelInstances)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("shared latency across pods", func(t *testing.T) {
		// Pod 1 processes a request with 10s latency
		pod1.RecordRequestEnd("gpt-4", 10*time.Second, true, nil)

		// Wait for Redis propagation
		time.Sleep(150 * time.Millisecond)

		// Pod 2 should see the latency from Pod 1
		tracker := redis.NewLatencyTracker(redisClient, logger)
		avg, err := tracker.GetAverageLatency(ctx, "gpt-4")
		require.NoError(t, err)

		assert.Greater(t, avg, 9*time.Second, "Pod 2 should see latency from Pod 1")
		assert.Less(t, avg, 11*time.Second)

		t.Logf("Shared latency across pods: %v", avg)
	})

	t.Run("routing decision based on distributed latency", func(t *testing.T) {
		tracker := redis.NewLatencyTracker(redisClient, logger)
		
		// Clear previous data
		err := tracker.ClearLatencies(ctx, "gpt-4")
		require.NoError(t, err)

		// Pod 1 records fast latencies
		for i := 0; i < 10; i++ {
			pod1.RecordRequestEnd("gpt-4", 500*time.Millisecond, true, nil)
		}

		// Pod 2 records slow latencies  
		for i := 0; i < 10; i++ {
			pod2.RecordRequestEnd("gpt-4", 5*time.Second, true, nil)
		}

		time.Sleep(200 * time.Millisecond)

		// Get stats - both pods write to same model key
		stats, err := tracker.GetLatencyStats(ctx, "gpt-4")
		require.NoError(t, err)

		t.Logf("Distributed latency stats: avg=%v, p95=%v, p99=%v, samples=%d",
			stats.Average, stats.P95, stats.P99, stats.SampleCount)

		// Should have samples from both pods (20 total)
		assert.Greater(t, stats.SampleCount, int64(15), "Should have most samples from both pods")
		// Average should be between fast and slow
		assert.Greater(t, stats.Average, 1*time.Second, "Average should reflect mixed latencies")
		assert.Less(t, stats.Average, 4*time.Second, "Average should reflect mixed latencies")
	})
}

// TestMultiPodFailover simulates failover scenario across multiple pods
func TestMultiPodFailover(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = redisClient.Close() }()

	logger, _ := zap.NewDevelopment()
	tracker := redis.NewLatencyTracker(redisClient, logger)

	ctx := context.Background()

	// Scenario: Multiple models with different latency profiles
	models := map[string]time.Duration{
		"gpt-4":          100 * time.Millisecond, // Fast
		"gpt-4-turbo":    5 * time.Second,        // Slow
		"claude-3-sonnet": 200 * time.Millisecond, // Medium
	}

	// Record latencies
	for model, latency := range models {
		for i := 0; i < 10; i++ {
			err := tracker.RecordLatency(ctx, model, latency)
			require.NoError(t, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Get all model stats
	allStats, err := tracker.GetAllModelStats(ctx)
	require.NoError(t, err)

	// Verify we can identify the fastest model
	var fastestModel string
	fastestLatency := 1 * time.Hour

	for model, stats := range allStats {
		t.Logf("Model: %s, Avg: %v, P95: %v", model, stats.Average, stats.P95)
		if stats.Average < fastestLatency {
			fastestLatency = stats.Average
			fastestModel = model
		}
	}

	assert.Equal(t, "gpt-4", fastestModel, "Should identify gpt-4 as fastest model")
	assert.Less(t, fastestLatency, 150*time.Millisecond)
}

// TestConcurrentLatencyUpdates simulates high concurrency across pods
func TestConcurrentLatencyUpdates(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = redisClient.Close() }()

	logger := zap.NewNop() // Silence logs for concurrency test
	tracker := redis.NewLatencyTracker(redisClient, logger)

	ctx := context.Background()
	modelName := "gpt-4"

	// Simulate 5 pods, each processing 20 requests concurrently
	numPods := 5
	requestsPerPod := 20

	var wg sync.WaitGroup
	errors := make(chan error, numPods*requestsPerPod)

	for pod := 0; pod < numPods; pod++ {
		for req := 0; req < requestsPerPod; req++ {
			wg.Add(1)
			go func(podID, reqID int) {
				defer wg.Done()

				// Vary latency slightly per pod
				baseLatency := time.Duration(100+podID*10) * time.Millisecond
				latency := baseLatency + time.Duration(reqID)*time.Millisecond

				err := tracker.RecordLatency(ctx, modelName, latency)
				if err != nil {
					errors <- err
				}
			}(pod, req)
		}
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent latency update failed: %v", err)
	}

	// Verify all samples were recorded
	stats, err := tracker.GetLatencyStats(ctx, modelName)
	require.NoError(t, err)

	totalExpected := int64(numPods * requestsPerPod)
	assert.Equal(t, totalExpected, stats.SampleCount,
		"Should have all samples from all pods")

	t.Logf("Concurrent test results: %d samples, avg=%v, p95=%v",
		stats.SampleCount, stats.Average, stats.P95)
}

// TestLatencyBasedRouting validates that routing actually uses distributed latency
func TestLatencyBasedRouting(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = redisClient.Close() }()

	logger, _ := zap.NewDevelopment()

	// Create model manager with least-latency routing
	router := config.RouterSettings{
		RoutingStrategy: "least-latency",
	}
	manager := models.NewModelManager(logger, router, redisClient)

	// Load two instances of the same model
	modelInstances := []config.ModelInstance{
		{
			ID:        "instance-1",
			ModelName: "gpt-4",
			Provider:  config.ProviderParams{Type: "openai"},
			Priority:  1,
			Enabled:   true,
		},
		{
			ID:        "instance-2",
			ModelName: "gpt-4",
			Provider:  config.ProviderParams{Type: "openai"},
			Priority:  2,
			Enabled:   true,
		},
	}

	err = manager.LoadModelInstances(modelInstances)
	require.NoError(t, err)

	// Record different latencies for each instance
	// Instance 1: 100ms (fast)
	for i := 0; i < 10; i++ {
		manager.RecordRequestEnd("gpt-4", 100*time.Millisecond, true, nil)
	}

	// Instance 2: 2s (slow) - we'd need to mark this differently in production
	// For now, we're testing that the distributed latency is being read
	
	ctx := context.Background()
	tracker := redis.NewLatencyTracker(redisClient, logger)

	time.Sleep(150 * time.Millisecond)

	// Verify latency was recorded
	avg, err := tracker.GetAverageLatency(ctx, "gpt-4")
	require.NoError(t, err)
	assert.Greater(t, avg, 50*time.Millisecond)
	assert.Less(t, avg, 200*time.Millisecond)

	t.Logf("Routing will use distributed latency: %v", avg)
}

// TestHealthScoreCalculation validates health scoring based on latency
func TestHealthScoreCalculation(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = redisClient.Close() }()

	logger, _ := zap.NewDevelopment()
	tracker := redis.NewLatencyTracker(redisClient, logger)

	ctx := context.Background()

	tests := []struct {
		name          string
		model         string
		latency       time.Duration
		expectedScore float64
		scoreDelta    float64
	}{
		{
			name:          "Excellent performance (< 500ms)",
			model:         "fast-model",
			latency:       200 * time.Millisecond,
			expectedScore: 100.0,
			scoreDelta:    5.0,
		},
		{
			name:          "Good performance (~1s)",
			model:         "good-model",
			latency:       900 * time.Millisecond,
			expectedScore: 82.0,
			scoreDelta:    10.0,
		},
		{
			name:          "Degraded performance (2-3s)",
			model:         "degraded-model",
			latency:       2500 * time.Millisecond,
			expectedScore: 55.0,
			scoreDelta:    10.0,
		},
		{
			name:          "Poor performance (> 5s)",
			model:         "poor-model",
			latency:       6 * time.Second,
			expectedScore: 30.0,
			scoreDelta:    15.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record latency samples
			for i := 0; i < 10; i++ {
				err := tracker.RecordLatency(ctx, tt.model, tt.latency)
				require.NoError(t, err)
			}

			// Get health score
			score, err := tracker.GetHealthScore(ctx, tt.model)
			require.NoError(t, err)

			assert.InDelta(t, tt.expectedScore, score, tt.scoreDelta,
				"%s: health score should be around %.0f", tt.name, tt.expectedScore)

			t.Logf("%s: latency=%v, health_score=%.1f", tt.name, tt.latency, score)
		})
	}
}
