package adapters

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/core/models"
)

// Teams renders pllm events as an Adaptive Card v1.5 wrapped in the
// "message with attachments" envelope expected by Microsoft Teams
// incoming webhooks (and the newer Workflow connectors). Auth is
// embedded in the URL — no per-webhook config needed. Teams returns
// 200 on success.
//
// We use Adaptive Cards rather than the legacy MessageCard format
// because Microsoft has deprecated MessageCard for new connectors and
// Workflow webhooks only accept the Adaptive Card shape.
type Teams struct{}

func init() { register(&Teams{}) }

func (t *Teams) Type() models.WebhookDestinationType { return models.WebhookDestTeams }

func (t *Teams) Label() string { return "Microsoft Teams" }

// teamsMessage is the outer wrapper Teams expects — the actual card
// lives in attachments[0].content.
type teamsMessage struct {
	Type        string             `json:"type"` // always "message"
	Attachments []teamsAttachment  `json:"attachments"`
}

type teamsAttachment struct {
	ContentType string         `json:"contentType"` // always "application/vnd.microsoft.card.adaptive"
	ContentURL  *string        `json:"contentUrl"`  // always null
	Content     adaptiveCard   `json:"content"`
}

type adaptiveCard struct {
	Schema  string             `json:"$schema"`
	Type    string             `json:"type"`    // "AdaptiveCard"
	Version string             `json:"version"` // "1.5"
	Body    []adaptiveElement  `json:"body"`
}

// adaptiveElement covers the small subset we need: TextBlock and
// FactSet. Optional fields are pointers so they're omitted when unset.
type adaptiveElement struct {
	Type   string         `json:"type"`
	Text   string         `json:"text,omitempty"`
	Size   string         `json:"size,omitempty"`
	Weight string         `json:"weight,omitempty"`
	Color  string         `json:"color,omitempty"` // "good" | "warning" | "attention"
	Wrap   bool           `json:"wrap,omitempty"`
	Facts  []adaptiveFact `json:"facts,omitempty"`
	Spacing string        `json:"spacing,omitempty"`
	IsSubtle bool         `json:"isSubtle,omitempty"`
}

type adaptiveFact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

func (t *Teams) BuildRequest(evt Event, hook *models.Webhook) (*http.Request, error) {
	summary := SummaryFor(evt)

	headerColor := "default"
	switch summary.Severity {
	case SeverityError:
		headerColor = "attention"
	case SeverityWarning:
		headerColor = "warning"
	case SeverityInfo:
		headerColor = "accent"
	}

	body := []adaptiveElement{
		{
			Type:   "TextBlock",
			Text:   summary.Title,
			Size:   "Large",
			Weight: "Bolder",
			Color:  headerColor,
			Wrap:   true,
		},
	}
	if summary.Body != "" {
		body = append(body, adaptiveElement{
			Type: "TextBlock",
			Text: summary.Body,
			Wrap: true,
		})
	}
	if len(summary.Fields) > 0 {
		facts := make([]adaptiveFact, 0, len(summary.Fields))
		for _, f := range summary.Fields {
			facts = append(facts, adaptiveFact{Title: f.Label + ":", Value: f.Value})
		}
		body = append(body, adaptiveElement{
			Type:    "FactSet",
			Facts:   facts,
			Spacing: "Medium",
		})
	}
	// Footer: small, subtle event-type / delivery-id row so the message
	// is traceable back to a pllm delivery without dominating the card.
	body = append(body, adaptiveElement{
		Type:     "TextBlock",
		Text:     string(evt.Type) + " · delivery " + evt.ID,
		Size:     "Small",
		IsSubtle: true,
		Spacing:  "Medium",
	})

	contentURL := (*string)(nil)
	msg := teamsMessage{
		Type: "message",
		Attachments: []teamsAttachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				ContentURL:  contentURL,
				Content: adaptiveCard{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: "1.5",
					Body:    body,
				},
			},
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "pllm-webhook/1 (teams)")
	return req, nil
}

// Success accepts any 2xx. Teams legacy connectors return 200, Workflow
// webhooks return 202; both are fine.
func (t *Teams) Success(status int, _ []byte) bool {
	return status >= 200 && status < 300
}

// DefaultEvents — Teams compliance channels typically want the alerts,
// not the full firehose. Approved-flag use_case events are noisy in a
// chat surface, so they're off by default.
func (t *Teams) DefaultEvents() []models.WebhookEventType {
	return []models.WebhookEventType{
		models.WebhookEventScopeDenied,
		models.WebhookEventUseCaseComplianceFlag,
		models.WebhookEventRegistryServerBlockedRisky,
	}
}

func (t *Teams) ConfigKeys() []ConfigKey { return nil }

func (t *Teams) ValidateConfig(_ map[string]any) error { return nil }
