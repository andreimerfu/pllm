package redis

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestEventPublisher(t *testing.T) {
	// Setup test Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1, // Use DB 1 for tests
	})
	defer client.Close()

	// Ping Redis to ensure it's available
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for testing, skipping")
	}

	logger, _ := zap.NewDevelopment()
	publisher := NewEventPublisher(client, logger)

	t.Run("PublishUsageEvent", func(t *testing.T) {
		err := publisher.PublishUsageEvent(
			ctx,
			"user-123",
			"key-456",
			"gpt-4",
			100, 200, 0.005,
			150*time.Millisecond,
		)

		if err != nil {
			t.Errorf("Failed to publish usage event: %v", err)
		}

		// Verify event was published by checking stream length
		length, err := client.XLen(ctx, "usage_events").Result()
		if err != nil {
			t.Errorf("Failed to get stream length: %v", err)
		}
		if length == 0 {
			t.Error("Expected events to be published to stream")
		}
	})

	t.Run("PublishBudgetEvent", func(t *testing.T) {
		err := publisher.PublishBudgetEvent(
			ctx,
			"budget-789",
			"user-123",
			"user",
			10.50,
			"exceeded",
		)

		if err != nil {
			t.Errorf("Failed to publish budget event: %v", err)
		}
	})

	// Cleanup test data
	client.Del(ctx, "usage_events", "budget_events")
}

func TestGenerateEventID(t *testing.T) {
	id1 := generateEventID()
	id2 := generateEventID()

	if id1 == id2 {
		t.Error("Expected unique event IDs")
	}

	if len(id1) < 10 {
		t.Error("Expected event ID to have reasonable length")
	}
}
