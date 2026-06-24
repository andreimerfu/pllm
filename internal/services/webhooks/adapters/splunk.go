package adapters

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/amerfu/pllm/internal/core/models"
)

// Splunk adapts pllm events to Splunk's HTTP Event Collector (HEC)
// format. The receiving URL is typically `https://<host>:8088/services/
// collector/event`. Auth is a token sent in the Authorization header.
//
// HEC accepts a JSON envelope with a top-level "event" key that can
// be either a string or an object. We pass the full pllm Event as the
// event payload so receivers can search on type/data.* without having
// to parse a stringified body.
type Splunk struct{}

func init() { register(&Splunk{}) }

func (s *Splunk) Type() models.WebhookDestinationType { return models.WebhookDestSplunk }

func (s *Splunk) Label() string { return "Splunk" }

// hecEvent is the wire shape Splunk HEC expects. Time is unix-seconds.
// sourcetype lets ops route pllm events to a dedicated index without
// touching the search-time field extractions.
type hecEvent struct {
	Event      Event  `json:"event"`
	Sourcetype string `json:"sourcetype,omitempty"`
	Source     string `json:"source,omitempty"`
	Time       int64  `json:"time,omitempty"`
}

// hecResponse covers the success / failure JSON Splunk returns.
//
//	{"text":"Success","code":0}
//	{"text":"Token disabled","code":1}
//
// We treat code==0 as success and surface anything else as an error
// even when the HTTP status is 200.
type hecResponse struct {
	Text string `json:"text"`
	Code int    `json:"code"`
}

func (s *Splunk) BuildRequest(evt Event, hook *models.Webhook) (*http.Request, error) {
	cfg := hook.ConfigMap()
	token, _ := cfg["hec_token"].(string)

	payload := hecEvent{
		Event:      evt,
		Sourcetype: "pllm:" + string(evt.Type),
		Source:     "pllm",
		Time:       evt.CreatedAt.Unix(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Splunk "+token)
	req.Header.Set("User-Agent", "pllm-webhook/1 (splunk)")
	return req, nil
}

// Success requires 2xx AND a HEC body with code 0. A 200 with code 4
// ("Invalid authorization") is a token problem, not a success.
func (s *Splunk) Success(status int, body []byte) bool {
	if status < 200 || status >= 300 {
		return false
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		// Some indexer-acknowledged setups return empty bodies on 200.
		// Trust the status code in that case.
		return true
	}
	var r hecResponse
	if err := json.Unmarshal(body, &r); err != nil {
		// Unparseable body but 2xx — still treat as success rather than
		// rejecting based on shape we don't recognize.
		return true
	}
	return r.Code == 0 || strings.EqualFold(r.Text, "Success")
}

// DefaultEvents — SIEM ingest typically wants everything; ops filter
// at search time, not subscription time.
func (s *Splunk) DefaultEvents() []models.WebhookEventType { return nil }

func (s *Splunk) ConfigKeys() []ConfigKey {
	return []ConfigKey{
		{
			Key:      "hec_token",
			Label:    "HEC token",
			Type:     "password",
			Required: true,
			Help:     "From Splunk Web → Settings → Data Inputs → HTTP Event Collector → New Token",
		},
	}
}

func (s *Splunk) ValidateConfig(cfg map[string]any) error {
	_, err := requireString(cfg, "hec_token", "HEC token")
	return err
}
