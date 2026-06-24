package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// FederationSyncMode controls which artifacts a peer pulls from a source.
//
//   - "all"        — every active version of every kind the source exports
//   - "whitelist"  — only artifact names listed in the per-kind whitelists
//   - "tag_filter" — only artifacts whose Metadata.tags intersect TagFilters
//
// Modes are exclusive. tag_filter is intentionally a substring-of-metadata
// check rather than a structured matcher so it stays vendor-agnostic.
type FederationSyncMode string

const (
	FederationSyncModeAll       FederationSyncMode = "all"
	FederationSyncModeWhitelist FederationSyncMode = "whitelist"
	FederationSyncModeTagFilter FederationSyncMode = "tag_filter"
)

// FederationSyncStatus is the last-known terminal state of a peer's sync.
type FederationSyncStatus string

const (
	FederationSyncStatusUnknown FederationSyncStatus = ""
	FederationSyncStatusSuccess FederationSyncStatus = "success"
	FederationSyncStatusFailed  FederationSyncStatus = "failed"
	FederationSyncStatusPartial FederationSyncStatus = "partial" // some kinds failed; some succeeded
)

// FederationPeer is a configured upstream pllm registry from which this
// instance pulls artifacts. Pulls are scheduled by the federation Runner
// and can also be triggered manually via the admin API.
//
// One row per peer. Disabled rows are kept around so operators don't lose
// the config when temporarily silencing a noisy peer.
type FederationPeer struct {
	BaseModel

	// PeerID is the stable slug used as the SourcePeerID on synced rows.
	// Operators see this in artifact badges ("from lob-a"); changing it
	// would orphan everything already synced, so it's effectively
	// immutable post-create.
	PeerID  string `gorm:"uniqueIndex;not null" json:"peer_id"`
	Name    string `gorm:"not null" json:"name"`
	Endpoint string `gorm:"not null" json:"endpoint"`

	Enabled bool `gorm:"default:true;index" json:"enabled"`

	SyncMode FederationSyncMode `gorm:"type:text;default:'all'" json:"sync_mode"`

	// Per-kind whitelists. Only consulted when SyncMode = whitelist.
	WhitelistServers pq.StringArray `gorm:"type:text[]" json:"whitelist_servers,omitempty"`
	WhitelistAgents  pq.StringArray `gorm:"type:text[]" json:"whitelist_agents,omitempty"`
	WhitelistSkills  pq.StringArray `gorm:"type:text[]" json:"whitelist_skills,omitempty"`
	WhitelistPrompts pq.StringArray `gorm:"type:text[]" json:"whitelist_prompts,omitempty"`

	// TagFilters is consulted when SyncMode = tag_filter. Matches any
	// substring against a row's serialized Metadata column.
	TagFilters pq.StringArray `gorm:"type:text[]" json:"tag_filters,omitempty"`

	// SyncIntervalMinutes controls how often the runner re-pulls this peer.
	// Zero falls back to the global default in config.
	SyncIntervalMinutes int `gorm:"default:60" json:"sync_interval_minutes"`

	// FederationToken is the static bearer pllm sends to the peer's
	// /v1/federation/* endpoints. Stored plaintext for now — matches the
	// existing Webhook.Secret + provider key handling; encryption-at-rest
	// is a cross-cutting follow-up.
	FederationToken string `json:"-"`

	// ExpectedClientID lets the peer's auth layer validate the caller
	// matches what was provisioned. Carried as an optional
	// X-PLLM-Federation-Client header; receiver may ignore.
	ExpectedClientID string `json:"expected_client_id,omitempty"`

	// LastSyncGeneration increments on every successful pull. Synced
	// artifacts carry the generation that minted them; rows with an older
	// generation than the peer's current value are considered orphans.
	LastSyncGeneration int                  `gorm:"default:0" json:"last_sync_generation"`
	LastSyncAt         *time.Time           `json:"last_sync_at,omitempty"`
	LastSyncStatus     FederationSyncStatus `gorm:"type:text" json:"last_sync_status,omitempty"`
	LastSyncError      string               `json:"last_sync_error,omitempty"`
	LastSyncSummary    string               `json:"last_sync_summary,omitempty"` // free-form, "12 servers, 3 agents"

	CreatedByUserID *uuid.UUID `gorm:"type:uuid" json:"created_by_user_id,omitempty"`
}

// MatchesWhitelist reports whether an artifact name passes the peer's
// whitelist for a given kind. Always true outside whitelist mode.
func (p *FederationPeer) MatchesWhitelist(kind RegistryKind, name string) bool {
	if p.SyncMode != FederationSyncModeWhitelist {
		return true
	}
	var list pq.StringArray
	switch kind {
	case RegistryKindServer:
		list = p.WhitelistServers
	case RegistryKindAgent:
		list = p.WhitelistAgents
	case RegistryKindSkill:
		list = p.WhitelistSkills
	case RegistryKindPrompt:
		list = p.WhitelistPrompts
	default:
		return false
	}
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

// MatchesTags reports whether at least one TagFilter substring appears in
// the marshaled metadata. Always true outside tag_filter mode. Substring
// match keeps the receiver agnostic to vendor metadata schemas.
func (p *FederationPeer) MatchesTags(metadataJSON string) bool {
	if p.SyncMode != FederationSyncModeTagFilter {
		return true
	}
	for _, t := range p.TagFilters {
		if t == "" {
			continue
		}
		if containsCI(metadataJSON, t) {
			return true
		}
	}
	return false
}

// containsCI is a tiny case-insensitive substring helper.
func containsCI(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	// Manual lower to avoid importing strings everywhere this is used.
	hl := toLower(haystack)
	nl := toLower(needle)
	for i := 0; i+len(nl) <= len(hl); i++ {
		if hl[i:i+len(nl)] == nl {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + ('a' - 'A')
		}
	}
	return string(out)
}
