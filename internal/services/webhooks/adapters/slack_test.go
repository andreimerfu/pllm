package adapters

import (
	"io"
	"strings"
	"testing"

	"github.com/amerfu/pllm/internal/core/models"
)

func TestSlack_BuildRequest_BlockKitShape(t *testing.T) {
	hook := sampleHook(t, models.WebhookDestSlack, nil, "https://hooks.slack.com/services/T/B/X")
	s := &Slack{}
	req, err := s.BuildRequest(sampleEvent(), hook)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	body, _ := io.ReadAll(req.Body)
	var msg blockKitMessage
	mustJSON(t, body, &msg)

	if len(msg.Blocks) == 0 {
		t.Fatal("expected at least one block")
	}
	// First block should be the header.
	if msg.Blocks[0].Type != "header" {
		t.Errorf("first block type = %q, want header", msg.Blocks[0].Type)
	}
	if !strings.Contains(msg.Blocks[0].Text.Text, ":no_entry:") {
		t.Errorf("error severity should use :no_entry: emoji, got: %q", msg.Blocks[0].Text.Text)
	}

	// Notification preview text — Slack uses this for push/screen-readers.
	if msg.Text == "" {
		t.Error("preview text should be set so notifications aren't empty")
	}

	// Body should reference the target name from the event.
	combined := strings.Builder{}
	for _, b := range msg.Blocks {
		if b.Text != nil {
			combined.WriteString(b.Text.Text)
		}
		for _, f := range b.Fields {
			combined.WriteString(f.Text)
		}
	}
	all := combined.String()
	if !strings.Contains(all, "claude-3-opus") {
		t.Errorf("body should mention target: %s", all)
	}
	if !strings.Contains(all, "fraud-investigation") {
		t.Errorf("body should mention use case: %s", all)
	}
}

func TestSlack_Success(t *testing.T) {
	s := &Slack{}
	cases := []struct {
		status int
		body   string
		want   bool
	}{
		{200, "ok", true},
		{200, "OK\n", true},
		{200, "no_service", false}, // 200 with error body — Slack quirk
		{200, "invalid_payload", false},
		{500, "ok", false},
	}
	for _, c := range cases {
		if got := s.Success(c.status, []byte(c.body)); got != c.want {
			t.Errorf("Success(%d, %q) = %v, want %v", c.status, c.body, got, c.want)
		}
	}
}

func TestSlack_DefaultEvents_IncludesCoreCompliance(t *testing.T) {
	want := map[models.WebhookEventType]bool{
		models.WebhookEventScopeDenied:                 true,
		models.WebhookEventUseCaseComplianceFlag:       true,
		models.WebhookEventRegistryServerBlockedRisky:  true,
	}
	for _, e := range (&Slack{}).DefaultEvents() {
		delete(want, e)
	}
	if len(want) > 0 {
		t.Errorf("missing expected default events: %v", want)
	}
}
