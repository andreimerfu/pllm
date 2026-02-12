package models

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/services/llm/providers"
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

// getOrCreateProvider creates or reuses a provider instance.
// For Azure, each unique deployment gets its own provider instance because the
// deployment name is baked into the URL path and looked up by provider-model name.
// Two models sharing the same provider-model but different deployments would conflict
// if they shared a provider, so we include the deployment in the cache key.
func (r *ModelRegistry) getOrCreateProvider(providerCfg config.ProviderParams) (providers.Provider, error) {
	// Normalise base URL: for Azure, BaseURL may be empty (AzureEndpoint is used).
	baseURL := providerCfg.BaseURL
	if baseURL == "" && providerCfg.AzureEndpoint != "" {
		baseURL = providerCfg.AzureEndpoint
	}

	providerKey := fmt.Sprintf("%s:%s:%s", providerCfg.Type, baseURL, providerCfg.APIKey)

	// Azure: include deployment in the key so each deployment gets its own provider.
	if providerCfg.Type == "azure" && providerCfg.AzureDeployment != "" {
		providerKey += ":" + providerCfg.AzureDeployment
	}

	// Check if provider already exists
	if provider, exists := r.providers[providerKey]; exists {
		return provider, nil
	}

	// Create new provider by calling appropriate constructor
	provider, err := r.CreateProvider(providerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// Store for reuse
	r.providers[providerKey] = provider
	return provider, nil
}

// CreateProvider creates a new provider instance based on type
func (r *ModelRegistry) CreateProvider(cfg config.ProviderParams) (providers.Provider, error) {
	// Create a ProviderConfig from ProviderParams
	providerCfg := providers.ProviderConfig{
		Type:      cfg.Type,
		APIKey:    cfg.APIKey,
		APISecret: cfg.APISecret,
		BaseURL:   cfg.BaseURL,
		OrgID:     cfg.OrgID,
		Region:    cfg.Region,
		Enabled:   true,
	}

	// Map provider-specific fields via Extra
	extra := make(map[string]interface{})

	switch cfg.Type {
	case "azure":
		// Azure uses BaseURL as the endpoint URL
		if providerCfg.BaseURL == "" && cfg.AzureEndpoint != "" {
			providerCfg.BaseURL = cfg.AzureEndpoint
		}
		if cfg.APIVersion != "" {
			providerCfg.APIVersion = cfg.APIVersion
		}
		if cfg.AzureDeployment != "" {
			extra["deployments"] = map[string]interface{}{
				cfg.Model: cfg.AzureDeployment,
			}
		}
	case "bedrock":
		// Bedrock uses APIKey/APISecret for AWS credentials
		if cfg.AWSAccessKeyID != "" {
			providerCfg.APIKey = cfg.AWSAccessKeyID
		}
		if cfg.AWSSecretAccessKey != "" {
			providerCfg.APISecret = cfg.AWSSecretAccessKey
		}
		if cfg.AWSRegionName != "" {
			providerCfg.Region = cfg.AWSRegionName
		}
	case "vertex":
		if cfg.VertexProject != "" {
			extra["project_id"] = cfg.VertexProject
		}
		if cfg.VertexLocation != "" {
			providerCfg.Region = cfg.VertexLocation
		}
	}

	if len(extra) > 0 {
		providerCfg.Extra = extra
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

// AddInstance adds a single model instance to the registry. Thread-safe.
// Returns an error if an instance with the same ID already exists.
func (r *ModelRegistry) AddInstance(cfg config.ModelInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.instances[cfg.ID]; exists {
		return fmt.Errorf("instance with ID %s already exists", cfg.ID)
	}

	if !cfg.Enabled {
		return nil
	}

	provider, err := r.getOrCreateProvider(cfg.Provider)
	if err != nil {
		return fmt.Errorf("failed to create provider for instance %s: %w", cfg.ID, err)
	}

	instance := NewModelInstance(cfg, provider)
	r.instances[cfg.ID] = instance

	if r.modelMap[cfg.ModelName] == nil {
		r.modelMap[cfg.ModelName] = make([]*ModelInstance, 0)
		r.roundRobinCounters[cfg.ModelName] = &atomic.Uint64{}
	}
	r.modelMap[cfg.ModelName] = append(r.modelMap[cfg.ModelName], instance)

	// Re-sort by priority
	insts := r.modelMap[cfg.ModelName]
	sort.Slice(insts, func(i, j int) bool {
		return insts[i].Config.Priority > insts[j].Config.Priority
	})

	r.logger.Info("Added instance",
		zap.String("id", cfg.ID),
		zap.String("model", cfg.ModelName),
		zap.String("provider", cfg.Provider.Type))

	return nil
}

// RemoveInstance removes a model instance from the registry. Thread-safe.
func (r *ModelRegistry) RemoveInstance(instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, exists := r.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	modelName := instance.Config.ModelName
	delete(r.instances, instanceID)

	// Remove from model map
	if insts, ok := r.modelMap[modelName]; ok {
		filtered := make([]*ModelInstance, 0, len(insts))
		for _, inst := range insts {
			if inst.Config.ID != instanceID {
				filtered = append(filtered, inst)
			}
		}
		if len(filtered) == 0 {
			delete(r.modelMap, modelName)
			delete(r.roundRobinCounters, modelName)
		} else {
			r.modelMap[modelName] = filtered
		}
	}

	r.logger.Info("Removed instance",
		zap.String("id", instanceID),
		zap.String("model", modelName))

	return nil
}

// UpdateInstance removes the old instance and adds the updated one. Thread-safe.
func (r *ModelRegistry) UpdateInstance(instanceID string, cfg config.ModelInstance) error {
	// Remove old instance (ignore not found - it may have been removed)
	_ = r.RemoveInstance(instanceID)

	// Add the new instance
	return r.AddInstance(cfg)
}

// GetInstanceSource returns the source field of an instance, or empty string if not found.
func (r *ModelRegistry) GetInstanceSource(instanceID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if instance, exists := r.instances[instanceID]; exists {
		return instance.Config.Source
	}
	return ""
}

// RegistryStats represents statistics about the model registry
type RegistryStats struct {
	TotalInstances int            `json:"total_instances"`
	TotalModels    int            `json:"total_models"`
	TotalProviders int            `json:"total_providers"`
	ModelCounts    map[string]int `json:"model_counts"`
}
