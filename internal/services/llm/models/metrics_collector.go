package models

import (
	"time"

	"go.uber.org/zap"
)

// MetricsCollector handles performance metrics for model instances
type MetricsCollector struct {
	logger *zap.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger *zap.Logger) *MetricsCollector {
	return &MetricsCollector{
		logger: logger,
	}
}

// RecordRequest records a request and its performance metrics
func (m *MetricsCollector) RecordRequest(instance *ModelInstance, tokens int64, latency time.Duration) {
	// Update counters
	instance.TotalRequests.Add(1)
	instance.TotalTokens.Add(tokens)

	// Update average latency using exponential moving average
	latencyMs := latency.Milliseconds()
	currentAvg := instance.AverageLatency.Load()

	// Simple moving average with weight factor 0.1 for new values
	if currentAvg == 0 {
		instance.AverageLatency.Store(latencyMs)
	} else {
		newAvg := int64(float64(currentAvg)*0.9 + float64(latencyMs)*0.1)
		instance.AverageLatency.Store(newAvg)
	}

	// Update rate limiting counters
	m.updateRateLimitCounters(instance)

	m.logger.Debug("Recorded request metrics",
		zap.String("instance_id", instance.Config.ID),
		zap.Int64("tokens", tokens),
		zap.Duration("latency", latency))
}

// updateRateLimitCounters updates the per-minute request/token counters
func (m *MetricsCollector) updateRateLimitCounters(instance *ModelInstance) {
	now := time.Now()
	windowStart, _ := instance.WindowStart.Load().(time.Time)

	// Reset counters if we've passed the minute boundary
	if now.Sub(windowStart) >= time.Minute {
		instance.RequestsThisMinute.Store(1)
		instance.TokensThisMinute.Store(0) // Will be updated by caller
		instance.WindowStart.Store(now)
	} else {
		instance.RequestsThisMinute.Add(1)
	}
}

// GetMetrics returns current metrics for an instance
func (m *MetricsCollector) GetMetrics(instance *ModelInstance) InstanceMetrics {
	windowStart, _ := instance.WindowStart.Load().(time.Time)

	return InstanceMetrics{
		InstanceID:         instance.Config.ID,
		TotalRequests:      instance.TotalRequests.Load(),
		TotalTokens:        instance.TotalTokens.Load(),
		AverageLatency:     time.Duration(instance.AverageLatency.Load()) * time.Millisecond,
		RequestsThisMinute: instance.RequestsThisMinute.Load(),
		TokensThisMinute:   instance.TokensThisMinute.Load(),
		WindowStart:        windowStart,
	}
}

// InstanceMetrics represents performance metrics for an instance
type InstanceMetrics struct {
	InstanceID         string        `json:"instance_id"`
	TotalRequests      int64         `json:"total_requests"`
	TotalTokens        int64         `json:"total_tokens"`
	AverageLatency     time.Duration `json:"average_latency"`
	RequestsThisMinute int32         `json:"requests_this_minute"`
	TokensThisMinute   int32         `json:"tokens_this_minute"`
	WindowStart        time.Time     `json:"window_start"`
}

// GetAllMetrics returns metrics for all instances
func (m *MetricsCollector) GetAllMetrics(instances []*ModelInstance) []InstanceMetrics {
	metrics := make([]InstanceMetrics, len(instances))
	for i, instance := range instances {
		metrics[i] = m.GetMetrics(instance)
	}
	return metrics
}

// CheckRateLimit checks if an instance is within its rate limits
func (m *MetricsCollector) CheckRateLimit(instance *ModelInstance, additionalTokens int32) bool {
	// Check RPM (requests per minute)
	if instance.Config.RPM > 0 {
		currentRPM := instance.RequestsThisMinute.Load()
		if currentRPM >= int32(instance.Config.RPM) {
			return false
		}
	}

	// Check TPM (tokens per minute)
	if instance.Config.TPM > 0 {
		currentTPM := instance.TokensThisMinute.Load() + additionalTokens
		if currentTPM >= int32(instance.Config.TPM) {
			return false
		}
	}

	return true
}

// UpdateTokenCount updates the token count for rate limiting
func (m *MetricsCollector) UpdateTokenCount(instance *ModelInstance, tokens int32) {
	instance.TokensThisMinute.Add(tokens)
}
