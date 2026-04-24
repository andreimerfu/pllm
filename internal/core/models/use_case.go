package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// UseCaseRiskTier mirrors the tier language regulated industries use
// (banks, insurers). Operators can ignore it for non-regulated contexts —
// it's just a string, not enforced at the DB layer.
type UseCaseRiskTier string

const (
	UseCaseRiskTier1 UseCaseRiskTier = "tier1" // highest impact
	UseCaseRiskTier2 UseCaseRiskTier = "tier2"
	UseCaseRiskTier3 UseCaseRiskTier = "tier3"
	UseCaseRiskTier4 UseCaseRiskTier = "tier4" // lowest impact
)

// UseCaseStatus tracks the approval lifecycle independently of deployment
// state. A use case can be "approved" but have zero running deployments.
type UseCaseStatus string

const (
	UseCaseStatusDraft       UseCaseStatus = "draft"
	UseCaseStatusUnderReview UseCaseStatus = "under_review"
	UseCaseStatusApproved    UseCaseStatus = "approved"
	UseCaseStatusRetired     UseCaseStatus = "retired"
)

// UseCase is the top-level "business activity" that consumes registry
// components (agents, MCP servers, skills, prompts, LLMs). It's what
// risk/compliance actually approves. Dependencies are edges in the
// existing RegistryRef table with OwnerKind = "use_case".
type UseCase struct {
	BaseModel

	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Slug        string `gorm:"uniqueIndex;not null" json:"slug"`
	Description string `json:"description"`

	RiskTier UseCaseRiskTier `gorm:"default:'tier4'" json:"risk_tier"`
	Status   UseCaseStatus   `gorm:"default:'draft';index" json:"status"`

	// Ownership: business owner authored it, approver signed off on it.
	// Keeping both nullable so we can seed use cases without a full RBAC
	// story in place.
	BusinessOwnerUserID *uuid.UUID `gorm:"type:uuid;index" json:"business_owner_user_id,omitempty"`
	ApproverUserID      *uuid.UUID `gorm:"type:uuid;index" json:"approver_user_id,omitempty"`
	TeamID              *uuid.UUID `gorm:"type:uuid;index" json:"team_id,omitempty"`

	// Declared data classifications — user-tagged.
	DataClassifications pq.StringArray `gorm:"type:text[]" json:"data_classifications,omitempty"`
	// EffectiveClassifications = own + union of every transitively-reachable
	// component's tags. Materialized by a reconciler (Phase 1.6). Not
	// populated by opening-move code; stays nil until that runs.
	EffectiveClassifications pq.StringArray `gorm:"type:text[]" json:"effective_classifications,omitempty"`

	// External GRC reference (OpenPages record ID, ServiceNow ticket, etc.).
	// Opaque on our side — we just persist and echo it.
	ApprovalRecord datatypes.JSON `json:"approval_record,omitempty"`

	// Compliance flag flipped by the change-propagation job when an upstream
	// change violates the use case's declared classifications / risk tier.
	ComplianceFlag       bool   `gorm:"index" json:"compliance_flag"`
	ComplianceFlagReason string `json:"compliance_flag_reason,omitempty"`

	// When the dependency graph was last walked for this use case.
	LastGraphComputedAt *time.Time `json:"last_graph_computed_at,omitempty"`

	// EnforceScope controls runtime enforcement for keys bound to this
	// use case. false (default) = flag mode: out-of-scope calls are
	// logged but allowed. true = block mode: out-of-scope calls return
	// 403. Operators typically run flag mode for a few weeks to build
	// confidence before flipping on block mode.
	EnforceScope bool `gorm:"default:false" json:"enforce_scope"`
}
