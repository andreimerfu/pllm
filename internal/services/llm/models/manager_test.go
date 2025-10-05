package models

import (
	"context"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestAdaptiveBreaker REMOVED
// AdaptiveBreaker was removed in favor of simple circuit breaker in Manager

// TestAdaptiveLoadBalancer REMOVED
// Routing is now handled by internal/services/llm/models/routing package

func TestModelManager_AdaptiveRouting(t *testing.T) {
	logger := zap.NewNop()
	router := config.RouterSettings{
		RoutingStrategy: "latency-based",
	}

	manager := NewModelManager(logger, router, nil)

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
	manager := NewModelManager(logger, router, nil)

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
