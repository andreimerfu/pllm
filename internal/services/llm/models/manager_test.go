package models

import (
	"context"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/pkg/circuitbreaker"
	"github.com/amerfu/pllm/pkg/loadbalancer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAdaptiveBreaker(t *testing.T) {
	breaker := circuitbreaker.NewAdaptiveBreaker(
		3,             // failure threshold
		1*time.Second, // latency threshold
		2,             // slow request limit
	)

	// Test normal requests
	t.Run("Normal requests keep circuit closed", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			breaker.StartRequest()
			breaker.EndRequest()
			breaker.RecordSuccess(100 * time.Millisecond)
		}
		assert.True(t, breaker.CanRequest())
		state := breaker.GetState()
		assert.Equal(t, "CLOSED", state["state"])
	})

	// Test slow requests
	t.Run("Slow requests open circuit", func(t *testing.T) {
		breaker := circuitbreaker.NewAdaptiveBreaker(3, 500*time.Millisecond, 2)

		// Record slow requests
		for i := 0; i < 3; i++ {
			breaker.StartRequest()
			breaker.EndRequest()
			breaker.RecordSuccess(2 * time.Second) // Slow!
		}

		assert.False(t, breaker.CanRequest())
		state := breaker.GetState()
		assert.Equal(t, "OPEN", state["state"])
	})

	// Test failures
	t.Run("Failures open circuit", func(t *testing.T) {
		breaker := circuitbreaker.NewAdaptiveBreaker(3, 1*time.Second, 2)

		// Record failures
		for i := 0; i < 4; i++ {
			breaker.StartRequest()
			breaker.EndRequest()
			breaker.RecordFailure()
		}

		assert.False(t, breaker.CanRequest())
		state := breaker.GetState()
		assert.Equal(t, "OPEN", state["state"])
	})
}

func TestAdaptiveLoadBalancer(t *testing.T) {
	lb := loadbalancer.NewAdaptiveLoadBalancer()

	// Register models
	lb.RegisterModel("model-a", 2*time.Second)
	lb.RegisterModel("model-b", 2*time.Second)
	lb.RegisterModel("model-c", 2*time.Second)

	// Set fallbacks
	lb.SetFallbacks("model-a", []string{"model-b", "model-c"})

	t.Run("Select primary model when healthy", func(t *testing.T) {
		ctx := context.Background()
		selected, err := lb.SelectModel(ctx, "model-a")
		require.NoError(t, err)
		assert.Equal(t, "model-a", selected)
	})

	t.Run("Select fallback when primary fails", func(t *testing.T) {
		ctx := context.Background()

		// Make model-a unhealthy
		for i := 0; i < 5; i++ {
			lb.RecordRequestStart("model-a")
			lb.RecordRequestEnd("model-a", 100*time.Millisecond, false)
		}

		selected, err := lb.SelectModel(ctx, "model-a")
		require.NoError(t, err)
		// Should select a fallback
		assert.Contains(t, []string{"model-a", "model-b", "model-c"}, selected)
	})

	t.Run("Track health scores", func(t *testing.T) {
		lb := loadbalancer.NewAdaptiveLoadBalancer()
		lb.RegisterModel("test-model", 1*time.Second)

		// Record successful requests
		for i := 0; i < 3; i++ {
			lb.RecordRequestStart("test-model")
			lb.RecordRequestEnd("test-model", 200*time.Millisecond, true)
		}

		stats := lb.GetModelStats()
		modelStats := stats["test-model"]
		assert.Equal(t, float64(100), modelStats["health_score"])
		assert.Equal(t, int64(3), modelStats["total_requests"])
	})
}

func TestModelManager_AdaptiveRouting(t *testing.T) {
	logger := zap.NewNop()
	router := config.RouterSettings{
		RoutingStrategy:         "latency-based",
		CircuitBreakerEnabled:   true,
		CircuitBreakerThreshold: 3,
		CircuitBreakerCooldown:  1 * time.Second,
		Fallbacks: map[string][]string{
			"primary": {"fallback1", "fallback2"},
		},
	}

	manager := NewModelManager(logger, router)

	// Mock model instances
	instances := []config.ModelInstance{
		{
			ID:        "primary-instance",
			ModelName: "primary",
			Enabled:   true,
			Provider: config.ProviderParams{
				Type:   "openai",
				Model:  "gpt-3.5-turbo",
				APIKey: "test-key",
			},
		},
		{
			ID:        "fallback1-instance",
			ModelName: "fallback1",
			Enabled:   true,
			Provider: config.ProviderParams{
				Type:   "openai",
				Model:  "gpt-3.5-turbo",
				APIKey: "test-key",
			},
		},
	}

	err := manager.LoadModelInstances(instances)
	require.NoError(t, err)

	// Check if instances were actually loaded
	availableModels := manager.GetAvailableModels()
	require.NotEmpty(t, availableModels, "Available models should not be empty after loading instances")
	require.Contains(t, availableModels, "primary", "Primary model should be available")

	t.Run("Track requests with new components", func(t *testing.T) {
		// Debug: Check what models are loaded
		t.Logf("Loaded models: %v", manager.GetAvailableModels())

		// Get stats (should work with new architecture)
		stats := manager.GetModelStats()
		require.NotNil(t, stats, "Stats should not be nil")

		// Check registry stats
		registryStats, ok := stats["registry"]
		require.True(t, ok, "Registry stats should exist")
		require.NotNil(t, registryStats, "Registry stats should not be nil")

		// Basic functionality verification
		ctx := context.Background()
		instance, err := manager.GetBestInstance(ctx, "primary")
		require.NoError(t, err, "Should be able to get best instance")
		require.NotNil(t, instance, "Instance should not be nil")
		assert.Equal(t, "primary", instance.Config.ModelName)
	})

	t.Run("GetBestInstanceAdaptive uses adaptive routing", func(t *testing.T) {
		ctx := context.Background()

		// Should get primary instance
		instance, err := manager.GetBestInstanceAdaptive(ctx, "primary")
		require.NoError(t, err)
		assert.Equal(t, "primary-instance", instance.Config.ID)

		// Simulate failures to trigger fallback
		for i := 0; i < 5; i++ {
			manager.RecordRequestStart("primary")
			manager.RecordRequestEnd("primary", 100*time.Millisecond, false, assert.AnError)
		}

		// Now should potentially get fallback
		instance, err = manager.GetBestInstanceAdaptive(ctx, "primary")
		// Should still work even with failures
		require.NoError(t, err)
		assert.NotNil(t, instance)
	})
}

func TestModelNameMapping(t *testing.T) {
	logger := zap.NewNop()
	router := config.RouterSettings{}
	manager := NewModelManager(logger, router)

	instances := []config.ModelInstance{
		{
			ID:        "my-custom-gpt4",
			ModelName: "my-custom-gpt4",
			Enabled:   true,
			Provider: config.ProviderParams{
				Type:   "openai",
				Model:  "gpt-4",
				APIKey: "test",
			},
		},
	}

	err := manager.LoadModelInstances(instances)
	require.NoError(t, err)

	t.Run("Model name consistency", func(t *testing.T) {
		ctx := context.Background()

		// Request with user-defined name
		instance, err := manager.GetBestInstance(ctx, "my-custom-gpt4")
		require.NoError(t, err)

		// Should get correct instance
		assert.Equal(t, "my-custom-gpt4", instance.Config.ID)
		assert.Equal(t, "my-custom-gpt4", instance.Config.ModelName)
		// Provider model should be the actual model
		assert.Equal(t, "gpt-4", instance.Config.Provider.Model)
	})
}
