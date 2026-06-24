package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// LifecycleScanTarget enumerates which table a custom rule scans. The
// CEL evaluator binds a single input variable named after the target
// (key, server, use_case, user) so rule expressions read naturally —
// e.g. `now - key.last_used_at > duration("90d")`.
type LifecycleScanTarget string

const (
	LifecycleScanKeys      LifecycleScanTarget = "keys"
	LifecycleScanServers   LifecycleScanTarget = "registry_servers"
	LifecycleScanUseCases  LifecycleScanTarget = "use_cases"
	LifecycleScanUsers     LifecycleScanTarget = "users"
)

// LifecycleAction is the mutation a rule applies when its predicate
// matches a row. notify is the audit-and-webhook-only variant — the row
// is left untouched but operators get the same trail they'd get from a
// mutation.
type LifecycleAction string

const (
	LifecycleActionRevoke LifecycleAction = "revoke" // keys
	LifecycleActionBlock  LifecycleAction = "block"  // registry_servers
	LifecycleActionFlag   LifecycleAction = "flag"   // use_cases (sets compliance_flag)
	LifecycleActionNotify LifecycleAction = "notify" // any
)

// LifecycleCustomRule is an operator-defined hygiene rule. Rules are
// authored from a small set of templates (see services/lifecycle/templates.go);
// the template generates the CEL expression and surfaces a typed param
// form to the UI. Storing the resolved Match expression alongside the
// TemplateID + Params lets the daemon stay template-agnostic at run
// time and keeps the data self-describing for audit.
type LifecycleCustomRule struct {
	BaseModel

	// Stable identifier surfaced in the lifecycle run summary, audit rows
	// (rule_name in NewValues), and webhooks. Required, must be unique
	// across the deployment, and conventionally snake_case so it composes
	// with built-in rule names like inactive_keys.
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Description string `json:"description"`

	// Scan target chooses which table the daemon walks. The CEL input
	// variable name (key, server, use_case, user) is implied by this
	// field — see services/lifecycle/cel/schemas.go.
	Scan LifecycleScanTarget `gorm:"type:varchar(32);not null" json:"scan"`

	// Match is the compiled-at-load CEL boolean expression. Operators
	// don't write this directly; the template builder produces it from
	// Params. Stored verbatim so audit can replay it.
	Match string `gorm:"type:text;not null" json:"match"`

	// Action and Reason are interpreted at run time by custom_rule.go.
	// Reason is the literal string written into the audit row + webhook
	// payload — keep it human-readable and operator-recognizable.
	Action LifecycleAction `gorm:"type:varchar(32);not null" json:"action"`
	Reason string          `gorm:"type:text" json:"reason"`

	// Enabled gates the rule independently of DryRun. A disabled rule is
	// loaded but not run; this lets operators stage rules without
	// committing to a dry-run cycle.
	Enabled bool `gorm:"default:true" json:"enabled"`

	// DryRun overrides the global Lifecycle.DryRun for this rule. nil
	// inherits the global; *false forces enforcement; *true forces
	// log-only. Useful for promoting one rule out of dry-run while the
	// rest of the daemon stays in observation mode.
	DryRun *bool `json:"dry_run,omitempty"`

	// TemplateID + Params let the UI re-render the original form when
	// editing. They have no runtime effect — the daemon only reads Match.
	TemplateID string         `gorm:"index" json:"template_id"`
	Params     datatypes.JSON `json:"params,omitempty"`

	// Audit attribution. Nil for config-bootstrapped rules.
	CreatedBy *uuid.UUID `gorm:"type:uuid" json:"created_by,omitempty"`
}
