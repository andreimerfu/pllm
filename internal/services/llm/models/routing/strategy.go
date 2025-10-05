package routing

import (
	"context"
	"fmt"
	"sync/atomic"

	redisService "github.com/amerfu/pllm/internal/services/data/redis"
	"go.uber.org/zap"
)

// Strategy defines the interface for routing strategies
type Strategy interface {
	// Name returns the strategy name
	Name() string

	// SelectInstance selects the best instance from the given list
	SelectInstance(ctx context.Context, instances []ModelInstance) (ModelInstance, error)
}

// ModelRegistry interface to avoid import cycle
type ModelRegistry interface {
	GetRoundRobinCounter(modelName string) *atomic.Uint64
}

// StrategyDependencies contains dependencies needed by routing strategies
type StrategyDependencies struct {
	LatencyTracker *redisService.LatencyTracker
	Registry       ModelRegistry
	Logger         *zap.Logger
}

// NewStrategy creates a routing strategy based on the strategy name
func NewStrategy(name string, deps StrategyDependencies) (Strategy, error) {
	switch name {
	case "priority":
		return NewPriorityStrategy(deps.Logger), nil

	case "least-latency":
		if deps.LatencyTracker == nil {
			deps.Logger.Warn("LatencyTracker not available, falling back to priority strategy")
			return NewPriorityStrategy(deps.Logger), nil
		}
		return NewLatencyStrategy(deps.LatencyTracker, deps.Logger), nil

	case "weighted-round-robin":
		if deps.Registry == nil {
			deps.Logger.Warn("Registry not available, falling back to priority strategy")
			return NewPriorityStrategy(deps.Logger), nil
		}
		return NewRoundRobinStrategy(deps.Registry, deps.Logger), nil

	case "random":
		return NewRandomStrategy(deps.Logger), nil

	default:
		deps.Logger.Warn("Unknown routing strategy, using priority", zap.String("strategy", name))
		return NewPriorityStrategy(deps.Logger), nil
	}
}

// ValidateStrategy checks if a strategy name is valid
func ValidateStrategy(name string) error {
	validStrategies := []string{"priority", "least-latency", "weighted-round-robin", "random"}
	for _, valid := range validStrategies {
		if name == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid routing strategy: %s, valid options: %v", name, validStrategies)
}
