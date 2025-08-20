package config

import (
	"strings"
	"time"
)

// ModelConfig represents a model configuration
type ModelConfig struct {
	// Required fields
	ModelName string      `mapstructure:"model_name" json:"model_name"` // User-facing name (e.g., "my-gpt-4")
	Params    ModelParams `mapstructure:"params" json:"params"`         // Provider configuration

	// Optional fields
	RPM      int           `mapstructure:"rpm" json:"rpm"`           // Requests per minute
	TPM      int           `mapstructure:"tpm" json:"tpm"`           // Tokens per minute
	Timeout  time.Duration `mapstructure:"timeout" json:"timeout"`   // Request timeout
	Priority int           `mapstructure:"priority" json:"priority"` // Priority for routing
	Weight   float64       `mapstructure:"weight" json:"weight"`     // Weight for load balancing
	Tags     []string      `mapstructure:"tags" json:"tags"`         // Tags for filtering
	Enabled  *bool         `mapstructure:"enabled" json:"enabled"`   // Default true if not specified
}

// ModelParams contains the provider-specific parameters
type ModelParams struct {
	// Required
	Model string `mapstructure:"model" json:"model"` // Provider model identifier (e.g., "gpt-4", "azure/my-deployment")

	// Optional - Authentication
	APIKey     string `mapstructure:"api_key" json:"api_key"`         // Can be env var reference like ${OPENAI_API_KEY}
	APIBase    string `mapstructure:"api_base" json:"api_base"`       // Base URL for the API
	APIVersion string `mapstructure:"api_version" json:"api_version"` // API version (for Azure)

	// Optional - Request defaults
	Temperature *float32 `mapstructure:"temperature" json:"temperature"`
	MaxTokens   *int     `mapstructure:"max_tokens" json:"max_tokens"`
	TopP        *float32 `mapstructure:"top_p" json:"top_p"`

	// Optional - Organization/Project
	OrgID     string `mapstructure:"org_id" json:"org_id"`
	ProjectID string `mapstructure:"project_id" json:"project_id"`
}

// ConvertToModelInstance converts new config format to internal ModelInstance
func ConvertToModelInstance(cfg ModelConfig) ModelInstance {
	// Detect provider type
	providerType := "openai" // default
	modelName := cfg.Params.Model

	// Check if it's OpenRouter based on API base URL
	if cfg.Params.APIBase != "" && strings.Contains(cfg.Params.APIBase, "openrouter.ai") {
		providerType = "openrouter"
		// Keep the full model name for OpenRouter
		modelName = cfg.Params.Model
	} else if strings.Contains(modelName, "/") {
		// Parse provider type from model string (e.g., "azure/gpt-4" -> provider: azure, model: gpt-4)
		parts := strings.SplitN(modelName, "/", 2)
		providerType = parts[0]
		modelName = parts[1]
	}

	// Set defaults
	enabled := true
	if cfg.Enabled != nil {
		enabled = *cfg.Enabled
	}

	rpm := cfg.RPM
	if rpm == 0 {
		rpm = 100 // Default RPM
	}

	tpm := cfg.TPM
	if tpm == 0 {
		tpm = 100000 // Default TPM
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	priority := cfg.Priority
	if priority == 0 {
		priority = 50 // Medium priority
	}

	weight := cfg.Weight
	if weight == 0 {
		weight = 1.0
	}

	// Build provider params
	provider := ProviderParams{
		Type:       providerType,
		Model:      modelName,
		APIKey:     cfg.Params.APIKey,
		BaseURL:    cfg.Params.APIBase,
		APIVersion: cfg.Params.APIVersion,
		OrgID:      cfg.Params.OrgID,
		ProjectID:  cfg.Params.ProjectID,
	}

	// Handle Azure-specific configuration
	if providerType == "azure" {
		provider.AzureDeployment = modelName
		provider.AzureEndpoint = cfg.Params.APIBase
	}

	// Handle OpenRouter-specific configuration
	if providerType == "openrouter" {
		// For OpenRouter, keep the full model name (e.g., "openai/gpt-4")
		provider.Model = cfg.Params.Model
	}

	// Create ModelInstance
	return ModelInstance{
		ID:             cfg.ModelName, // Use model_name as ID
		ModelName:      cfg.ModelName,
		Provider:       provider,
		RPM:            rpm,
		TPM:            tpm,
		Priority:       priority,
		Weight:         weight,
		Timeout:        timeout,
		Tags:           cfg.Tags,
		Enabled:        enabled,
		MaxRetries:     3,                // Default
		CooldownPeriod: 30 * time.Second, // Default
		ModelInfo: ModelInfo{
			Mode:              "chat",
			SupportsStreaming: true,
			SupportsFunctions: true,
			MaxTokens:         128000, // Default large context
			DefaultMaxTokens:  2000,
		},
	}
}
