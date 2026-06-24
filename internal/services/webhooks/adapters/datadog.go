package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/amerfu/pllm/internal/core/models"
)

// Datadog posts to the Events API v1, which produces an item in the
// Datadog event stream. The receiving URL is site-specific
// (api.datadoghq.com / .eu / .ap1.datadoghq.com / etc.) — the UI
// helps construct it but we forward whatever URL the operator saved.
//
// Auth is a `DD-API-KEY` header carrying the org's API key. An
// optional application key is supported for installs that gate the
// Events API behind one.
type Datadog struct{}

func init() { register(&Datadog{}) }

func (d *Datadog) Type() models.WebhookDestinationType { return models.WebhookDestDatadog }

func (d *Datadog) Label() string { return "Datadog" }

// datadogEvent matches the v1 Events API payload. Tags are flat
// "key:value" strings — matches Datadog's own tag convention.
type datadogEvent struct {
	Title          string   `json:"title"`
	Text           string   `json:"text"`
	AlertType      string   `json:"alert_type,omitempty"` // "info" | "warning" | "error" | "success"
	Priority       string   `json:"priority,omitempty"`   // "normal" | "low"
	SourceTypeName string   `json:"source_type_name"`
	Tags           []string `json:"tags,omitempty"`
	DateHappened   int64    `json:"date_happened,omitempty"`
}

func (d *Datadog) BuildRequest(evt Event, hook *models.Webhook) (*http.Request, error) {
	cfg := hook.ConfigMap()
	apiKey, _ := cfg["api_key"].(string)
	appKey, _ := cfg["app_key"].(string)

	summary := SummaryFor(evt)
	alertType := "info"
	priority := "normal"
	switch summary.Severity {
	case SeverityError:
		alertType = "error"
	case SeverityWarning:
		alertType = "warning"
	}

	tags := []string{
		"event_type:" + string(evt.Type),
		"source:pllm",
	}
	for _, f := range summary.Fields {
		// Tag values can't contain spaces; collapse + lower-case so
		// they remain searchable in dashboards. Skip the value if it's
		// long-form prose (reasons) — those belong in the body.
		if f.Label == "Reason" {
			continue
		}
		tags = append(tags, datadogTag(f.Label, f.Value))
	}

	body := summary.Body
	// Datadog renders %%% as markdown delimiters in the event stream;
	// wrap the body so it's typeset properly.
	if body != "" {
		body = "%%% \n" + body + "\n %%%"
	}

	payload := datadogEvent{
		Title:          summary.Title,
		Text:           body,
		AlertType:      alertType,
		Priority:       priority,
		SourceTypeName: "pllm",
		Tags:           tags,
		DateHappened:   evt.CreatedAt.Unix(),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", apiKey)
	if appKey != "" {
		req.Header.Set("DD-APPLICATION-KEY", appKey)
	}
	req.Header.Set("User-Agent", "pllm-webhook/1 (datadog)")
	return req, nil
}

// Success accepts any 2xx. Datadog returns 202 for queued events.
func (d *Datadog) Success(status int, _ []byte) bool {
	return status >= 200 && status < 300
}

// DefaultEvents — Datadog is observability/monitoring; subscribers
// typically want the full firehose so monitors can be authored against
// any event type.
func (d *Datadog) DefaultEvents() []models.WebhookEventType { return nil }

func (d *Datadog) ConfigKeys() []ConfigKey {
	return []ConfigKey{
		{
			Key:      "api_key",
			Label:    "API key",
			Type:     "password",
			Required: true,
			Help:     "Org-level API key from Organization Settings → API Keys.",
		},
		{
			Key:      "app_key",
			Label:    "Application key (optional)",
			Type:     "password",
			Required: false,
			Help:     "Some installs require both. Leave blank if not enforced.",
		},
		{
			Key:      "site",
			Label:    "Site",
			Type:     "select",
			Required: false,
			Default:  "us1",
			Options:  []string{"us1", "us3", "us5", "eu", "ap1"},
			Help:     "Used by the UI to suggest the correct API URL. The saved URL is what the dispatcher actually POSTs to.",
		},
	}
}

func (d *Datadog) ValidateConfig(cfg map[string]any) error {
	_, err := requireString(cfg, "api_key", "Datadog API key")
	return err
}

// datadogTag normalizes a Summary field into a Datadog tag. Datadog
// tags must be lowercase, no spaces, and use ':' to separate key/value.
func datadogTag(key, value string) string {
	k := normalizeTag(key)
	v := normalizeTag(value)
	if v == "" {
		return k
	}
	return fmt.Sprintf("%s:%s", k, v)
}

func normalizeTag(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+('a'-'A'))
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			out = append(out, c)
		case c == '-' || c == '_' || c == '.' || c == '/':
			out = append(out, c)
		default:
			// Spaces and any other char collapse to underscore.
			out = append(out, '_')
		}
	}
	return string(out)
}
