package adapters

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/core/models"
)

// sampleEvent returns a representative scope.denied event with all the
// fields adapter summary helpers look for. Used by every adapter test
// so the rendered output is comparable across vendors.
func sampleEvent() Event {
	return Event{
		ID:        "delivery-abc-123",
		Type:      models.WebhookEventScopeDenied,
		CreatedAt: time.Date(2026, 5, 4, 12, 30, 0, 0, time.UTC),
		Data: map[string]any{
			"target_name":   "anthropic/claude-3-opus",
			"kind":          "llm",
			"use_case_name": "fraud-investigation",
			"key_name":      "retail-fraud-key",
			"reason":        "target not declared in use case scope",
		},
	}
}

// sampleHook builds a minimal webhook with the given destination and
// optional config map.
func sampleHook(t *testing.T, dest models.WebhookDestinationType, cfg map[string]any, url string) *models.Webhook {
	t.Helper()
	h := &models.Webhook{
		Name:            "test-hook",
		URL:             url,
		DestinationType: dest,
		Secret:          "test-secret",
		Enabled:         true,
	}
	if cfg != nil {
		if err := h.SetConfig(cfg); err != nil {
			t.Fatalf("SetConfig: %v", err)
		}
	}
	return h
}

// readBody reads the request body as bytes (consuming it) so callers
// can decode JSON without rebuilding the reader.
func mustJSON(t *testing.T, b []byte, into any) {
	t.Helper()
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, string(b))
	}
}

func TestRegistry_For_FallsBackToGeneric(t *testing.T) {
	if a := For(""); a.Type() != models.WebhookDestGeneric {
		t.Errorf("empty type should fall back to generic, got %q", a.Type())
	}
	if a := For("nonexistent"); a.Type() != models.WebhookDestGeneric {
		t.Errorf("unknown type should fall back to generic, got %q", a.Type())
	}
}

func TestRegistry_All_ContainsExpectedAdapters(t *testing.T) {
	all := All()
	want := []models.WebhookDestinationType{
		models.WebhookDestGeneric,
		models.WebhookDestSlack,
		models.WebhookDestTeams,
		models.WebhookDestSplunk,
		models.WebhookDestDatadog,
		models.WebhookDestServiceNow,
	}
	if len(all) != len(want) {
		t.Fatalf("expected %d adapters, got %d", len(want), len(all))
	}
	for i, w := range want {
		if all[i].Type() != w {
			t.Errorf("position %d: expected %q, got %q", i, w, all[i].Type())
		}
	}
}

func TestSeverityOf(t *testing.T) {
	cases := []struct {
		t    models.WebhookEventType
		want Severity
	}{
		{models.WebhookEventScopeDenied, SeverityError},
		{models.WebhookEventScopeFlagged, SeverityWarning},
		{models.WebhookEventUseCaseComplianceFlag, SeverityWarning},
		{models.WebhookEventUseCaseCreated, SeverityInfo},
		{models.WebhookEventRegistryServerBlockedRisky, SeverityError},
	}
	for _, c := range cases {
		if got := SeverityOf(c.t); got != c.want {
			t.Errorf("SeverityOf(%q) = %q, want %q", c.t, got, c.want)
		}
	}
}

func TestSummaryFor_ScopeDenied_PullsKnownFields(t *testing.T) {
	s := SummaryFor(sampleEvent())
	if s.Title == "" {
		t.Fatal("expected non-empty title")
	}
	want := map[string]string{
		"Target":   "anthropic/claude-3-opus",
		"Use case": "fraud-investigation",
		"Key":      "retail-fraud-key",
		"Reason":   "target not declared in use case scope",
	}
	for k, v := range want {
		found := false
		for _, f := range s.Fields {
			if f.Label == k && f.Value == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing field %q=%q in summary; got %+v", k, v, s.Fields)
		}
	}
	if s.Severity != SeverityError {
		t.Errorf("scope.denied should be error severity, got %q", s.Severity)
	}
}
