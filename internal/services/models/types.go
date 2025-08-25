package models

import (
	"sync/atomic"
	"time"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/providers"
)

// ModelInstance represents a runtime model instance with both configuration and state
type ModelInstance struct {
	// Configuration from config package
	Config config.ModelInstance

	// Provider instance
	Provider providers.Provider

	// Health tracking
	Healthy        atomic.Bool
	FailureCount   atomic.Int32
	LastFailure    atomic.Value // time.Time
	LastSuccess    atomic.Value // time.Time
	LastError      atomic.Value // error
	ConsecutiveOK  atomic.Int32
	LastHealthy    atomic.Value // time.Time

	// Performance metrics
	TotalRequests      atomic.Int64
	TotalTokens        atomic.Int64
	AverageLatency     atomic.Int64 // in milliseconds
	RequestsThisMinute atomic.Int32
	TokensThisMinute   atomic.Int32
	WindowStart        atomic.Value // time.Time

	// Circuit breaker state
	CircuitState     atomic.Int32 // 0=closed, 1=half-open, 2=open
	LastCircuitCheck atomic.Value // time.Time
}

// NewModelInstance creates a new runtime model instance from configuration
func NewModelInstance(cfg config.ModelInstance, provider providers.Provider) *ModelInstance {
	instance := &ModelInstance{
		Config:   cfg,
		Provider: provider,
	}
	
	// Initialize atomic values
	instance.Healthy.Store(true)
	instance.WindowStart.Store(time.Now())
	instance.LastHealthy.Store(time.Now())
	
	return instance
}

// Legacy methods for backward compatibility with handlers
// TODO: Update handlers to use manager methods instead

// RecordError records an error (deprecated - use manager.RecordFailure)
func (m *ModelInstance) RecordError(err error) {
	m.FailureCount.Add(1)
	m.LastError.Store(err)
}

// RecordRequest records a successful request (deprecated - use manager.RecordSuccess)
func (m *ModelInstance) RecordRequest(tokens int32, latencyMs int64) {
	m.TotalRequests.Add(1)
	m.TotalTokens.Add(int64(tokens))
	m.LastSuccess.Store(time.Now())
	
	// Update latency using exponential moving average
	currentAvg := m.AverageLatency.Load()
	if currentAvg == 0 {
		m.AverageLatency.Store(latencyMs)
	} else {
		newAvg := int64(float64(currentAvg)*0.9 + float64(latencyMs)*0.1)
		m.AverageLatency.Store(newAvg)
	}
}