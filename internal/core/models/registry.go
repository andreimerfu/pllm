package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// RegistryKind enumerates the four artifact types stored in the registry.
// Kept as strings for easy persistence / JSON output.
type RegistryKind string

const (
	RegistryKindServer  RegistryKind = "server"
	RegistryKindAgent   RegistryKind = "agent"
	RegistryKindSkill   RegistryKind = "skill"
	RegistryKindPrompt  RegistryKind = "prompt"
	// RegistryKindLLM refers to a model onboarded via the existing
	// `user_models` / config `model_list` pipeline. Edges to this kind
	// resolve by model name — no dedicated registry table.
	RegistryKindLLM RegistryKind = "llm"
	// RegistryKindUseCase is used as OwnerKind on edges that originate
	// from a UseCase declaring its component dependencies.
	RegistryKindUseCase RegistryKind = "use_case"
)

// IsValidTargetKind reports whether v can appear as a RegistryRef.TargetKind.
// Used by services that accept user-supplied ref lists so invalid kinds
// don't silently accumulate in the graph.
func IsValidTargetKind(v RegistryKind) bool {
	switch v {
	case RegistryKindServer, RegistryKindAgent, RegistryKindSkill,
		RegistryKindPrompt, RegistryKindLLM:
		return true
	}
	return false
}

// IsValidOwnerKind reports whether v can appear as RegistryRef.OwnerKind.
// A use case can own edges (it "consumes" components); so can an agent
// (composition / delegation).
func IsValidOwnerKind(v RegistryKind) bool {
	switch v {
	case RegistryKindAgent, RegistryKindUseCase:
		return true
	}
	return false
}

// RegistryStatus marks the visibility / lifecycle of a version.
type RegistryStatus string

const (
	RegistryStatusActive     RegistryStatus = "active"
	RegistryStatusDeprecated RegistryStatus = "deprecated"
	RegistryStatusDeleted    RegistryStatus = "deleted"
	// Blocked is a non-deletable, machine-set state used by the lifecycle
	// daemon when a server's vulnerability score falls below the configured
	// floor. Operators clear it manually after triage.
	RegistryStatusBlocked RegistryStatus = "blocked"
)

// --- Servers --------------------------------------------------------------

// RegistryServer is a catalog entry for an MCP server. Packages describe
// how to install it (npm, pypi, oci); Remotes describe any hosted endpoints.
// One row per (Name, Version).
type RegistryServer struct {
	BaseModel

	Name        string         `gorm:"uniqueIndex:idx_reg_server_name_version;not null" json:"name"`
	Version     string         `gorm:"uniqueIndex:idx_reg_server_name_version;not null" json:"version"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	Status      RegistryStatus `gorm:"default:'active'" json:"status"`
	IsLatest    bool           `gorm:"index" json:"is_latest"`

	WebsiteURL   string         `json:"website_url,omitempty"`
	Repository   datatypes.JSON `json:"repository,omitempty"` // {type, url, path}
	Packages     datatypes.JSON `json:"packages,omitempty"`   // [{registry_type, identifier, version, transport}]
	Remotes      datatypes.JSON `json:"remotes,omitempty"`    // [{type, url, headers}]
	Metadata     datatypes.JSON `json:"metadata,omitempty"`

	// Optional owner (publisher). Nil = global / master-key.
	PublishedByUserID *uuid.UUID `gorm:"type:uuid;index" json:"published_by_user_id,omitempty"`
	PublishedAt       time.Time  `json:"published_at"`

	// Federation provenance — set on rows pulled from a peer registry.
	// Empty SourcePeerID = locally published.
	SourcePeerID        string     `gorm:"index" json:"source_peer_id,omitempty"`
	FederatedGeneration int        `json:"federated_generation,omitempty"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
}

// --- Agents ---------------------------------------------------------------

// RegistryAgent is an agent manifest + dependency references. The deps
// themselves (MCP servers, skills, prompts) live in RegistryRef rows.
type RegistryAgent struct {
	BaseModel

	Name        string         `gorm:"uniqueIndex:idx_reg_agent_name_version;not null" json:"name"`
	Version     string         `gorm:"uniqueIndex:idx_reg_agent_name_version;not null" json:"version"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	Status      RegistryStatus `gorm:"default:'active'" json:"status"`
	IsLatest    bool           `gorm:"index" json:"is_latest"`

	// Execution hints (all optional; agent runtime interprets them).
	Language      string `json:"language,omitempty"`
	Framework     string `json:"framework,omitempty"`
	ModelProvider string `json:"model_provider,omitempty"`
	ModelName     string `json:"model_name,omitempty"`

	WebsiteURL string         `json:"website_url,omitempty"`
	Repository datatypes.JSON `json:"repository,omitempty"`
	Packages   datatypes.JSON `json:"packages,omitempty"`
	Remotes    datatypes.JSON `json:"remotes,omitempty"`
	Metadata   datatypes.JSON `json:"metadata,omitempty"`

	PublishedByUserID *uuid.UUID `gorm:"type:uuid;index" json:"published_by_user_id,omitempty"`
	PublishedAt       time.Time  `json:"published_at"`

	SourcePeerID        string     `gorm:"index" json:"source_peer_id,omitempty"`
	FederatedGeneration int        `json:"federated_generation,omitempty"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
}

// --- Skills ---------------------------------------------------------------

// RegistrySkill is a bundle of knowledge (SKILL.md + assets).
// The bundle itself is pulled from Image (OCI) or a registry URL.
type RegistrySkill struct {
	BaseModel

	Name        string         `gorm:"uniqueIndex:idx_reg_skill_name_version;not null" json:"name"`
	Version     string         `gorm:"uniqueIndex:idx_reg_skill_name_version;not null" json:"version"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	Status      RegistryStatus `gorm:"default:'active'" json:"status"`
	IsLatest    bool           `gorm:"index" json:"is_latest"`

	Image    string         `json:"image,omitempty"`       // oci ref
	Manifest datatypes.JSON `json:"manifest,omitempty"`    // SKILL.md + entries
	Metadata datatypes.JSON `json:"metadata,omitempty"`

	PublishedByUserID *uuid.UUID `gorm:"type:uuid;index" json:"published_by_user_id,omitempty"`
	PublishedAt       time.Time  `json:"published_at"`

	SourcePeerID        string     `gorm:"index" json:"source_peer_id,omitempty"`
	FederatedGeneration int        `json:"federated_generation,omitempty"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
}

// --- Prompts --------------------------------------------------------------

// RegistryPrompt is a reusable prompt template.
type RegistryPrompt struct {
	BaseModel

	Name        string         `gorm:"uniqueIndex:idx_reg_prompt_name_version;not null" json:"name"`
	Version     string         `gorm:"uniqueIndex:idx_reg_prompt_name_version;not null" json:"version"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	Status      RegistryStatus `gorm:"default:'active'" json:"status"`
	IsLatest    bool           `gorm:"index" json:"is_latest"`

	Template  string         `json:"template"`            // the actual text; users render client-side
	Arguments datatypes.JSON `json:"arguments,omitempty"` // [{name, description, required}]
	Metadata  datatypes.JSON `json:"metadata,omitempty"`

	PublishedByUserID *uuid.UUID `gorm:"type:uuid;index" json:"published_by_user_id,omitempty"`
	PublishedAt       time.Time  `json:"published_at"`

	SourcePeerID        string     `gorm:"index" json:"source_peer_id,omitempty"`
	FederatedGeneration int        `json:"federated_generation,omitempty"`
	LastSyncedAt        *time.Time `json:"last_synced_at,omitempty"`
}

// IsFederated reports whether a registry row was pulled from a peer.
// Each of the four artifact types embeds the same SourcePeerID field;
// callers use this helper instead of comparing strings inline so the
// "what counts as federated" rule stays in one place.
func (s RegistryServer) IsFederated() bool { return s.SourcePeerID != "" }

// IsFederated for agents.
func (a RegistryAgent) IsFederated() bool { return a.SourcePeerID != "" }

// IsFederated for skills.
func (s RegistrySkill) IsFederated() bool { return s.SourcePeerID != "" }

// IsFederated for prompts.
func (p RegistryPrompt) IsFederated() bool { return p.SourcePeerID != "" }

// --- Cross-kind references ------------------------------------------------

// RegistryRef models the "agent depends on" edges:
//
//	(OwnerKind=agent, OwnerID=agent uuid) --> (TargetKind={server,skill,prompt}, TargetName, TargetVersion)
//
// Stored as name+version (not FK-resolved IDs) so a ref can point at a
// version that is later deleted without cascading corruption. Optional.
type RegistryRef struct {
	BaseModel

	OwnerKind     RegistryKind `gorm:"index:idx_reg_ref_owner;not null" json:"owner_kind"`
	OwnerID       uuid.UUID    `gorm:"type:uuid;index:idx_reg_ref_owner;not null" json:"owner_id"`
	TargetKind    RegistryKind `gorm:"index:idx_reg_ref_target;not null" json:"target_kind"`
	TargetName    string       `gorm:"index:idx_reg_ref_target;not null" json:"target_name"`
	TargetVersion string       `json:"target_version,omitempty"` // empty = "latest"

	// Optional local alias the agent uses to refer to the target.
	LocalName string `json:"local_name,omitempty"`
}
