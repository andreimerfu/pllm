package routing

import (
	"context"
	"time"

	redisService "github.com/amerfu/pllm/internal/services/data/redis"
	"go.uber.org/zap"
)

// LatencyStrategy selects instances based on lowest average latency
// Uses distributed Redis-based latency tracking for multi-instance deployments
type LatencyStrategy struct {
	latencyTracker *redisService.LatencyTracker
	logger         *zap.Logger
}

// NewLatencyStrategy creates a new latency-based routing strategy
func NewLatencyStrategy(tracker *redisService.LatencyTracker, logger *zap.Logger) *LatencyStrategy {
	return &LatencyStrategy{
		latencyTracker: tracker,
		logger:         logger,
	}
}

// Name returns the strategy name
func (s *LatencyStrategy) Name() string {
	return "least-latency"
}

// SelectInstance selects the instance with the lowest average latency
// Queries distributed latency from Redis, falls back to in-memory if unavailable
func (s *LatencyStrategy) SelectInstance(ctx context.Context, instances []ModelInstance) (ModelInstance, error) {
	if len(instances) == 0 {
		return nil, nil
	}

	// If distributed latency tracker available, use it for accurate cross-instance data
	if s.latencyTracker != nil {
		return s.selectUsingDistributedLatency(ctx, instances)
	}

	// Fallback to in-memory latency tracking
	return s.selectUsingInMemoryLatency(instances)
}

// selectUsingDistributedLatency queries Redis for distributed latency metrics
func (s *LatencyStrategy) selectUsingDistributedLatency(ctx context.Context, instances []ModelInstance) (ModelInstance, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	var bestInstance ModelInstance
	var bestLatency time.Duration

	for _, instance := range instances {
		config := instance.GetConfig()
		
		// Get distributed latency from Redis
		latency, err := s.latencyTracker.GetAverageLatency(queryCtx, config.ModelName)
		if err != nil {
			// Fallback to in-memory if Redis fails for this instance
			s.logger.Debug("Failed to get distributed latency, using in-memory",
				zap.String("model", config.ModelName),
				zap.Error(err))
			latency = time.Duration(instance.GetAverageLatency().Load()) * time.Millisecond
		}

		// Select instance with lowest latency
		if bestInstance == nil || (latency > 0 && (bestLatency == 0 || latency < bestLatency)) {
			bestInstance = instance
			bestLatency = latency
		}
	}

	if bestInstance != nil {
		config := bestInstance.GetConfig()
		s.logger.Debug("Selected instance by distributed latency",
			zap.String("instance_id", config.ID),
			zap.Duration("latency", bestLatency))
		return bestInstance, nil
	}

	// If Redis failed for all instances, fallback to in-memory
	s.logger.Warn("Distributed latency unavailable for all instances, falling back to in-memory")
	return s.selectUsingInMemoryLatency(instances)
}

// selectUsingInMemoryLatency uses local in-memory latency metrics
func (s *LatencyStrategy) selectUsingInMemoryLatency(instances []ModelInstance) (ModelInstance, error) {
	bestInstance := instances[0]
	bestLatency := bestInstance.GetAverageLatency().Load()

	for _, instance := range instances[1:] {
		latency := instance.GetAverageLatency().Load()
		if latency > 0 && (bestLatency == 0 || latency < bestLatency) {
			bestInstance = instance
			bestLatency = latency
		}
	}

	config := bestInstance.GetConfig()
	s.logger.Debug("Selected instance by in-memory latency",
		zap.String("instance_id", config.ID),
		zap.Int64("latency_ms", bestLatency))

	return bestInstance, nil
}
