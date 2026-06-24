package adapters

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/core/models"
)

// ServiceNow posts to the Table API to create a record (typically an
// `incident` or a custom GRC table). The receiving URL is the full
// table-API endpoint, e.g. `https://<instance>.service-now.com/api/
// now/table/incident`.
//
// Auth is HTTP Basic (username + password) — the most common
// integration mode and what most enterprise GRC integrations are
// configured for. OAuth bearer tokens are a follow-up.
type ServiceNow struct{}

func init() { register(&ServiceNow{}) }

func (s *ServiceNow) Type() models.WebhookDestinationType { return models.WebhookDestServiceNow }

func (s *ServiceNow) Label() string { return "ServiceNow" }

// snPayload is the Table API row body. Custom `u_pllm_*` fields are
// the convention many GRC implementations follow when they extend a
// table; they remain harmless if the target table doesn't define them
// (ServiceNow ignores unknown columns by default).
type snPayload struct {
	ShortDescription string `json:"short_description"`
	Description      string `json:"description"`
	Category         string `json:"category,omitempty"`
	Priority         string `json:"priority,omitempty"`
	Impact           string `json:"impact,omitempty"`
	Urgency          string `json:"urgency,omitempty"`
	UPllmEventType   string `json:"u_pllm_event_type,omitempty"`
	UPllmTarget      string `json:"u_pllm_target,omitempty"`
	UPllmUseCase     string `json:"u_pllm_use_case,omitempty"`
	UPllmDeliveryID  string `json:"u_pllm_delivery_id,omitempty"`
	UPllmReason      string `json:"u_pllm_reason,omitempty"`
}

func (s *ServiceNow) BuildRequest(evt Event, hook *models.Webhook) (*http.Request, error) {
	cfg := hook.ConfigMap()
	username, _ := cfg["username"].(string)
	password, _ := cfg["password"].(string)

	summary := SummaryFor(evt)
	priority, impact, urgency := snTriad(summary.Severity)

	data := dataMap(evt.Data)
	body := snPayload{
		ShortDescription: trimTo(summary.Title, 160),
		Description:      summary.Body,
		Category:         "compliance",
		Priority:         priority,
		Impact:           impact,
		Urgency:          urgency,
		UPllmEventType:   string(evt.Type),
		UPllmTarget:      stringValue(data, "target_name", "target", "name"),
		UPllmUseCase:     stringValue(data, "use_case_name", "use_case_id"),
		UPllmDeliveryID:  evt.ID,
		UPllmReason:      stringValue(data, "reason"),
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if username != "" || password != "" {
		credential := username + ":" + password
		req.Header.Set("Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte(credential)))
	}
	req.Header.Set("User-Agent", "pllm-webhook/1 (servicenow)")
	return req, nil
}

// Success — ServiceNow Table API returns 201 Created on insert. We also
// accept other 2xx in case the operator points at a Scripted REST
// endpoint that uses a different code.
func (s *ServiceNow) Success(status int, _ []byte) bool {
	return status >= 200 && status < 300
}

// DefaultEvents — GRC integrations usually want the load-bearing
// compliance signals (denials, compliance flags, risky-server blocks),
// not the noisy lifecycle bookkeeping.
func (s *ServiceNow) DefaultEvents() []models.WebhookEventType {
	return []models.WebhookEventType{
		models.WebhookEventScopeDenied,
		models.WebhookEventUseCaseComplianceFlag,
		models.WebhookEventRegistryServerBlockedRisky,
	}
}

func (s *ServiceNow) ConfigKeys() []ConfigKey {
	return []ConfigKey{
		{
			Key:      "username",
			Label:    "Username",
			Type:     "text",
			Required: true,
			Help:     "Service account with write access to the target table (typically `incident` or a custom GRC table).",
		},
		{
			Key:      "password",
			Label:    "Password",
			Type:     "password",
			Required: true,
		},
	}
}

func (s *ServiceNow) ValidateConfig(cfg map[string]any) error {
	if _, err := requireString(cfg, "username", "ServiceNow username"); err != nil {
		return err
	}
	if _, err := requireString(cfg, "password", "ServiceNow password"); err != nil {
		return err
	}
	return nil
}

// snTriad maps a Severity to ServiceNow's standard 1-3 priority/impact/
// urgency scale. Errors map to (2, 2, 2) — "high" — rather than (1, 1, 1)
// to avoid pulling all denials into Sev1 inboxes.
func snTriad(sev Severity) (priority, impact, urgency string) {
	switch sev {
	case SeverityError:
		return "2", "2", "2"
	case SeverityWarning:
		return "3", "3", "3"
	default:
		return "4", "4", "4"
	}
}
