package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// WebhookEventType is the string tag sent in the X-PLLM-Event header and
// used by the admin UI to filter subscriptions. Keep the vocabulary small
// and stable — each new type is a contract with subscribers.
type WebhookEventType string

const (
	// Runtime events — the bread-and-butter for compliance alerting.
	WebhookEventScopeDenied  WebhookEventType = "scope.denied"  // block-mode: call was blocked
	WebhookEventScopeFlagged WebhookEventType = "scope.flagged" // flag-mode: call was logged but allowed

	// Governance lifecycle events — for GRC-system sync.
	WebhookEventUseCaseCreated        WebhookEventType = "use_case.created"
	WebhookEventUseCaseUpdated        WebhookEventType = "use_case.updated"
	WebhookEventUseCaseDeleted        WebhookEventType = "use_case.deleted"
	WebhookEventUseCaseComplianceFlag WebhookEventType = "use_case.compliance_flag"

	// Registry lifecycle — for "vendor pushed a new version, re-review"
	// flows. Not emitted today but reserved so subscribers can future-proof.
	WebhookEventRegistryServerSaved WebhookEventType = "registry.server.saved"

	// Lifecycle hygiene — automatic key/server cleanup actions taken by
	// the lifecycle daemon. Subscribers typically forward these to SIEM
	// or to a "weekly cleanup digest" channel.
	WebhookEventKeyRevokedInactive         WebhookEventType = "key.revoked.inactive"
	WebhookEventKeyRevokedOwnerless        WebhookEventType = "key.revoked.ownerless"
	WebhookEventRegistryServerBlockedRisky WebhookEventType = "registry.server.blocked.risky"

	// Custom-rule lifecycle actions. The rule_name field on the payload
	// distinguishes which operator-defined rule fired; the action field
	// distinguishes revoke vs block vs flag vs notify so a single
	// subscriber can route on action type rather than per-rule.
	WebhookEventLifecycleCustom WebhookEventType = "lifecycle.custom"

	// Deployment OOM auto-scale — noteworthy operational events that
	// typically matter to SIEM/on-call ("workload X has escalated memory
	// 4 times and is now down — manual intervention needed").
	WebhookEventDeploymentOOMBumped     WebhookEventType = "deployment.oom.bumped"
	WebhookEventDeploymentOOMCapReached WebhookEventType = "deployment.oom.cap_reached"

	// Federation — peer-registry sync lifecycle. *.completed fires once
	// per successful run with a summary; *.failed surfaces transport or
	// auth errors. *.item_orphaned fires when a previously-synced row no
	// longer appears upstream and is flagged for operator review.
	WebhookEventFederationSyncCompleted WebhookEventType = "federation.sync.completed"
	WebhookEventFederationSyncFailed    WebhookEventType = "federation.sync.failed"
	WebhookEventFederationItemOrphaned  WebhookEventType = "federation.item.orphaned"
)

// WebhookDestinationType controls how the dispatcher renders an event
// before POSTing it. Each destination has a vendor-specific adapter
// living under internal/services/webhooks/adapters/. The default
// "generic" sends pllm's native HMAC-signed JSON envelope to any
// arbitrary HTTPS endpoint.
type WebhookDestinationType string

const (
	WebhookDestGeneric    WebhookDestinationType = "generic"
	WebhookDestSlack      WebhookDestinationType = "slack"
	WebhookDestTeams      WebhookDestinationType = "teams"
	WebhookDestSplunk     WebhookDestinationType = "splunk"
	WebhookDestDatadog    WebhookDestinationType = "datadog"
	WebhookDestServiceNow WebhookDestinationType = "servicenow"
)

// Webhook is an external HTTP endpoint that receives pllm events.
// Delivery is best-effort async with bounded retries; we don't persist
// full delivery bodies, but LastDeliveryStatus + FailureCount give
// operators a quick "is this endpoint alive" signal.
type Webhook struct {
	BaseModel

	Name string `gorm:"not null" json:"name"`
	URL  string `gorm:"not null" json:"url"`

	// DestinationType picks the adapter that formats the event for the
	// receiver. Defaults to "generic" — the pre-adapter behavior — so
	// existing rows keep working unchanged across the migration.
	DestinationType WebhookDestinationType `gorm:"type:text;not null;default:'generic'" json:"destination_type"`

	// Secret is the HMAC-SHA256 key used by the Generic adapter for the
	// X-PLLM-Signature header. Vendor-specific adapters (Slack, Teams,
	// etc.) ignore it because they use vendor-native auth instead.
	// Stored plaintext for now — operators rotate via delete+create.
	Secret string `gorm:"not null" json:"secret"`

	// Config carries vendor-specific credentials and options as opaque
	// JSON, e.g. {"hec_token":"..."} for Splunk or
	// {"api_key":"...","site":"us1"} for Datadog. Hidden from JSON
	// responses so credentials don't leak via GET; the API exposes a
	// redacted ConfigSummary instead. Stored plaintext at rest, matching
	// the existing pattern for provider API keys — encryption-at-rest is
	// a follow-up that should land for both surfaces together.
	Config datatypes.JSON `gorm:"type:jsonb" json:"-"`

	// Events is a subset of WebhookEventType values the endpoint subscribes
	// to. Empty = subscribed to everything. Stored as a text[] for simple
	// containment queries from the dispatcher.
	Events  pq.StringArray `gorm:"type:text[]" json:"events,omitempty"`
	Enabled bool           `gorm:"default:true;index" json:"enabled"`

	// Optional scoping for multi-tenant setups: restrict which team's
	// events fire this webhook. Nil = all teams.
	TeamID *uuid.UUID `gorm:"type:uuid;index" json:"team_id,omitempty"`

	// Observability: lightweight "last ping" markers that let the admin
	// UI show a green/red dot without a separate delivery log.
	LastDeliveryStatus int        `json:"last_delivery_status,omitempty"` // 2xx = healthy, others = probable problem
	LastDeliveryError  string     `json:"last_delivery_error,omitempty"`
	LastDeliveryAt     *time.Time `json:"last_delivery_at,omitempty"`
	FailureCount       int        `json:"failure_count"`

	CreatedByUserID *uuid.UUID `gorm:"type:uuid" json:"created_by_user_id,omitempty"`
}

// Subscribes reports whether the webhook should receive events of the
// given type. Empty Events = subscribe-to-all is the common case.
func (w *Webhook) Subscribes(t WebhookEventType) bool {
	if len(w.Events) == 0 {
		return true
	}
	for _, e := range w.Events {
		if WebhookEventType(e) == t {
			return true
		}
	}
	return false
}

// ConfigMap unmarshals the opaque JSON config into a plain map for
// adapter consumption. Returns an empty map if the config is empty or
// malformed — adapters validate required fields themselves.
func (w *Webhook) ConfigMap() map[string]any {
	if len(w.Config) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(w.Config, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// SetConfig encodes a typed map into the JSON column. Used by the
// admin handler when accepting upsert payloads.
func (w *Webhook) SetConfig(m map[string]any) error {
	if len(m) == 0 {
		w.Config = nil
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	w.Config = datatypes.JSON(b)
	return nil
}

// ConfigSummary returns a redacted view of the config keys for safe
// display in API responses. Sensitive values are masked to a constant
// "***" so the UI knows a credential is set without revealing it.
// Non-sensitive keys (e.g. "site" for Datadog) are passed through.
func (w *Webhook) ConfigSummary() map[string]any {
	cfg := w.ConfigMap()
	out := make(map[string]any, len(cfg))
	for k, v := range cfg {
		if isSensitiveConfigKey(k) {
			if s, ok := v.(string); ok && s != "" {
				out[k] = "***"
				continue
			}
			out[k] = ""
			continue
		}
		out[k] = v
	}
	return out
}

// isSensitiveConfigKey decides which config keys are masked in API
// responses. Keep this conservative — false negatives leak; false
// positives just hide a non-sensitive value, which is fine.
func isSensitiveConfigKey(k string) bool {
	switch k {
	case "hec_token", "api_key", "app_key", "password", "token", "secret", "auth":
		return true
	}
	return false
}

// webhookAlias avoids infinite recursion in MarshalJSON. The aliased
// type loses its custom marshaler so the embedded encoder runs the
// default field-by-field path.
type webhookAlias Webhook

// MarshalJSON exposes a redacted ConfigSummary alongside the model so
// the admin UI can show "you have a Datadog API key configured"
// without ever surfacing the raw value. The Config field itself stays
// hidden via its `json:"-"` tag.
func (w Webhook) MarshalJSON() ([]byte, error) {
	type wrapper struct {
		webhookAlias
		ConfigSummary map[string]any `json:"config_summary,omitempty"`
	}
	return json.Marshal(wrapper{
		webhookAlias:  webhookAlias(w),
		ConfigSummary: w.ConfigSummary(),
	})
}
