package models

import (
	"sync/atomic"

	"github.com/amerfu/pllm/internal/core/config"
)

// RouteModelProxy wraps a route model entry as a routing.ModelInstance so that
// existing routing strategies (priority, least-latency, weighted-round-robin, random)
// can be reused at the route level to select between models.
type RouteModelProxy struct {
	modelName string
	weight    float64
	priority  int
	latency   atomic.Int64 // populated from actual model instance metrics
}

// NewRouteModelProxy creates a new proxy for route-level model selection.
func NewRouteModelProxy(modelName string, weight float64, priority int) *RouteModelProxy {
	return &RouteModelProxy{
		modelName: modelName,
		weight:    weight,
		priority:  priority,
	}
}

// GetConfig returns a config.ModelInstance with the relevant fields set.
func (p *RouteModelProxy) GetConfig() config.ModelInstance {
	return config.ModelInstance{
		ModelName: p.modelName,
		Priority:  p.priority,
		Weight:    p.weight,
		Enabled:   true,
	}
}

// GetAverageLatency returns a pointer to the latency atomic value.
func (p *RouteModelProxy) GetAverageLatency() *atomic.Int64 {
	return &p.latency
}
