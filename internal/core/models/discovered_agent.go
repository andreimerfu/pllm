package models

import (
	"time"

	"github.com/google/uuid"
)

// DiscoveredAgentStatus is the operator's lifecycle for an agent
// observed via OTel but not yet registered in the catalog.
type DiscoveredAgentStatus string

const (
	DiscoveredStatusPending  DiscoveredAgentStatus = "pending"
	DiscoveredStatusPromoted DiscoveredAgentStatus = "promoted"
	DiscoveredStatusIgnored  DiscoveredAgentStatus = "ignored"
)

// DiscoveredAgent represents an agent fingerprint that pllm has seen
// in OpenTelemetry traffic but that has not yet been published to the
// `RegistryAgent` catalog. Identity is keyed by service.name (the
// OTel resource attribute) plus the API key the agent uses for
// pllm-mediated traffic, when known.
type DiscoveredAgent struct {
	BaseModel

	// ServiceName is the OTel resource `service.name`. Required.
	ServiceName string `gorm:"size:255;uniqueIndex;not null" json:"service_name"`

	// ServiceVersion is the OTel resource `service.version`, if present.
	ServiceVersion string `gorm:"size:64" json:"service_version,omitempty"`

	// KeyID is the pllm API key that the agent attaches to OTel spans
	// via the `pllm.key_id` resource attribute. Optional — agents that
	// only call non-pllm targets won't have one. When set, governance
	// is derived from the bound key's UseCase.
	KeyID *uuid.UUID `gorm:"type:uuid;index" json:"key_id,omitempty"`

	// PromotedAgentID points at the RegistryAgent the operator created
	// from this discovery, once promoted.
	PromotedAgentID *uuid.UUID `gorm:"type:uuid" json:"promoted_agent_id,omitempty"`

	Status DiscoveredAgentStatus `gorm:"size:16;index;default:pending;not null" json:"status"`

	FirstSeenAt time.Time  `json:"first_seen_at"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	IgnoredAt   *time.Time `json:"ignored_at,omitempty"`
	PromotedAt  *time.Time `json:"promoted_at,omitempty"`

	// FlaggedCount is incremented every time policy evaluation produces
	// a flagged outcome. The revoker reads this; the API exposes it.
	FlaggedCount int `gorm:"default:0;not null" json:"flagged_count"`
}

func (DiscoveredAgent) TableName() string { return "discovered_agents" }
