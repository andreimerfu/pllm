package config

import "time"

// GuardrailsConfig holds all guardrails configuration
type GuardrailsConfig struct {
	Enabled    bool              `mapstructure:"enabled"`
	Guardrails []GuardrailConfig `mapstructure:"guardrails"`
	Providers  ProviderConfigs   `mapstructure:"providers"`
}

// GuardrailConfig defines a single guardrail rule
type GuardrailConfig struct {
	Name        string                 `mapstructure:"guardrail_name"`
	Provider    string                 `mapstructure:"provider"`
	Mode        []string               `mapstructure:"mode"` // pre_call, post_call, during_call, logging_only
	Enabled     bool                   `mapstructure:"enabled"`
	DefaultOn   bool                   `mapstructure:"default_on"`   // Apply to all requests by default
	Config      map[string]interface{} `mapstructure:"config"`       // Provider-specific config
	APIKey      string                 `mapstructure:"api_key"`      // Provider API key
	APIBase     string                 `mapstructure:"api_base"`     // Provider base URL
	Timeout     time.Duration          `mapstructure:"timeout"`      // Request timeout
	RetryConfig RetryConfig            `mapstructure:"retry"`        // Retry configuration
}

// ProviderConfigs holds provider-specific global configuration
type ProviderConfigs struct {
	Presidio PresidioConfig `mapstructure:"presidio"`
	Lakera   LakeraConfig   `mapstructure:"lakera"`
	OpenAI   OpenAIConfig   `mapstructure:"openai"`
	Aporia   AporiaConfig   `mapstructure:"aporia"`
}

// Provider-specific configurations
type PresidioConfig struct {
	AnalyzerURL   string        `mapstructure:"analyzer_url"`
	AnonymizerURL string        `mapstructure:"anonymizer_url"`
	Language      string        `mapstructure:"language"`
	Timeout       time.Duration `mapstructure:"timeout"`
}

type LakeraConfig struct {
	APIKey  string        `mapstructure:"api_key"`
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type OpenAIConfig struct {
	APIKey  string        `mapstructure:"api_key"`
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type AporiaConfig struct {
	APIKey  string        `mapstructure:"api_key"`
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type RetryConfig struct {
	MaxRetries int           `mapstructure:"max_retries"`
	InitialDelay time.Duration `mapstructure:"initial_delay"`
	MaxDelay     time.Duration `mapstructure:"max_delay"`
}

// GuardrailMode represents execution modes
type GuardrailMode string

const (
	PreCall     GuardrailMode = "pre_call"
	PostCall    GuardrailMode = "post_call"
	DuringCall  GuardrailMode = "during_call"
	LoggingOnly GuardrailMode = "logging_only"
)

// IsValidMode checks if the mode is valid
func (m GuardrailMode) IsValid() bool {
	switch m {
	case PreCall, PostCall, DuringCall, LoggingOnly:
		return true
	default:
		return false
	}
}

// Default configurations
func DefaultGuardrailsConfig() GuardrailsConfig {
	return GuardrailsConfig{
		Enabled:    false,
		Guardrails: []GuardrailConfig{},
		Providers: ProviderConfigs{
			Presidio: PresidioConfig{
				AnalyzerURL:   "http://localhost:5002",
				AnonymizerURL: "http://localhost:5001",
				Language:      "en",
				Timeout:       10 * time.Second,
			},
			Lakera: LakeraConfig{
				BaseURL: "https://api.lakera.ai",
				Timeout: 5 * time.Second,
			},
			OpenAI: OpenAIConfig{
				BaseURL: "https://api.openai.com/v1",
				Timeout: 30 * time.Second,
			},
			Aporia: AporiaConfig{
				Timeout: 10 * time.Second,
			},
		},
	}
}