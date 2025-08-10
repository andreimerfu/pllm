package providers

import "time"

// ProviderConfig contains configuration for a provider
type ProviderConfig struct {
	Type       string
	APIKey     string
	APISecret  string
	BaseURL    string
	APIVersion string
	OrgID      string
	Region     string
	Enabled    bool
	Priority   int
	Models     []string
	Timeout    time.Duration
}