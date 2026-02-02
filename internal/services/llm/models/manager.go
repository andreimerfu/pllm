package models

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amerfu/pllm/internal/core/config"
	redisService "github.com/amerfu/pllm/internal/services/data/redis"
	"github.com/amerfu/pllm/internal/services/llm/models/routing"
	"github.com/amerfu/pllm/internal/services/llm/providers"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RouteEntry represents a registered route in the model manager.
type RouteEntry struct {
	Slug           string
	Strategy       routing.Strategy
	Models         []RouteModelEntry
	FallbackModels []string
	rrCounter      atomic.Uint64 // route-level round-robin counter
}

// RouteModelEntry represents a model within a route.
type RouteModelEntry struct {
	ModelName string
	Weight    int
	Priority  int
	Enabled   bool
}

// ModelManager is the refactored model manager using focused components
type ModelManager struct {
	registry         *ModelRegistry
	healthTracker    *HealthTracker
	metricsCollector *MetricsCollector
	latencyTracker   *redisService.LatencyTracker // Distributed latency tracking
	healthStore      *redisService.HealthStore    // Distributed health check results
	routingStrategy  routing.Strategy              // Routing strategy (priority, latency, etc.)
	router           config.RouterSettings
	logger           *zap.Logger

	// Route registry
	routes  map[string]*RouteEntry // key: slug
	routeMu sync.RWMutex
}

// NewModelManager creates a new refactored model manager
func NewModelManager(logger *zap.Logger, router config.RouterSettings, redisClient *redis.Client) *ModelManager {
	// Initialize distributed latency tracker and health store
	var latencyTracker *redisService.LatencyTracker
	var healthStore *redisService.HealthStore
	if redisClient != nil {
		latencyTracker = redisService.NewLatencyTracker(redisClient, logger)
		healthStore = redisService.NewHealthStore(redisClient, logger)
	}

	// Initialize model registry
	registry := NewModelRegistry(logger)

	// Create routing strategy
	strategy, err := routing.NewStrategy(router.RoutingStrategy, routing.StrategyDependencies{
		LatencyTracker: latencyTracker,
		Registry:       registry,
		Logger:         logger,
	})
	if err != nil {
		logger.Warn("Failed to create routing strategy, using priority", zap.Error(err))
		strategy, _ = routing.NewStrategy("priority", routing.StrategyDependencies{Logger: logger})
	}

	return &ModelManager{
		registry:         registry,
		healthTracker:    NewHealthTracker(logger),
		metricsCollector: NewMetricsCollector(logger),
		latencyTracker:   latencyTracker,
		healthStore:      healthStore,
		routingStrategy:  strategy,
		router:           router,
		logger:           logger,
		routes:           make(map[string]*RouteEntry),
	}
}

// LoadModelInstances loads model instances from configuration
func (m *ModelManager) LoadModelInstances(instances []config.ModelInstance) error {
	return m.registry.LoadModelInstances(instances)
}

// GetBestInstance returns the best instance for a model based on routing strategy
func (m *ModelManager) GetBestInstance(ctx context.Context, modelName string) (*ModelInstance, error) {
	// Get available instances for the model
	instances, exists := m.registry.GetModelInstances(modelName)
	if !exists || len(instances) == 0 {
		return nil, fmt.Errorf("no instances available for model: %s", modelName)
	}

	// Filter healthy instances
	var healthyInstances []routing.ModelInstance
	for _, instance := range instances {
		if m.healthTracker.IsHealthy(instance) {
			healthyInstances = append(healthyInstances, instance)
		}
	}

	if len(healthyInstances) == 0 {
		return nil, fmt.Errorf("no healthy instances available for model: %s", modelName)
	}

	// Delegate to routing strategy
	selected, err := m.routingStrategy.SelectInstance(ctx, healthyInstances)
	if err != nil {
		return nil, err
	}
	
	// Convert back to concrete type
	return selected.(*ModelInstance), nil
}

// FailoverRequest contains the request details for failover execution
type FailoverRequest struct {
	ModelName      string
	ExecuteFunc    func(context.Context, *ModelInstance) (interface{}, error) // Function to execute against an instance
	ValidateFunc   func(interface{}) error                                    // Optional validation of response
	IsStreamFunc   bool                                                       // If true, ExecuteFunc returns a channel
}

// FailoverResult contains the result of a failover execution
type FailoverResult struct {
	Response     interface{}
	Instance     *ModelInstance
	AttemptCount int
	Failovers    []string // List of models/instances tried before success
}

// LoadRoutes loads routes from configuration at startup.
func (m *ModelManager) LoadRoutes(configs []config.RouteConfig) {
	for _, rc := range configs {
		enabled := true
		if rc.Enabled != nil {
			enabled = *rc.Enabled
		}
		if !enabled {
			continue
		}

		strategy := rc.Strategy
		if strategy == "" {
			strategy = "priority"
		}

		routeStrategy, err := routing.NewStrategy(strategy, routing.StrategyDependencies{
			LatencyTracker: m.latencyTracker,
			Registry:       m.registry,
			Logger:         m.logger,
		})
		if err != nil {
			m.logger.Warn("Failed to create route strategy, using priority",
				zap.String("route", rc.Slug), zap.Error(err))
			routeStrategy, _ = routing.NewStrategy("priority", routing.StrategyDependencies{Logger: m.logger})
		}

		var models []RouteModelEntry
		for _, rm := range rc.Models {
			models = append(models, RouteModelEntry{
				ModelName: rm.ModelName,
				Weight:    rm.Weight,
				Priority:  rm.Priority,
				Enabled:   true,
			})
		}

		entry := &RouteEntry{
			Slug:           rc.Slug,
			Strategy:       routeStrategy,
			Models:         models,
			FallbackModels: rc.FallbackModels,
		}

		m.routeMu.Lock()
		m.routes[rc.Slug] = entry
		m.routeMu.Unlock()

		m.logger.Info("Loaded route from config",
			zap.String("slug", rc.Slug),
			zap.String("strategy", strategy),
			zap.Int("models", len(models)))
	}
}

// RegisterRoute adds or replaces a route in the runtime registry.
// strategyName specifies which routing strategy to use (e.g. "priority", "weighted-round-robin").
func (m *ModelManager) RegisterRoute(entry *RouteEntry, strategyName string) {
	if strategyName == "" {
		strategyName = "priority"
	}

	routeStrategy, err := routing.NewStrategy(strategyName, routing.StrategyDependencies{
		LatencyTracker: m.latencyTracker,
		Registry:       m.registry,
		Logger:         m.logger,
	})
	if err != nil {
		m.logger.Warn("Failed to create route strategy, using priority",
			zap.String("route", entry.Slug), zap.Error(err))
		routeStrategy, _ = routing.NewStrategy("priority", routing.StrategyDependencies{Logger: m.logger})
	}
	entry.Strategy = routeStrategy

	m.routeMu.Lock()
	m.routes[entry.Slug] = entry
	m.routeMu.Unlock()

	m.logger.Info("Registered route",
		zap.String("slug", entry.Slug),
		zap.String("strategy", strategyName),
		zap.Int("models", len(entry.Models)))
}

// UnregisterRoute removes a route from the runtime registry.
func (m *ModelManager) UnregisterRoute(slug string) {
	m.routeMu.Lock()
	delete(m.routes, slug)
	m.routeMu.Unlock()

	m.logger.Info("Unregistered route", zap.String("slug", slug))
}

// ResolveRoute checks if a model name is a route slug and returns the route entry.
func (m *ModelManager) ResolveRoute(modelName string) (*RouteEntry, bool) {
	m.routeMu.RLock()
	defer m.routeMu.RUnlock()

	entry, exists := m.routes[modelName]
	return entry, exists
}

// GetRoutes returns all registered routes.
func (m *ModelManager) GetRoutes() map[string]*RouteEntry {
	m.routeMu.RLock()
	defer m.routeMu.RUnlock()

	result := make(map[string]*RouteEntry, len(m.routes))
	for k, v := range m.routes {
		result[k] = v
	}
	return result
}

// selectRouteModel picks one model from the route's proxy list.
// For weighted-round-robin it uses the route's own counter (not the model-level one).
// For other strategies it delegates to the strategy interface.
func (m *ModelManager) selectRouteModel(ctx context.Context, route *RouteEntry, proxies []routing.ModelInstance) (string, error) {
	if len(proxies) == 0 {
		return "", fmt.Errorf("no models available in route %q", route.Slug)
	}

	if route.Strategy.Name() == "weighted-round-robin" {
		// Smooth weighted round-robin: interleave models proportionally.
		// For each counter value c and model i with weight w_i out of totalWeight T,
		// model i is selected when floor((c)*w_i/T) > floor((c-1)*w_i/T).
		// This spreads selections evenly (like nginx's smooth WRR).
		c := route.rrCounter.Add(1)
		totalWeight := 0
		for _, p := range proxies {
			w := int(p.GetConfig().Weight)
			if w <= 0 {
				w = 1
			}
			totalWeight += w
		}
		for _, p := range proxies {
			w := int(p.GetConfig().Weight)
			if w <= 0 {
				w = 1
			}
			cur := (c * uint64(w)) / uint64(totalWeight)
			prev := ((c - 1) * uint64(w)) / uint64(totalWeight)
			if cur > prev {
				return p.GetConfig().ModelName, nil
			}
		}
		// Fallback: simple modulo
		idx := c % uint64(len(proxies))
		return proxies[idx].GetConfig().ModelName, nil
	}

	selected, err := route.Strategy.SelectInstance(ctx, proxies)
	if err != nil {
		return "", fmt.Errorf("route strategy failed for %q: %w", route.Slug, err)
	}
	return selected.GetConfig().ModelName, nil
}

// executeRouteWithFailover tries each model in a route using the route strategy.
func (m *ModelManager) executeRouteWithFailover(ctx context.Context, route *RouteEntry, req *FailoverRequest) (*FailoverResult, error) {
	instanceRetries := m.router.InstanceRetryAttempts
	if instanceRetries <= 0 {
		instanceRetries = 2
	}

	var failovers []string
	attemptCount := 0

	// Build a working copy of route models we can remove from
	remaining := make([]RouteModelEntry, 0, len(route.Models))
	for _, rm := range route.Models {
		if rm.Enabled {
			remaining = append(remaining, rm)
		}
	}

	for len(remaining) > 0 {
		// Build proxies from remaining models
		var proxies []routing.ModelInstance
		for _, rm := range remaining {
			instances, exists := m.registry.GetModelInstances(rm.ModelName)
			if !exists || len(instances) == 0 {
				continue
			}
			hasHealthy := false
			for _, inst := range instances {
				if m.healthTracker.IsHealthy(inst) {
					hasHealthy = true
					break
				}
			}
			if !hasHealthy {
				continue
			}
			proxy := NewRouteModelProxy(rm.ModelName, float64(rm.Weight), rm.Priority)
			proxies = append(proxies, proxy)
		}

		if len(proxies) == 0 {
			break
		}

		selectedModel, err := m.selectRouteModel(ctx, route, proxies)
		if err != nil {
			break
		}

		m.logger.Info("Route selected model",
			zap.String("route", route.Slug),
			zap.String("model", selectedModel))

		// Try the selected model's instances
		result, err := m.tryModelInstances(ctx, selectedModel, req, instanceRetries, &attemptCount, &failovers)
		if err == nil {
			return result, nil
		}

		// Remove failed model from remaining
		failovers = append(failovers, fmt.Sprintf("route-model:%s(failed)", selectedModel))
		newRemaining := make([]RouteModelEntry, 0, len(remaining))
		for _, rm := range remaining {
			if rm.ModelName != selectedModel {
				newRemaining = append(newRemaining, rm)
			}
		}
		remaining = newRemaining
	}

	return nil, fmt.Errorf("route %q: all models exhausted", route.Slug)
}

// ExecuteWithFailover executes a request with automatic instance retry and model fallback
// This provides transparent failover - end users don't see errors if an instance/model fails
func (m *ModelManager) ExecuteWithFailover(ctx context.Context, req *FailoverRequest) (*FailoverResult, error) {
	// Check if the model name is a route slug
	if route, isRoute := m.ResolveRoute(req.ModelName); isRoute && route != nil {
		result, err := m.executeRouteWithFailover(ctx, route, req)
		if err == nil {
			return result, nil
		}
		// Route exhausted — fall through to route's fallback models
		for _, fb := range route.FallbackModels {
			req2 := *req
			req2.ModelName = fb
			result, err := m.ExecuteWithFailover(ctx, &req2)
			if err == nil {
				return result, nil
			}
		}
		return nil, fmt.Errorf("route %q: all models and fallbacks exhausted: %w", req.ModelName, err)
	}

	if !m.router.EnableFailover {
		// Failover disabled - use simple execution
		instance, err := m.GetBestInstance(ctx, req.ModelName)
		if err != nil {
			return nil, err
		}

		response, err := req.ExecuteFunc(ctx, instance)
		if err != nil {
			m.RecordFailure(instance, err)
			return nil, err
		}

		return &FailoverResult{
			Response:     response,
			Instance:     instance,
			AttemptCount: 1,
			Failovers:    []string{},
		}, nil
	}

	// Calculate retry attempts (default to 2 if not configured)
	instanceRetries := m.router.InstanceRetryAttempts
	if instanceRetries <= 0 {
		instanceRetries = 2
	}

	var failovers []string
	attemptCount := 0
	currentModel := req.ModelName

	// Try models in fallback chain
	for {
		m.logger.Info("Attempting request with failover",
			zap.String("model", currentModel),
			zap.Int("attempt", attemptCount+1))

		// Try multiple instances of current model
		result, err := m.tryModelInstances(ctx, currentModel, req, instanceRetries, &attemptCount, &failovers)
		if err == nil {
			m.logger.Info("Request succeeded with failover",
				zap.String("final_model", currentModel),
				zap.String("final_instance", result.Instance.Config.ID),
				zap.Int("total_attempts", attemptCount),
				zap.Strings("failovers", failovers))
			return result, nil
		}

		// All instances of current model failed
		m.logger.Warn("All instances failed for model",
			zap.String("model", currentModel),
			zap.Error(err))

		// Check if model fallback is enabled
		if !m.router.EnableModelFallback {
			return nil, fmt.Errorf("all instances failed for model %s: %w", currentModel, err)
		}

		// Try fallback model
		fallbackModel, hasFallback := m.router.ModelFallbacks[currentModel]
		if !hasFallback {
			return nil, fmt.Errorf("no fallback configured for model %s after all instances failed", currentModel)
		}

		m.logger.Info("Failing over to fallback model",
			zap.String("from", currentModel),
			zap.String("to", fallbackModel))

		failovers = append(failovers, fmt.Sprintf("model:%s(all instances failed)", currentModel))
		currentModel = fallbackModel
		
		// Prevent infinite loops - max 5 model fallbacks
		if len(failovers) > 10 {
			return nil, fmt.Errorf("too many failover attempts (%d), giving up", len(failovers))
		}
	}
}

// tryModelInstances attempts to execute request against multiple instances of a model
func (m *ModelManager) tryModelInstances(
	ctx context.Context,
	modelName string,
	req *FailoverRequest,
	maxRetries int,
	attemptCount *int,
	failovers *[]string,
) (*FailoverResult, error) {
	var lastErr error

	// Get all available instances for the model
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

	// Try each healthy instance up to maxRetries times
	for retry := 0; retry < maxRetries && len(healthyInstances) > 0; retry++ {
		// Use routing strategy to select best instance
		var instance *ModelInstance
		
		// Convert to routing.ModelInstance interface for strategy
		var routingInstances []routing.ModelInstance
		for _, inst := range healthyInstances {
			routingInstances = append(routingInstances, inst)
		}
		
		selected, err := m.routingStrategy.SelectInstance(ctx, routingInstances)
		if err != nil {
			lastErr = err
			continue
		}
		instance = selected.(*ModelInstance)

		*attemptCount++

		m.logger.Info("Trying instance",
			zap.String("model", modelName),
			zap.String("instance", instance.Config.ID),
			zap.Int("attempt", *attemptCount),
			zap.Int("retry", retry+1),
			zap.Int("max_retries", maxRetries))

		// Apply timeout multiplier for failover attempts
		timeoutMultiple := m.router.FailoverTimeoutMultiple
		if timeoutMultiple <= 0 {
			timeoutMultiple = 1.5
		}
		
		timeout := time.Duration(float64(instance.Config.Timeout) * timeoutMultiple)
		executeCtx, cancel := context.WithTimeout(ctx, timeout)
		
		// Execute request
		response, err := req.ExecuteFunc(executeCtx, instance)
		cancel()

		if err != nil {
			m.logger.Warn("Instance request failed",
				zap.String("model", modelName),
				zap.String("instance", instance.Config.ID),
				zap.Error(err))
			
			m.RecordFailure(instance, err)
			*failovers = append(*failovers, fmt.Sprintf("instance:%s(%s)", instance.Config.ID, err.Error()))
			lastErr = err
			
			// Remove this instance from healthy list to avoid retrying it
			healthyInstances = removeInstance(healthyInstances, instance)
			continue
		}

		// Validate response if validation function provided
		if req.ValidateFunc != nil {
			if err := req.ValidateFunc(response); err != nil {
				m.logger.Warn("Response validation failed",
					zap.String("model", modelName),
					zap.String("instance", instance.Config.ID),
					zap.Error(err))
				
				*failovers = append(*failovers, fmt.Sprintf("instance:%s(validation failed)", instance.Config.ID))
				lastErr = err
				healthyInstances = removeInstance(healthyInstances, instance)
				continue
			}
		}

		// Success!
		m.logger.Info("Instance request succeeded",
			zap.String("model", modelName),
			zap.String("instance", instance.Config.ID))

		return &FailoverResult{
			Response:     response,
			Instance:     instance,
			AttemptCount: *attemptCount,
			Failovers:    *failovers,
		}, nil
	}

	return nil, fmt.Errorf("all instance attempts failed for model %s: %w", modelName, lastErr)
}

// removeInstance removes an instance from a slice
func removeInstance(instances []*ModelInstance, toRemove *ModelInstance) []*ModelInstance {
	result := make([]*ModelInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Config.ID != toRemove.Config.ID {
			result = append(result, inst)
		}
	}
	return result
}

// RecordSuccess records a successful request
func (m *ModelManager) RecordSuccess(instance *ModelInstance, tokens int64, latency time.Duration) {
	m.healthTracker.RecordSuccess(instance)
	m.metricsCollector.RecordRequest(instance, tokens, latency)
}

// RecordFailure records a failed request
func (m *ModelManager) RecordFailure(instance *ModelInstance, err error) {
	m.healthTracker.RecordFailure(instance, err)
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

// GetAvailableModels returns list of available models (including route slugs)
func (m *ModelManager) GetAvailableModels() []string {
	models := m.registry.GetAvailableModels()

	// Add route slugs
	m.routeMu.RLock()
	for slug := range m.routes {
		models = append(models, slug)
	}
	m.routeMu.RUnlock()

	return models
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

// RecordRequestEnd records the end of a request with distributed latency tracking
func (m *ModelManager) RecordRequestEnd(modelName string, latency time.Duration, success bool, err error) {
	// Record to distributed latency tracker (async, non-blocking)
	if m.latencyTracker != nil && success {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		if err := m.latencyTracker.RecordLatency(ctx, modelName, latency); err != nil {
			m.logger.Warn("Failed to record distributed latency",
				zap.String("model", modelName),
				zap.Duration("latency", latency),
				zap.Error(err))
		}
	}
}

// GetBestInstanceAdaptive returns the best instance (alias for GetBestInstance)
func (m *ModelManager) GetBestInstanceAdaptive(ctx context.Context, modelName string) (*ModelInstance, error) {
	return m.GetBestInstance(ctx, modelName)
}

// ListModels returns available models (alias for GetAvailableModels)
func (m *ModelManager) ListModels() []string {
	return m.GetAvailableModels()
}

// GetDetailedModelInfo returns detailed model information for API consumption.
// Only returns real models (not route slugs) so that /v1/models stays clean.
func (m *ModelManager) GetDetailedModelInfo() []ModelInfo {
	availableModels := m.registry.GetAvailableModels()
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
				Source:  instance.Config.Source,
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

// GetModelTags returns tags associated with a model
func (m *ModelManager) GetModelTags(modelName string) []string {
	allInstances := m.registry.GetAllInstances()
	
	// Collect tags from all instances of this model
	tagSet := make(map[string]bool)
	for _, instance := range allInstances {
		if instance.Config.ModelName == modelName {
			for _, tag := range instance.Config.Tags {
				if tag != "" {
					tagSet[tag] = true
				}
			}
		}
	}
	
	// Convert to slice
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	
	return tags
}

// AddInstance adds a model instance to the registry
func (m *ModelManager) AddInstance(cfg config.ModelInstance) error {
	return m.registry.AddInstance(cfg)
}

// RemoveInstance removes a model instance from the registry
func (m *ModelManager) RemoveInstance(instanceID string) error {
	return m.registry.RemoveInstance(instanceID)
}

// UpdateInstance updates a model instance in the registry
func (m *ModelManager) UpdateInstance(instanceID string, cfg config.ModelInstance) error {
	return m.registry.UpdateInstance(instanceID, cfg)
}

// GetInstanceSource returns the source of an instance ("system" or "user")
func (m *ModelManager) GetInstanceSource(instanceID string) string {
	return m.registry.GetInstanceSource(instanceID)
}

// CreateProvider creates a new provider instance from configuration parameters.
// This is used for test-connection to create a temporary provider without registering it.
func (m *ModelManager) CreateProvider(cfg config.ProviderParams) (providers.Provider, error) {
	return m.registry.CreateProvider(cfg)
}

// GetRegistry returns the model registry.
func (m *ModelManager) GetRegistry() *ModelRegistry {
	return m.registry
}

// GetHealthTracker returns the in-memory health tracker.
func (m *ModelManager) GetHealthTracker() *HealthTracker {
	return m.healthTracker
}

// GetHealthStore returns the Redis-backed health store (nil if Redis unavailable).
func (m *ModelManager) GetHealthStore() *redisService.HealthStore {
	return m.healthStore
}

// NewHealthChecker creates a HealthChecker wired to this manager's registry, health tracker, and health store.
func (m *ModelManager) NewHealthChecker(interval time.Duration) *HealthChecker {
	return NewHealthChecker(m.registry, m.healthTracker, m.healthStore, interval, m.logger)
}

// ModelInfo represents detailed model information for API responses
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
	Created int64  `json:"created"`
	Source  string `json:"source,omitempty"`
}
