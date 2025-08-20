package providers

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/amerfu/pllm/internal/models"
	"go.uber.org/zap"
)

type Manager struct {
	providers map[string]Provider
	mu        sync.RWMutex
	logger    *zap.Logger
}

func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		providers: make(map[string]Provider),
		logger:    logger,
	}
}

func (m *Manager) LoadProviders(configs map[string]ProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// First try to load from configs
	for name, cfg := range configs {
		// Check if the provider has a valid API key
		if cfg.APIKey == "" || cfg.APIKey == "${OPENAI_API_KEY}" {
			// Try to get from environment
			if cfg.Type == "openai" {
				if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" && apiKey != "sk-your-openai-api-key-here" {
					cfg.APIKey = apiKey
					cfg.Enabled = true
				}
			}
		}

		if cfg.APIKey == "" || cfg.APIKey == "sk-your-openai-api-key-here" {
			m.logger.Debug("Skipping provider with invalid API key",
				zap.String("provider", name))
			continue
		}

		provider, err := m.createProvider(name, cfg)
		if err != nil {
			m.logger.Error("Failed to create provider",
				zap.String("provider", name),
				zap.Error(err))
			continue
		}

		m.providers[name] = provider
		m.logger.Info("Loaded provider",
			zap.String("provider", name),
			zap.String("type", cfg.Type),
			zap.Int("models", len(cfg.Models)))
	}

	// If no providers loaded from config, try to load from environment
	if len(m.providers) == 0 {
		m.logger.Info("No providers in config, checking environment variables")
		if err := m.loadProvidersFromEnv(); err != nil {
			m.logger.Warn("Failed to load providers from environment", zap.Error(err))
		}
	}

	if len(m.providers) == 0 {
		m.logger.Warn("No providers loaded. API will not be functional until providers are configured.")
		// Allow server to start anyway for configuration
		// return fmt.Errorf("no providers loaded")
	} else {
		m.logger.Info("Successfully loaded providers", zap.Int("count", len(m.providers)))
	}

	return nil
}

func (m *Manager) createProvider(name string, cfg ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case "openai":
		return NewOpenAIProvider(name, cfg)
	case "anthropic":
		return NewAnthropicProvider(name, cfg)
	case "azure":
		return NewAzureProvider(name, cfg)
	case "bedrock":
		return NewBedrockProvider(name, cfg)
	case "vertex":
		return NewVertexProvider(name, cfg)
	case "cohere":
		// TODO: Implement CohereProvider
		return nil, fmt.Errorf("cohere provider not implemented yet")
	case "huggingface":
		// TODO: Implement HuggingFaceProvider
		return nil, fmt.Errorf("huggingface provider not implemented yet")
	case "openrouter":
		return NewOpenRouterProvider(name, cfg)
	case "custom":
		// TODO: Implement CustomProvider
		return nil, fmt.Errorf("custom provider not implemented yet")
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}

func (m *Manager) GetProvider(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, exists := m.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", name)
	}

	return provider, nil
}

func (m *Manager) GetProviderForModel(model string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Try to find provider that supports this model
	for _, provider := range m.providers {
		if provider.SupportsModel(model) {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no provider found for model: %s", model)
}

func (m *Manager) GetBestProvider(ctx context.Context, request *ChatRequest) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get all healthy providers that support the model
	var candidates []Provider
	for _, provider := range m.providers {
		if provider.IsHealthy() && provider.SupportsModel(request.Model) {
			candidates = append(candidates, provider)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no healthy provider found for model: %s", request.Model)
	}

	// Sort by priority (higher priority first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].GetPriority() > candidates[j].GetPriority()
	})

	// Return the highest priority provider
	return candidates[0], nil
}

func (m *Manager) ListProviders() []ProviderInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var providers []ProviderInfo
	for name, provider := range m.providers {
		providers = append(providers, ProviderInfo{
			Name:      name,
			Type:      provider.GetType(),
			IsHealthy: provider.IsHealthy(),
			Priority:  provider.GetPriority(),
			Models:    provider.ListModels(),
		})
	}

	return providers
}

func (m *Manager) ListModels() []ModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	modelMap := make(map[string]ModelInfo)

	for providerName, provider := range m.providers {
		for _, model := range provider.ListModels() {
			if existing, ok := modelMap[model]; ok {
				existing.Providers = append(existing.Providers, providerName)
				modelMap[model] = existing
			} else {
				modelMap[model] = ModelInfo{
					ID:        model,
					Name:      model,
					Providers: []string{providerName},
				}
			}
		}
	}

	var models []ModelInfo
	for _, model := range modelMap {
		models = append(models, model)
	}

	// Sort models by ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	return models
}

func (m *Manager) AddProvider(name string, provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[name] = provider
	m.logger.Info("Added provider", zap.String("provider", name))
}

func (m *Manager) RemoveProvider(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.providers, name)
	m.logger.Info("Removed provider", zap.String("provider", name))
}

func (m *Manager) UpdateProvider(name string, provider Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.providers[name]; !exists {
		return fmt.Errorf("provider not found: %s", name)
	}

	m.providers[name] = provider
	m.logger.Info("Updated provider", zap.String("provider", name))

	return nil
}

func (m *Manager) HealthCheck(ctx context.Context) map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := make(map[string]bool)

	for name, provider := range m.providers {
		health[name] = provider.IsHealthy()
	}

	return health
}

func (m *Manager) GetProviderFromDB(providerID string) (*models.Provider, error) {
	// TODO: Implement database lookup
	return nil, nil
}

func (m *Manager) LoadProvidersFromDB() error {
	// TODO: Implement loading providers from database
	return nil
}

func (m *Manager) loadProvidersFromEnv() error {
	// Check for OpenAI API keys
	openaiKeys := []string{
		os.Getenv("OPENAI_API_KEY"),
		os.Getenv("OPENAI_API_KEY_1"),
		os.Getenv("OPENAI_API_KEY_2"),
		os.Getenv("OPENAI_API_KEY_3"),
	}

	providersLoaded := 0

	for i, key := range openaiKeys {
		if key != "" && key != "sk-your-openai-api-key" && key != "sk-your-openai-api-key-here" {
			name := fmt.Sprintf("openai-%d", i)
			cfg := ProviderConfig{
				Type:     "openai",
				APIKey:   key,
				BaseURL:  "https://api.openai.com/v1",
				OrgID:    os.Getenv(fmt.Sprintf("OPENAI_ORG_%d", i)),
				Enabled:  true,
				Priority: 10,
				Models:   []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo", "gpt-3.5-turbo-16k"},
			}

			provider, err := NewOpenAIProvider(name, cfg)
			if err != nil {
				m.logger.Error("Failed to create OpenAI provider",
					zap.String("name", name),
					zap.Error(err))
				continue
			}

			m.providers[name] = provider
			providersLoaded++
			m.logger.Info("Loaded OpenAI provider from environment",
				zap.String("name", name))
		}
	}

	// Check for Anthropic API keys
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		anthropicKey = os.Getenv("ANTHROPIC_API_KEY_1")
	}

	if anthropicKey != "" {
		name := "anthropic-0"
		cfg := ProviderConfig{
			Type:     "anthropic",
			APIKey:   anthropicKey,
			BaseURL:  "https://api.anthropic.com",
			Enabled:  true,
			Priority: 10,
			Models:   []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307"},
		}

		provider, err := NewAnthropicProvider(name, cfg)
		if err != nil {
			m.logger.Error("Failed to create Anthropic provider",
				zap.String("name", name),
				zap.Error(err))
		} else {
			m.providers[name] = provider
			providersLoaded++
			m.logger.Info("Loaded Anthropic provider from environment",
				zap.String("name", name))
		}
	}

	// Check for OpenRouter API keys
	openRouterKey := os.Getenv("OPENROUTER_API_KEY")
	if openRouterKey == "" {
		openRouterKey = os.Getenv("OPENROUTER_API_KEY_1")
	}

	if openRouterKey != "" {
		name := "openrouter-0"
		extraConfig := map[string]interface{}{
			"http_referer": os.Getenv("OPENROUTER_HTTP_REFERER"),
			"x_title":      os.Getenv("OPENROUTER_X_TITLE"),
			"app_name":     os.Getenv("OPENROUTER_APP_NAME"),
		}

		cfg := ProviderConfig{
			Type:     "openrouter",
			APIKey:   openRouterKey,
			BaseURL:  "https://openrouter.ai/api/v1",
			Enabled:  true,
			Priority: 10,
			Models: []string{
				"openai/gpt-4-turbo",
				"openai/gpt-4",
				"openai/gpt-3.5-turbo",
				"anthropic/claude-3-opus",
				"anthropic/claude-3-sonnet",
				"anthropic/claude-3-haiku",
				"meta-llama/llama-2-70b-chat",
				"mistralai/mixtral-8x7b-instruct",
				"google/gemini-pro",
			},
			Extra: extraConfig,
		}

		provider, err := NewOpenRouterProvider(name, cfg)
		if err != nil {
			m.logger.Error("Failed to create OpenRouter provider",
				zap.String("name", name),
				zap.Error(err))
		} else {
			m.providers[name] = provider
			providersLoaded++
			m.logger.Info("Loaded OpenRouter provider from environment",
				zap.String("name", name))
		}
	}

	if providersLoaded == 0 {
		return fmt.Errorf("no valid provider API keys found in environment")
	}

	m.logger.Info("Successfully loaded providers from environment",
		zap.Int("count", providersLoaded))
	return nil
}

type ProviderInfo struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	IsHealthy bool     `json:"is_healthy"`
	Priority  int      `json:"priority"`
	Models    []string `json:"models"`
}

type ModelInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Providers []string `json:"providers"`
}
