package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// ProviderProfile stores reusable provider credentials.
// Multiple UserModels can reference a single profile.
type ProviderProfile struct {
	BaseModel
	Name   string                    `gorm:"uniqueIndex;not null" json:"name"`
	Type   string                    `gorm:"not null;index" json:"type"`
	Config ProviderProfileConfigJSON `gorm:"type:jsonb;not null" json:"config"`
}

// ProviderProfileConfigJSON stores provider-specific credentials as JSONB.
type ProviderProfileConfigJSON struct {
	APIKey             string `json:"api_key,omitempty"`
	BaseURL            string `json:"base_url,omitempty"`
	OAuthToken         string `json:"oauth_token,omitempty"`
	AzureEndpoint      string `json:"azure_endpoint,omitempty"`
	AzureDeployment    string `json:"azure_deployment,omitempty"`
	APIVersion         string `json:"api_version,omitempty"`
	AWSRegionName      string `json:"aws_region_name,omitempty"`
	AWSAccessKeyID     string `json:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string `json:"aws_secret_access_key,omitempty"`
	VertexProject      string `json:"vertex_project,omitempty"`
	VertexLocation     string `json:"vertex_location,omitempty"`
}

func (c ProviderProfileConfigJSON) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *ProviderProfileConfigJSON) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ProviderProfileConfigJSON: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, c)
}
