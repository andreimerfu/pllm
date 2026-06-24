package adapters

import (
	"io"
	"strings"
	"testing"

	"github.com/amerfu/pllm/internal/core/models"
)

func TestDatadog_BuildRequest_AddsAPIKeyAndPayload(t *testing.T) {
	hook := sampleHook(t, models.WebhookDestDatadog,
		map[string]any{"api_key": "dd-secret", "site": "us1"},
		"https://api.datadoghq.com/api/v1/events",
	)
	d := &Datadog{}
	req, err := d.BuildRequest(sampleEvent(), hook)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	if got := req.Header.Get("DD-API-KEY"); got != "dd-secret" {
		t.Errorf("DD-API-KEY = %q", got)
	}
	if got := req.Header.Get("DD-APPLICATION-KEY"); got != "" {
		t.Errorf("DD-APPLICATION-KEY should be unset when app_key absent, got %q", got)
	}

	body, _ := io.ReadAll(req.Body)
	var ev datadogEvent
	mustJSON(t, body, &ev)

	if ev.AlertType != "error" {
		t.Errorf("scope.denied → alert_type = %q, want error", ev.AlertType)
	}
	if ev.SourceTypeName != "pllm" {
		t.Errorf("source_type_name = %q", ev.SourceTypeName)
	}
	// Tags should include event_type and source markers, plus
	// normalized field tags. Spaces lowercased and underscored.
	tags := strings.Join(ev.Tags, " ")
	if !strings.Contains(tags, "event_type:scope.denied") {
		t.Errorf("expected event_type tag, got: %s", tags)
	}
	if !strings.Contains(tags, "source:pllm") {
		t.Errorf("expected source tag, got: %s", tags)
	}
	if !strings.Contains(tags, "use_case:fraud-investigation") {
		t.Errorf("expected normalized use_case tag, got: %s", tags)
	}
	if strings.Contains(tags, "Reason:") {
		t.Errorf("Reason should not appear as a tag: %s", tags)
	}
}

func TestDatadog_BuildRequest_AppKeyHeader(t *testing.T) {
	hook := sampleHook(t, models.WebhookDestDatadog,
		map[string]any{"api_key": "k", "app_key": "a"},
		"https://api.datadoghq.com/api/v1/events",
	)
	req, err := (&Datadog{}).BuildRequest(sampleEvent(), hook)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	if got := req.Header.Get("DD-APPLICATION-KEY"); got != "a" {
		t.Errorf("DD-APPLICATION-KEY = %q", got)
	}
}

func TestDatadog_Success(t *testing.T) {
	d := &Datadog{}
	cases := []struct {
		status int
		want   bool
	}{
		{200, true},
		{202, true},
		{301, false},
		{401, false},
	}
	for _, c := range cases {
		if got := d.Success(c.status, nil); got != c.want {
			t.Errorf("Success(%d) = %v", c.status, got)
		}
	}
}

func TestDatadog_NormalizeTag(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Use case", "use_case"},
		{"fraud-investigation", "fraud-investigation"},
		{"Some Value!", "some_value_"},
		{"UPPER", "upper"},
	}
	for _, c := range cases {
		if got := normalizeTag(c.in); got != c.want {
			t.Errorf("normalizeTag(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDatadog_ValidateConfig(t *testing.T) {
	d := &Datadog{}
	if err := d.ValidateConfig(map[string]any{}); err == nil {
		t.Error("expected error for missing api_key")
	}
	if err := d.ValidateConfig(map[string]any{"api_key": "k"}); err != nil {
		t.Errorf("expected ok, got %v", err)
	}
}
