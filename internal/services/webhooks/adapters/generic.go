package adapters

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/amerfu/pllm/internal/core/models"
)

// Generic is the pre-adapter behavior — pllm's native HMAC-signed JSON
// envelope POSTed to whatever URL the operator supplies. Used for
// receivers that consume the raw event format (your own service, an
// in-house relay, a generic webhook proxy).
type Generic struct{}

func init() { register(&Generic{}) }

func (g *Generic) Type() models.WebhookDestinationType { return models.WebhookDestGeneric }

func (g *Generic) Label() string { return "Generic webhook" }

func (g *Generic) BuildRequest(evt Event, hook *models.Webhook) (*http.Request, error) {
	body, err := json.Marshal(evt)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	sig := hmacSign(hook.Secret, body)
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "pllm-webhook/1")
	req.Header.Set("X-PLLM-Event", string(evt.Type))
	req.Header.Set("X-PLLM-Delivery-ID", evt.ID)
	req.Header.Set("X-PLLM-Signature", "sha256="+sig)
	return req, nil
}

// Success accepts any 2xx — generic receivers don't have a vendor-defined
// success body shape.
func (g *Generic) Success(status int, _ []byte) bool {
	return status >= 200 && status < 300
}

// DefaultEvents = nil meaning "subscribe to all". Generic receivers
// usually want everything and filter on their side.
func (g *Generic) DefaultEvents() []models.WebhookEventType { return nil }

func (g *Generic) ConfigKeys() []ConfigKey { return nil }

func (g *Generic) ValidateConfig(cfg map[string]any) error {
	// Generic carries no vendor config — extra keys are silently ignored
	// rather than rejected so the UI can leave a partially-filled form
	// from a prior destination intact.
	return nil
}

// hmacSign reproduces the dispatcher's signing helper. Kept here so the
// adapter is self-contained — the dispatcher file no longer needs to
// know about HMAC at all.
func hmacSign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
