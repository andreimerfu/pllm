// Package adapters renders pllm webhook events into vendor-specific
// HTTP requests. The dispatcher delegates request construction here so
// it doesn't need to know how to format a Slack Block Kit message,
// where to put a Datadog API key, or what Splunk's HEC envelope looks
// like.
//
// Each destination is a separate file with the same shape:
//
//   - BuildRequest(evt, hook) -> *http.Request
//   - Success(status, body)   -> bool
//   - DefaultEvents()         -> reasonable starter subscription
//   - ConfigKeys()            -> typed schema the UI renders as a form
//
// Adding a new destination is one new file plus one entry in
// registry.go — no dispatcher changes needed.
package adapters

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/amerfu/pllm/internal/core/models"
)

// Event is the dispatcher-level envelope passed to adapters. The
// canonical shape lives here so adapters don't have to depend on the
// parent webhooks package (which would create a cycle). The parent
// dispatcher type-aliases this for backwards compat.
type Event struct {
	ID        string                  `json:"id"`
	Type      models.WebhookEventType `json:"type"`
	CreatedAt time.Time               `json:"created_at"`
	// Team is an optional scoping hint used by the dispatcher's
	// fan-out — webhooks bound to a specific team only fire when the
	// emitting event sets a matching value, or when the field is nil
	// (broadcast). Hidden from the wire payload via the json tag.
	Team *uuid.UUID `json:"-"`
	// Data is opaque to the dispatcher. Adapters that produce
	// human-readable summaries (Slack, Teams, ServiceNow) read the
	// well-known keys defined in event_payload.go.
	Data any `json:"data"`
}

// TeamID returns the optional scoping team. Method form lets callers
// keep using `evt.TeamID()` without committing to a specific field name
// — the field has been renamed to Team for less collision with the
// well-known data key, and this preserves dispatcher call sites.
func (e Event) TeamID() *uuid.UUID { return e.Team }

// Adapter renders an Event into a vendor-specific HTTP request and
// validates the response.
type Adapter interface {
	// Type returns the destination type this adapter handles. Used by
	// the registry; the model column carries this value.
	Type() models.WebhookDestinationType

	// Label is the human-readable name shown in the UI picker.
	Label() string

	// BuildRequest constructs the HTTP request the dispatcher should
	// send. The dispatcher attaches its own context + per-attempt
	// timeout, so adapters don't need to wire ctx themselves.
	BuildRequest(evt Event, hook *models.Webhook) (*http.Request, error)

	// Success reports whether the response indicates a successful
	// delivery. Vendors disagree: Slack returns 200 + body "ok",
	// Datadog returns 202, ServiceNow returns 201, Splunk returns 200
	// with a JSON success envelope. The dispatcher uses this rather
	// than a fixed 2xx check.
	Success(status int, body []byte) bool

	// DefaultEvents are the smart-default subscription applied when
	// the user picks this destination in the UI without explicitly
	// choosing events. Avoids spamming chat surfaces with high-volume
	// flag-mode traffic and avoids underwhelming SIEM ingest.
	DefaultEvents() []models.WebhookEventType

	// ConfigKeys describes the vendor-specific config fields the UI
	// must render — e.g. {"hec_token", password, required} for Splunk.
	// The Generic adapter returns nothing.
	ConfigKeys() []ConfigKey

	// ValidateConfig checks the supplied config map against the
	// adapter's schema. Returns a user-facing error string when
	// required fields are missing or malformed.
	ValidateConfig(cfg map[string]any) error
}

// ConfigKey is a typed schema entry the UI uses to render a per-
// destination config form. Kept simple — int/select/checkbox can be
// layered on later if a real adapter needs them.
type ConfigKey struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`     // "text" | "password" | "select"
	Required bool     `json:"required"`
	Help     string   `json:"help,omitempty"`
	Default  string   `json:"default,omitempty"`
	// Options is populated when Type == "select".
	Options  []string `json:"options,omitempty"`
}

// requireString is a small helper adapters use in ValidateConfig to
// keep their error paths uniform. Returns the trimmed value or an error.
func requireString(cfg map[string]any, key, label string) (string, error) {
	v, ok := cfg[key]
	if !ok {
		return "", fmt.Errorf("%s is required", label)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", label)
	}
	if s == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return s, nil
}
