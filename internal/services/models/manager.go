package models

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/circuitbreaker"
	"github.com/amerfu/pllm/internal/services/loadbalancer"
	"github.com/amerfu/pllm/internal/services/providers"
	"go.uber.org/zap"
)

// ModelManager manages model instances with load balancing and failover
type ModelManager struct {
	instances map[string]*ModelInstance // key: instance ID
	modelMap  map[string][]*ModelInstance // key: model name, value: instances for that model
	providers map[string]providers.Provider // Provider instances by unique key
	
	router     config.RouterSettings
	logger     *zap.Logger
	mu         sync.RWMutex
	
	// Round-robin state
	roundRobinCounters map[string]*atomic.Uint64
	
	// Circuit breaker for model failures (simple version)
	circuitBreaker *circuitbreaker.Manager
	
	// Advanced circuit breakers for performance-aware routing
	adaptiveBreakers map[string]*circuitbreaker.AdaptiveBreaker
	
	// Load balancer for intelligent routing
	loadBalancer *loadbalancer.AdaptiveLoadBalancer
}

// ModelInstance represents a single model instance
type ModelInstance struct {
	Config   config.ModelInstance
	Provider providers.Provider
	
	// Health tracking
	Healthy     atomic.Bool
	LastError   atomic.Value // stores error
	LastSuccess atomic.Value // stores time.Time
	FailureCount atomic.Int32
	
	// Performance metrics
	TotalRequests atomic.Int64
	TotalTokens   atomic.Int64
	AverageLatency atomic.Int64 // in milliseconds
	
	// Rate limiting state
	RequestsThisMinute atomic.Int32
	TokensThisMinute   atomic.Int32
	WindowStart       atomic.Value // stores time.Time
}

// NewModelManager creates a new model manager
func NewModelManager(logger *zap.Logger, router config.RouterSettings) *ModelManager {
	// Initialize circuit breaker if enabled
	var cb *circuitbreaker.Manager
	if router.CircuitBreakerEnabled {
		threshold := router.CircuitBreakerThreshold
		if threshold <= 0 {
			threshold = 5 // Default threshold
		}
		cooldown := router.CircuitBreakerCooldown
		if cooldown <= 0 {
			cooldown = 30 * time.Second // Default cooldown
		}
		cb = circuitbreaker.NewManager(threshold, cooldown)
	}
	
	// Initialize adaptive load balancer for high-load scenarios
	lb := loadbalancer.NewAdaptiveLoadBalancer()
	
	return &ModelManager{
		instances:         make(map[string]*ModelInstance),
		modelMap:          make(map[string][]*ModelInstance),
		providers:         make(map[string]providers.Provider),
		router:            router,
		logger:            logger,
		roundRobinCounters: make(map[string]*atomic.Uint64),
		circuitBreaker:    cb,
		adaptiveBreakers:  make(map[string]*circuitbreaker.AdaptiveBreaker),
		loadBalancer:      lb,
	}
}

// LoadModelInstances loads model instances from configuration
func (m *ModelManager) LoadModelInstances(instances []config.ModelInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, cfg := range instances {
		if !cfg.Enabled {
			m.logger.Debug("Skipping disabled instance", zap.String("id", cfg.ID))
			continue
		}
		
		// Create or reuse provider instance
		provider, err := m.getOrCreateProvider(cfg.Provider)
		if err != nil {
			m.logger.Error("Failed to create provider for instance",
				zap.String("instance", cfg.ID),
				zap.Error(err))
			continue
		}
		
		// Create instance
		instance := &ModelInstance{
			Config:   cfg,
			Provider: provider,
		}
		instance.Healthy.Store(true)
		instance.WindowStart.Store(time.Now())
		
		// Store instance
		m.instances[cfg.ID] = instance
		
		// Add to model map
		if m.modelMap[cfg.ModelName] == nil {
			m.modelMap[cfg.ModelName] = make([]*ModelInstance, 0)
			m.roundRobinCounters[cfg.ModelName] = &atomic.Uint64{}
		}
		m.modelMap[cfg.ModelName] = append(m.modelMap[cfg.ModelName], instance)
		
		m.logger.Info("Loaded instance",
			zap.String("id", cfg.ID),
			zap.String("model", cfg.ModelName),
			zap.String("provider", cfg.Provider.Type))
	}
	
	// Sort instances by priority for each model
	for modelName, insts := range m.modelMap {
		sort.Slice(insts, func(i, j int) bool {
			return insts[i].Config.Priority > insts[j].Config.Priority
		})
		m.modelMap[modelName] = insts
		
		// Register model with adaptive load balancer
		maxResponseTime := 10 * time.Second // Default max response time
		if len(insts) > 0 && insts[0].Config.Timeout > 0 {
			maxResponseTime = insts[0].Config.Timeout
		}
		m.loadBalancer.RegisterModel(modelName, maxResponseTime)
		
		// Create adaptive circuit breaker for performance-aware routing
		m.adaptiveBreakers[modelName] = circuitbreaker.NewAdaptiveBreaker(
			5,                // failure threshold
			2*time.Second,    // latency threshold (requests slower than this are "slow")
			3,                // slow request limit before opening circuit
		)
		
		m.logger.Debug("Registered model with adaptive components",
			zap.String("model", modelName),
			zap.Int("instances", len(insts)))
	}
	
	// Set up fallback chains in load balancer
	if m.router.Fallbacks != nil {
		for model, fallbacks := range m.router.Fallbacks {
			m.loadBalancer.SetFallbacks(model, fallbacks)
		}
	}
	
	m.logger.Info("Successfully loaded instances",
		zap.Int("total", len(m.instances)),
		zap.Int("models", len(m.modelMap)))
	
	// Start health check routine if enabled
	if m.router.HealthCheckInterval > 0 {
		go m.healthCheckLoop()
	}
	
	return nil
}

// getOrCreateProvider creates or returns an existing provider instance
func (m *ModelManager) getOrCreateProvider(params config.ProviderParams) (providers.Provider, error) {
	// Create a unique key for this provider configuration
	key := fmt.Sprintf("%s-%s-%s", params.Type, params.BaseURL, params.APIKey[:min(8, len(params.APIKey))])
	
	if provider, exists := m.providers[key]; exists {
		return provider, nil
	}
	
	// Create provider configuration
	providerCfg := providers.ProviderConfig{
		Type:       params.Type,
		APIKey:     params.APIKey,
		APISecret:  params.APISecret,
		BaseURL:    params.BaseURL,
		APIVersion: params.APIVersion,
		OrgID:      params.OrgID,
		Region:     params.Region,
		Enabled:    true,
	}
	
	// Create provider based on type
	var provider providers.Provider
	var err error
	
	switch params.Type {
	case "openai":
		provider, err = providers.NewOpenAIProvider(key, providerCfg)
	case "anthropic":
		provider, err = providers.NewAnthropicProvider(key, providerCfg)
	case "azure":
		provider, err = providers.NewAzureProvider(key, providerCfg)
	case "bedrock":
		provider, err = providers.NewBedrockProvider(key, providerCfg)
	case "vertex":
		provider, err = providers.NewVertexProvider(key, providerCfg)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", params.Type)
	}
	
	if err != nil {
		return nil, err
	}
	
	m.providers[key] = provider
	return provider, nil
}

// GetBestInstanceAdaptive uses adaptive load balancing for high-load scenarios
func (m *ModelManager) GetBestInstanceAdaptive(ctx context.Context, modelName string) (*ModelInstance, error) {
	// First check if the requested model exists
	m.mu.RLock()
	_, modelExists := m.modelMap[modelName]
	m.mu.RUnlock()
	
	if !modelExists {
		return nil, fmt.Errorf("model not found: %s", modelName)
	}
	
	// Use adaptive load balancer to select the best model considering performance
	selectedModel, err := m.loadBalancer.SelectModel(ctx, modelName)
	if err != nil {
		m.logger.Debug("Adaptive load balancer failed to select model", 
			zap.String("model", modelName),
			zap.Error(err))
		// Fall back to regular GetBestInstance
		return m.GetBestInstance(ctx, modelName)
	}

	m.logger.Debug("Adaptive load balancer selected model",
		zap.String("requested", modelName),
		zap.String("selected", selectedModel))

	// Check adaptive circuit breaker for the selected model
	if breaker, exists := m.adaptiveBreakers[selectedModel]; exists {
		if !breaker.CanRequest() {
			m.logger.Debug("Adaptive circuit breaker is open for model", 
				zap.String("model", selectedModel))
			// Try fallback through regular routing
			return m.GetBestInstance(ctx, modelName)
		}
	}

	// Get the actual instance for the selected model
	m.mu.RLock()
	instances, exists := m.modelMap[selectedModel]
	m.mu.RUnlock()

	if !exists || len(instances) == 0 {
		m.logger.Warn("No instances found for selected model, falling back",
			zap.String("selected", selectedModel),
			zap.String("original", modelName))
		return m.GetBestInstance(ctx, modelName)
	}

	// Select best instance for the chosen model
	instance, err := m.selectBestInstanceForModel(ctx, selectedModel, instances)
	if err != nil {
		m.logger.Debug("Failed to select instance for model",
			zap.String("model", selectedModel),
			zap.Error(err))
		return m.GetBestInstance(ctx, modelName)
	}
	
	return instance, nil
}

// GetBestInstance returns the best instance for a model based on routing strategy
func (m *ModelManager) GetBestInstance(ctx context.Context, modelName string) (*ModelInstance, error) {
	// Check circuit breaker first if enabled
	if m.circuitBreaker != nil && m.circuitBreaker.IsOpen(modelName) {
		m.logger.Debug("Circuit breaker is open for model, trying fallbacks", 
			zap.String("model", modelName))
		
		// Try fallback models
		if fallbacks, exists := m.router.Fallbacks[modelName]; exists {
			for _, fallbackModel := range fallbacks {
				// Check if fallback's circuit is also open
				if m.circuitBreaker.IsOpen(fallbackModel) {
					continue
				}
				
				instance, err := m.getBestInstanceInternal(ctx, fallbackModel)
				if err == nil {
					m.logger.Info("Using fallback model",
						zap.String("original", modelName),
						zap.String("fallback", fallbackModel))
					return instance, nil
				}
			}
		}
		
		return nil, fmt.Errorf("circuit breaker open for model %s and all fallbacks failed", modelName)
	}
	
	// Try primary model
	instance, err := m.getBestInstanceInternal(ctx, modelName)
	if err == nil {
		// Record success if circuit breaker is enabled
		if m.circuitBreaker != nil {
			m.circuitBreaker.RecordSuccess(modelName)
		}
		return instance, nil
	}
	
	// Primary failed, record failure if circuit breaker is enabled
	if m.circuitBreaker != nil {
		m.circuitBreaker.RecordFailure(modelName)
	}
	
	// Try fallback models
	if fallbacks, exists := m.router.Fallbacks[modelName]; exists {
		m.logger.Debug("Primary model failed, trying fallbacks",
			zap.String("model", modelName),
			zap.Error(err))
		
		for _, fallbackModel := range fallbacks {
			// Check circuit breaker for fallback
			if m.circuitBreaker != nil && m.circuitBreaker.IsOpen(fallbackModel) {
				continue
			}
			
			instance, fallbackErr := m.getBestInstanceInternal(ctx, fallbackModel)
			if fallbackErr == nil {
				m.logger.Info("Using fallback model",
					zap.String("original", modelName),
					zap.String("fallback", fallbackModel))
				// Record success for fallback
				if m.circuitBreaker != nil {
					m.circuitBreaker.RecordSuccess(fallbackModel)
				}
				return instance, nil
			}
			
			// Record fallback failure
			if m.circuitBreaker != nil {
				m.circuitBreaker.RecordFailure(fallbackModel)
			}
		}
	}
	
	return nil, fmt.Errorf("model %s and all fallbacks failed: %w", modelName, err)
}

// selectBestInstanceForModel selects the best instance from a list of instances
func (m *ModelManager) selectBestInstanceForModel(ctx context.Context, modelName string, instances []*ModelInstance) (*ModelInstance, error) {
	// Filter healthy instances
	var healthyInstances []*ModelInstance
	for _, inst := range instances {
		if inst.IsHealthy() && !inst.IsRateLimited() {
			healthyInstances = append(healthyInstances, inst)
		}
	}
	
	if len(healthyInstances) == 0 {
		// Try to find any instance that's not rate limited
		for _, inst := range instances {
			if !inst.IsRateLimited() {
				return inst, nil
			}
		}
		return nil, fmt.Errorf("no healthy instances available for model: %s", modelName)
	}
	
	// Select based on routing strategy
	switch m.router.RoutingStrategy {
	case "round-robin":
		return m.selectRoundRobin(modelName, healthyInstances), nil
	case "weighted":
		return m.selectWeighted(healthyInstances), nil
	case "least-busy":
		return m.selectLeastBusy(healthyInstances), nil
	case "latency-based":
		return m.selectLowestLatency(healthyInstances), nil
	case "usage-based":
		return m.selectByUsage(healthyInstances), nil
	case "priority":
		fallthrough
	default:
		return m.selectByPriority(healthyInstances), nil
	}
}

// getBestInstanceInternal is the internal instance selection logic
func (m *ModelManager) getBestInstanceInternal(ctx context.Context, modelName string) (*ModelInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	instances, exists := m.modelMap[modelName]
	if !exists || len(instances) == 0 {
		return nil, fmt.Errorf("no instances found for model: %s", modelName)
	}
	
	// Filter healthy instances
	var healthyInstances []*ModelInstance
	for _, inst := range instances {
		if inst.IsHealthy() && !inst.IsRateLimited() {
			healthyInstances = append(healthyInstances, inst)
		}
	}
	
	if len(healthyInstances) == 0 {
		// Try to find any instance that's not rate limited
		for _, inst := range instances {
			if !inst.IsRateLimited() {
				return inst, nil
			}
		}
		return nil, fmt.Errorf("no healthy instances available for model: %s", modelName)
	}
	
	// Select based on routing strategy
	switch m.router.RoutingStrategy {
	case "round-robin":
		return m.selectRoundRobin(modelName, healthyInstances), nil
	case "weighted":
		return m.selectWeighted(healthyInstances), nil
	case "least-busy":
		return m.selectLeastBusy(healthyInstances), nil
	case "latency-based":
		return m.selectLowestLatency(healthyInstances), nil
	case "usage-based":
		return m.selectByUsage(healthyInstances), nil
	case "priority":
		fallthrough
	default:
		return m.selectByPriority(healthyInstances), nil
	}
}

// Routing strategy implementations

func (m *ModelManager) selectByPriority(instances []*ModelInstance) *ModelInstance {
	// Already sorted by priority
	return instances[0]
}

func (m *ModelManager) selectRoundRobin(modelName string, instances []*ModelInstance) *ModelInstance {
	counter := m.roundRobinCounters[modelName]
	index := counter.Add(1) % uint64(len(instances))
	return instances[index]
}

func (m *ModelManager) selectWeighted(instances []*ModelInstance) *ModelInstance {
	// Calculate total weight
	var totalWeight float64
	for _, d := range instances {
		weight := d.Config.Weight
		if weight <= 0 {
			weight = 1.0
		}
		totalWeight += weight
	}
	
	// Random selection based on weights
	r := rand.Float64() * totalWeight
	var cumWeight float64
	
	for _, d := range instances {
		weight := d.Config.Weight
		if weight <= 0 {
			weight = 1.0
		}
		cumWeight += weight
		if r <= cumWeight {
			return d
		}
	}
	
	return instances[len(instances)-1]
}

func (m *ModelManager) selectLeastBusy(instances []*ModelInstance) *ModelInstance {
	var leastBusy *ModelInstance
	minRequests := int64(^uint64(0) >> 1) // Max int64
	
	for _, d := range instances {
		requests := d.TotalRequests.Load()
		if requests < minRequests {
			minRequests = requests
			leastBusy = d
		}
	}
	
	return leastBusy
}

func (m *ModelManager) selectLowestLatency(instances []*ModelInstance) *ModelInstance {
	var fastest *ModelInstance
	minLatency := int64(^uint64(0) >> 1) // Max int64
	
	for _, d := range instances {
		latency := d.AverageLatency.Load()
		if latency > 0 && latency < minLatency {
			minLatency = latency
			fastest = d
		}
	}
	
	// If no latency data, fall back to priority
	if fastest == nil {
		return instances[0]
	}
	
	return fastest
}

func (m *ModelManager) selectByUsage(instances []*ModelInstance) *ModelInstance {
	// Select instance with most remaining capacity
	var bestInstance *ModelInstance
	maxRemainingRPM := int32(0)
	
	for _, d := range instances {
		if d.Config.RPM > 0 {
			remaining := d.Config.RPM - int(d.RequestsThisMinute.Load())
			if remaining > int(maxRemainingRPM) {
				maxRemainingRPM = int32(remaining)
				bestInstance = d
			}
		}
	}
	
	if bestInstance == nil {
		return instances[0]
	}
	
	return bestInstance
}

// ModelListItem represents a model in the API response
type ModelListItem struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	OwnedBy           string   `json:"owned_by"`
	Permission        []string `json:"permission,omitempty"`
	Root              string   `json:"root,omitempty"`
	Parent            string   `json:"parent,omitempty"`
}

// ListModels returns all available models in OpenAI-compatible format
func (m *ModelManager) ListModels() []ModelListItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	modelMap := make(map[string]ModelListItem)
	createdTime := time.Now().Unix()
	
	for modelName, instances := range m.modelMap {
		if len(instances) > 0 {
			// Collect provider types
			providerTypes := make(map[string]bool)
			for _, d := range instances {
				providerTypes[d.Config.Provider.Type] = true
			}
			
			// Get first provider type for owned_by
			var ownedBy string
			for p := range providerTypes {
				ownedBy = p
				break
			}
			
			modelMap[modelName] = ModelListItem{
				ID:      modelName,
				Object:  "model",
				Created: createdTime,
				OwnedBy: ownedBy,
				Root:    modelName,
			}
		}
	}
	
	// Convert to slice
	models := make([]ModelListItem, 0, len(modelMap))
	for _, item := range modelMap {
		models = append(models, item)
	}
	
	// Sort by model ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})
	
	return models
}

// Deployment methods

func (d *ModelInstance) IsHealthy() bool {
	return d.Healthy.Load()
}

func (d *ModelInstance) IsRateLimited() bool {
	if d.Config.RPM <= 0 && d.Config.TPM <= 0 {
		return false // No rate limits configured
	}
	
	// Check if we need to reset the window
	windowStart := d.WindowStart.Load().(time.Time)
	if time.Since(windowStart) > time.Minute {
		d.RequestsThisMinute.Store(0)
		d.TokensThisMinute.Store(0)
		d.WindowStart.Store(time.Now())
		return false
	}
	
	// Check rate limits
	if d.Config.RPM > 0 && int(d.RequestsThisMinute.Load()) >= d.Config.RPM {
		return true
	}
	
	if d.Config.TPM > 0 && int(d.TokensThisMinute.Load()) >= d.Config.TPM {
		return true
	}
	
	return false
}

func (d *ModelInstance) RecordRequest(tokens int32, latencyMs int64) {
	d.TotalRequests.Add(1)
	d.TotalTokens.Add(int64(tokens))
	d.RequestsThisMinute.Add(1)
	d.TokensThisMinute.Add(tokens)
	
	// Update average latency (simple moving average)
	oldAvg := d.AverageLatency.Load()
	totalReqs := d.TotalRequests.Load()
	newAvg := (oldAvg*(totalReqs-1) + latencyMs) / totalReqs
	d.AverageLatency.Store(newAvg)
	
	d.LastSuccess.Store(time.Now())
	d.FailureCount.Store(0)
	d.Healthy.Store(true)
}

func (d *ModelInstance) RecordError(err error) {
	d.LastError.Store(err)
	failures := d.FailureCount.Add(1)
	
	// Mark unhealthy after allowed failures
	if failures >= 3 { // TODO: make configurable
		d.Healthy.Store(false)
	}
}

// RecordRequestStart marks the beginning of a request (for tracking concurrent requests)
func (m *ModelManager) RecordRequestStart(modelName string) {
	// Debug logging
	m.logger.Debug("RecordRequestStart called", 
		zap.String("model", modelName),
		zap.Int("breakers_count", len(m.adaptiveBreakers)))
	
	// Record in load balancer
	m.loadBalancer.RecordRequestStart(modelName)
	
	// Track in adaptive circuit breaker
	if breaker, exists := m.adaptiveBreakers[modelName]; exists {
		m.logger.Debug("Found breaker for model", zap.String("model", modelName))
		breaker.StartRequest()
	} else {
		m.logger.Debug("No breaker found for model", 
			zap.String("model", modelName),
			zap.Any("available_breakers", m.getAvailableBreakerNames()))
	}
}

func (m *ModelManager) getAvailableBreakerNames() []string {
	var names []string
	for name := range m.adaptiveBreakers {
		names = append(names, name)
	}
	return names
}

// RecordRequestEnd marks the end of a request with performance metrics
func (m *ModelManager) RecordRequestEnd(modelName string, latency time.Duration, success bool, err error) {
	// Record in load balancer
	m.loadBalancer.RecordRequestEnd(modelName, latency, success)
	
	// Record in adaptive circuit breaker
	if breaker, exists := m.adaptiveBreakers[modelName]; exists {
		breaker.EndRequest()
		if success {
			breaker.RecordSuccess(latency)
		} else {
			// Check if it's a timeout
			if err != nil && (contains(err.Error(), "timeout") || contains(err.Error(), "deadline")) {
				breaker.RecordTimeout()
			} else {
				breaker.RecordFailure()
			}
		}
	}
	
	// Record in simple circuit breaker if enabled
	if m.circuitBreaker != nil {
		if success {
			m.circuitBreaker.RecordSuccess(modelName)
		} else {
			m.circuitBreaker.RecordFailure(modelName)
		}
	}
}

// GetModelStats returns performance statistics for all models
func (m *ModelManager) GetModelStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	// Get stats from load balancer
	lbStats := m.loadBalancer.GetModelStats()
	stats["load_balancer"] = lbStats
	
	// Get stats from adaptive circuit breakers
	breakerStats := make(map[string]interface{})
	for model, breaker := range m.adaptiveBreakers {
		breakerStats[model] = breaker.GetState()
	}
	stats["adaptive_breakers"] = breakerStats
	
	// Check if we should shed load
	stats["should_shed_load"] = m.loadBalancer.ShouldShedLoad()
	
	return stats
}

// contains is a helper function for string matching
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// healthCheckLoop periodically checks instance health
func (m *ModelManager) healthCheckLoop() {
	ticker := time.NewTicker(m.router.HealthCheckInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		m.mu.RLock()
		instances := make([]*ModelInstance, 0, len(m.instances))
		for _, d := range m.instances {
			instances = append(instances, d)
		}
		m.mu.RUnlock()
		
		for _, d := range instances {
			// Check if instance should be marked healthy again
			if !d.IsHealthy() {
				lastSuccess := d.LastSuccess.Load()
				if lastSuccess != nil {
					if time.Since(lastSuccess.(time.Time)) > d.Config.CooldownPeriod {
						d.Healthy.Store(true)
						d.FailureCount.Store(0)
						m.logger.Info("Instance marked healthy after cooldown",
							zap.String("id", d.Config.ID))
					}
				}
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}