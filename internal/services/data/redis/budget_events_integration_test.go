package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestBudgetEvents_Integration(t *testing.T) {
	redisClient, cleanup := testutil.NewTestRedis(t)
	defer cleanup()

	logger := zap.NewNop()
	ctx := context.Background()

	// Clear any existing events
	redisClient.Del(ctx, "budget_events", "usage_events")

	publisher := NewEventPublisher(redisClient, logger)

	t.Run("PublishBudgetEvent_Check", func(t *testing.T) {
		budgetID := uuid.New().String()
		entityID := uuid.New().String()

		err := publisher.PublishBudgetEvent(ctx, budgetID, entityID, "key", 25.50, "check")
		require.NoError(t, err)

		// Verify event was published
		events, err := redisClient.XRead(ctx, &redis.XReadArgs{
			Streams: []string{"budget_events", "0"},
			Count:   1,
			Block:   100 * time.Millisecond,
		}).Result()
		require.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Len(t, events[0].Messages, 1)

		// Parse event data
		msg := events[0].Messages[0]
		eventData := msg.Values["data"].(string)

		var event Event
		err = json.Unmarshal([]byte(eventData), &event)
		require.NoError(t, err)

		assert.Equal(t, EventTypeBudget, event.Type)
		assert.Equal(t, budgetID, event.Data["budget_id"])
		assert.Equal(t, entityID, event.Data["entity_id"])
		assert.Equal(t, "key", event.Data["entity_type"])
		assert.Equal(t, 25.50, event.Data["amount"])
		assert.Equal(t, "check", event.Data["event_type"])
	})

	t.Run("PublishBudgetEvent_Update", func(t *testing.T) {
		budgetID := uuid.New().String()
		entityID := uuid.New().String()

		err := publisher.PublishBudgetEvent(ctx, budgetID, entityID, "team", 150.0, "update")
		require.NoError(t, err)

		// Read latest event
		events, err := redisClient.XRevRange(ctx, "budget_events", "+", "-").Result()
		require.NoError(t, err)
		require.NotEmpty(t, events)

		// Parse latest event
		latestMsg := events[0]
		eventData := latestMsg.Values["data"].(string)

		var event Event
		err = json.Unmarshal([]byte(eventData), &event)
		require.NoError(t, err)

		assert.Equal(t, EventTypeBudget, event.Type)
		assert.Equal(t, "team", event.Data["entity_type"])
		assert.Equal(t, "update", event.Data["event_type"])
	})

	t.Run("PublishBudgetEvent_Exceeded", func(t *testing.T) {
		budgetID := uuid.New().String()
		entityID := uuid.New().String()

		err := publisher.PublishBudgetEvent(ctx, budgetID, entityID, "user", 100.0, "exceeded")
		require.NoError(t, err)

		// Read latest event
		events, err := redisClient.XRevRange(ctx, "budget_events", "+", "-").Result()
		require.NoError(t, err)
		require.NotEmpty(t, events)

		latestMsg := events[0]
		eventData := latestMsg.Values["data"].(string)

		var event Event
		err = json.Unmarshal([]byte(eventData), &event)
		require.NoError(t, err)

		assert.Equal(t, "exceeded", event.Data["event_type"])
		assert.Equal(t, 100.0, event.Data["amount"])
	})

	t.Run("PublishBudgetEvent_Alert", func(t *testing.T) {
		budgetID := uuid.New().String()
		entityID := uuid.New().String()

		err := publisher.PublishBudgetEvent(ctx, budgetID, entityID, "key", 80.0, "alert")
		require.NoError(t, err)

		events, err := redisClient.XRevRange(ctx, "budget_events", "+", "-").Result()
		require.NoError(t, err)
		require.NotEmpty(t, events)

		latestMsg := events[0]
		eventData := latestMsg.Values["data"].(string)

		var event Event
		err = json.Unmarshal([]byte(eventData), &event)
		require.NoError(t, err)

		assert.Equal(t, "alert", event.Data["event_type"])
		assert.Equal(t, "key", event.Data["entity_type"])
	})

	t.Run("Multiple_BudgetEvents_Ordered", func(t *testing.T) {
		// Clear events
		redisClient.Del(ctx, "budget_events")

		budgetID := uuid.New().String()
		entityID := uuid.New().String()

		eventTypes := []string{"check", "check", "update", "check", "alert", "exceeded"}

		// Publish events in order
		for i, eventType := range eventTypes {
			err := publisher.PublishBudgetEvent(ctx, budgetID, entityID, "key", float64(i*10), eventType)
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond) // Small delay to ensure ordering
		}

		// Read all events
		events, err := redisClient.XRange(ctx, "budget_events", "-", "+").Result()
		require.NoError(t, err)
		assert.Len(t, events, len(eventTypes))

		// Verify order and content
		for i, msg := range events {
			eventData := msg.Values["data"].(string)
			var event Event
			err = json.Unmarshal([]byte(eventData), &event)
			require.NoError(t, err)

			assert.Equal(t, eventTypes[i], event.Data["event_type"])
			assert.Equal(t, float64(i*10), event.Data["amount"])
		}
	})

	t.Run("BudgetEvent_Consumption", func(t *testing.T) {
		// Clear events
		redisClient.Del(ctx, "budget_events")

		// Publish events
		budgetID := uuid.New().String()
		entityID := uuid.New().String()

		for i := 0; i < 5; i++ {
			err := publisher.PublishBudgetEvent(ctx, budgetID, entityID, "user", float64(i*5), "check")
			require.NoError(t, err)
		}

		// Create consumer group
		err := redisClient.XGroupCreate(ctx, "budget_events", "test_consumer_group", "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			require.NoError(t, err)
		}

		// Read events as consumer
		streams, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    "test_consumer_group",
			Consumer: "test_consumer_1",
			Streams:  []string{"budget_events", ">"},
			Count:    10,
			Block:    100 * time.Millisecond,
		}).Result()
		require.NoError(t, err)

		assert.Len(t, streams, 1)
		assert.Len(t, streams[0].Messages, 5)

		// Acknowledge events
		for _, msg := range streams[0].Messages {
			err := redisClient.XAck(ctx, "budget_events", "test_consumer_group", msg.ID).Err()
			require.NoError(t, err)
		}

		// Verify pending count is 0
		pending, err := redisClient.XPending(ctx, "budget_events", "test_consumer_group").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), pending.Count)
	})
}

func TestBudgetEvents_WithBudgetCache(t *testing.T) {
	redisClient, cleanup := testutil.NewTestRedis(t)
	defer cleanup()

	logger := zap.NewNop()
	ctx := context.Background()

	publisher := NewEventPublisher(redisClient, logger)
	budgetCache := NewBudgetCache(redisClient, logger, 5*time.Minute)

	t.Run("BudgetCheck_PublishEvent_OnExceeded", func(t *testing.T) {
		entityType := "key"
		entityID := uuid.New().String()

		// Setup budget (available=5.0, spent=95.0, limit=100.0)
		err := budgetCache.UpdateBudgetCache(ctx, entityType, entityID, 5.0, 95.0, 100.0, false)
		require.NoError(t, err)

		// Check budget - within limit
		available, err := budgetCache.CheckBudgetAvailable(ctx, entityType, entityID, 3.0)
		require.NoError(t, err)
		assert.True(t, available)

		// Publish check event
		err = publisher.PublishBudgetEvent(ctx, "budget-1", entityID, entityType, 98.0, "check")
		require.NoError(t, err)

		// Check budget - would exceed
		available, err = budgetCache.CheckBudgetAvailable(ctx, entityType, entityID, 10.0)
		require.NoError(t, err)
		assert.False(t, available)

		// Publish exceeded event
		err = publisher.PublishBudgetEvent(ctx, "budget-1", entityID, entityType, 105.0, "exceeded")
		require.NoError(t, err)

		// Verify events were published
		events, err := redisClient.XRange(ctx, "budget_events", "-", "+").Result()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(events), 2)
	})

	t.Run("BudgetUpdate_PublishEvent", func(t *testing.T) {
		entityType := "team"
		entityID := uuid.New().String()

		// Initial setup
		err := budgetCache.UpdateBudgetCache(ctx, entityType, entityID, 100.0, 0.0, 100.0, false)
		require.NoError(t, err)

		// Publish initial event
		err = publisher.PublishBudgetEvent(ctx, "budget-2", entityID, entityType, 0.0, "update")
		require.NoError(t, err)

		// Increment spending
		err = budgetCache.IncrementSpent(ctx, entityType, entityID, 25.0)
		require.NoError(t, err)

		// Publish update event
		err = publisher.PublishBudgetEvent(ctx, "budget-2", entityID, entityType, 25.0, "update")
		require.NoError(t, err)

		// Verify budget status
		status, err := budgetCache.GetBudgetStats(ctx, entityType, entityID)
		require.NoError(t, err)
		assert.NotNil(t, status)
	})

	t.Run("BudgetAlert_PublishEvent_At80Percent", func(t *testing.T) {
		entityType := "key"
		entityID := uuid.New().String()

		// Setup budget (available=100.0, spent=0.0, limit=100.0)
		// Note: For this test we don't increment, we just set up different states
		err := budgetCache.UpdateBudgetCache(ctx, entityType, entityID, 21.0, 79.0, 100.0, false)
		require.NoError(t, err)

		// Check if we should alert at 79%
		status, err := budgetCache.GetBudgetStats(ctx, entityType, entityID)
		require.NoError(t, err)

		percentUsed := (status.Spent / status.Limit) * 100
		t.Logf("Budget at %.2f%%, no alert yet", percentUsed)
		assert.Less(t, percentUsed, 80.0)

		// Update to 81% (should alert)
		err = budgetCache.UpdateBudgetCache(ctx, entityType, entityID, 19.0, 81.0, 100.0, false)
		require.NoError(t, err)

		status, err = budgetCache.GetBudgetStats(ctx, entityType, entityID)
		require.NoError(t, err)
		
		percentUsed = (status.Spent / status.Limit) * 100
		if percentUsed >= 80 {
			// Publish alert event
			err = publisher.PublishBudgetEvent(ctx, "budget-3", entityID, entityType, status.Spent, "alert")
			require.NoError(t, err)
			t.Logf("Budget at %.2f%%, alert published", percentUsed)
		}

		// Verify alert event exists
		events, err := redisClient.XRevRange(ctx, "budget_events", "+", "-").Result()
		require.NoError(t, err)
		require.NotEmpty(t, events)

		// Find alert event
		var alertEvent *Event
		for _, msg := range events {
			eventData := msg.Values["data"].(string)
			var event Event
			_ = json.Unmarshal([]byte(eventData), &event)
			if event.Data["event_type"] == "alert" {
				alertEvent = &event
				break
			}
		}

		require.NotNil(t, alertEvent)
		assert.Equal(t, "alert", alertEvent.Data["event_type"])
	})
}

func TestBudgetEvents_Performance(t *testing.T) {
	redisClient, cleanup := testutil.NewTestRedis(t)
	defer cleanup()

	logger := zap.NewNop()
	ctx := context.Background()

	publisher := NewEventPublisher(redisClient, logger)

	t.Run("HighVolume_EventPublishing", func(t *testing.T) {
		// Clear events
		redisClient.Del(ctx, "budget_events")

		const numEvents = 1000
		budgetID := uuid.New().String()

		start := time.Now()

		for i := 0; i < numEvents; i++ {
			entityID := uuid.New().String()
			err := publisher.PublishBudgetEvent(ctx, budgetID, entityID, "key", float64(i), "check")
			require.NoError(t, err)
		}

		duration := time.Since(start)

		t.Logf("Published %d events in %v (avg: %v per event)", 
			numEvents, duration, duration/numEvents)

		// Should complete in reasonable time (< 2 seconds for 1000 events)
		assert.Less(t, duration.Seconds(), 2.0)

		// Verify all events were published
		count, err := redisClient.XLen(ctx, "budget_events").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(numEvents), count)
	})

	t.Run("Concurrent_EventPublishing", func(t *testing.T) {
		// Clear events
		redisClient.Del(ctx, "budget_events")

		const numGoroutines = 50
		const eventsPerGoroutine = 20

		start := time.Now()

		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines*eventsPerGoroutine)

		for g := 0; g < numGoroutines; g++ {
			go func(goroutineID int) {
				budgetID := uuid.New().String()
				for i := 0; i < eventsPerGoroutine; i++ {
					entityID := uuid.New().String()
					err := publisher.PublishBudgetEvent(ctx, budgetID, entityID, "key", float64(i), "check")
					if err != nil {
						errors <- err
						return
					}
				}
				done <- true
			}(g)
		}

		// Wait for all goroutines
		for g := 0; g < numGoroutines; g++ {
			select {
			case <-done:
				// Success
			case err := <-errors:
				t.Fatalf("Error publishing event: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("Timeout waiting for concurrent publishing")
			}
		}

		duration := time.Since(start)
		totalEvents := numGoroutines * eventsPerGoroutine

		t.Logf("Published %d events concurrently in %v (avg: %v per event)", 
			totalEvents, duration, duration/time.Duration(totalEvents))

		// Verify all events were published
		count, err := redisClient.XLen(ctx, "budget_events").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(totalEvents), count)
	})
}
