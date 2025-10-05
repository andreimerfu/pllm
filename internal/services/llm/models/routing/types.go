package routing

import (
	"sync/atomic"

	"github.com/amerfu/pllm/internal/core/config"
)

// ModelInstance interface to avoid import cycle with models package
// This defines the minimal interface needed by routing strategies
type ModelInstance interface {
	GetConfig() config.ModelInstance
	GetAverageLatency() *atomic.Int64
}
