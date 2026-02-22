package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// UserModel stores user-created model configurations in PostgreSQL.
// Models loaded from config.yaml are "system" models and are not stored here.
type UserModel struct {
	BaseModel

	// Model identification
	ModelName    string `gorm:"uniqueIndex;not null" json:"model_name"`
	InstanceName string `json:"instance_name,omitempty"`

	// Provider configuration stored as JSONB
	ProviderConfig ProviderConfigJSON `gorm:"type:jsonb;not null" json:"provider_config"`

	// Model info stored as JSONB
	ModelInfoConfig ModelInfoJSON `gorm:"type:jsonb" json:"model_info_config"`

	// Rate limiting
	RPM int `json:"rpm"`
	TPM int `json:"tpm"`

	// Load balancing
	Priority int     `json:"priority"`
	Weight   float64 `json:"weight"`

	// Cost tracking
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`

	// Timeout in seconds (stored as int, converted to time.Duration at runtime)
	TimeoutSeconds int `json:"timeout_seconds"`

	// Tags stored as JSONB
	Tags StringArrayJSON `gorm:"type:jsonb" json:"tags"`

	// Status
	Enabled bool `gorm:"default:true" json:"enabled"`

	// Audit
	CreatedByID *uuid.UUID `gorm:"type:uuid" json:"created_by_id,omitempty"`
}

// TableName overrides the default table name
func (UserModel) TableName() string {
	return "user_models"
}

// ProviderConfigJSON is a JSONB wrapper for provider configuration
type ProviderConfigJSON struct {
	Type               string `json:"type"`
	Model              string `json:"model"`
	APIKey             string `json:"api_key,omitempty"`
	APISecret          string `json:"api_secret,omitempty"`
	BaseURL            string `json:"base_url,omitempty"`
	APIVersion         string `json:"api_version,omitempty"`
	OrgID              string `json:"org_id,omitempty"`
	ProjectID          string `json:"project_id,omitempty"`
	Region             string `json:"region,omitempty"`
	Location           string `json:"location,omitempty"`
	AzureDeployment    string `json:"azure_deployment,omitempty"`
	AzureEndpoint      string `json:"azure_endpoint,omitempty"`
	AWSAccessKeyID     string `json:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string `json:"aws_secret_access_key,omitempty"`
	AWSRegionName      string `json:"aws_region_name,omitempty"`
	VertexProject      string `json:"vertex_project,omitempty"`
	VertexLocation     string `json:"vertex_location,omitempty"`
	ReasoningEffort    string `json:"reasoning_effort,omitempty"`
}

// Scan implements the sql.Scanner interface for JSONB
func (p *ProviderConfigJSON) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ProviderConfigJSON: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, p)
}

// Value implements the driver.Valuer interface for JSONB
func (p ProviderConfigJSON) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// ModelInfoJSON is a JSONB wrapper for model info configuration
type ModelInfoJSON struct {
	Mode               string   `json:"mode,omitempty"`
	SupportsFunctions  bool     `json:"supports_functions,omitempty"`
	SupportsVision     bool     `json:"supports_vision,omitempty"`
	SupportsStreaming  bool     `json:"supports_streaming,omitempty"`
	MaxTokens          int      `json:"max_tokens,omitempty"`
	MaxInputTokens     int      `json:"max_input_tokens,omitempty"`
	MaxOutputTokens    int      `json:"max_output_tokens,omitempty"`
	DefaultMaxTokens   int      `json:"default_max_tokens,omitempty"`
	SupportedLanguages []string `json:"supported_languages,omitempty"`
}

// Scan implements the sql.Scanner interface for JSONB
func (m *ModelInfoJSON) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ModelInfoJSON: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, m)
}

// Value implements the driver.Valuer interface for JSONB
func (m ModelInfoJSON) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// StringArrayJSON is a JSONB wrapper for string arrays
type StringArrayJSON []string

// Scan implements the sql.Scanner interface for JSONB
func (s *StringArrayJSON) Scan(value interface{}) error {
	if value == nil {
		*s = StringArrayJSON{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan StringArrayJSON: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, s)
}

// Value implements the driver.Valuer interface for JSONB
func (s StringArrayJSON) Value() (driver.Value, error) {
	if s == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(s)
}
