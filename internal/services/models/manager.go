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
	return &ModelManager{
		instances:         make(map[string]*ModelInstance),
		modelMap:          make(map[string][]*ModelInstance),
		providers:         make(map[string]providers.Provider),
		router:            router,
		logger:            logger,
		roundRobinCounters: make(map[string]*atomic.Uint64),
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
		provider, err = providers.NewVertexAIProvider(key, providerCfg)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", params.Type)
	}
	
	if err != nil {
		return nil, err
	}
	
	m.providers[key] = provider
	return provider, nil
}

// GetBestInstance returns the best instance for a model based on routing strategy
func (m *ModelManager) GetBestInstance(ctx context.Context, modelName string) (*ModelInstance, error) {
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