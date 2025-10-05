package routing

import (
	"context"

	"go.uber.org/zap"
)

// RoundRobinStrategy distributes requests evenly across instances
// Currently implements simple round-robin (weights not yet implemented)
type RoundRobinStrategy struct {
	registry ModelRegistry
	logger   *zap.Logger
}

// NewRoundRobinStrategy creates a new round-robin routing strategy
func NewRoundRobinStrategy(registry ModelRegistry, logger *zap.Logger) *RoundRobinStrategy {
	return &RoundRobinStrategy{
		registry: registry,
		logger:   logger,
	}
}

// Name returns the strategy name
func (s *RoundRobinStrategy) Name() string {
	return "weighted-round-robin"
}

// SelectInstance selects the next instance using round-robin
// TODO: Implement weight support (currently ignores weights)
func (s *RoundRobinStrategy) SelectInstance(ctx context.Context, instances []ModelInstance) (ModelInstance, error) {
	if len(instances) == 0 {
		return nil, nil
	}

	// Get the model name from the first instance (all have same model name)
	modelName := instances[0].GetConfig().ModelName

	// Get round-robin counter for this model
	counter := s.registry.GetRoundRobinCounter(modelName)
	if counter == nil {
		// No counter available, return first instance
		s.logger.Debug("Round-robin counter not available, using first instance",
			zap.String("model", modelName))
		return instances[0], nil
	}

	// Simple round-robin: increment and modulo by instance count
	index := counter.Add(1) % uint64(len(instances))
	selected := instances[index]
	config := selected.GetConfig()

	s.logger.Debug("Selected instance by round-robin",
		zap.String("instance_id", config.ID),
		zap.Uint64("counter", counter.Load()),
		zap.Int("index", int(index)))

	return selected, nil
}
