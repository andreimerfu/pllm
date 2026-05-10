package models

import (
	"time"

	"github.com/google/uuid"
)

// ScopeEventKind distinguishes the surface where the event originated.
// Kept as a string so adding new surfaces later (A2A, embeddings, etc.)
// doesn't require a migration.
type ScopeEventKind string

const (
	ScopeEventKindMCPTool ScopeEventKind = "mcp_tool"
	ScopeEventKindLLM     ScopeEventKind = "llm"
	ScopeEventKindAgent   ScopeEventKind = "agent"
	ScopeEventKindPrompt  ScopeEventKind = "prompt"
	// ScopeEventKindObserved is emitted when OTel ingestion discovers a
	// call to a target that the bound use case does not declare.
	// Cannot be denied at the gateway (the call already happened); always
	// recorded with Outcome=ScopeOutcomeFlagged.
	ScopeEventKindObserved ScopeEventKind = "observed"
)

// ScopeEventOutcome reflects what the runtime decided:
//   - "denied"   = block-mode, the call failed 403
//   - "flagged"  = flag-mode, the call went through but was recorded
//
// Operators typically run flag mode for a week or two to clean up
// accidental out-of-scope configurations before flipping on enforcement;
// the log is how they audit that transition.
type ScopeEventOutcome string

const (
	ScopeOutcomeDenied  ScopeEventOutcome = "denied"
	ScopeOutcomeFlagged ScopeEventOutcome = "flagged"
	// ScopeOutcomeAllowed is used by detect-only paths (OTel discovery)
	// that report a positive evaluation result; not persisted as a
	// ScopeEvent row but consumed by callers via PolicyResult.
	ScopeOutcomeAllowed ScopeEventOutcome = "allowed"
	// ScopeOutcomeBlocked is recorded when a client-side SDK refused
	// to make the call (e.g. pllm_sdk's httpx patch raised
	// PllmPolicyViolation). Distinct from "denied" (gateway) and
	// "flagged" (after-the-fact observation) — the call never
	// happened AND the agent's process knew it was prevented.
	ScopeOutcomeBlocked ScopeEventOutcome = "blocked"
)

// ScopeEvent is a persistent record of one runtime scope check that
// resulted in "not allowed." Written from the MCP gateway and chat
// handler (and any future surface) so the compliance team can answer
// "what got blocked last week, and under which use case?"
//
// Intentionally narrow: we don't log allowed calls — those live in the
// existing Usage / Metrics tables. This table is for the audit edge only.
type ScopeEvent struct {
	BaseModel

	// Who: the bound use case + the key that made the call. Key is
	// nullable because the log survives after the key is deleted.
	UseCaseID uuid.UUID  `gorm:"type:uuid;index;not null" json:"use_case_id"`
	KeyID     *uuid.UUID `gorm:"type:uuid;index" json:"key_id,omitempty"`
	KeyName   string     `json:"key_name,omitempty"` // cached at write time so UI doesn't need a join
	UserID    *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`

	// What: which surface, which target.
	Kind          ScopeEventKind    `gorm:"index;not null" json:"kind"`
	TargetName    string            `gorm:"index;not null" json:"target_name"`
	Outcome       ScopeEventOutcome `gorm:"index;not null" json:"outcome"`
	Reason        string            `json:"reason,omitempty"`

	// Request envelope hints — useful for operators tracing "who called what":
	RemoteAddr string `json:"remote_addr,omitempty"`
	RequestID  string `json:"request_id,omitempty"`

	// When: inherited from BaseModel.CreatedAt, but we alias as OccurredAt
	// in the JSON response for clearer semantics in the audit UI.
}

// OccurredAtJSON is a helper to format the CreatedAt field into the
// response under a clearer key than "created_at" — not strictly required
// but clarifies semantics for auditors reading the JSON.
func (e ScopeEvent) OccurredAt() time.Time { return e.CreatedAt }
