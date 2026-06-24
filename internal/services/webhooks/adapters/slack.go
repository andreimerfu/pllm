package adapters

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/amerfu/pllm/internal/core/models"
)

// Slack renders pllm events as Block Kit messages and POSTs them to a
// Slack incoming webhook URL. The URL itself carries the auth, so no
// per-webhook config is needed. Slack returns 200 + body "ok" on
// success; non-ok bodies (e.g. "no_service") indicate the webhook URL
// is invalid even when the status is 200, so we check the body too.
type Slack struct{}

func init() { register(&Slack{}) }

func (s *Slack) Type() models.WebhookDestinationType { return models.WebhookDestSlack }

func (s *Slack) Label() string { return "Slack" }

// blockKitMessage is the subset of the Block Kit schema we use. Slack
// accepts other top-level fields (text, attachments, icon_url, ...)
// but the block-based shape is the most readable in modern clients
// and degrades gracefully in old ones via the optional top-level text.
type blockKitMessage struct {
	Text   string  `json:"text,omitempty"` // fallback for notifications + screen readers
	Blocks []block `json:"blocks"`
}

type block struct {
	Type     string         `json:"type"`
	Text     *blockText     `json:"text,omitempty"`
	Fields   []blockText    `json:"fields,omitempty"`
	Elements []blockElement `json:"elements,omitempty"`
}

type blockText struct {
	Type string `json:"type"` // "plain_text" | "mrkdwn"
	Text string `json:"text"`
}

type blockElement struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func (s *Slack) BuildRequest(evt Event, hook *models.Webhook) (*http.Request, error) {
	summary := SummaryFor(evt)

	// Title block — emoji prefix tells humans the severity at a glance.
	headerEmoji := ":information_source:"
	switch summary.Severity {
	case SeverityError:
		headerEmoji = ":no_entry:"
	case SeverityWarning:
		headerEmoji = ":warning:"
	}

	blocks := []block{
		{
			Type: "header",
			Text: &blockText{
				Type: "plain_text",
				// Slack header blocks max ~150 chars; trim defensively.
				Text: trimTo(headerEmoji+" "+summary.Title, 140),
			},
		},
	}

	if summary.Body != "" {
		blocks = append(blocks, block{
			Type: "section",
			Text: &blockText{
				Type: "mrkdwn",
				Text: trimTo(summary.Body, 2900),
			},
		})
	}

	if len(summary.Fields) > 0 {
		// Block Kit limits a single fields-block to 10 entries; chunk
		// rather than truncate so longer events still display fully.
		chunks := chunkFields(summary.Fields, 10)
		for _, c := range chunks {
			fields := make([]blockText, 0, len(c)*2)
			for _, f := range c {
				fields = append(fields,
					blockText{Type: "mrkdwn", Text: "*" + f.Label + "*"},
					blockText{Type: "mrkdwn", Text: "`" + slackEscape(f.Value) + "`"},
				)
			}
			blocks = append(blocks, block{Type: "section", Fields: fields})
		}
	}

	// Footer context — event id + type, mono-styled, lets receivers tie
	// a Slack message back to a delivery row.
	blocks = append(blocks, block{
		Type: "context",
		Elements: []blockElement{
			{Type: "mrkdwn", Text: "`" + string(evt.Type) + "` · delivery `" + evt.ID + "`"},
		},
	})

	msg := blockKitMessage{
		Text:   summary.Title, // notification preview
		Blocks: blocks,
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "pllm-webhook/1 (slack)")
	return req, nil
}

// Success requires both a 2xx and a body of "ok". Slack returns 200
// for "no_service", "invalid_payload" etc. with the error string in
// the body; treating those as success would silently drop alerts.
// Comparison is case-insensitive — Slack canonically returns lowercase
// "ok" but we don't want to misfire on the rare upstream that varies.
func (s *Slack) Success(status int, body []byte) bool {
	if status < 200 || status >= 300 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(string(body)), "ok")
}

// DefaultEvents reflects what humans want in #compliance: blocked
// requests, compliance flags, and (most of the time) flag-mode events
// for visibility. Subscriptions can be widened to all events from the UI.
func (s *Slack) DefaultEvents() []models.WebhookEventType {
	return []models.WebhookEventType{
		models.WebhookEventScopeDenied,
		models.WebhookEventScopeFlagged,
		models.WebhookEventUseCaseComplianceFlag,
		models.WebhookEventRegistryServerBlockedRisky,
	}
}

func (s *Slack) ConfigKeys() []ConfigKey { return nil }

func (s *Slack) ValidateConfig(_ map[string]any) error { return nil }

// --- helpers --------------------------------------------------------------

func trimTo(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

// slackEscape replaces backticks in a value so we don't break out of
// the inline-code wrapper. Slack's mrkdwn doesn't have backslash
// escapes, so we substitute the visually similar U+2018.
func slackEscape(v string) string {
	if !strings.Contains(v, "`") {
		return v
	}
	return strings.ReplaceAll(v, "`", "‘")
}

func chunkFields(fs []SummaryField, n int) [][]SummaryField {
	if n <= 0 || len(fs) <= n {
		return [][]SummaryField{fs}
	}
	out := make([][]SummaryField, 0, (len(fs)+n-1)/n)
	for i := 0; i < len(fs); i += n {
		end := i + n
		if end > len(fs) {
			end = len(fs)
		}
		out = append(out, fs[i:end])
	}
	return out
}
