package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
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
)

// Webhook is an external HTTP endpoint that receives pllm events.
// Delivery is best-effort async with bounded retries; we don't persist
// full delivery bodies, but LastDeliveryStatus + FailureCount give
// operators a quick "is this endpoint alive" signal.
type Webhook struct {
	BaseModel

	Name        string `gorm:"not null" json:"name"`
	URL         string `gorm:"not null" json:"url"`
	// Secret is used as the HMAC-SHA256 key for the X-PLLM-Signature
	// header. Stored plaintext for now — operators rotate via delete+create.
	Secret string `gorm:"not null" json:"secret"`

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
