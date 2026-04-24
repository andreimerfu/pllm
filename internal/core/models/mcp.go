package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// MCP transport types.
const (
	MCPTransportStdio = "stdio"
	MCPTransportSSE   = "sse"
	MCPTransportHTTP  = "http"
)

// MCP health states.
const (
	MCPHealthUnknown   = "unknown"
	MCPHealthHealthy   = "healthy"
	MCPHealthUnhealthy = "unhealthy"
)

// MCPServer is a registered backend MCP server the gateway proxies to.
type MCPServer struct {
	BaseModel

	// Human-facing identity.
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Slug        string `gorm:"uniqueIndex;not null" json:"slug"`
	Description string `json:"description"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`

	// Ownership: team scope for visibility. Nil = global (master key / admin only).
	TeamID *uuid.UUID `gorm:"type:uuid;index" json:"team_id,omitempty"`

	// Transport: "stdio" | "sse" | "http".
	Transport string `gorm:"not null" json:"transport"`

	// Remote backends: URL + optional headers (jsonb: map[string]string).
	Endpoint string         `json:"endpoint,omitempty"`
	Headers  datatypes.JSON `json:"headers,omitempty"`

	// Stdio backends: spawned process.
	Command string      `json:"command,omitempty"`
	Args    StringArray `gorm:"type:text[]" json:"args,omitempty"`
	// Env is stored encrypted at rest (caller responsibility for encrypt/decrypt).
	Env datatypes.JSON `json:"env,omitempty"`
	// WorkingDir for stdio processes (optional).
	WorkingDir string `json:"working_dir,omitempty"`

	// Observability.
	HealthStatus string                `gorm:"default:'unknown'" json:"health_status"`
	LastError    string                `json:"last_error,omitempty"`
	LastSeenAt *time.Time      `json:"last_seen_at,omitempty"`
	Metadata   datatypes.JSON  `json:"metadata,omitempty"`
	Tools      []MCPServerTool `gorm:"foreignKey:MCPServerID" json:"tools,omitempty"`
}

// MCPServerTool is a cached view of a tool exposed by a backend server.
// Refreshed on health check / (re)connect so /tools/list is O(1) across backends.
type MCPServerTool struct {
	BaseModel

	MCPServerID uuid.UUID      `gorm:"type:uuid;index;not null" json:"mcp_server_id"`
	Name        string         `gorm:"index;not null" json:"name"`
	Description string         `json:"description"`
	InputSchema datatypes.JSON `json:"input_schema,omitempty"`
}

// MCPRoute groups backends under a slug for routed access (future: pin | round-robin | failover).
// Phase 1 uses a single target; the schema is forward-compatible for load balancing.
type MCPRoute struct {
	BaseModel

	Slug        string         `gorm:"uniqueIndex;not null" json:"slug"`
	Description string         `json:"description"`
	Enabled     bool           `gorm:"default:true" json:"enabled"`
	Strategy    string         `gorm:"default:'pin'" json:"strategy"`
	Targets     datatypes.JSON `json:"targets"` // []uuid.UUID serialized
}
