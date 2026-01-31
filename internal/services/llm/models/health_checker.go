package models

import (
	"context"
	"sync"
	"time"

	redisService "github.com/amerfu/pllm/internal/services/data/redis"
	"go.uber.org/zap"
)

// HealthChecker periodically runs provider health checks on all registered instances.
type HealthChecker struct {
	registry      *ModelRegistry
	healthTracker *HealthTracker
	healthStore   *redisService.HealthStore
	interval      time.Duration
	timeout       time.Duration
	logger        *zap.Logger
	stopCh        chan struct{}
}

// NewHealthChecker creates a HealthChecker.
// If healthStore is nil, results are only recorded in-memory via healthTracker.
func NewHealthChecker(
	registry *ModelRegistry,
	healthTracker *HealthTracker,
	healthStore *redisService.HealthStore,
	interval time.Duration,
	logger *zap.Logger,
) *HealthChecker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &HealthChecker{
		registry:      registry,
		healthTracker: healthTracker,
		healthStore:   healthStore,
		interval:      interval,
		timeout:       10 * time.Second,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start begins periodic health checking. It blocks until ctx is cancelled.
func (hc *HealthChecker) Start(ctx context.Context) {
	hc.logger.Info("Starting periodic health checker",
		zap.Duration("interval", hc.interval))

	// Run an initial check immediately.
	hc.runAllChecks(ctx)

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			hc.logger.Info("Health checker stopped (context cancelled)")
			return
		case <-hc.stopCh:
			hc.logger.Info("Health checker stopped")
			return
		case <-ticker.C:
			hc.runAllChecks(ctx)
		}
	}
}

// Stop signals the health checker to stop.
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// runAllChecks runs health checks on every registered instance concurrently.
func (hc *HealthChecker) runAllChecks(ctx context.Context) {
	instances := hc.registry.GetAllInstances()
	if len(instances) == 0 {
		return
	}

	hc.logger.Debug("Running health checks", zap.Int("instances", len(instances)))

	var wg sync.WaitGroup
	wg.Add(len(instances))

	for _, inst := range instances {
		go func(instance *ModelInstance) {
			defer wg.Done()
			hc.checkInstance(ctx, instance)
		}(inst)
	}

	wg.Wait()
	hc.logger.Debug("Health checks complete", zap.Int("instances", len(instances)))
}

// checkInstance performs a health check on a single instance.
func (hc *HealthChecker) checkInstance(ctx context.Context, instance *ModelInstance) {
	checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()

	start := time.Now()
	err := instance.Provider.HealthCheck(checkCtx)
	latency := time.Since(start)

	result := redisService.HealthCheckResult{
		InstanceID:   instance.Config.ID,
		ModelName:    instance.Config.ModelName,
		ProviderType: instance.Config.Provider.Type,
		Healthy:      err == nil,
		LatencyMs:    latency.Milliseconds(),
		CheckedAt:    time.Now(),
	}

	if err != nil {
		result.Error = err.Error()
		hc.healthTracker.RecordFailure(instance, err)
		hc.logger.Debug("Health check failed",
			zap.String("instance", instance.Config.ID),
			zap.String("model", instance.Config.ModelName),
			zap.Error(err))
	} else {
		hc.healthTracker.RecordSuccess(instance)
		hc.logger.Debug("Health check passed",
			zap.String("instance", instance.Config.ID),
			zap.String("model", instance.Config.ModelName),
			zap.Duration("latency", latency))
	}

	// Persist to Redis if store is available.
	if hc.healthStore != nil {
		storeCtx, storeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer storeCancel()
		if storeErr := hc.healthStore.StoreResult(storeCtx, result); storeErr != nil {
			hc.logger.Warn("Failed to persist health check result",
				zap.String("instance", instance.Config.ID),
				zap.Error(storeErr))
		}
	}
}
