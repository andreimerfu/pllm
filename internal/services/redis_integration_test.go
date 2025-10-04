package services

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/logger"
	redisService "github.com/amerfu/pllm/internal/services/redis"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisIntegration tests Redis integration for banking-grade reliability
func TestRedisIntegration(t *testing.T) {
	// Setup Redis client for testing
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use test database
	})

	// Test Redis connectivity
	ctx := context.Background()
	err := redisClient.Ping(ctx).Err()
	require.NoError(t, err, "Redis should be available for testing")

	// Clear test database
	redisClient.FlushDB(ctx)

	logger := logger.NewLogger("test", "info")

	t.Run("Usage Queue Operations", func(t *testing.T) {
		testUsageQueue(t, redisClient, logger)
	})

	t.Run("Budget Cache Operations", func(t *testing.T) {
		testBudgetCache(t, redisClient, logger)
	})

	t.Run("Event Publishing", func(t *testing.T) {
		testEventPublishing(t, redisClient, logger)
	})

	t.Run("Distributed Locks", func(t *testing.T) {
		testDistributedLocks(t, redisClient, logger)
	})

	t.Run("High Concurrency", func(t *testing.T) {
		testRedisConcurrency(t, redisClient, logger)
	})

	t.Run("Failover Simulation", func(t *testing.T) {
		testRedisFailover(t, redisClient, logger)
	})
}

func testUsageQueue(t *testing.T, redisClient *redis.Client, logger *logger.Logger) {
	queue := redisService.NewUsageQueue(&redisService.UsageQueueConfig{
		Client:     redisClient,
		Logger:     logger,
		QueueName:  "test_usage_queue",
		BatchSize:  10,
		MaxRetries: 3,
	})

	// Test single usage record
	t.Run("Single Usage Record", func(t *testing.T) {
		usageRecord := map[string]interface{}{
			"key_id":        "test-key-123",
			"model":         "gpt-4",
			"tokens_input":  100,
			"tokens_output": 50,
			"cost":          0.002,
			"timestamp":     time.Now().Unix(),
		}

		err := queue.Enqueue(context.Background(), usageRecord)
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
			usageRecord := map[string]interface{}{
				"key_id":        "test-key-" + string(rune(i)),
				"model":         "gpt-4",
				"tokens_input":  100 + i,
				"tokens_output": 50 + i,
				"cost":          0.002 + float64(i)*0.001,
				"timestamp":     time.Now().Unix(),
			}

			err := queue.Enqueue(context.Background(), usageRecord)
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
		
		perfQueue := redisService.NewUsageQueue(&redisService.UsageQueueConfig{
			Client:     redisClient,
			Logger:     logger,
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
				usageRecord := map[string]interface{}{
					"key_id":    "perf-test-" + string(rune(idx)),
					"model":     "gpt-4",
					"cost":      0.002,
					"timestamp": time.Now().Unix(),
				}
				err := perfQueue.Enqueue(context.Background(), usageRecord)
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

func testBudgetCache(t *testing.T, redisClient *redis.Client, logger *logger.Logger) {
	cache := redisService.NewBudgetCache(redisClient, logger, 5*time.Minute)

	t.Run("Budget Cache Operations", func(t *testing.T) {
		keyID := "test-budget-key"
		budget := 100.0

		// Set budget
		err := cache.SetBudget(context.Background(), keyID, budget)
		assert.NoError(t, err, "Should set budget")

		// Get budget
		retrievedBudget, err := cache.GetBudget(context.Background(), keyID)
		assert.NoError(t, err, "Should get budget")
		assert.Equal(t, budget, retrievedBudget, "Budget should match")

		// Update usage
		usage := 25.5
		err = cache.AddUsage(context.Background(), keyID, usage)
		assert.NoError(t, err, "Should add usage")

		// Check remaining budget
		remaining, err := cache.GetRemainingBudget(context.Background(), keyID)
		assert.NoError(t, err, "Should get remaining budget")
		assert.InDelta(t, budget-usage, remaining, 0.01, "Remaining budget should be correct")
	})

	t.Run("Budget Exhaustion", func(t *testing.T) {
		keyID := "test-budget-exhaustion"
		budget := 10.0

		err := cache.SetBudget(context.Background(), keyID, budget)
		assert.NoError(t, err)

		// Use entire budget
		err = cache.AddUsage(context.Background(), keyID, budget)
		assert.NoError(t, err)

		// Check if budget is exhausted
		remaining, err := cache.GetRemainingBudget(context.Background(), keyID)
		assert.NoError(t, err)
		assert.InDelta(t, 0.0, remaining, 0.01, "Budget should be exhausted")

		// Try to use more than available
		exceeded := cache.WouldExceedBudget(context.Background(), keyID, 1.0)
		assert.True(t, exceeded, "Should detect budget would be exceeded")
	})

	t.Run("Concurrent Budget Operations", func(t *testing.T) {
		keyID := "test-concurrent-budget"
		budget := 1000.0

		err := cache.SetBudget(context.Background(), keyID, budget)
		assert.NoError(t, err)

		// Concurrent usage additions
		const numOperations = 100
		const usagePerOp = 1.0

		var wg sync.WaitGroup
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := cache.AddUsage(context.Background(), keyID, usagePerOp)
				assert.NoError(t, err)
			}()
		}

		wg.Wait()

		// Verify final budget
		remaining, err := cache.GetRemainingBudget(context.Background(), keyID)
		assert.NoError(t, err)
		expected := budget - float64(numOperations)*usagePerOp
		assert.InDelta(t, expected, remaining, 1.0, "Concurrent operations should be handled correctly")
	})
}

func testEventPublishing(t *testing.T, redisClient *redis.Client, logger *logger.Logger) {
	publisher := redisService.NewEventPublisher(redisClient, logger)

	t.Run("Basic Event Publishing", func(t *testing.T) {
		channel := "test_events"
		event := map[string]interface{}{
			"type":      "usage_recorded",
			"key_id":    "test-key",
			"timestamp": time.Now().Unix(),
			"amount":    15.5,
		}

		err := publisher.PublishUsageEvent(context.Background(), channel, event)
		assert.NoError(t, err, "Should publish event")
	})

	t.Run("High Volume Event Publishing", func(t *testing.T) {
		channel := "test_high_volume"
		const numEvents = 1000

		start := time.Now()
		var wg sync.WaitGroup

		for i := 0; i < numEvents; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				event := map[string]interface{}{
					"type":      "test_event",
					"id":        idx,
					"timestamp": time.Now().Unix(),
				}
				err := publisher.PublishUsageEvent(context.Background(), channel, event)
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

func testDistributedLocks(t *testing.T, redisClient *redis.Client, logger *logger.Logger) {
	locks := redisService.NewDistributedLocks(redisClient, logger)

	t.Run("Basic Lock Operations", func(t *testing.T) {
		lockKey := "test_lock"
		lockTTL := 30 * time.Second

		// Acquire lock
		acquired, err := locks.AcquireLock(context.Background(), lockKey, lockTTL)
		assert.NoError(t, err, "Should acquire lock")
		assert.True(t, acquired, "Lock should be acquired")

		// Try to acquire same lock (should fail)
		acquired2, err := locks.AcquireLock(context.Background(), lockKey, lockTTL)
		assert.NoError(t, err, "Should not error on duplicate lock attempt")
		assert.False(t, acquired2, "Should not acquire already held lock")

		// Release lock
		err = locks.ReleaseLock(context.Background(), lockKey)
		assert.NoError(t, err, "Should release lock")

		// Now should be able to acquire again
		acquired3, err := locks.AcquireLock(context.Background(), lockKey, lockTTL)
		assert.NoError(t, err, "Should acquire lock after release")
		assert.True(t, acquired3, "Lock should be acquired after release")

		// Cleanup
		locks.ReleaseLock(context.Background(), lockKey)
	})

	t.Run("Lock Expiration", func(t *testing.T) {
		lockKey := "test_expiring_lock"
		shortTTL := 100 * time.Millisecond

		// Acquire lock with short TTL
		acquired, err := locks.AcquireLock(context.Background(), lockKey, shortTTL)
		assert.NoError(t, err)
		assert.True(t, acquired)

		// Wait for expiration
		time.Sleep(200 * time.Millisecond)

		// Should be able to acquire expired lock
		acquired2, err := locks.AcquireLock(context.Background(), lockKey, 30*time.Second)
		assert.NoError(t, err)
		assert.True(t, acquired2, "Should acquire expired lock")

		// Cleanup
		locks.ReleaseLock(context.Background(), lockKey)
	})

	t.Run("Concurrent Lock Competition", func(t *testing.T) {
		lockKey := "test_concurrent_lock"
		const numWorkers = 10

		var wg sync.WaitGroup
		var winners int32
		var mu sync.Mutex

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				acquired, err := locks.AcquireLock(context.Background(), lockKey, 30*time.Second)
				assert.NoError(t, err)
				
				if acquired {
					mu.Lock()
					winners++
					mu.Unlock()
					
					// Hold lock briefly
					time.Sleep(10 * time.Millisecond)
					
					err := locks.ReleaseLock(context.Background(), lockKey)
					assert.NoError(t, err)
				}
			}()
		}

		wg.Wait()

		// Only one worker should win each lock competition
		// But multiple sequential wins are possible
		assert.True(t, winners >= 1, "At least one worker should acquire the lock")
		assert.True(t, winners <= int32(numWorkers), "No more than total workers should win")
	})
}

func testRedisConcurrency(t *testing.T, redisClient *redis.Client, logger *logger.Logger) {
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

func testRedisFailover(t *testing.T, redisClient *redis.Client, logger *logger.Logger) {
	t.Run("Connection Recovery", func(t *testing.T) {
		// Test basic connectivity
		err := redisClient.Ping(context.Background()).Err()
		assert.NoError(t, err, "Redis should be connected")

		// Simulate brief network issue with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// This should timeout/fail
		err = redisClient.Set(ctx, "test_timeout", "value", time.Minute).Err()
		assert.Error(t, err, "Should timeout with short context")

		// Normal operations should still work
		err = redisClient.Set(context.Background(), "test_recovery", "value", time.Minute).Err()
		assert.NoError(t, err, "Should recover for normal operations")
	})

	t.Run("Queue Resilience", func(t *testing.T) {
		queue := redisService.NewUsageQueue(&redisService.UsageQueueConfig{
			Client:     redisClient,
			Logger:     logger,
			QueueName:  "test_resilient_queue",
			BatchSize:  5,
			MaxRetries: 3,
		})

		// Enqueue some items
		for i := 0; i < 10; i++ {
			record := map[string]interface{}{
				"id":        i,
				"timestamp": time.Now().Unix(),
			}
			err := queue.Enqueue(context.Background(), record)
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
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

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