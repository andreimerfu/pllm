package routing

import (
	"context"
	"math/rand"

	"go.uber.org/zap"
)

// RandomStrategy selects instances randomly
type RandomStrategy struct {
	logger *zap.Logger
}

// NewRandomStrategy creates a new random routing strategy
func NewRandomStrategy(logger *zap.Logger) *RandomStrategy {
	return &RandomStrategy{
		logger: logger,
	}
}

// Name returns the strategy name
func (s *RandomStrategy) Name() string {
	return "random"
}

// SelectInstance selects a random instance from the available instances
func (s *RandomStrategy) SelectInstance(ctx context.Context, instances []ModelInstance) (ModelInstance, error) {
	if len(instances) == 0 {
		return nil, nil
	}

	// Select random index
	index := rand.Intn(len(instances))
	selected := instances[index]
	config := selected.GetConfig()

	s.logger.Debug("Selected instance randomly",
		zap.String("instance_id", config.ID),
		zap.Int("index", index),
		zap.Int("total_instances", len(instances)))

	return selected, nil
}
