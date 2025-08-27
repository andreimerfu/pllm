package models

import (
	"context"
	"fmt"
	"time"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/circuitbreaker"
	"github.com/amerfu/pllm/internal/services/loadbalancer"
	"go.uber.org/zap"
)

// ModelManager is the refactored model manager using focused components
type ModelManager struct {
	registry         *ModelRegistry
	healthTracker    *HealthTracker
	metricsCollector *MetricsCollector
	loadBalancer     *loadbalancer.AdaptiveLoadBalancer
	circuitBreaker   *circuitbreaker.Manager
	adaptiveBreakers map[string]*circuitbreaker.AdaptiveBreaker
	router           config.RouterSettings
	logger           *zap.Logger
}

// NewModelManager creates a new refactored model manager
func NewModelManager(logger *zap.Logger, router config.RouterSettings) *ModelManager {
	// Initialize circuit breaker if enabled
	var cb *circuitbreaker.Manager
	if router.CircuitBreakerEnabled {
		threshold := router.CircuitBreakerThreshold
		if threshold <= 0 {
			threshold = 5
		}
		cooldown := router.CircuitBreakerCooldown
		if cooldown <= 0 {
			cooldown = 30 * time.Second
		}
		cb = circuitbreaker.NewManager(threshold, cooldown)
	}

	return &ModelManager{
		registry:         NewModelRegistry(logger),
		healthTracker:    NewHealthTracker(logger),
		metricsCollector: NewMetricsCollector(logger),
		loadBalancer:     loadbalancer.NewAdaptiveLoadBalancer(),
		circuitBreaker:   cb,
		adaptiveBreakers: make(map[string]*circuitbreaker.AdaptiveBreaker),
		router:           router,
		logger:           logger,
	}
}

// LoadModelInstances loads model instances from configuration
func (m *ModelManager) LoadModelInstances(instances []config.ModelInstance) error {
	return m.registry.LoadModelInstances(instances)
}

// GetBestInstance returns the best instance for a model based on routing strategy
func (m *ModelManager) GetBestInstance(ctx context.Context, modelName string) (*ModelInstance, error) {
	// Check circuit breaker first if enabled
	if m.circuitBreaker != nil && m.circuitBreaker.IsOpen(modelName) {
		m.logger.Debug("Circuit breaker is open for model", zap.String("model", modelName))
		return nil, fmt.Errorf("circuit breaker is open for model: %s", modelName)
	}

	// Get available instances for the model
	instances, exists := m.registry.GetModelInstances(modelName)
	if !exists || len(instances) == 0 {
		return nil, fmt.Errorf("no instances available for model: %s", modelName)
	}

	// Filter healthy instances
	var healthyInstances []*ModelInstance
	for _, instance := range instances {
		if m.healthTracker.IsHealthy(instance) {
			healthyInstances = append(healthyInstances, instance)
		}
	}

	if len(healthyInstances) == 0 {
		return nil, fmt.Errorf("no healthy instances available for model: %s", modelName)
	}

	// Select instance based on routing strategy
	return m.selectInstanceByStrategy(ctx, modelName, healthyInstances)
}

// selectInstanceByStrategy selects an instance based on the routing strategy
func (m *ModelManager) selectInstanceByStrategy(ctx context.Context, modelName string, instances []*ModelInstance) (*ModelInstance, error) {
	switch m.router.RoutingStrategy {
	case "weighted-round-robin":
		return m.selectWeightedRoundRobin(modelName, instances), nil
	case "least-latency":
		return m.selectLeastLatency(instances), nil
	case "random":
		return m.selectRandom(instances), nil
	default: // priority-based (default)
		return instances[0], nil // Already sorted by priority in registry
	}
}

// selectWeightedRoundRobin selects instance using weighted round-robin
func (m *ModelManager) selectWeightedRoundRobin(modelName string, instances []*ModelInstance) *ModelInstance {
	counter := m.registry.GetRoundRobinCounter(modelName)
	if counter == nil {
		return instances[0]
	}

	// Simple round-robin for now (can be enhanced with weights)
	index := counter.Add(1) % uint64(len(instances))
	return instances[index]
}

// selectLeastLatency selects instance with lowest average latency
func (m *ModelManager) selectLeastLatency(instances []*ModelInstance) *ModelInstance {
	bestInstance := instances[0]
	bestLatency := bestInstance.AverageLatency.Load()

	for _, instance := range instances[1:] {
		latency := instance.AverageLatency.Load()
		if latency > 0 && (bestLatency == 0 || latency < bestLatency) {
			bestInstance = instance
			bestLatency = latency
		}
	}

	return bestInstance
}

// selectRandom selects a random instance
func (m *ModelManager) selectRandom(instances []*ModelInstance) *ModelInstance {
	return instances[len(instances)%len(instances)] // Simplified random selection
}

// RecordSuccess records a successful request
func (m *ModelManager) RecordSuccess(instance *ModelInstance, tokens int64, latency time.Duration) {
	m.healthTracker.RecordSuccess(instance)
	m.metricsCollector.RecordRequest(instance, tokens, latency)

	// Record success for circuit breaker
	if m.circuitBreaker != nil {
		m.circuitBreaker.RecordSuccess(instance.Config.ModelName)
	}
}

// RecordFailure records a failed request
func (m *ModelManager) RecordFailure(instance *ModelInstance, err error) {
	m.healthTracker.RecordFailure(instance, err)

	// Record failure for circuit breaker
	if m.circuitBreaker != nil {
		m.circuitBreaker.RecordFailure(instance.Config.ModelName)
	}
}

// GetModelStats returns statistics for all models
func (m *ModelManager) GetModelStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Registry stats
	registryStats := m.registry.GetRegistryStats()
	stats["registry"] = registryStats

	// Health status for all instances
	allInstances := m.registry.GetAllInstances()
	healthStatuses := m.healthTracker.GetAllHealthStatuses(allInstances)
	stats["health"] = healthStatuses

	// Metrics for all instances
	metrics := m.metricsCollector.GetAllMetrics(allInstances)
	stats["metrics"] = metrics

	// Legacy compatibility: Create load_balancer format expected by dashboard
	loadBalancerStats := make(map[string]interface{})
	for _, instance := range allInstances {
		modelName := instance.Config.ModelName

		// Get health status
		healthStatus := m.healthTracker.GetHealthStatus(instance)
		healthScore := 100
		if !healthStatus.IsHealthy {
			healthScore = 50 // Simplified health scoring
		}

		// Get metrics
		instanceMetrics := m.metricsCollector.GetMetrics(instance)

		loadBalancerStats[modelName] = map[string]interface{}{
			"health_score":    healthScore,
			"total_requests":  instanceMetrics.TotalRequests,
			"avg_latency":     fmt.Sprintf("%.0f", float64(instanceMetrics.AverageLatency.Milliseconds())),
			"requests_minute": instanceMetrics.RequestsThisMinute,
			"tokens_minute":   instanceMetrics.TokensThisMinute,
		}
	}
	stats["load_balancer"] = loadBalancerStats

	// Legacy compatibility: Add summary fields expected by admin analytics
	var totalRequests int64
	var totalTokens int64
	activeModels := len(loadBalancerStats)

	for _, instance := range allInstances {
		instanceMetrics := m.metricsCollector.GetMetrics(instance)
		totalRequests += instanceMetrics.TotalRequests
		totalTokens += instanceMetrics.TotalTokens
	}

	stats["total_requests"] = totalRequests
	stats["total_tokens"] = totalTokens
	stats["total_cost"] = float64(totalTokens) * 0.0001 // Rough cost estimate
	stats["active_users"] = 0                           // TODO: Track active users
	stats["should_shed_load"] = false                   // TODO: Implement load shedding logic
	stats["active_models"] = activeModels

	return stats
}

// GetAvailableModels returns list of available models
func (m *ModelManager) GetAvailableModels() []string {
	return m.registry.GetAvailableModels()
}

// CheckRateLimit checks if an instance can handle additional tokens
func (m *ModelManager) CheckRateLimit(instance *ModelInstance, additionalTokens int32) bool {
	return m.metricsCollector.CheckRateLimit(instance, additionalTokens)
}

// UpdateTokenCount updates the token count for rate limiting
func (m *ModelManager) UpdateTokenCount(instance *ModelInstance, tokens int32) {
	m.metricsCollector.UpdateTokenCount(instance, tokens)
}

// Legacy methods for backward compatibility with handlers
// TODO: Update handlers to use new API and remove these methods

// RecordRequestStart records the start of a request (no-op for now)
func (m *ModelManager) RecordRequestStart(modelName string) {
	// No-op - tracking is now done at success/failure level
}

// RecordRequestEnd records the end of a request (no-op for now)
func (m *ModelManager) RecordRequestEnd(modelName string, latency time.Duration, success bool, err error) {
	// No-op - use RecordSuccess/RecordFailure instead
}

// GetBestInstanceAdaptive returns the best instance (alias for GetBestInstance)
func (m *ModelManager) GetBestInstanceAdaptive(ctx context.Context, modelName string) (*ModelInstance, error) {
	return m.GetBestInstance(ctx, modelName)
}

// ListModels returns available models (alias for GetAvailableModels)
func (m *ModelManager) ListModels() []string {
	return m.GetAvailableModels()
}

// GetDetailedModelInfo returns detailed model information for API consumption
func (m *ModelManager) GetDetailedModelInfo() []ModelInfo {
	availableModels := m.GetAvailableModels()
	allInstances := m.registry.GetAllInstances()

	// Create a map to track model info
	modelInfoMap := make(map[string]*ModelInfo)

	// Build model info from instances
	for _, instance := range allInstances {
		modelName := instance.Config.ModelName
		if _, exists := modelInfoMap[modelName]; !exists {
			// Determine provider/owner from provider type
			var ownedBy string
			switch instance.Config.Provider.Type {
			case "openai":
				ownedBy = "openai"
			case "anthropic":
				ownedBy = "anthropic"
			case "azure":
				ownedBy = "azure"
			case "bedrock":
				ownedBy = "aws"
			case "vertex":
				ownedBy = "google"
			case "openrouter":
				ownedBy = "openrouter"
			default:
				ownedBy = instance.Config.Provider.Type
			}

			modelInfoMap[modelName] = &ModelInfo{
				ID:      modelName,
				Object:  "model",
				OwnedBy: ownedBy,
				Created: time.Now().Unix(),
			}
		}
	}

	// Convert to slice
	var result []ModelInfo
	for _, modelName := range availableModels {
		if info, exists := modelInfoMap[modelName]; exists {
			result = append(result, *info)
		} else {
			// Fallback for models without instances
			result = append(result, ModelInfo{
				ID:      modelName,
				Object:  "model",
				OwnedBy: "unknown",
				Created: time.Now().Unix(),
			})
		}
	}

	return result
}

// ModelInfo represents detailed model information for API responses
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
	Created int64  `json:"created"`
}
