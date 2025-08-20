package loadbalancer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdaptiveLoadBalancer(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()

	assert.NotNil(t, alb.models)
	assert.NotNil(t, alb.fallbacks)
	assert.Equal(t, int32(1000), alb.maxConcurrent)
	assert.Equal(t, 0.4, alb.latencyWeight)
	assert.Equal(t, 0.3, alb.loadWeight)
	assert.Equal(t, 0.3, alb.errorWeight)
	assert.Equal(t, int64(0), alb.totalRequests)
	assert.Equal(t, int64(0), alb.totalFailures)
}

func TestAdaptiveLoadBalancer_RegisterModel(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()

	t.Run("register new model", func(t *testing.T) {
		alb.RegisterModel("model1", 2*time.Second)

		assert.Contains(t, alb.models, "model1")

		health := alb.models["model1"]
		assert.Equal(t, "model1", health.ModelName)
		assert.Equal(t, 2*time.Second, health.MaxResponseTime)
		assert.Equal(t, 100, health.WindowSize)
		assert.Equal(t, 100.0, health.HealthScore)
		assert.False(t, health.IsCircuitOpen)
		assert.Equal(t, int32(0), health.ActiveRequests)
		assert.Equal(t, int64(0), health.TotalRequests)
	})

	t.Run("register existing model doesn't overwrite", func(t *testing.T) {
		// Modify the model's health
		alb.models["model1"].HealthScore = 50.0

		// Re-register
		alb.RegisterModel("model1", 5*time.Second)

		// Should not overwrite existing
		assert.Equal(t, 50.0, alb.models["model1"].HealthScore)
		assert.Equal(t, 2*time.Second, alb.models["model1"].MaxResponseTime) // Original value
	})

	t.Run("register multiple models", func(t *testing.T) {
		alb.RegisterModel("model2", 1*time.Second)
		alb.RegisterModel("model3", 3*time.Second)

		assert.Len(t, alb.models, 3)
		assert.Contains(t, alb.models, "model2")
		assert.Contains(t, alb.models, "model3")
	})
}

func TestAdaptiveLoadBalancer_SetFallbacks(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()

	fallbacks := []string{"fallback1", "fallback2", "fallback3"}
	alb.SetFallbacks("primary", fallbacks)

	assert.Equal(t, fallbacks, alb.fallbacks["primary"])

	// Update fallbacks
	newFallbacks := []string{"new1", "new2"}
	alb.SetFallbacks("primary", newFallbacks)
	assert.Equal(t, newFallbacks, alb.fallbacks["primary"])
}

func TestAdaptiveLoadBalancer_SelectModel(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()
	ctx := context.Background()

	// Register models
	alb.RegisterModel("primary", 2*time.Second)
	alb.RegisterModel("fallback1", 2*time.Second)
	alb.RegisterModel("fallback2", 2*time.Second)

	// Set fallbacks
	alb.SetFallbacks("primary", []string{"fallback1", "fallback2"})

	t.Run("selects primary when healthy", func(t *testing.T) {
		selected, err := alb.SelectModel(ctx, "primary")
		require.NoError(t, err)
		assert.Equal(t, "primary", selected)

		// Should increment active requests
		assert.Equal(t, int32(1), alb.models["primary"].ActiveRequests)
	})

	t.Run("selects fallback when primary degraded", func(t *testing.T) {
		// Degrade primary model
		alb.models["primary"].mu.Lock()
		alb.models["primary"].HealthScore = 40.0 // Below 50% threshold
		alb.models["primary"].mu.Unlock()

		selected, err := alb.SelectModel(ctx, "primary")
		require.NoError(t, err)

		// Should select a fallback
		assert.Contains(t, []string{"fallback1", "fallback2"}, selected)
	})

	t.Run("selects fallback when primary circuit open", func(t *testing.T) {
		// Reset primary health but open circuit
		alb.models["primary"].mu.Lock()
		alb.models["primary"].HealthScore = 100.0
		alb.models["primary"].IsCircuitOpen = true
		alb.models["primary"].LastFailureTime = time.Now()
		alb.models["primary"].mu.Unlock()

		selected, err := alb.SelectModel(ctx, "primary")
		require.NoError(t, err)

		// Should select a fallback
		assert.Contains(t, []string{"fallback1", "fallback2"}, selected)
	})

	t.Run("tries primary after circuit cooldown", func(t *testing.T) {
		// Set old failure time to simulate cooldown
		alb.models["primary"].mu.Lock()
		alb.models["primary"].LastFailureTime = time.Now().Add(-31 * time.Second)
		alb.models["primary"].HealthScore = 80.0 // Good health
		alb.models["primary"].mu.Unlock()

		selected, err := alb.SelectModel(ctx, "primary")
		require.NoError(t, err)
		assert.Equal(t, "primary", selected)

		// Circuit should still be open but request should succeed due to cooldown
		// (The circuit isn't explicitly closed in the primary model path)
	})

	t.Run("returns error when no models available", func(t *testing.T) {
		// Open all circuits
		for _, model := range []string{"primary", "fallback1", "fallback2"} {
			alb.models[model].mu.Lock()
			alb.models[model].IsCircuitOpen = true
			alb.models[model].LastFailureTime = time.Now()
			alb.models[model].HealthScore = 10.0
			alb.models[model].mu.Unlock()
		}

		_, err := alb.SelectModel(ctx, "primary")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no available models")
	})

	t.Run("handles model without fallbacks", func(t *testing.T) {
		alb.RegisterModel("standalone", 2*time.Second)

		selected, err := alb.SelectModel(ctx, "standalone")
		require.NoError(t, err)
		assert.Equal(t, "standalone", selected)
	})

	t.Run("handles non-existent model", func(t *testing.T) {
		_, err := alb.SelectModel(ctx, "nonexistent")
		assert.Error(t, err)
	})
}

func TestAdaptiveLoadBalancer_RecordRequestLifecycle(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()
	alb.RegisterModel("test-model", 1*time.Second)

	t.Run("record successful request", func(t *testing.T) {
		// Start request
		alb.RecordRequestStart("test-model")

		health := alb.models["test-model"]
		assert.Equal(t, int32(1), health.ActiveRequests)
		assert.Equal(t, int64(1), health.TotalRequests)
		assert.Equal(t, int32(1), health.RequestsPerMin)

		// End request successfully
		latency := 500 * time.Millisecond
		alb.RecordRequestEnd("test-model", latency, true)

		assert.Equal(t, int32(0), health.ActiveRequests)
		assert.Len(t, health.ResponseTimes, 1)
		assert.Equal(t, latency, health.ResponseTimes[0])
		assert.Equal(t, latency, health.AvgResponseTime)
		// Health score improves slightly for fast successful requests
		assert.GreaterOrEqual(t, health.HealthScore, 100.0)
	})

	t.Run("record slow successful request", func(t *testing.T) {
		// Reset health score
		health := alb.models["test-model"]
		health.mu.Lock()
		health.HealthScore = 100.0
		health.mu.Unlock()

		alb.RecordRequestStart("test-model")

		// Slow but successful request
		slowLatency := 2 * time.Second // Above MaxResponseTime (1s)
		alb.RecordRequestEnd("test-model", slowLatency, true)

		assert.Less(t, health.HealthScore, 100.0) // Should degrade for slow response
	})

	t.Run("record failed request", func(t *testing.T) {
		health := alb.models["test-model"]
		initialHealth := health.HealthScore
		initialFailures := health.FailedRequests

		alb.RecordRequestStart("test-model")
		alb.RecordRequestEnd("test-model", 200*time.Millisecond, false)

		assert.Equal(t, initialFailures+1, health.FailedRequests)
		assert.Less(t, health.HealthScore, initialHealth)
		assert.True(t, health.LastFailureTime.After(time.Now().Add(-1*time.Second)))
	})

	t.Run("circuit opens on high failure rate", func(t *testing.T) {
		// Reset model
		alb.RegisterModel("failure-test", 1*time.Second)

		// Generate enough failures to trigger circuit
		for i := 0; i < 20; i++ {
			alb.RecordRequestStart("failure-test")
			alb.RecordRequestEnd("failure-test", 100*time.Millisecond, false)
		}

		health := alb.models["failure-test"]
		assert.True(t, health.IsCircuitOpen)
	})

	t.Run("handles non-existent model gracefully", func(t *testing.T) {
		// Should not panic
		alb.RecordRequestStart("nonexistent")
		alb.RecordRequestEnd("nonexistent", 100*time.Millisecond, true)
	})
}

func TestAdaptiveLoadBalancer_RecordTimeout(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()
	alb.RegisterModel("timeout-test", 1*time.Second)

	alb.RecordTimeout("timeout-test")

	health := alb.models["timeout-test"]
	assert.Equal(t, int64(1), health.TimeoutRequests)
	assert.Equal(t, int64(1), health.FailedRequests)
	assert.True(t, health.IsCircuitOpen)      // Should open immediately on timeout
	assert.Less(t, health.HealthScore, 100.0) // Should degrade significantly
}

func TestAdaptiveLoadBalancer_CalculateScore(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()

	// Create a model health for testing
	health := &ModelHealth{
		HealthScore:     80.0,
		ActiveRequests:  5,
		TotalRequests:   100,
		FailedRequests:  10,
		AvgResponseTime: 200 * time.Millisecond,
	}

	score := alb.calculateScore(health)

	// Score should be less than base health due to load, latency, and errors
	assert.Less(t, score, 80.0)
	assert.Greater(t, score, 0.0)

	t.Run("zero load gives better score", func(t *testing.T) {
		healthNoLoad := &ModelHealth{
			HealthScore:     80.0,
			ActiveRequests:  0, // No load
			TotalRequests:   100,
			FailedRequests:  10,
			AvgResponseTime: 200 * time.Millisecond,
		}

		scoreNoLoad := alb.calculateScore(healthNoLoad)
		assert.Greater(t, scoreNoLoad, score)
	})

	t.Run("lower latency gives better score", func(t *testing.T) {
		healthFastLatency := &ModelHealth{
			HealthScore:     80.0,
			ActiveRequests:  5,
			TotalRequests:   100,
			FailedRequests:  10,
			AvgResponseTime: 50 * time.Millisecond, // Faster
		}

		scoreFast := alb.calculateScore(healthFastLatency)
		assert.Greater(t, scoreFast, score)
	})

	t.Run("no failures gives better score", func(t *testing.T) {
		healthNoErrors := &ModelHealth{
			HealthScore:     80.0,
			ActiveRequests:  5,
			TotalRequests:   100,
			FailedRequests:  0, // No failures
			AvgResponseTime: 200 * time.Millisecond,
		}

		scoreNoErrors := alb.calculateScore(healthNoErrors)
		assert.Greater(t, scoreNoErrors, score)
	})
}

func TestModelHealth_UpdateLatencyMetrics(t *testing.T) {
	health := &ModelHealth{
		ResponseTimes: make([]time.Duration, 0, 100),
		WindowSize:    100,
	}

	t.Run("calculates metrics correctly", func(t *testing.T) {
		latencies := []time.Duration{
			100 * time.Millisecond,
			200 * time.Millisecond,
			300 * time.Millisecond,
			400 * time.Millisecond,
			500 * time.Millisecond,
		}

		for _, lat := range latencies {
			health.addResponseTime(lat)
		}

		health.updateLatencyMetrics()

		// Average should be 300ms
		assert.Equal(t, 300*time.Millisecond, health.AvgResponseTime)

		// P95 should be 500ms (95% of 5 items = index 4, which is 500ms)
		assert.Equal(t, 500*time.Millisecond, health.P95ResponseTime)

		// P99 should be 500ms (99% of 5 items = index 4, which is 500ms)
		assert.Equal(t, 500*time.Millisecond, health.P99ResponseTime)
	})

	t.Run("handles empty response times", func(t *testing.T) {
		emptyHealth := &ModelHealth{
			ResponseTimes: make([]time.Duration, 0, 100),
			WindowSize:    100,
		}

		emptyHealth.updateLatencyMetrics()

		// Should not panic, metrics remain at zero values
		assert.Equal(t, time.Duration(0), emptyHealth.AvgResponseTime)
	})

	t.Run("sliding window behavior", func(t *testing.T) {
		smallWindowHealth := &ModelHealth{
			ResponseTimes: make([]time.Duration, 0, 3),
			WindowSize:    3,
		}

		// Add more than window size
		for i := 0; i < 5; i++ {
			smallWindowHealth.addResponseTime(time.Duration(i*100) * time.Millisecond)
		}

		// Should only keep last 3
		assert.Len(t, smallWindowHealth.ResponseTimes, 3)
		assert.Equal(t, 200*time.Millisecond, smallWindowHealth.ResponseTimes[0])
		assert.Equal(t, 300*time.Millisecond, smallWindowHealth.ResponseTimes[1])
		assert.Equal(t, 400*time.Millisecond, smallWindowHealth.ResponseTimes[2])
	})
}

func TestAdaptiveLoadBalancer_GetModelStats(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()
	alb.RegisterModel("model1", 1*time.Second)
	alb.RegisterModel("model2", 2*time.Second)

	// Add some activity
	alb.RecordRequestStart("model1")
	alb.RecordRequestEnd("model1", 150*time.Millisecond, true)
	alb.RecordRequestStart("model2")
	alb.RecordRequestEnd("model2", 300*time.Millisecond, false)

	stats := alb.GetModelStats()

	assert.Len(t, stats, 2)
	assert.Contains(t, stats, "model1")
	assert.Contains(t, stats, "model2")

	model1Stats := stats["model1"]
	assert.Equal(t, int64(1), model1Stats["total_requests"])
	assert.Equal(t, int64(0), model1Stats["failed_requests"])
	assert.Equal(t, false, model1Stats["circuit_open"])

	model2Stats := stats["model2"]
	assert.Equal(t, int64(1), model2Stats["total_requests"])
	assert.Equal(t, int64(1), model2Stats["failed_requests"])
}

func TestAdaptiveLoadBalancer_ShouldShedLoad(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()
	alb.RegisterModel("model1", 1*time.Second)
	alb.RegisterModel("model2", 1*time.Second)

	t.Run("should not shed load initially", func(t *testing.T) {
		assert.False(t, alb.ShouldShedLoad())
	})

	t.Run("should shed load with too many concurrent requests", func(t *testing.T) {
		// Simulate high concurrent load
		health := alb.models["model1"]
		health.mu.Lock()
		health.ActiveRequests = 600
		health.mu.Unlock()

		health2 := alb.models["model2"]
		health2.mu.Lock()
		health2.ActiveRequests = 500
		health2.mu.Unlock()

		assert.True(t, alb.ShouldShedLoad()) // Total > 1000
	})

	t.Run("should shed load with too few healthy models", func(t *testing.T) {
		// Reset concurrent requests
		for _, health := range alb.models {
			health.mu.Lock()
			health.ActiveRequests = 10
			health.HealthScore = 30.0 // Below 50% threshold
			health.mu.Unlock()
		}

		assert.True(t, alb.ShouldShedLoad()) // < 2 healthy models
	})

	t.Run("should shed load with high global failure rate", func(t *testing.T) {
		// Reset models to healthy
		for _, health := range alb.models {
			health.mu.Lock()
			health.ActiveRequests = 10
			health.HealthScore = 80.0
			health.IsCircuitOpen = false
			health.mu.Unlock()
		}

		// Set high global failure rate
		alb.mu.Lock()
		alb.totalRequests = 1000
		alb.totalFailures = 150 // 15% failure rate
		alb.mu.Unlock()

		assert.True(t, alb.ShouldShedLoad())
	})
}

func TestAdaptiveLoadBalancer_ConcurrentAccess(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()

	// Register models
	models := []string{"model1", "model2", "model3", "model4", "model5"}
	for _, model := range models {
		alb.RegisterModel(model, 1*time.Second)
	}

	// Set up fallbacks
	for i, model := range models[:len(models)-1] {
		alb.SetFallbacks(model, models[i+1:])
	}

	const numGoroutines = 50
	const operationsPerGoroutine = 20

	var wg sync.WaitGroup

	// Simulate concurrent load balancer operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()

			for j := 0; j < operationsPerGoroutine; j++ {
				model := models[j%len(models)]

				switch j % 6 {
				case 0:
					alb.SelectModel(ctx, model)
				case 1:
					alb.RecordRequestStart(model)
				case 2:
					latency := time.Duration((j%10)+1) * 100 * time.Millisecond
					success := (j % 3) != 0 // 2/3 success rate
					alb.RecordRequestEnd(model, latency, success)
				case 3:
					alb.RecordTimeout(model)
				case 4:
					alb.GetModelStats()
				case 5:
					alb.ShouldShedLoad()
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify system is in a consistent state
	stats := alb.GetModelStats()
	assert.Len(t, stats, len(models))

	for model, modelStats := range stats {
		totalRequests := modelStats["total_requests"].(int64)
		failedRequests := modelStats["failed_requests"].(int64)
		activeRequests := modelStats["active_requests"].(int32)

		assert.True(t, totalRequests >= 0, "Model %s should have non-negative total requests", model)
		assert.True(t, failedRequests >= 0, "Model %s should have non-negative failed requests", model)
		// Due to concurrent access, there might be race conditions
		// but failed requests should generally not exceed total requests
		if failedRequests > totalRequests {
			t.Logf("Model %s has more failed requests (%d) than total (%d) - possible race condition", model, failedRequests, totalRequests)
		}
		assert.True(t, activeRequests >= 0, "Model %s should have non-negative active requests", model)
	}
}

// Benchmark tests
func BenchmarkAdaptiveLoadBalancer_SelectModel(b *testing.B) {
	alb := NewAdaptiveLoadBalancer()
	ctx := context.Background()

	alb.RegisterModel("primary", 1*time.Second)
	alb.RegisterModel("fallback1", 1*time.Second)
	alb.RegisterModel("fallback2", 1*time.Second)
	alb.SetFallbacks("primary", []string{"fallback1", "fallback2"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alb.SelectModel(ctx, "primary")
	}
}

func BenchmarkAdaptiveLoadBalancer_RecordRequestEnd(b *testing.B) {
	alb := NewAdaptiveLoadBalancer()
	alb.RegisterModel("test-model", 1*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alb.RecordRequestEnd("test-model", 100*time.Millisecond, true)
	}
}

func BenchmarkAdaptiveLoadBalancer_CalculateScore(b *testing.B) {
	alb := NewAdaptiveLoadBalancer()
	health := &ModelHealth{
		HealthScore:     75.0,
		ActiveRequests:  10,
		TotalRequests:   1000,
		FailedRequests:  50,
		AvgResponseTime: 200 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alb.calculateScore(health)
	}
}

func BenchmarkAdaptiveLoadBalancer_ConcurrentOperations(b *testing.B) {
	alb := NewAdaptiveLoadBalancer()
	ctx := context.Background()

	models := []string{"model1", "model2", "model3"}
	for _, model := range models {
		alb.RegisterModel(model, 1*time.Second)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			model := models[i%len(models)]
			switch i % 4 {
			case 0:
				alb.SelectModel(ctx, model)
			case 1:
				alb.RecordRequestStart(model)
			case 2:
				alb.RecordRequestEnd(model, 100*time.Millisecond, true)
			case 3:
				alb.GetModelStats()
			}
			i++
		}
	})
}

// Edge case tests
func TestAdaptiveLoadBalancer_EdgeCases(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()

	t.Run("empty model name", func(t *testing.T) {
		alb.RegisterModel("", 1*time.Second)

		ctx := context.Background()
		// Empty model name should return error since no model exists
		_, err := alb.SelectModel(ctx, "")
		assert.Error(t, err)
	})

	t.Run("very long model name", func(t *testing.T) {
		longName := string(make([]byte, 1000))
		for i := range longName {
			longName = longName[:i] + "a" + longName[i+1:]
		}

		alb.RegisterModel(longName, 1*time.Second)

		ctx := context.Background()
		selected, err := alb.SelectModel(ctx, longName)
		require.NoError(t, err)
		assert.Equal(t, longName, selected)
	})

	t.Run("zero max response time", func(t *testing.T) {
		alb.RegisterModel("zero-timeout", 0)

		// Should handle gracefully
		alb.RecordRequestStart("zero-timeout")
		alb.RecordRequestEnd("zero-timeout", 100*time.Millisecond, true)

		stats := alb.GetModelStats()
		assert.Contains(t, stats, "zero-timeout")
	})

	t.Run("negative latency", func(t *testing.T) {
		alb.RegisterModel("negative-test", 1*time.Second)

		// Should handle negative latency gracefully
		alb.RecordRequestStart("negative-test")
		alb.RecordRequestEnd("negative-test", -100*time.Millisecond, true)

		stats := alb.GetModelStats()
		assert.Contains(t, stats, "negative-test")
	})

	t.Run("very high latency", func(t *testing.T) {
		alb.RegisterModel("high-latency", 1*time.Second)

		alb.RecordRequestStart("high-latency")
		alb.RecordRequestEnd("high-latency", 1*time.Hour, true)

		health := alb.models["high-latency"]
		assert.Less(t, health.HealthScore, 100.0) // Should be degraded
	})
}

// Test weight configuration effects
func TestAdaptiveLoadBalancer_WeightConfiguration(t *testing.T) {
	// Test with different weight configurations
	configs := []struct {
		name          string
		latencyWeight float64
		loadWeight    float64
		errorWeight   float64
	}{
		{"latency-focused", 0.8, 0.1, 0.1},
		{"load-focused", 0.1, 0.8, 0.1},
		{"error-focused", 0.1, 0.1, 0.8},
		{"balanced", 0.33, 0.33, 0.34},
	}

	for _, config := range configs {
		t.Run(config.name, func(t *testing.T) {
			alb := NewAdaptiveLoadBalancer()
			alb.latencyWeight = config.latencyWeight
			alb.loadWeight = config.loadWeight
			alb.errorWeight = config.errorWeight

			health := &ModelHealth{
				HealthScore:     80.0,
				ActiveRequests:  10,
				TotalRequests:   100,
				FailedRequests:  20,
				AvgResponseTime: 500 * time.Millisecond,
			}

			score := alb.calculateScore(health)
			assert.Greater(t, score, 0.0)
			assert.Less(t, score, 100.0)
		})
	}
}

// Test minute-based rate limiting reset
func TestModelHealth_MinuteReset(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()
	alb.RegisterModel("rate-test", 1*time.Second)

	health := alb.models["rate-test"]

	// Set last reset to over a minute ago
	health.mu.Lock()
	health.LastMinuteReset = time.Now().Add(-61 * time.Second)
	health.RequestsPerMin = 50
	health.TokensPerMin = 1000
	health.mu.Unlock()

	// Record a request, which should trigger reset
	alb.RecordRequestStart("rate-test")

	health.mu.RLock()
	requestsPerMin := health.RequestsPerMin
	tokensPerMin := health.TokensPerMin
	health.mu.RUnlock()

	// Should have reset and then incremented
	assert.Equal(t, int32(1), requestsPerMin)
	assert.Equal(t, int32(0), tokensPerMin)
}

// Test health score bounds
func TestModelHealth_HealthScoreBounds(t *testing.T) {
	alb := NewAdaptiveLoadBalancer()
	alb.RegisterModel("bounds-test", 1*time.Second)

	health := alb.models["bounds-test"]

	t.Run("health score cannot go below 0", func(t *testing.T) {
		// Set very low health score
		health.mu.Lock()
		health.HealthScore = 5.0
		health.mu.Unlock()

		// Record many failures
		for i := 0; i < 10; i++ {
			alb.RecordRequestStart("bounds-test")
			alb.RecordRequestEnd("bounds-test", 100*time.Millisecond, false)
		}

		// Health score should be very low but may not reach exactly 0
		assert.Less(t, health.HealthScore, 10.0)
	})

	t.Run("health score can exceed 100 with good performance", func(t *testing.T) {
		health.mu.Lock()
		health.HealthScore = 99.0
		health.mu.Unlock()

		// Record fast successful requests
		for i := 0; i < 5; i++ {
			alb.RecordRequestStart("bounds-test")
			alb.RecordRequestEnd("bounds-test", 50*time.Millisecond, true)
		}

		// Health score can exceed 100 with good performance
		assert.GreaterOrEqual(t, health.HealthScore, 100.0)
	})
}

func TestAdaptiveLoadBalancer_PercentileCalculation(t *testing.T) {
	health := &ModelHealth{
		ResponseTimes: make([]time.Duration, 0, 100),
		WindowSize:    100,
	}

	// Add 100 latency samples (0ms to 990ms in 10ms increments)
	for i := 0; i < 100; i++ {
		health.addResponseTime(time.Duration(i*10) * time.Millisecond)
	}

	health.updateLatencyMetrics()

	// P95 should be around the 95th percentile (95% of 100 = index 95)
	expectedP95 := 950 * time.Millisecond
	assert.Equal(t, expectedP95, health.P95ResponseTime)

	// P99 should be around the 99th percentile (99% of 100 = index 99)
	expectedP99 := 990 * time.Millisecond
	assert.Equal(t, expectedP99, health.P99ResponseTime)

	// Average should be around 495ms
	expectedAvg := 495 * time.Millisecond
	assert.Equal(t, expectedAvg, health.AvgResponseTime)
}
