package models

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/providers"
	"go.uber.org/zap"
)

// ModelRegistry manages model instances and their organization
type ModelRegistry struct {
	instances          map[string]*ModelInstance     // key: instance ID
	modelMap           map[string][]*ModelInstance   // key: model name, value: instances for that model
	providers          map[string]providers.Provider // Provider instances by unique key
	roundRobinCounters map[string]*atomic.Uint64
	logger             *zap.Logger
	mu                 sync.RWMutex
}

// NewModelRegistry creates a new model registry
func NewModelRegistry(logger *zap.Logger) *ModelRegistry {
	return &ModelRegistry{
		instances:          make(map[string]*ModelInstance),
		modelMap:           make(map[string][]*ModelInstance),
		providers:          make(map[string]providers.Provider),
		roundRobinCounters: make(map[string]*atomic.Uint64),
		logger:             logger,
	}
}

// LoadModelInstances loads model instances from configuration
func (r *ModelRegistry) LoadModelInstances(instances []config.ModelInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, cfg := range instances {
		if !cfg.Enabled {
			r.logger.Debug("Skipping disabled instance", zap.String("id", cfg.ID))
			continue
		}

		// Create or reuse provider instance
		provider, err := r.getOrCreateProvider(cfg.Provider)
		if err != nil {
			r.logger.Error("Failed to create provider for instance",
				zap.String("instance", cfg.ID),
				zap.Error(err))
			continue
		}

		// Create instance
		instance := NewModelInstance(cfg, provider)

		// Store instance
		r.instances[cfg.ID] = instance

		// Add to model map
		if r.modelMap[cfg.ModelName] == nil {
			r.modelMap[cfg.ModelName] = make([]*ModelInstance, 0)
			r.roundRobinCounters[cfg.ModelName] = &atomic.Uint64{}
		}
		r.modelMap[cfg.ModelName] = append(r.modelMap[cfg.ModelName], instance)

		r.logger.Info("Loaded instance",
			zap.String("id", cfg.ID),
			zap.String("model", cfg.ModelName),
			zap.String("provider", cfg.Provider.Type))
	}

	// Sort instances by priority for each model
	for modelName, insts := range r.modelMap {
		sort.Slice(insts, func(i, j int) bool {
			return insts[i].Config.Priority > insts[j].Config.Priority
		})
		r.modelMap[modelName] = insts
	}

	r.logger.Info("Loaded model instances",
		zap.Int("total_instances", len(r.instances)),
		zap.Int("models", len(r.modelMap)))

	return nil
}

// getOrCreateProvider creates or reuses a provider instance
func (r *ModelRegistry) getOrCreateProvider(providerCfg config.ProviderParams) (providers.Provider, error) {
	// Create unique key for provider configuration
	providerKey := fmt.Sprintf("%s:%s:%s", providerCfg.Type, providerCfg.BaseURL, providerCfg.APIKey)

	// Check if provider already exists
	if provider, exists := r.providers[providerKey]; exists {
		return provider, nil
	}

	// Create new provider by calling appropriate constructor
	provider, err := r.createProvider(providerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// Store for reuse
	r.providers[providerKey] = provider
	return provider, nil
}

// createProvider creates a new provider instance based on type
func (r *ModelRegistry) createProvider(cfg config.ProviderParams) (providers.Provider, error) {
	// Create a ProviderConfig from ProviderParams
	providerCfg := providers.ProviderConfig{
		Type:    cfg.Type,
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		OrgID:   cfg.OrgID,
		Enabled: true, // Assume enabled if we're creating it
	}
	
	// Use a temporary name for provider creation
	providerName := fmt.Sprintf("%s-instance", cfg.Type)
	
	switch cfg.Type {
	case "openai":
		return providers.NewOpenAIProvider(providerName, providerCfg)
	case "anthropic":
		return providers.NewAnthropicProvider(providerName, providerCfg)
	case "azure":
		return providers.NewAzureProvider(providerName, providerCfg)
	case "bedrock":
		return providers.NewBedrockProvider(providerName, providerCfg)
	case "vertex":
		return providers.NewVertexProvider(providerName, providerCfg)
	case "openrouter":
		return providers.NewOpenRouterProvider(providerName, providerCfg)
	case "cohere":
		return nil, fmt.Errorf("cohere provider not implemented yet")
	case "huggingface":
		return nil, fmt.Errorf("huggingface provider not implemented yet")
	case "custom":
		return nil, fmt.Errorf("custom provider not implemented yet")
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}

// GetInstance returns a specific model instance by ID
func (r *ModelRegistry) GetInstance(instanceID string) (*ModelInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	instance, exists := r.instances[instanceID]
	return instance, exists
}

// GetModelInstances returns all instances for a given model
func (r *ModelRegistry) GetModelInstances(modelName string) ([]*ModelInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	instances, exists := r.modelMap[modelName]
	if !exists {
		return nil, false
	}
	
	// Return a copy to avoid concurrent access issues
	result := make([]*ModelInstance, len(instances))
	copy(result, instances)
	return result, true
}

// GetAllInstances returns all registered instances
func (r *ModelRegistry) GetAllInstances() []*ModelInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	instances := make([]*ModelInstance, 0, len(r.instances))
	for _, instance := range r.instances {
		instances = append(instances, instance)
	}
	return instances
}

// GetAvailableModels returns list of all available model names
func (r *ModelRegistry) GetAvailableModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	models := make([]string, 0, len(r.modelMap))
	for modelName := range r.modelMap {
		models = append(models, modelName)
	}
	return models
}

// GetRoundRobinCounter returns the round-robin counter for a model
func (r *ModelRegistry) GetRoundRobinCounter(modelName string) *atomic.Uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	return r.roundRobinCounters[modelName]
}

// GetRegistryStats returns statistics about the registry
func (r *ModelRegistry) GetRegistryStats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	stats := RegistryStats{
		TotalInstances: len(r.instances),
		TotalModels:    len(r.modelMap),
		TotalProviders: len(r.providers),
		ModelCounts:    make(map[string]int),
	}
	
	for modelName, instances := range r.modelMap {
		stats.ModelCounts[modelName] = len(instances)
	}
	
	return stats
}

// RegistryStats represents statistics about the model registry
type RegistryStats struct {
	TotalInstances int            `json:"total_instances"`
	TotalModels    int            `json:"total_models"`
	TotalProviders int            `json:"total_providers"`
	ModelCounts    map[string]int `json:"model_counts"`
}