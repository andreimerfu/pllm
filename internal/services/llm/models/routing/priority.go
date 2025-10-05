package routing

import (
	"context"

	"go.uber.org/zap"
)

// PriorityStrategy selects instances based on priority
// Instances are pre-sorted by priority in the registry (lower number = higher priority)
type PriorityStrategy struct {
	logger *zap.Logger
}

// NewPriorityStrategy creates a new priority-based routing strategy
func NewPriorityStrategy(logger *zap.Logger) *PriorityStrategy {
	return &PriorityStrategy{
		logger: logger,
	}
}

// Name returns the strategy name
func (s *PriorityStrategy) Name() string {
	return "priority"
}

// SelectInstance returns the first instance (highest priority)
// Instances are already sorted by priority in the registry
func (s *PriorityStrategy) SelectInstance(ctx context.Context, instances []ModelInstance) (ModelInstance, error) {
	if len(instances) == 0 {
		return nil, nil
	}

	selected := instances[0]
	config := selected.GetConfig()
	s.logger.Debug("Selected instance by priority",
		zap.String("instance_id", config.ID),
		zap.Int("priority", config.Priority))

	return selected, nil
}
