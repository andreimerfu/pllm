package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestBudgetCache(t *testing.T) {
	// Setup embedded Redis server for testing
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	// Setup test Redis client connected to miniredis
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	logger, _ := zap.NewDevelopment()
	cache := NewBudgetCache(client, logger, time.Minute)

	// Clean up any existing test data
	defer func() {
		client.FlushDB(ctx)
	}()

	t.Run("CheckBudgetAvailable_CacheMiss", func(t *testing.T) {
		// First call should be cache miss and return optimistic true
		available, err := cache.CheckBudgetAvailable(ctx, "user", "test-user", 5.0)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !available {
			t.Error("Expected optimistic approval on cache miss")
		}
	})

	t.Run("UpdateAndCheckBudgetCache", func(t *testing.T) {
		// Update cache
		err := cache.UpdateBudgetCache(ctx, "user", "test-user", 100.0, 25.0, 125.0, false)
		if err != nil {
			t.Errorf("Failed to update budget cache: %v", err)
		}

		// Check budget available (should hit cache this time)
		available, err := cache.CheckBudgetAvailable(ctx, "user", "test-user", 50.0)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !available {
			t.Error("Expected budget to be available (100 - 50 >= 0)")
		}

		// Check budget not available
		available, err = cache.CheckBudgetAvailable(ctx, "user", "test-user", 150.0)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if available {
			t.Error("Expected budget to be unavailable (100 - 150 < 0)")
		}
	})

	t.Run("IncrementSpent", func(t *testing.T) {
		// First, set up a budget
		err := cache.UpdateBudgetCache(ctx, "user", "test-user-2", 100.0, 0.0, 100.0, false)
		if err != nil {
			t.Errorf("Failed to update budget cache: %v", err)
		}

		// Increment spent amount
		err = cache.IncrementSpent(ctx, "user", "test-user-2", 25.5)
		if err != nil {
			t.Errorf("Failed to increment spent: %v", err)
		}

		// Verify the increment worked by checking Redis directly
		spentKey := "budget:user:test-user-2:spent"
		spent, err := client.Get(ctx, spentKey).Float64()
		if err != nil {
			t.Errorf("Failed to get spent amount: %v", err)
		}
		if spent != 25.5 {
			t.Errorf("Expected spent to be 25.5, got %f", spent)
		}
	})

	t.Run("InvalidateBudgetCache", func(t *testing.T) {
		// Set up cache
		err := cache.UpdateBudgetCache(ctx, "user", "test-user-3", 100.0, 50.0, 150.0, false)
		if err != nil {
			t.Errorf("Failed to update budget cache: %v", err)
		}

		// Verify it exists
		status, err := cache.GetBudgetStats(ctx, "user", "test-user-3")
		if err != nil {
			t.Errorf("Failed to get budget stats: %v", err)
		}
		if status == nil {
			t.Error("Expected budget status to exist")
		}

		// Invalidate cache
		err = cache.InvalidateBudgetCache(ctx, "user", "test-user-3")
		if err != nil {
			t.Errorf("Failed to invalidate cache: %v", err)
		}

		// Verify it's gone
		status, err = cache.GetBudgetStats(ctx, "user", "test-user-3")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if status != nil {
			t.Error("Expected budget status to be nil after invalidation")
		}
	})

	t.Run("SetupBudgetLimits", func(t *testing.T) {
		limits := map[string]map[string]float64{
			"user": {
				"user-1": 100.0,
				"user-2": 200.0,
			},
			"team": {
				"team-1": 500.0,
			},
		}

		err := cache.SetupBudgetLimits(ctx, limits)
		if err != nil {
			t.Errorf("Failed to setup budget limits: %v", err)
		}

		// Verify limits were set
		status, err := cache.GetBudgetStats(ctx, "user", "user-1")
		if err != nil {
			t.Errorf("Failed to get budget stats: %v", err)
		}
		if status == nil {
			t.Error("Expected budget status to exist")
			return
		}
		if status.Limit != 100.0 {
			t.Errorf("Expected limit to be 100.0, got %f", status.Limit)
		}
	})
}
