package lifecycle

import (
	"fmt"
	"strings"

	"github.com/amerfu/pllm/internal/core/models"
)

// TemplateParam describes one form field on a rule-builder template.
// The UI renders a control based on Type; values flow back as
// JSON-typed primitives the BuildExpr closure substitutes into the
// CEL expression. Type is intentionally a small enum — anything
// richer should be a separate template, not a polymorphic field.
type TemplateParam struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"` // "int", "string", "string[]", "duration_days"
	Default     any    `json:"default,omitempty"`
	Min         *int   `json:"min,omitempty"`
	Max         *int   `json:"max,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Help        string `json:"help,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Template is the operator-facing recipe for a custom rule. Each
// template has a fixed scan target + action; the form generates the
// CEL Match expression and the audit Reason string. When the user
// saves, we persist the resolved Match + Reason on
// LifecycleCustomRule alongside the TemplateID + Params so the form
// can be re-rendered later for editing.
type Template struct {
	ID          string                     `json:"id"`
	Title       string                     `json:"title"`
	Category    string                     `json:"category"` // "keys" | "servers" | "use_cases" | "users"
	Description string                     `json:"description"`
	Scan        models.LifecycleScanTarget `json:"scan"`
	Action      models.LifecycleAction     `json:"action"`
	Params      []TemplateParam            `json:"params"`

	// BuildExpr/BuildReason are not serialized; they only live in-process.
	BuildExpr   func(p map[string]any) (string, error) `json:"-"`
	BuildReason func(p map[string]any) string         `json:"-"`
}

// intParam coerces a JSON-decoded number (float64 from encoding/json)
// or already-typed int into a Go int with a sane default. Returns the
// value plus whether the caller-supplied value was actually present —
// useful when a template treats "missing" different from "zero".
func intParam(p map[string]any, key string, def int) (int, bool) {
	raw, ok := p[key]
	if !ok || raw == nil {
		return def, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	}
	return def, false
}

func stringParam(p map[string]any, key, def string) string {
	if raw, ok := p[key]; ok && raw != nil {
		if s, ok := raw.(string); ok && s != "" {
			return s
		}
	}
	return def
}

func stringSliceParam(p map[string]any, key string) []string {
	raw, ok := p[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// hoursLiteral renders an integer day count as the string CEL's
// duration() builtin accepts ("Nh"). We pick hours instead of days
// because cel-go's parser doesn't accept "d" — see cel.dev/expr.
func hoursLiteral(days int) string { return fmt.Sprintf("%dh", days*24) }

// celStringList quotes a slice of strings into a CEL list literal,
// safe against the values the UI typically forwards (operator-typed
// publisher names, tags). Anything containing a quote is rejected at
// caller level — keep this function focused on formatting.
func celStringList(items []string) string {
	q := make([]string, 0, len(items))
	for _, it := range items {
		if it == "" {
			continue
		}
		// Quote with double-quotes, escape embedded ones.
		q = append(q, fmt.Sprintf("%q", it))
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// Templates is the read-only catalog. Order is the order the UI
// renders them.
var Templates = []Template{
	// ────────────────── Keys ──────────────────
	{
		ID:          "rotate_aged_keys",
		Title:       "Rotate aged keys",
		Category:    "keys",
		Scan:        models.LifecycleScanKeys,
		Action:      models.LifecycleActionRevoke,
		Description: "Revoke any non-master key older than the rotation deadline, regardless of recent activity. Maps to controls like 'prod keys must rotate every N days'.",
		Params: []TemplateParam{
			{Key: "days", Label: "Maximum age", Type: "duration_days", Default: 365, Min: ptrInt(1), Max: ptrInt(3650), Help: "Days since the key was created.", Required: true},
		},
		BuildExpr: func(p map[string]any) (string, error) {
			d, _ := intParam(p, "days", 365)
			if d < 1 {
				return "", fmt.Errorf("days must be ≥ 1")
			}
			return fmt.Sprintf(
				`key.is_active && key.type != "master" && (now - key.created_at) > duration(%q)`,
				hoursLiteral(d)), nil
		},
		BuildReason: func(p map[string]any) string {
			d, _ := intParam(p, "days", 365)
			return fmt.Sprintf("rotation policy: key older than %d days", d)
		},
	},
	{
		ID:          "revoke_keys_by_tag",
		Title:       "Revoke tagged keys past deadline",
		Category:    "keys",
		Scan:        models.LifecycleScanKeys,
		Action:      models.LifecycleActionRevoke,
		Description: "Revoke keys carrying a specific tag (e.g. env:prod) once they exceed an age limit. Useful when different environments have different rotation policies.",
		Params: []TemplateParam{
			{Key: "tag", Label: "Tag to match", Type: "string", Default: "env:prod", Placeholder: "env:prod", Required: true, Help: "Exact-match tag on the key."},
			{Key: "days", Label: "Maximum age", Type: "duration_days", Default: 90, Min: ptrInt(1), Max: ptrInt(3650), Required: true},
		},
		BuildExpr: func(p map[string]any) (string, error) {
			tag := stringParam(p, "tag", "")
			if tag == "" {
				return "", fmt.Errorf("tag is required")
			}
			if strings.ContainsAny(tag, `"\`) {
				return "", fmt.Errorf("tag must not contain quote or backslash")
			}
			d, _ := intParam(p, "days", 90)
			return fmt.Sprintf(
				`key.is_active && %q in key.tags && (now - key.created_at) > duration(%q)`,
				tag, hoursLiteral(d)), nil
		},
		BuildReason: func(p map[string]any) string {
			tag := stringParam(p, "tag", "")
			d, _ := intParam(p, "days", 90)
			return fmt.Sprintf("tag %q + older than %d days", tag, d)
		},
	},

	// ────────────────── Use cases ──────────────────
	{
		ID:          "flag_stale_drafts",
		Title:       "Flag stale draft use cases",
		Category:    "use_cases",
		Scan:        models.LifecycleScanUseCases,
		Action:      models.LifecycleActionFlag,
		Description: "Flag use cases stuck in draft beyond a deadline. Surfaces work that's been started but never approved — typically a sign of an abandoned initiative or a stalled review.",
		Params: []TemplateParam{
			{Key: "days", Label: "Maximum draft age", Type: "duration_days", Default: 30, Min: ptrInt(1), Max: ptrInt(365), Required: true},
		},
		BuildExpr: func(p map[string]any) (string, error) {
			d, _ := intParam(p, "days", 30)
			return fmt.Sprintf(
				`use_case.status == "draft" && (now - use_case.created_at) > duration(%q)`,
				hoursLiteral(d)), nil
		},
		BuildReason: func(p map[string]any) string {
			d, _ := intParam(p, "days", 30)
			return fmt.Sprintf("draft use case unchanged for %d+ days", d)
		},
	},
	{
		ID:          "flag_unattested_approved",
		Title:       "Flag approved use cases needing re-attestation",
		Category:    "use_cases",
		Scan:        models.LifecycleScanUseCases,
		Action:      models.LifecycleActionFlag,
		Description: "Flag approved use cases whose dependency graph hasn't been recomputed within the re-attestation window. Maps to annual control-recertification requirements.",
		Params: []TemplateParam{
			{Key: "days", Label: "Re-attestation window", Type: "duration_days", Default: 365, Min: ptrInt(30), Max: ptrInt(3650), Required: true, Help: "How often approved use cases must be re-checked."},
		},
		BuildExpr: func(p map[string]any) (string, error) {
			d, _ := intParam(p, "days", 365)
			// last_graph_computed_at is nullable — null counts as "never re-checked".
			return fmt.Sprintf(
				`use_case.status == "approved" && (use_case.last_graph_computed_at == null || (now - use_case.last_graph_computed_at) > duration(%q))`,
				hoursLiteral(d)), nil
		},
		BuildReason: func(p map[string]any) string {
			d, _ := intParam(p, "days", 365)
			return fmt.Sprintf("approved use case not re-attested in %d+ days", d)
		},
	},

	// ────────────────── Servers ──────────────────
	{
		ID:          "block_unowned_servers",
		Title:       "Block unowned registry servers",
		Category:    "servers",
		Scan:        models.LifecycleScanServers,
		Action:      models.LifecycleActionBlock,
		Description: "Block registry servers that have no publisher attribution and have been around longer than a grace period. Good hygiene for catalogs that grew via bulk imports.",
		Params: []TemplateParam{
			{Key: "days", Label: "Grace period", Type: "duration_days", Default: 30, Min: ptrInt(1), Max: ptrInt(365), Required: true},
		},
		BuildExpr: func(p map[string]any) (string, error) {
			d, _ := intParam(p, "days", 30)
			return fmt.Sprintf(
				`server.status == "active" && !server.has_publisher && (now - server.created_at) > duration(%q)`,
				hoursLiteral(d)), nil
		},
		BuildReason: func(p map[string]any) string {
			d, _ := intParam(p, "days", 30)
			return fmt.Sprintf("unowned server older than %d days", d)
		},
	},

	// ────────────────── Users ──────────────────
	{
		ID:          "notify_inactive_users",
		Title:       "Notify on inactive users",
		Category:    "users",
		Scan:        models.LifecycleScanUsers,
		Action:      models.LifecycleActionNotify,
		Description: "Emit a webhook + audit row when an active user hasn't logged in for too long. Doesn't deactivate the user — that's an HR decision; this is the signal.",
		Params: []TemplateParam{
			{Key: "days", Label: "Days since last login", Type: "duration_days", Default: 90, Min: ptrInt(7), Max: ptrInt(3650), Required: true},
		},
		BuildExpr: func(p map[string]any) (string, error) {
			d, _ := intParam(p, "days", 90)
			return fmt.Sprintf(
				`user.is_active && user.last_login_at != null && (now - user.last_login_at) > duration(%q)`,
				hoursLiteral(d)), nil
		},
		BuildReason: func(p map[string]any) string {
			d, _ := intParam(p, "days", 90)
			return fmt.Sprintf("user dormant for %d+ days", d)
		},
	},
}

// FindTemplate returns the catalog entry by ID, or nil. Callers must
// nil-check — we don't pre-validate IDs at the API layer.
func FindTemplate(id string) *Template {
	for i := range Templates {
		if Templates[i].ID == id {
			return &Templates[i]
		}
	}
	return nil
}

func ptrInt(i int) *int { return &i }
