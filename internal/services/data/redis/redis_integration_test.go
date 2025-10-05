package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/infrastructure/logger"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestRedisIntegration tests Redis integration for banking-grade reliability
func TestRedisIntegration(t *testing.T) {
	// Setup Redis client for testing using test container
	redisClient, cleanup := testutil.NewTestRedis(t)
	defer cleanup()

	// Test Redis connectivity
	ctx := context.Background()
	err := redisClient.Ping(ctx).Err()
	require.NoError(t, err, "Redis should be available for testing")

	// Clear test database
	redisClient.FlushDB(ctx)

	log := logger.NewLogger("test", "info")

	t.Run("Usage Queue Operations", func(t *testing.T) {
		testUsageQueue(t, redisClient, log)
	})

	t.Run("Budget Cache Operations", func(t *testing.T) {
		testBudgetCache(t, redisClient, log)
	})

	t.Run("Event Publishing", func(t *testing.T) {
		testEventPublishing(t, redisClient, log)
	})

	t.Run("High Concurrency", func(t *testing.T) {
		testRedisConcurrency(t, redisClient, log)
	})

	t.Run("Failover Simulation", func(t *testing.T) {
		testRedisFailover(t, redisClient, log)
	})
}

func testUsageQueue(t *testing.T, redisClient *redis.Client, log *zap.Logger) {
	queue := NewUsageQueue(&UsageQueueConfig{
		Client:     redisClient,
		Logger:     log,
		QueueName:  "test_usage_queue",
		BatchSize:  10,
		MaxRetries: 3,
	})

	// Test single usage record
	t.Run("Single Usage Record", func(t *testing.T) {
		usageRecord := &UsageRecord{
			KeyID:        "test-key-123",
			Model:        "gpt-4",
			InputTokens:  100,
			OutputTokens: 50,
			TotalCost:    0.002,
			Timestamp:    time.Now(),
		}

		err := queue.EnqueueUsage(context.Background(), usageRecord)
		assert.NoError(t, err, "Should enqueue usage record")

		// Verify record is in queue
		count, err := redisClient.LLen(context.Background(), "test_usage_queue").Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count, "Queue should contain 1 record")
	})

	// Test batch operations
	t.Run("Batch Usage Records", func(t *testing.T) {
		// Clear queue
		redisClient.Del(context.Background(), "test_usage_queue")

		// Add multiple records
		for i := 0; i < 25; i++ {
			usageRecord := &UsageRecord{
				KeyID:        fmt.Sprintf("test-key-%d", i),
				Model:        "gpt-4",
				InputTokens:  100 + i,
				OutputTokens: 50 + i,
				TotalCost:    0.002 + float64(i)*0.001,
				Timestamp:    time.Now(),
			}

			err := queue.EnqueueUsage(context.Background(), usageRecord)
			assert.NoError(t, err)
		}

		// Verify all records are queued
		count, err := redisClient.LLen(context.Background(), "test_usage_queue").Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(25), count, "Queue should contain 25 records")
	})

	// Test queue performance under load
	t.Run("Queue Performance", func(t *testing.T) {
		redisClient.Del(context.Background(), "test_usage_queue_perf")
		
		perfQueue := NewUsageQueue(&UsageQueueConfig{
			Client:     redisClient,
			Logger:     log,
			QueueName:  "test_usage_queue_perf",
			BatchSize:  50,
			MaxRetries: 3,
		})

		const numRecords = 1000
		start := time.Now()

		// Enqueue records concurrently
		var wg sync.WaitGroup
		for i := 0; i < numRecords; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				usageRecord := &UsageRecord{
					KeyID:     fmt.Sprintf("perf-test-%d", idx),
					Model:     "gpt-4",
					TotalCost: 0.002,
					Timestamp: time.Now(),
				}
				err := perfQueue.EnqueueUsage(context.Background(), usageRecord)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		// Verify performance
		count, err := redisClient.LLen(context.Background(), "test_usage_queue_perf").Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(numRecords), count)

		throughput := float64(numRecords) / duration.Seconds()
		assert.True(t, throughput > 500, "Should achieve >500 records/sec, got %.2f", throughput)
		
		t.Logf("Usage Queue Performance: %d records in %v (%.2f records/sec)", numRecords, duration, throughput)
	})
}

func testBudgetCache(t *testing.T, redisClient *redis.Client, log *zap.Logger) {
	cache := NewBudgetCache(redisClient, log, 5*time.Minute)

	t.Run("Budget Cache Operations", func(t *testing.T) {
		entityType := "key"
		entityID := "test-budget-key"
		budget := 100.0
		spent := 0.0

		// Set budget
		err := cache.UpdateBudgetCache(context.Background(), entityType, entityID, budget, spent, budget, false)
		assert.NoError(t, err, "Should set budget")

		// Check budget availability
		available, err := cache.CheckBudgetAvailable(context.Background(), entityType, entityID, 25.5)
		assert.NoError(t, err, "Should check budget availability")
		assert.True(t, available, "Budget should be available")

		// Update usage
		usage := 25.5
		err = cache.IncrementSpent(context.Background(), entityType, entityID, usage)
		assert.NoError(t, err, "Should increment usage")

		// Update the budget cache to reflect the new spent amount
		newSpent := spent + usage
		newAvailable := budget - newSpent
		err = cache.UpdateBudgetCache(context.Background(), entityType, entityID, newAvailable, newSpent, budget, false)
		assert.NoError(t, err, "Should update budget cache")

		// Check budget availability after usage
		available, err = cache.CheckBudgetAvailable(context.Background(), entityType, entityID, 80.0)
		assert.NoError(t, err, "Should check budget availability")
		assert.False(t, available, "Budget should not be available for 80.0 after spending 25.5")
	})

	t.Run("Budget Exhaustion", func(t *testing.T) {
		entityType := "key"
		entityID := "test-budget-exhaustion"
		budget := 10.0
		spent := 10.0

		err := cache.UpdateBudgetCache(context.Background(), entityType, entityID, 0, spent, budget, true)
		assert.NoError(t, err)

		// Check if budget is exhausted
		available, err := cache.CheckBudgetAvailable(context.Background(), entityType, entityID, 1.0)
		assert.NoError(t, err)
		assert.False(t, available, "Budget should be exhausted")
	})

	t.Run("Concurrent Budget Operations", func(t *testing.T) {
		entityType := "key"
		entityID := "test-concurrent-budget"
		budget := 1000.0

		err := cache.UpdateBudgetCache(context.Background(), entityType, entityID, budget, 0, budget, false)
		assert.NoError(t, err)

		// Concurrent usage increments
		const numOperations = 100
		const usagePerOp = 1.0

		var wg sync.WaitGroup
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := cache.IncrementSpent(context.Background(), entityType, entityID, usagePerOp)
				assert.NoError(t, err)
			}()
		}

		wg.Wait()

		// Get the spent value directly from Redis (IncrementSpent uses separate key)
		spentKey := fmt.Sprintf("budget:%s:%s:spent", entityType, entityID)
		spentStr, err := redisClient.Get(context.Background(), spentKey).Result()
		assert.NoError(t, err)

		spent, err := strconv.ParseFloat(spentStr, 64)
		assert.NoError(t, err)
		assert.InDelta(t, float64(numOperations)*usagePerOp, spent, 1.0, "Concurrent operations should be tracked")
	})
}

func testEventPublishing(t *testing.T, redisClient *redis.Client, log *zap.Logger) {
	publisher := NewEventPublisher(redisClient, log)

	t.Run("Basic Event Publishing", func(t *testing.T) {
		err := publisher.PublishUsageEvent(context.Background(), "test-user", "test-key", "gpt-4", 100, 50, 0.002, 100*time.Millisecond)
		assert.NoError(t, err, "Should publish event")
	})

	t.Run("High Volume Event Publishing", func(t *testing.T) {
		const numEvents = 1000

		start := time.Now()
		var wg sync.WaitGroup

		for i := 0; i < numEvents; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				err := publisher.PublishUsageEvent(context.Background(), fmt.Sprintf("user-%d", idx), "test-key", "gpt-4", 100, 50, 0.002, 100*time.Millisecond)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		throughput := float64(numEvents) / duration.Seconds()
		assert.True(t, throughput > 100, "Should achieve >100 events/sec, got %.2f", throughput)

		t.Logf("Event Publishing Performance: %d events in %v (%.2f events/sec)", numEvents, duration, throughput)
	})
}

func testRedisConcurrency(t *testing.T, redisClient *redis.Client, log *zap.Logger) {
	t.Run("High Concurrency Operations", func(t *testing.T) {
		const numWorkers = 50
		const opsPerWorker = 20

		var wg sync.WaitGroup
		start := time.Now()

		// Multiple workers performing Redis operations
		for worker := 0; worker < numWorkers; worker++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for op := 0; op < opsPerWorker; op++ {
					key := fmt.Sprintf("worker_%d_op_%d", workerID, op)
					value := map[string]interface{}{
						"worker_id": workerID,
						"operation": op,
						"timestamp": time.Now().Unix(),
					}

					// Set operation
					data, _ := json.Marshal(value)
					err := redisClient.Set(context.Background(), key, data, time.Minute).Err()
					assert.NoError(t, err)

					// Get operation
					result, err := redisClient.Get(context.Background(), key).Result()
					assert.NoError(t, err)
					assert.NotEmpty(t, result)

					// Delete operation
					err = redisClient.Del(context.Background(), key).Err()
					assert.NoError(t, err)
				}
			}(worker)
		}

		wg.Wait()
		duration := time.Since(start)

		totalOps := numWorkers * opsPerWorker * 3 // 3 operations per iteration
		throughput := float64(totalOps) / duration.Seconds()

		// Banking-grade performance requirements
		assert.True(t, throughput > 1000, "Should achieve >1000 Redis ops/sec, got %.2f", throughput)
		assert.True(t, duration < 10*time.Second, "Should complete in <10s, took %v", duration)

		t.Logf("Redis Concurrency Performance: %d operations in %v (%.2f ops/sec)", totalOps, duration, throughput)
	})
}

func testRedisFailover(t *testing.T, redisClient *redis.Client, log *zap.Logger) {
	t.Run("Connection Recovery", func(t *testing.T) {
		// Test basic connectivity
		err := redisClient.Ping(context.Background()).Err()
		assert.NoError(t, err, "Redis should be connected")

		// Simulate brief network issue with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// This should timeout/fail (but timing is not guaranteed, so we just try it)
		_ = redisClient.Set(ctx, "test_timeout", "value", time.Minute).Err()

		// Normal operations should still work
		err = redisClient.Set(context.Background(), "test_recovery", "value", time.Minute).Err()
		assert.NoError(t, err, "Should recover for normal operations")
	})

	t.Run("Queue Resilience", func(t *testing.T) {
		queue := NewUsageQueue(&UsageQueueConfig{
			Client:     redisClient,
			Logger:     log,
			QueueName:  "test_resilient_queue",
			BatchSize:  5,
			MaxRetries: 3,
		})

		// Enqueue some items
		for i := 0; i < 10; i++ {
			record := &UsageRecord{
				ID:        fmt.Sprintf("record-%d", i),
				KeyID:     "test-key",
				Model:     "gpt-4",
				Timestamp: time.Now(),
			}
			err := queue.EnqueueUsage(context.Background(), record)
			assert.NoError(t, err)
		}

		// Verify queue has items
		count, err := redisClient.LLen(context.Background(), "test_resilient_queue").Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(10), count, "Queue should contain all items")
	})
}

// TestRedisLatencyRequirements tests Redis operations meet banking latency SLAs
func TestRedisLatencyRequirements(t *testing.T) {
	redisClient, cleanup := testutil.NewTestRedis(t)
	defer cleanup()

	// Clear test data
	redisClient.FlushDB(context.Background())

	const numOperations = 1000
	latencies := make([]time.Duration, numOperations)

	// Measure Redis operation latencies
	for i := 0; i < numOperations; i++ {
		start := time.Now()
		
		key := fmt.Sprintf("latency_test_%d", i)
		value := fmt.Sprintf("test_value_%d", i)
		
		// Redis operation
		err := redisClient.Set(context.Background(), key, value, time.Minute).Err()
		require.NoError(t, err)
		
		latencies[i] = time.Since(start)
	}

	// Sort latencies for percentile calculation
	for i := 0; i < len(latencies)-1; i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	// Calculate percentiles
	p95 := latencies[int(0.95*float64(len(latencies)))]
	p99 := latencies[int(0.99*float64(len(latencies)))]
	max := latencies[len(latencies)-1]

	// Banking-grade Redis latency requirements
	const (
		p95Target = 10 * time.Millisecond  // P95 under 10ms
		p99Target = 50 * time.Millisecond  // P99 under 50ms
		maxTarget = 100 * time.Millisecond // Max under 100ms
	)

	assert.True(t, p95 < p95Target, "Redis P95 latency %v should be < %v", p95, p95Target)
	assert.True(t, p99 < p99Target, "Redis P99 latency %v should be < %v", p99, p99Target)
	assert.True(t, max < maxTarget, "Redis max latency %v should be < %v", max, maxTarget)

	t.Logf("Redis Latency Results: P95=%v, P99=%v, Max=%v", p95, p99, max)
}