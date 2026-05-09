package models

import (
	"time"

	"github.com/google/uuid"
)

// DiscoveredCallKind classifies the target of an observed span.
type DiscoveredCallKind string

const (
	DiscoveredCallKindLLM      DiscoveredCallKind = "llm"
	DiscoveredCallKindMCP      DiscoveredCallKind = "mcp"
	DiscoveredCallKindVectorDB DiscoveredCallKind = "vector_db"
	DiscoveredCallKindHTTP     DiscoveredCallKind = "http"
	DiscoveredCallKindUnknown  DiscoveredCallKind = "unknown"
)

// DiscoveredCall is a deduplicated rollup of (agent, target) edges
// observed in OTel spans. We don't persist every span — we count.
type DiscoveredCall struct {
	BaseModel

	DiscoveredAgentID uuid.UUID `gorm:"type:uuid;index;not null" json:"discovered_agent_id"`

	Kind       DiscoveredCallKind `gorm:"size:16;index;not null" json:"kind"`
	TargetName string             `gorm:"size:512;index;not null" json:"target_name"`

	Count       int       `gorm:"default:1;not null" json:"count"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`

	// Outcome of the most recent policy evaluation; "" if not yet evaluated.
	LastOutcome string `gorm:"size:16" json:"last_outcome,omitempty"`
	LastReason  string `gorm:"type:text" json:"last_reason,omitempty"`
}

func (DiscoveredCall) TableName() string { return "discovered_calls" }
