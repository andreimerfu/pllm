package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// LatencyTracker provides distributed latency tracking across multiple instances
type LatencyTracker struct {
	client *redis.Client
	logger *zap.Logger
	
	// Configuration
	windowSize   time.Duration // Time window for latency samples (default: 5 minutes)
	maxSamples   int64         // Max samples per model (default: 1000)
	updatePeriod time.Duration // How often to update aggregates (default: 10s)
}

// NewLatencyTracker creates a new distributed latency tracker
func NewLatencyTracker(client *redis.Client, logger *zap.Logger) *LatencyTracker {
	return &LatencyTracker{
		client:       client,
		logger:       logger,
		windowSize:   5 * time.Minute,
		maxSamples:   1000,
		updatePeriod: 10 * time.Second,
	}
}

// RecordLatency records a latency sample for a model
func (lt *LatencyTracker) RecordLatency(ctx context.Context, modelName string, latency time.Duration) error {
	latencyMs := latency.Milliseconds()
	timestamp := float64(time.Now().UnixMilli())
	
	// Store in sorted set: score = timestamp, member = "latency_ms:timestamp" (unique)
	key := lt.latencyKey(modelName)
	
	// Make member unique by combining latency and timestamp
	member := fmt.Sprintf("%d:%d", latencyMs, time.Now().UnixNano())
	
	pipe := lt.client.Pipeline()
	
	// Add latency sample with timestamp as score
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  timestamp,
		Member: member,
	})
	
	// Trim old samples (keep last windowSize)
	cutoff := float64(time.Now().Add(-lt.windowSize).UnixMilli())
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%.0f", cutoff))
	
	// Limit total samples (keep most recent)
	pipe.ZRemRangeByRank(ctx, key, 0, -lt.maxSamples-1)
	
	// Set TTL to prevent memory leaks
	pipe.Expire(ctx, key, lt.windowSize*2)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		lt.logger.Error("Failed to record latency",
			zap.String("model", modelName),
			zap.Duration("latency", latency),
			zap.Error(err))
		return err
	}
	
	// Also update moving average asynchronously
	go lt.updateMovingAverage(context.Background(), modelName, latencyMs)
	
	return nil
}

// GetAverageLatency returns the average latency for a model
func (lt *LatencyTracker) GetAverageLatency(ctx context.Context, modelName string) (time.Duration, error) {
	key := lt.avgKey(modelName)
	
	result, err := lt.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil // No data yet
	}
	if err != nil {
		return 0, err
	}
	
	avgMs, err := strconv.ParseFloat(result, 64)
	if err != nil {
		return 0, err
	}
	
	return time.Duration(avgMs) * time.Millisecond, nil
}

// GetPercentileLatency returns the Pxx latency (e.g., P95, P99)
func (lt *LatencyTracker) GetPercentileLatency(ctx context.Context, modelName string, percentile float64) (time.Duration, error) {
	key := lt.latencyKey(modelName)
	
	// Get total count
	count, err := lt.client.ZCard(ctx, key).Result()
	if err != nil || count == 0 {
		return 0, err
	}
	
	// Calculate index for percentile
	index := int64(float64(count) * percentile / 100.0)
	if index >= count {
		index = count - 1
	}
	
	// Get value at percentile index
	values, err := lt.client.ZRange(ctx, key, index, index).Result()
	if err != nil || len(values) == 0 {
		return 0, err
	}
	
	latencyMs, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return 0, err
	}
	
	return time.Duration(latencyMs) * time.Millisecond, nil
}

// GetLatencyStats returns comprehensive latency statistics
func (lt *LatencyTracker) GetLatencyStats(ctx context.Context, modelName string) (*LatencyStats, error) {
	key := lt.latencyKey(modelName)
	
	// Get all samples
	values, err := lt.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	
	if len(values) == 0 {
		return &LatencyStats{
			ModelName:   modelName,
			SampleCount: 0,
		}, nil
	}
	
	// Parse values (format: "latency_ms:timestamp")
	latencies := make([]int64, 0, len(values))
	var sum int64
	var min int64 = 1<<63 - 1
	var max int64
	
	for _, v := range values {
		// Split "latency_ms:timestamp"
		parts := splitString(v, ":")
		if len(parts) < 1 {
			continue
		}
		
		latency, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		latencies = append(latencies, latency)
		sum += latency
		if latency < min {
			min = latency
		}
		if latency > max {
			max = latency
		}
	}
	
	if len(latencies) == 0 {
		return &LatencyStats{
			ModelName:   modelName,
			SampleCount: 0,
		}, nil
	}
	
	avg := sum / int64(len(latencies))
	
	// Calculate percentiles
	p50Index := int(float64(len(latencies)) * 0.50)
	p95Index := int(float64(len(latencies)) * 0.95)
	p99Index := int(float64(len(latencies)) * 0.99)
	
	if p50Index >= len(latencies) {
		p50Index = len(latencies) - 1
	}
	if p95Index >= len(latencies) {
		p95Index = len(latencies) - 1
	}
	if p99Index >= len(latencies) {
		p99Index = len(latencies) - 1
	}
	
	return &LatencyStats{
		ModelName:   modelName,
		SampleCount: int64(len(latencies)),
		Average:     time.Duration(avg) * time.Millisecond,
		Min:         time.Duration(min) * time.Millisecond,
		Max:         time.Duration(max) * time.Millisecond,
		P50:         time.Duration(latencies[p50Index]) * time.Millisecond,
		P95:         time.Duration(latencies[p95Index]) * time.Millisecond,
		P99:         time.Duration(latencies[p99Index]) * time.Millisecond,
	}, nil
}

// GetHealthScore calculates a health score based on latency (0-100)
func (lt *LatencyTracker) GetHealthScore(ctx context.Context, modelName string) (float64, error) {
	stats, err := lt.GetLatencyStats(ctx, modelName)
	if err != nil {
		return 100.0, err
	}
	
	if stats.SampleCount == 0 {
		return 100.0, nil // No data = healthy
	}
	
	// Score based on P95 latency
	// < 500ms = 100, 1s = 80, 2s = 60, 5s = 40, 10s+ = 20
	p95Ms := float64(stats.P95.Milliseconds())
	
	var score float64
	switch {
	case p95Ms < 500:
		score = 100.0
	case p95Ms < 1000:
		score = 100.0 - (p95Ms-500)*0.04 // Linear from 100 to 80
	case p95Ms < 2000:
		score = 80.0 - (p95Ms-1000)*0.02 // Linear from 80 to 60
	case p95Ms < 5000:
		score = 60.0 - (p95Ms-2000)*0.0067 // Linear from 60 to 40
	case p95Ms < 10000:
		score = 40.0 - (p95Ms-5000)*0.004 // Linear from 40 to 20
	default:
		score = 20.0
	}
	
	if score < 0 {
		score = 0
	}
	
	return score, nil
}

// ClearLatencies clears all latency data for a model (for testing/reset)
func (lt *LatencyTracker) ClearLatencies(ctx context.Context, modelName string) error {
	pipe := lt.client.Pipeline()
	pipe.Del(ctx, lt.latencyKey(modelName))
	pipe.Del(ctx, lt.avgKey(modelName))
	_, err := pipe.Exec(ctx)
	return err
}

// GetAllModelStats returns latency stats for all tracked models
func (lt *LatencyTracker) GetAllModelStats(ctx context.Context) (map[string]*LatencyStats, error) {
	// Scan for all latency keys
	pattern := "pllm:latency:*"
	keys, err := lt.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}
	
	stats := make(map[string]*LatencyStats)
	for _, key := range keys {
		// Extract model name from key
		modelName := key[len("pllm:latency:"):]
		
		modelStats, err := lt.GetLatencyStats(ctx, modelName)
		if err != nil {
			lt.logger.Warn("Failed to get stats for model",
				zap.String("model", modelName),
				zap.Error(err))
			continue
		}
		
		stats[modelName] = modelStats
	}
	
	return stats, nil
}

// updateMovingAverage updates the exponential moving average (async)
func (lt *LatencyTracker) updateMovingAverage(ctx context.Context, modelName string, latencyMs int64) {
	key := lt.avgKey(modelName)
	
	// Get current average
	currentAvgStr, err := lt.client.Get(ctx, key).Result()
	var newAvg float64
	
	if err == redis.Nil {
		// First sample
		newAvg = float64(latencyMs)
	} else if err != nil {
		lt.logger.Error("Failed to get current average", zap.Error(err))
		return
	} else {
		currentAvg, err := strconv.ParseFloat(currentAvgStr, 64)
		if err != nil {
			newAvg = float64(latencyMs)
		} else {
			// Exponential moving average: new = old * 0.9 + new * 0.1
			newAvg = currentAvg*0.9 + float64(latencyMs)*0.1
		}
	}
	
	// Update average
	err = lt.client.Set(ctx, key, newAvg, lt.windowSize*2).Err()
	if err != nil {
		lt.logger.Error("Failed to update moving average", zap.Error(err))
	}
}

// Helper methods for Redis keys
func (lt *LatencyTracker) latencyKey(modelName string) string {
	return fmt.Sprintf("pllm:latency:%s", modelName)
}

func (lt *LatencyTracker) avgKey(modelName string) string {
	return fmt.Sprintf("pllm:latency:avg:%s", modelName)
}

// LatencyStats represents comprehensive latency statistics
type LatencyStats struct {
	ModelName   string        `json:"model_name"`
	SampleCount int64         `json:"sample_count"`
	Average     time.Duration `json:"average"`
	Min         time.Duration `json:"min"`
	Max         time.Duration `json:"max"`
	P50         time.Duration `json:"p50"`
	P95         time.Duration `json:"p95"`
	P99         time.Duration `json:"p99"`
}

// splitString is a simple string split helper
func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	
	var result []string
	start := 0
	
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	
	result = append(result, s[start:])
	return result
}
