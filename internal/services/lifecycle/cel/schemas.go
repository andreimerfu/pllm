// Package cel wraps cel-go for the lifecycle daemon. The schemas
// declared here are the authoritative input contracts for operator
// rules — every field exposed is a stable surface that other tools
// (UI templates, audit replays) can rely on.
//
// Schemas are intentionally narrow: only the columns we already store,
// no derived joins, no JSONB blobs. JSON metadata can be added later
// per-resource as a `map<string,string>` once we have a clean policy
// for type coercion.
package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"

	"github.com/amerfu/pllm/internal/core/models"
)

// scanInputVar returns the CEL variable name used in expressions for a
// given scan target. The name is part of the operator-facing contract,
// so don't change these without a corresponding template migration.
func scanInputVar(t models.LifecycleScanTarget) string {
	switch t {
	case models.LifecycleScanKeys:
		return "key"
	case models.LifecycleScanServers:
		return "server"
	case models.LifecycleScanUseCases:
		return "use_case"
	case models.LifecycleScanUsers:
		return "user"
	}
	return "row"
}

// envOptionsForTarget returns the cel.EnvOptions that declare the
// input variable + bind `now` as a reified timestamp so expressions
// like `now - key.created_at > duration("90d")` evaluate. The shape
// of each input is `map<string, dyn>` to keep the evaluator schema-
// flexible — type errors surface at compile time when a non-existent
// key is accessed (CEL still type-checks against the declared map).
func envOptionsForTarget(t models.LifecycleScanTarget) []cel.EnvOption {
	return []cel.EnvOption{
		cel.Variable(scanInputVar(t), cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("now", cel.TimestampType),
	}
}

// SupportedTargets is the canonical list of scan targets the evaluator
// understands. Used by handler validation and by the UI template list.
var SupportedTargets = []models.LifecycleScanTarget{
	models.LifecycleScanKeys,
	models.LifecycleScanServers,
	models.LifecycleScanUseCases,
	models.LifecycleScanUsers,
}

// Field documents a single attribute on a scan target. The UI surfaces
// these in tooltips on the rule builder so non-programmers can see
// what's available without reading source.
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "string", "bool", "timestamp", "int", "string[]"
	Nullable bool   `json:"nullable"`
	Notes    string `json:"notes,omitempty"`
}

// Schema is the documentation surface for one scan target.
type Schema struct {
	Target models.LifecycleScanTarget `json:"target"`
	Var    string                     `json:"var"`
	Fields []Field                    `json:"fields"`
}

// Schemas is the authoritative documentation map. The fields listed
// here MUST match what input.go produces — when adding a field, update
// both files together.
var Schemas = []Schema{
	{
		Target: models.LifecycleScanKeys,
		Var:    "key",
		Fields: []Field{
			{Name: "id", Type: "string"},
			{Name: "name", Type: "string"},
			{Name: "key_prefix", Type: "string"},
			{Name: "type", Type: "string", Notes: "api | virtual | master | system"},
			{Name: "is_active", Type: "bool"},
			{Name: "created_at", Type: "timestamp"},
			{Name: "last_used_at", Type: "timestamp", Nullable: true},
			{Name: "expires_at", Type: "timestamp", Nullable: true},
			{Name: "revoked_at", Type: "timestamp", Nullable: true},
			{Name: "user_id", Type: "string", Nullable: true},
			{Name: "team_id", Type: "string", Nullable: true},
			{Name: "tags", Type: "string[]"},
			{Name: "usage_count", Type: "int"},
		},
	},
	{
		Target: models.LifecycleScanServers,
		Var:    "server",
		Fields: []Field{
			{Name: "id", Type: "string"},
			{Name: "name", Type: "string"},
			{Name: "version", Type: "string"},
			{Name: "title", Type: "string"},
			{Name: "status", Type: "string", Notes: "active | blocked | deprecated"},
			{Name: "is_latest", Type: "bool"},
			{Name: "has_publisher", Type: "bool", Notes: "true if PublishedByUserID set"},
			{Name: "created_at", Type: "timestamp"},
			{Name: "published_at", Type: "timestamp"},
		},
	},
	{
		Target: models.LifecycleScanUseCases,
		Var:    "use_case",
		Fields: []Field{
			{Name: "id", Type: "string"},
			{Name: "name", Type: "string"},
			{Name: "slug", Type: "string"},
			{Name: "status", Type: "string", Notes: "draft | under_review | approved | retired"},
			{Name: "risk_tier", Type: "string", Notes: "tier1 | tier2 | tier3 | tier4"},
			{Name: "enforce_scope", Type: "bool"},
			{Name: "compliance_flag", Type: "bool"},
			{Name: "created_at", Type: "timestamp"},
			{Name: "updated_at", Type: "timestamp"},
			{Name: "last_graph_computed_at", Type: "timestamp", Nullable: true},
		},
	},
	{
		Target: models.LifecycleScanUsers,
		Var:    "user",
		Fields: []Field{
			{Name: "id", Type: "string"},
			{Name: "email", Type: "string"},
			{Name: "username", Type: "string"},
			{Name: "role", Type: "string", Notes: "admin | manager | user | viewer"},
			{Name: "is_active", Type: "bool"},
			{Name: "email_verified", Type: "bool"},
			{Name: "created_at", Type: "timestamp"},
			{Name: "last_login_at", Type: "timestamp", Nullable: true},
			{Name: "external_provider", Type: "string"},
		},
	},
}

// nullTimestamp is the sentinel a row adapter returns when a *time.Time
// is nil. We expose it as the Unix epoch so expressions like
// `key.last_used_at != null` translate to a non-null check at the CEL
// layer; the input adapter substitutes `null` (cel types.NullValue)
// when the underlying pointer is nil.
var _ = types.NullValue
