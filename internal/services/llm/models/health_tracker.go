package models

import (
	"time"

	"go.uber.org/zap"
)

// HealthTracker monitors the health status of model instances
type HealthTracker struct {
	logger *zap.Logger
}

// NewHealthTracker creates a new health tracker
func NewHealthTracker(logger *zap.Logger) *HealthTracker {
	return &HealthTracker{
		logger: logger,
	}
}

// RecordSuccess records a successful request for the instance
func (h *HealthTracker) RecordSuccess(instance *ModelInstance) {
	instance.Healthy.Store(true)
	instance.LastSuccess.Store(time.Now())
	instance.FailureCount.Store(0)

	h.logger.Debug("Recorded success for instance",
		zap.String("instance_id", instance.Config.ID))
}

// RecordFailure records a failed request for the instance
func (h *HealthTracker) RecordFailure(instance *ModelInstance, err error) {
	instance.LastError.Store(err)
	instance.LastFailure.Store(time.Now())
	failureCount := instance.FailureCount.Add(1)

	// Mark as unhealthy after 3 failures
	if failureCount >= 3 {
		instance.Healthy.Store(false)
		h.logger.Warn("Instance marked as unhealthy",
			zap.String("instance_id", instance.Config.ID),
			zap.Int32("failure_count", failureCount),
			zap.Error(err))
	} else {
		h.logger.Debug("Recorded failure for instance",
			zap.String("instance_id", instance.Config.ID),
			zap.Int32("failure_count", failureCount),
			zap.Error(err))
	}
}

// healthRecoveryCooldown is the time after which an unhealthy instance is allowed
// to receive traffic again (half-open circuit breaker). If the request succeeds,
// RecordSuccess will mark it fully healthy; if it fails, RecordFailure will reset
// the cooldown timer.
const healthRecoveryCooldown = 30 * time.Second

// IsHealthy checks if an instance is currently healthy.
// Implements a half-open circuit breaker: after healthRecoveryCooldown has elapsed
// since the last failure, the instance is allowed to receive traffic again.
func (h *HealthTracker) IsHealthy(instance *ModelInstance) bool {
	if instance.Healthy.Load() {
		return true
	}

	// Half-open: allow retry after cooldown
	if lastFailure, ok := instance.LastFailure.Load().(time.Time); ok {
		if time.Since(lastFailure) > healthRecoveryCooldown {
			h.logger.Info("Instance entering half-open state (recovery attempt)",
				zap.String("instance_id", instance.Config.ID))
			return true
		}
	}

	return false
}

// GetHealthStatus returns detailed health information for an instance
func (h *HealthTracker) GetHealthStatus(instance *ModelInstance) HealthStatus {
	var lastError error
	var lastSuccess time.Time

	if err, ok := instance.LastError.Load().(error); ok {
		lastError = err
	}
	if ts, ok := instance.LastSuccess.Load().(time.Time); ok {
		lastSuccess = ts
	}

	return HealthStatus{
		InstanceID:   instance.Config.ID,
		IsHealthy:    instance.Healthy.Load(),
		FailureCount: instance.FailureCount.Load(),
		LastError:    lastError,
		LastSuccess:  lastSuccess,
	}
}

// HealthStatus represents the health status of an instance
type HealthStatus struct {
	InstanceID   string    `json:"instance_id"`
	IsHealthy    bool      `json:"is_healthy"`
	FailureCount int32     `json:"failure_count"`
	LastError    error     `json:"last_error,omitempty"`
	LastSuccess  time.Time `json:"last_success,omitempty"`
}

// GetAllHealthStatuses returns health status for all instances
func (h *HealthTracker) GetAllHealthStatuses(instances []*ModelInstance) []HealthStatus {
	statuses := make([]HealthStatus, len(instances))
	for i, instance := range instances {
		statuses[i] = h.GetHealthStatus(instance)
	}
	return statuses
}
