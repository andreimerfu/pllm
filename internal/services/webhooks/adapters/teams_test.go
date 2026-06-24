package adapters

import (
	"io"
	"strings"
	"testing"

	"github.com/amerfu/pllm/internal/core/models"
)

func TestTeams_BuildRequest_AdaptiveCardShape(t *testing.T) {
	hook := sampleHook(t, models.WebhookDestTeams, nil,
		"https://outlook.office.com/webhook/abc/IncomingWebhook/def/ghi")
	tt := &Teams{}
	req, err := tt.BuildRequest(sampleEvent(), hook)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}

	body, _ := io.ReadAll(req.Body)
	var msg teamsMessage
	mustJSON(t, body, &msg)

	if msg.Type != "message" {
		t.Errorf("envelope type = %q, want message", msg.Type)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}
	att := msg.Attachments[0]
	if att.ContentType != "application/vnd.microsoft.card.adaptive" {
		t.Errorf("attachment content-type = %q", att.ContentType)
	}
	if att.Content.Type != "AdaptiveCard" {
		t.Errorf("card type = %q, want AdaptiveCard", att.Content.Type)
	}
	if att.Content.Version == "" {
		t.Error("card version must be set")
	}
	// First body element should be the title TextBlock with attention color.
	if len(att.Content.Body) < 2 {
		t.Fatalf("expected at least title + body elements, got %d", len(att.Content.Body))
	}
	title := att.Content.Body[0]
	if title.Type != "TextBlock" {
		t.Errorf("title element type = %q", title.Type)
	}
	if title.Color != "attention" {
		t.Errorf("denied event should color title 'attention', got %q", title.Color)
	}

	// FactSet should contain the well-known fields.
	hasFactSet := false
	for _, el := range att.Content.Body {
		if el.Type == "FactSet" {
			hasFactSet = true
			labels := []string{}
			for _, f := range el.Facts {
				labels = append(labels, f.Title)
			}
			joined := strings.Join(labels, ",")
			if !strings.Contains(joined, "Target") || !strings.Contains(joined, "Use case") {
				t.Errorf("FactSet missing core fields: %s", joined)
			}
		}
	}
	if !hasFactSet {
		t.Error("expected a FactSet element in the card body")
	}
}

func TestTeams_Success_AcceptsAll2xx(t *testing.T) {
	tt := &Teams{}
	cases := []struct {
		status int
		want   bool
	}{
		{200, true},
		{202, true}, // Workflow connectors
		{301, false},
		{400, false},
	}
	for _, c := range cases {
		if got := tt.Success(c.status, nil); got != c.want {
			t.Errorf("Success(%d) = %v, want %v", c.status, got, c.want)
		}
	}
}
