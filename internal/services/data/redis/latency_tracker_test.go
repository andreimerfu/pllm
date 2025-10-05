package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

func TestLatencyTracker_RecordAndRetrieve(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger, _ := zap.NewDevelopment()
	tracker := NewLatencyTracker(client, logger)

	ctx := context.Background()
	modelName := "gpt-4"

	// Record multiple latencies
	latencies := []time.Duration{
		100 * time.Millisecond,
		150 * time.Millisecond,
		120 * time.Millisecond,
		200 * time.Millisecond,
		110 * time.Millisecond,
	}

	for _, lat := range latencies {
		err := tracker.RecordLatency(ctx, modelName, lat)
		require.NoError(t, err)
	}

	// Give time for async moving average update
	time.Sleep(50 * time.Millisecond)

	// Get average latency
	avg, err := tracker.GetAverageLatency(ctx, modelName)
	require.NoError(t, err)
	assert.Greater(t, avg, 0*time.Millisecond)
	assert.Less(t, avg, 300*time.Millisecond)

	// Get stats
	stats, err := tracker.GetLatencyStats(ctx, modelName)
	require.NoError(t, err)
	assert.Equal(t, int64(5), stats.SampleCount)
	assert.Equal(t, 100*time.Millisecond, stats.Min)
	assert.Equal(t, 200*time.Millisecond, stats.Max)
}

func TestLatencyTracker_Percentiles(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger, _ := zap.NewDevelopment()
	tracker := NewLatencyTracker(client, logger)

	ctx := context.Background()
	modelName := "gpt-4"

	// Record 100 latencies from 100ms to 199ms
	for i := 100; i < 200; i++ {
		err := tracker.RecordLatency(ctx, modelName, time.Duration(i)*time.Millisecond)
		require.NoError(t, err)
	}

	stats, err := tracker.GetLatencyStats(ctx, modelName)
	require.NoError(t, err)

	// P50 should be around 150ms
	assert.InDelta(t, 150, stats.P50.Milliseconds(), 10)

	// P95 should be around 195ms
	assert.InDelta(t, 195, stats.P95.Milliseconds(), 10)

	// P99 should be around 199ms
	assert.InDelta(t, 199, stats.P99.Milliseconds(), 10)
}

func TestLatencyTracker_HealthScore(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger, _ := zap.NewDevelopment()
	tracker := NewLatencyTracker(client, logger)

	ctx := context.Background()

	tests := []struct {
		name           string
		modelName      string
		latencies      []time.Duration
		expectedScore  float64
		scoreDelta     float64
	}{
		{
			name:          "Fast model (< 500ms)",
			modelName:     "fast-model",
			latencies:     []time.Duration{100 * time.Millisecond, 150 * time.Millisecond, 200 * time.Millisecond},
			expectedScore: 100.0,
			scoreDelta:    5.0,
		},
		{
			name:          "Medium model (~1s)",
			modelName:     "medium-model",
			latencies:     []time.Duration{800 * time.Millisecond, 900 * time.Millisecond, 1000 * time.Millisecond},
			expectedScore: 85.0,
			scoreDelta:    10.0,
		},
		{
			name:          "Slow model (2-5s)",
			modelName:     "slow-model",
			latencies:     []time.Duration{2000 * time.Millisecond, 3000 * time.Millisecond, 4000 * time.Millisecond},
			expectedScore: 50.0,
			scoreDelta:    15.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record latencies
			for _, lat := range tt.latencies {
				err := tracker.RecordLatency(ctx, tt.modelName, lat)
				require.NoError(t, err)
			}

			// Get health score
			score, err := tracker.GetHealthScore(ctx, tt.modelName)
			require.NoError(t, err)

			assert.InDelta(t, tt.expectedScore, score, tt.scoreDelta,
				"Health score for %s should be around %.0f", tt.modelName, tt.expectedScore)
		})
	}
}

func TestLatencyTracker_WindowExpiry(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger, _ := zap.NewDevelopment()
	tracker := NewLatencyTracker(client, logger)
	tracker.windowSize = 2 * time.Second // Short window for testing

	ctx := context.Background()
	modelName := "gpt-4"

	// Record initial latency
	err := tracker.RecordLatency(ctx, modelName, 100*time.Millisecond)
	require.NoError(t, err)

	// Fast-forward time in miniredis
	mr.FastForward(3 * time.Second)

	// Record new latency (should trigger cleanup of old samples)
	err = tracker.RecordLatency(ctx, modelName, 200*time.Millisecond)
	require.NoError(t, err)

	stats, err := tracker.GetLatencyStats(ctx, modelName)
	require.NoError(t, err)

	// Should only have recent sample (or both if window hasn't fully expired)
	assert.LessOrEqual(t, stats.SampleCount, int64(2), "Should have at most 2 samples")
	assert.GreaterOrEqual(t, stats.Average, 100*time.Millisecond, "Average should be at least 100ms")
	assert.LessOrEqual(t, stats.Average, 200*time.Millisecond, "Average should be at most 200ms")
}

func TestLatencyTracker_MultiInstance(t *testing.T) {
	// This simulates multiple PLLM instances sharing Redis
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger, _ := zap.NewDevelopment()

	// Create two "instances" (simulating two pods)
	tracker1 := NewLatencyTracker(client, logger)
	tracker2 := NewLatencyTracker(client, logger)

	ctx := context.Background()
	modelName := "gpt-4"

	// Instance 1 records 10s latency
	err := tracker1.RecordLatency(ctx, modelName, 10*time.Second)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond) // Wait for async update

	// Instance 2 should see the latency from Instance 1
	avg, err := tracker2.GetAverageLatency(ctx, modelName)
	require.NoError(t, err)
	assert.Greater(t, avg, 9*time.Second, "Instance 2 should see latency recorded by Instance 1")
	assert.Less(t, avg, 11*time.Second)

	// Instance 2 records 2s latency
	err = tracker2.RecordLatency(ctx, modelName, 2*time.Second)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Both instances should see updated average
	avg1, err := tracker1.GetAverageLatency(ctx, modelName)
	require.NoError(t, err)
	avg2, err := tracker2.GetAverageLatency(ctx, modelName)
	require.NoError(t, err)

	// Both should see similar average (EMA weighted)
	assert.InDelta(t, avg1.Milliseconds(), avg2.Milliseconds(), 500,
		"Both instances should see similar average latency")

	// Average should be between 2s and 10s
	assert.Greater(t, avg1, 2*time.Second)
	assert.Less(t, avg1, 10*time.Second)
}

func TestLatencyTracker_MaxSamples(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger, _ := zap.NewDevelopment()
	tracker := NewLatencyTracker(client, logger)
	tracker.maxSamples = 10 // Small limit for testing

	ctx := context.Background()
	modelName := "gpt-4"

	// Record 20 samples (should keep only last 10)
	for i := 0; i < 20; i++ {
		err := tracker.RecordLatency(ctx, modelName, time.Duration(i+100)*time.Millisecond)
		require.NoError(t, err)
	}

	stats, err := tracker.GetLatencyStats(ctx, modelName)
	require.NoError(t, err)

	// Should only have maxSamples
	assert.LessOrEqual(t, stats.SampleCount, int64(10))

	// Min should be from newer samples (not 100ms)
	assert.GreaterOrEqual(t, stats.Min, 110*time.Millisecond)
}

func TestLatencyTracker_ClearLatencies(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger, _ := zap.NewDevelopment()
	tracker := NewLatencyTracker(client, logger)

	ctx := context.Background()
	modelName := "gpt-4"

	// Record latencies
	err := tracker.RecordLatency(ctx, modelName, 100*time.Millisecond)
	require.NoError(t, err)

	// Clear
	err = tracker.ClearLatencies(ctx, modelName)
	require.NoError(t, err)

	// Should have no data
	stats, err := tracker.GetLatencyStats(ctx, modelName)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.SampleCount)
}

func TestLatencyTracker_GetAllModelStats(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger, _ := zap.NewDevelopment()
	tracker := NewLatencyTracker(client, logger)

	ctx := context.Background()

	// Record latencies for multiple models
	models := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-sonnet"}
	for _, model := range models {
		err := tracker.RecordLatency(ctx, model, 100*time.Millisecond)
		require.NoError(t, err)
	}

	// Get all stats
	allStats, err := tracker.GetAllModelStats(ctx)
	require.NoError(t, err)

	assert.Len(t, allStats, 3)
	for _, model := range models {
		stats, exists := allStats[model]
		assert.True(t, exists, "Should have stats for %s", model)
		assert.Greater(t, stats.SampleCount, int64(0))
	}
}

func BenchmarkLatencyTracker_RecordLatency(b *testing.B) {
	client, mr := setupTestRedis(&testing.T{})
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	tracker := NewLatencyTracker(client, logger)

	ctx := context.Background()
	modelName := "gpt-4"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracker.RecordLatency(ctx, modelName, 100*time.Millisecond)
	}
}

func BenchmarkLatencyTracker_GetAverageLatency(b *testing.B) {
	client, mr := setupTestRedis(&testing.T{})
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	tracker := NewLatencyTracker(client, logger)

	ctx := context.Background()
	modelName := "gpt-4"

	// Seed some data
	for i := 0; i < 100; i++ {
		_ = tracker.RecordLatency(ctx, modelName, 100*time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tracker.GetAverageLatency(ctx, modelName)
	}
}
