package adapters

import (
	"io"
	"strings"
	"testing"

	"github.com/amerfu/pllm/internal/core/models"
)

func TestSplunk_BuildRequest_HECEnvelopeAndAuth(t *testing.T) {
	hook := sampleHook(t, models.WebhookDestSplunk,
		map[string]any{"hec_token": "sekret-hec-token"},
		"https://splunk.example.com:8088/services/collector/event",
	)
	s := &Splunk{}
	req, err := s.BuildRequest(sampleEvent(), hook)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Splunk sekret-hec-token" {
		t.Errorf("Authorization = %q", got)
	}

	body, _ := io.ReadAll(req.Body)
	var env hecEvent
	mustJSON(t, body, &env)

	if !strings.HasPrefix(env.Sourcetype, "pllm:") {
		t.Errorf("sourcetype should start with pllm:, got %q", env.Sourcetype)
	}
	if env.Source != "pllm" {
		t.Errorf("source = %q", env.Source)
	}
	if env.Time == 0 {
		t.Error("time should be set from event CreatedAt")
	}
	if env.Event.ID != "delivery-abc-123" {
		t.Errorf("event nested incorrectly: %+v", env.Event)
	}
}

func TestSplunk_Success(t *testing.T) {
	s := &Splunk{}
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"success body", 200, `{"text":"Success","code":0}`, true},
		{"empty body 200", 200, "", true}, // indexer-ack mode
		{"unparseable 200 trusted", 200, "<html>ok</html>", true},
		{"invalid token", 200, `{"text":"Invalid token","code":4}`, false},
		{"non-2xx", 401, `{"text":"Token disabled","code":1}`, false},
	}
	for _, c := range cases {
		if got := s.Success(c.status, []byte(c.body)); got != c.want {
			t.Errorf("%s: Success(%d, %q) = %v, want %v", c.name, c.status, c.body, got, c.want)
		}
	}
}

func TestSplunk_ValidateConfig_RequiresToken(t *testing.T) {
	s := &Splunk{}
	if err := s.ValidateConfig(map[string]any{}); err == nil {
		t.Error("expected error for missing hec_token")
	}
	if err := s.ValidateConfig(map[string]any{"hec_token": ""}); err == nil {
		t.Error("expected error for empty hec_token")
	}
	if err := s.ValidateConfig(map[string]any{"hec_token": "x"}); err != nil {
		t.Errorf("expected ok with token, got %v", err)
	}
}
