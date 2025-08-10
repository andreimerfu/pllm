package config

import (
	"time"
)

// ModelInstance represents a single model instance configuration
// Each instance can have its own API key, rate limits, and configuration
type ModelInstance struct {
	// Instance identification
	ID           string `mapstructure:"id" json:"id"`                       // Unique instance ID (auto-generated if not provided)
	ModelName    string `mapstructure:"model_name" json:"model_name"`       // User-facing model name (e.g., "gpt-4")
	InstanceName string `mapstructure:"instance_name" json:"instance_name"` // Optional instance name

	// Provider configuration
	Provider ProviderParams `mapstructure:"provider" json:"provider"`
	
	// Model information
	ModelInfo ModelInfo `mapstructure:"model_info" json:"model_info"`
	
	// Rate limiting (per deployment)
	RPM int `mapstructure:"rpm" json:"rpm"` // Requests per minute
	TPM int `mapstructure:"tpm" json:"tpm"` // Tokens per minute
	
	// Load balancing configuration
	Priority int     `mapstructure:"priority" json:"priority"` // Higher priority = preferred (1-100)
	Weight   float64 `mapstructure:"weight" json:"weight"`     // Weight for weighted round-robin
	
	// Health and retry configuration
	MaxRetries      int           `mapstructure:"max_retries" json:"max_retries"`
	Timeout         time.Duration `mapstructure:"timeout" json:"timeout"`
	CooldownPeriod  time.Duration `mapstructure:"cooldown_period" json:"cooldown_period"` // After failure
	
	// Cost tracking
	InputCostPerToken  float64 `mapstructure:"input_cost_per_token" json:"input_cost_per_token"`
	OutputCostPerToken float64 `mapstructure:"output_cost_per_token" json:"output_cost_per_token"`
	
	// Custom headers
	CustomHeaders map[string]string `mapstructure:"custom_headers" json:"custom_headers"`
	
	// Tags for filtering and grouping
	Tags []string `mapstructure:"tags" json:"tags"`
	
	// Enabled flag
	Enabled bool `mapstructure:"enabled" json:"enabled"`
}

// ProviderParams contains provider-specific parameters
type ProviderParams struct {
	// Provider type and model
	Type  string `mapstructure:"type" json:"type"`   // "openai", "anthropic", "azure", "bedrock", "vertex", etc.
	Model string `mapstructure:"model" json:"model"` // Actual model identifier (e.g., "gpt-4-turbo-preview")
	
	// Authentication
	APIKey    string `mapstructure:"api_key" json:"api_key"`
	APISecret string `mapstructure:"api_secret" json:"api_secret"` // For providers that need both
	
	// Endpoints
	BaseURL    string `mapstructure:"base_url" json:"base_url"`       // Base URL
	APIVersion string `mapstructure:"api_version" json:"api_version"` // API version
	
	// Organization/Project
	OrgID     string `mapstructure:"org_id" json:"org_id"`
	ProjectID string `mapstructure:"project_id" json:"project_id"`
	
	// Region/Location
	Region   string `mapstructure:"region" json:"region"`     // AWS/Azure region
	Location string `mapstructure:"location" json:"location"` // GCP location
	
	// Azure specific
	AzureDeployment string `mapstructure:"azure_deployment" json:"azure_deployment"`
	AzureEndpoint   string `mapstructure:"azure_endpoint" json:"azure_endpoint"`
	
	// AWS Bedrock specific
	AWSAccessKeyID     string `mapstructure:"aws_access_key_id" json:"aws_access_key_id"`
	AWSSecretAccessKey string `mapstructure:"aws_secret_access_key" json:"aws_secret_access_key"`
	AWSRegionName      string `mapstructure:"aws_region_name" json:"aws_region_name"`
	
	// Vertex AI specific
	VertexProject  string `mapstructure:"vertex_project" json:"vertex_project"`
	VertexLocation string `mapstructure:"vertex_location" json:"vertex_location"`
}

// ModelInfo contains model capabilities and metadata
type ModelInfo struct {
	Mode            string   `mapstructure:"mode" json:"mode"`                         // "chat", "completion", "embedding", "image", "audio"
	SupportsFunctions bool   `mapstructure:"supports_functions" json:"supports_functions"`
	SupportsVision  bool     `mapstructure:"supports_vision" json:"supports_vision"`
	SupportsStreaming bool   `mapstructure:"supports_streaming" json:"supports_streaming"`
	MaxTokens       int      `mapstructure:"max_tokens" json:"max_tokens"`             // Model's context window
	MaxInputTokens  int      `mapstructure:"max_input_tokens" json:"max_input_tokens"`
	MaxOutputTokens int      `mapstructure:"max_output_tokens" json:"max_output_tokens"`
	DefaultMaxTokens int     `mapstructure:"default_max_tokens" json:"default_max_tokens"`
	SupportedLanguages []string `mapstructure:"supported_languages" json:"supported_languages"`
}

// RouterSettings contains load balancing and routing configuration
type RouterSettings struct {
	RoutingStrategy    string        `mapstructure:"routing_strategy" json:"routing_strategy"`       // "simple", "least-busy", "usage-based", "latency-based", "priority", "weighted"
	AllowedFailures    int           `mapstructure:"allowed_failures" json:"allowed_failures"`       // Before marking unhealthy
	FallbackModels     []string      `mapstructure:"fallback_models" json:"fallback_models"`         // Model names to fallback to
	CacheTTL           time.Duration `mapstructure:"cache_ttl" json:"cache_ttl"`                     // Cache duration
	DefaultTimeout     time.Duration `mapstructure:"default_timeout" json:"default_timeout"`
	MaxRetries         int           `mapstructure:"max_retries" json:"max_retries"`
	EnableLoadBalancing bool         `mapstructure:"enable_load_balancing" json:"enable_load_balancing"`
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval" json:"health_check_interval"`
}

// ModelGroup represents a logical grouping of model instances
type ModelGroup struct {
	Name         string   `mapstructure:"name" json:"name"`
	Description  string   `mapstructure:"description" json:"description"`
	Models       []string `mapstructure:"models" json:"models"`       // Model instance IDs
	DefaultModel string   `mapstructure:"default_model" json:"default_model"`
	Tags         []string `mapstructure:"tags" json:"tags"`
}