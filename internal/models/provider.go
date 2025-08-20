package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Provider represents a unique provider instance (e.g., different API keys, regions, or accounts)
type Provider struct {
	BaseModel
	Name     string       `gorm:"uniqueIndex;not null" json:"name"`
	Alias    string       `json:"alias"` // User-friendly name for this deployment
	Type     ProviderType `gorm:"not null" json:"type"`
	IsActive bool         `gorm:"default:true" json:"is_active"`
	Priority int          `gorm:"default:0" json:"priority"`
	Weight   int          `gorm:"default:1" json:"weight"` // For weighted load balancing

	// Configuration
	Config datatypes.JSON `json:"config,omitempty"`

	// Credentials (encrypted)
	APIKey    string `gorm:"type:text" json:"-"`
	APISecret string `gorm:"type:text" json:"-"`
	Endpoint  string `json:"endpoint,omitempty"`
	Region    string `json:"region,omitempty"`

	// Rate Limiting (per deployment)
	RPM             int `gorm:"default:60" json:"rpm"`     // Requests per minute
	TPM             int `gorm:"default:100000" json:"tpm"` // Tokens per minute
	ConcurrentLimit int `gorm:"default:10" json:"concurrent_limit"`

	// Current usage tracking
	CurrentRPM     int `gorm:"-" json:"current_rpm"`
	CurrentTPM     int `gorm:"-" json:"current_tpm"`
	ActiveRequests int `gorm:"-" json:"active_requests"`

	// Health
	IsHealthy       bool    `gorm:"default:true" json:"is_healthy"`
	LastHealthCheck string  `json:"last_health_check,omitempty"`
	ErrorRate       float64 `gorm:"default:0" json:"error_rate"`
	AvgLatency      float64 `gorm:"default:0" json:"avg_latency"`
	FailureCount    int     `gorm:"default:0" json:"failure_count"`
	CooldownUntil   *string `json:"cooldown_until,omitempty"`

	// Cost Configuration
	InputCostPer1K  float64 `gorm:"default:0" json:"input_cost_per_1k"`
	OutputCostPer1K float64 `gorm:"default:0" json:"output_cost_per_1k"`

	// Supported Models
	SupportedModels []Model `gorm:"foreignKey:ProviderID" json:"supported_models,omitempty"`

	// Model Groups (for load balancing)
	ModelGroup string `gorm:"index" json:"model_group,omitempty"`

	// Metadata
	Metadata map[string]interface{} `gorm:"type:jsonb" json:"metadata,omitempty"`
}

type ProviderType string

const (
	ProviderOpenAI      ProviderType = "openai"
	ProviderAnthropic   ProviderType = "anthropic"
	ProviderAzure       ProviderType = "azure"
	ProviderBedrock     ProviderType = "bedrock"
	ProviderVertexAI    ProviderType = "vertex"
	ProviderCohere      ProviderType = "cohere"
	ProviderHuggingFace ProviderType = "huggingface"
	ProviderReplicate   ProviderType = "replicate"
	ProviderGroq        ProviderType = "groq"
	ProviderTogether    ProviderType = "together"
	ProviderPerplexity  ProviderType = "perplexity"
	ProviderMistral     ProviderType = "mistral"
	ProviderCustom      ProviderType = "custom"
)

type Model struct {
	BaseModel
	Name          string  `gorm:"not null;index" json:"name"`
	DisplayName   string  `json:"display_name"`
	ModelGroup    string  `gorm:"index" json:"model_group"` // For grouping same models across providers
	IsActive      bool    `gorm:"default:true" json:"is_active"`
	MaxTokens     int     `gorm:"default:4096" json:"max_tokens"`
	ContextWindow int     `gorm:"default:4096" json:"context_window"`
	InputCost     float64 `json:"input_cost"`
	OutputCost    float64 `json:"output_cost"`

	// Provider
	ProviderID uuid.UUID `gorm:"type:uuid;not null" json:"provider_id"`
	Provider   Provider  `gorm:"foreignKey:ProviderID" json:"-"`

	// Capabilities
	SupportStreaming bool `gorm:"default:true" json:"support_streaming"`
	SupportFunctions bool `gorm:"default:false" json:"support_functions"`
	SupportVision    bool `gorm:"default:false" json:"support_vision"`
	SupportTools     bool `gorm:"default:false" json:"support_tools"`

	// Configuration
	Config datatypes.JSON `json:"config,omitempty"`

	// Metadata
	Metadata map[string]interface{} `gorm:"type:jsonb" json:"metadata,omitempty"`
}

// ProviderDeployment represents a specific deployment configuration
// This is loaded from config file and stored in DB
type ProviderDeployment struct {
	BaseModel
	Name         string `gorm:"uniqueIndex;not null" json:"name"`
	ModelName    string `gorm:"index" json:"model_name"` // The model alias (e.g., "gpt-4")
	LiteLLMModel string `json:"litellm_model"`           // The actual model (e.g., "azure/gpt-4-deployment")

	// Provider details
	ProviderType ProviderType `json:"provider_type"`
	APIBase      string       `json:"api_base,omitempty"`
	APIKey       string       `json:"-"`
	APIVersion   string       `json:"api_version,omitempty"`
	Region       string       `json:"region,omitempty"`
	Organization string       `json:"organization,omitempty"`
	Project      string       `json:"project,omitempty"`

	// Rate limits
	RPM int `json:"rpm,omitempty"`
	TPM int `json:"tpm,omitempty"`

	// Load balancing
	Priority int `json:"priority,omitempty"`
	Weight   int `json:"weight,omitempty"`

	// Additional params
	Temperature   *float32               `json:"temperature,omitempty"`
	MaxTokens     *int                   `json:"max_tokens,omitempty"`
	TopP          *float32               `json:"top_p,omitempty"`
	StreamTimeout int                    `json:"stream_timeout,omitempty"`
	MaxRetries    int                    `json:"max_retries,omitempty"`
	CustomHeaders map[string]string      `json:"custom_headers,omitempty"`
	ExtraParams   map[string]interface{} `json:"extra_params,omitempty"`

	// Status
	IsActive  bool `gorm:"default:true" json:"is_active"`
	IsHealthy bool `gorm:"default:true" json:"is_healthy"`
}

type ProviderConfig struct {
	// OpenAI
	OpenAIOrg     string `json:"openai_org,omitempty"`
	OpenAIBaseURL string `json:"openai_base_url,omitempty"`

	// Azure
	AzureDeployment string `json:"azure_deployment,omitempty"`
	AzureAPIVersion string `json:"azure_api_version,omitempty"`
	AzureEndpoint   string `json:"azure_endpoint,omitempty"`

	// AWS Bedrock
	AWSRegion       string `json:"aws_region,omitempty"`
	AWSAccessKey    string `json:"aws_access_key,omitempty"`
	AWSSecretKey    string `json:"aws_secret_key,omitempty"`
	AWSSessionToken string `json:"aws_session_token,omitempty"`

	// Google Vertex AI
	VertexProject  string `json:"vertex_project,omitempty"`
	VertexLocation string `json:"vertex_location,omitempty"`

	// Anthropic
	AnthropicVersion string `json:"anthropic_version,omitempty"`

	// Custom
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`
	Timeout       int               `json:"timeout,omitempty"`
	RetryAttempts int               `json:"retry_attempts,omitempty"`
	RetryDelay    int               `json:"retry_delay,omitempty"`
}

// LoadBalancingStrategy defines how to select providers
type LoadBalancingStrategy string

const (
	StrategyRoundRobin    LoadBalancingStrategy = "round-robin"
	StrategyLeastBusy     LoadBalancingStrategy = "least-busy"
	StrategyWeighted      LoadBalancingStrategy = "weighted"
	StrategyPriority      LoadBalancingStrategy = "priority"
	StrategyLatencyBased  LoadBalancingStrategy = "latency-based"
	StrategyUsageBased    LoadBalancingStrategy = "usage-based"
	StrategySimpleShuffle LoadBalancingStrategy = "simple-shuffle"
	StrategyHealthFirst   LoadBalancingStrategy = "health-first"
)

// RouterSettings for load balancing configuration
type RouterSettings struct {
	BaseModel
	RoutingStrategy        LoadBalancingStrategy `json:"routing_strategy"`
	NumRetries             int                   `json:"num_retries"`
	Timeout                int                   `json:"timeout"`
	AllowedFails           int                   `json:"allowed_fails"`
	CooldownTime           int                   `json:"cooldown_time"`
	Fallbacks              map[string][]string   `json:"fallbacks"`
	ContextWindowFallbacks map[string][]string   `json:"context_window_fallbacks"`
	ModelGroupAlias        map[string]string     `json:"model_group_alias"`
	RedisHost              string                `json:"redis_host,omitempty"`
	RedisPassword          string                `json:"-"`
	RedisPort              int                   `json:"redis_port,omitempty"`
	RedisDB                int                   `json:"redis_db,omitempty"`
}
